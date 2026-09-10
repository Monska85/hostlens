package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	linux "github.com/Monska85/hostlens/internal/platform/linux"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
func main() {
	if len(os.Args) == 4 && os.Args[1] == "ipc" {
		probeIPC(os.Args[2], os.Args[3])
		return
	}
	if len(os.Args) != 3 {
		fail("usage: containment MODE SECRET_PATH")
	}
	if os.Geteuid() == 0 {
		fail("probe must use diagnostic identity")
	}
	status, e := os.ReadFile("/proc/self/status")
	if e != nil {
		fail("cannot inspect capability state")
	}
	caps := uint64(0)
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "CapEff:") {
			caps, e = strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
			if e != nil {
				fail("invalid capability observation")
			}
		}
	}
	standard := os.Args[1] == "standard"
	if standard && caps != 4 {
		fail("standard probe must retain only CAP_DAC_READ_SEARCH")
	}
	if !standard && caps != 0 {
		fail("restricted probe has extra effective capability")
	}
	b, e := os.ReadFile("/opt/hostlens-root-private.conf")
	if standard && (e != nil || string(b) != "positive control\n") {
		fail("broad-read positive control failed")
	}
	if !standard && !os.IsPermission(e) {
		fail("restricted positive control must report permission denial")
	}
	if standard {
		if e := linux.SecretIsMasked(os.Args[2]); e != nil {
			fail("no verified secret masking mount")
		}
	}
	f, e := os.Open(os.Args[2])
	if e == nil {
		defer f.Close()
		var one [1]byte
		n, _ := f.Read(one[:])
		if !standard || n != 0 {
			fail("gateway secret accessible independently of policy")
		}
	} else if !os.IsPermission(e) && !(standard && os.IsNotExist(e)) {
		fail("secret probe failed for an unrelated reason")
	}

	fmt.Println("credential containment with measured capability control: passed")
}

func probeIPC(path, expectation string) {
	if expectation != "allow" && expectation != "deny" {
		fail("IPC expectation must be allow or deny")
	}
	if expectation == "deny" && os.Geteuid() == 0 {
		fail("denied IPC probe must use an unrelated non-root identity")
	}
	conn, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		fail("IPC socket connection failed before peer authorization could be tested")
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		fail("cannot bound IPC probe")
	}
	// A successful connect proves directory and socket permissions allowed this
	// identity through. Only the server's peer-credential check should close it.
	_, err = io.WriteString(conn, "POST /status HTTP/1.1\r\nHost: unix\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	var response *http.Response
	if err == nil {
		response, err = http.ReadResponse(bufio.NewReader(conn), nil)
	}
	if expectation == "deny" {
		if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNRESET) && !errors.Is(err, syscall.EPIPE) {
			fail("IPC peer was not promptly rejected before an HTTP response")
		}
		fmt.Println("IPC filesystem access succeeded; unrelated peer rejected: passed")
		return
	}
	if err != nil {
		fail("authorized IPC caller failed")
	}
	defer response.Body.Close()
	var status struct {
		Instance   string `json:"instance"`
		Generation string `json:"generation"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&status) != nil || status.Instance == "" || status.Generation == "" {
		fail("authorized IPC caller did not receive active status")
	}
	fmt.Println("authorized IPC peer status: passed")
}
