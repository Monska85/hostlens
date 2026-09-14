package observerapp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// allowSocketType is the disposable stand-in for the root-ownership socket
// check; production keeps validateEngineSocket.
func allowSocketType(path string, _ int) error {
	st, e := os.Lstat(path)
	if e != nil || st.Mode()&os.ModeSocket == 0 {
		return errors.New("engine socket is not a Unix socket")
	}
	return nil
}

func TestObserverRejectsUnauthorizedPeers(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ipc := filepath.Join(dir, "observer.sock")
	l, e := net.Listen("unix", ipc)
	if e != nil {
		t.Fatal(e)
	}
	// The checked listener accepts only its configured UID; the test process
	// connects as its own UID, so a different target UID must be rejected
	// before any Docker access.
	rejecting := checkedListener{l, uint32(os.Getuid() + 777)}
	go func() {
		for {
			c, e := rejecting.Accept()
			if e == nil {
				c.Close()
			}
		}
	}()
	conn, e := net.DialTimeout("unix", ipc, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	// The listener closes rejected peers: no response bytes are returned,
	// whatever the close surfaces (EOF or connection reset).
	if _, e := conn.Write([]byte("GET / HTTP/1.1\r\n\r\n")); e != nil {
		t.Fatalf("rejected peer write: %v", e)
	}
	if n, e := conn.Read(make([]byte, 64)); n != 0 || e == nil {
		t.Fatalf("rejected peer received a response: n=%d err=%v", n, e)
	}
	// The matching identity is accepted and may exchange bytes. A dedicated
	// listener keeps the accept deterministic: the rejecting loop above stays
	// parked inside its own Accept and must not compete for this connection.
	ipc2 := filepath.Join(dir, "observer-accept.sock")
	l2, e := net.Listen("unix", ipc2)
	if e != nil {
		t.Fatal(e)
	}
	accepting := checkedListener{l2, uint32(os.Getuid())}
	accepted := make(chan net.Conn, 1)
	go func() {
		c, e := accepting.Accept()
		if e == nil {
			accepted <- c
		}
	}()
	conn2, e := net.DialTimeout("unix", ipc2, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	defer conn2.Close()
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("authorized peer not accepted")
	}
}

func TestEngineSocketValidationRefusals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	l, e := net.Listen("unix", sock)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if os.Getuid() == 0 {
		// Non-root owner must be refused: rootful system-wide engines only.
		if e := os.Chown(sock, 1000, -1); e == nil {
			if e := validateEngineSocket(sock, os.Getgid()); e == nil || !strings.Contains(e.Error(), "owned by root") {
				t.Fatalf("non-root-owned socket accepted: %v", e)
			}
			if e := os.Chown(sock, 0, -1); e != nil {
				t.Fatal(e)
			}
		}
		// Group mismatch must be refused.
		if e := os.Chown(sock, -1, 1000); e == nil {
			if e := validateEngineSocket(sock, os.Getgid()); e == nil || !strings.Contains(e.Error(), "group") {
				t.Fatalf("group-mismatched socket accepted: %v", e)
			}
			if e := os.Chown(sock, -1, os.Getgid()); e != nil {
				t.Fatal(e)
			}
		}
	}
	// World-accessible sockets must be refused: world-readable sockets are
	// root-equivalent on the host.
	if e := os.Chmod(sock, 0666); e != nil {
		t.Fatal(e)
	}
	if e := validateEngineSocket(sock, os.Getgid()); e == nil {
		t.Fatal("unsafe socket accepted")
	} else if os.Getuid() == 0 && !strings.Contains(e.Error(), "world access") {
		t.Fatalf("world-accessible socket accepted: %v", e)
	}
	if e := os.Chmod(sock, 0644); e != nil {
		t.Fatal(e)
	}
	if e := validateEngineSocket(sock, os.Getgid()); e == nil {
		t.Fatal("world-readable socket accepted")
	}
	if e := os.Chmod(sock, 0660); e != nil {
		t.Fatal(e)
	}
	if os.Getuid() != 0 {
		// Non-root environments cannot produce a root-owned socket; the
		// ownership refusal itself is the verified behavior.
		if e := validateEngineSocket(sock, os.Getgid()); e == nil || !strings.Contains(e.Error(), "owned by root") {
			t.Fatalf("non-root-owned socket accepted: %v", e)
		}
		return
	}
	if e := validateEngineSocket(sock, os.Getgid()); e != nil {
		t.Fatalf("compliant socket refused: %v", e)
	}
}

func TestObserverOversizedAndMalformedDaemonResponses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	l, e := net.Listen("unix", sock)
	if e != nil {
		t.Fatal(e)
	}
	var mode atomic.Value
	mode.Store("ok")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		switch mode.Load().(string) {
		case "oversized":
			w.Write(make([]byte, dockerobs.MaxListBytes+1024))
		case "malformed":
			w.Write([]byte("{not json"))
		default:
			w.Write([]byte("[]"))
		}
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })

	s := NewServer(sock, 0)
	defer s.Close()
	s.engine.validate = allowSocketType

	mode.Store("oversized")
	response := s.run(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if !response.Failed || !strings.Contains(response.Reason, "ceiling") {
		t.Fatalf("oversized daemon body accepted: %+v", response)
	}
	mode.Store("malformed")
	response = s.run(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if !response.Failed || !strings.Contains(response.Reason, "malformed") {
		t.Fatalf("malformed daemon body accepted: %+v", response)
	}
}

func TestObserverStalledDaemonStaysBounded(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	l, e := net.Listen("unix", sock)
	if e != nil {
		t.Fatal(e)
	}
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		<-release
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { close(release); _ = server.Close() })

	s := NewServer(sock, 0)
	defer s.Close()
	s.engine.validate = allowSocketType
	saved := observationBudget
	observationBudget = 30 * time.Millisecond
	t.Cleanup(func() { observationBudget = saved })
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	response := s.run(ctx, dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if !response.Failed {
		t.Fatal("stalled daemon returned evidence")
	}
	// The stalled work must release its slot; repeated abandoned calls
	// cannot accumulate workers or queue unbounded work.
	for range dockerobs.MaxConcurrent + 4 {
		s.run(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	}
	release <- struct{}{}
}

func TestObserverIPCRejectsHostileBodies(t *testing.T) {
	t.Parallel()

	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	dir := t.TempDir()
	ipc := filepath.Join(dir, "observer.sock")
	listener, e := net.Listen("unix", ipc)
	if e != nil {
		t.Fatal(e)
	}
	httpServer := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: time.Second}
	go func() { _ = httpServer.Serve(listener) }()
	t.Cleanup(func() { _ = httpServer.Close() })

	hostile := map[string]string{
		"unknown field":   `{"version":1,"operation":"container_list","evil":"x"}`,
		"bad version":     `{"version":9,"operation":"container_list"}`,
		"unknown op":      `{"version":1,"operation":"mutate_everything"}`,
		"multi document":  `{"version":1,"operation":"container_list"}{"version":1}`,
		"truncated":       `{"version":1,"oper`,
		"mutation-shaped": `{"version":1,"operation":"container_list","logs":{"max_bytes":999999999999}}`,
	}
	before := len(engine.requests)
	for name, body := range hostile {
		conn, e := net.DialTimeout("unix", ipc, time.Second)
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		payload := fmt.Sprintf("POST /observe HTTP/1.1\r\nHost: o\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
		if _, e := conn.Write([]byte(payload)); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 512)
		n, _ := conn.Read(buf)
		conn.Close()
		status := ""
		if i := strings.Index(string(buf[:n]), "\r\n"); i > 0 {
			status = string(buf[:i])
		}
		if !strings.Contains(status, "400") && !strings.Contains(status, "404") {
			t.Fatalf("%s: hostile body accepted with %q", name, status)
		}
	}
	// Rejection must happen before any Docker access: the engine transcript
	// stays empty.
	if len(engine.requests) != before {
		t.Fatalf("hostile bodies reached the engine: %v", engine.requests)
	}
}

func TestObserverConcurrentColdStartRace(t *testing.T) {
	t.Parallel()

	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	s.engine.validate = allowSocketType
	// Cold start with the maximum admitted burst: negotiation serializes
	// under the mutex and every observation completes without a race.
	done := make(chan struct{})
	for range dockerobs.MaxConcurrent {
		go func() {
			defer func() { done <- struct{}{} }()
			response := s.run(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpEngineInfo})
			if response.Failed {
				t.Error(response.Reason)
			}
		}()
	}
	for range dockerobs.MaxConcurrent {
		<-done
	}
	if !strings.Contains(strings.Join(engine.requests, ","), "GET /version?") {
		t.Fatal("negotiation probe missing")
	}
}

func TestLogDecodeHonestExactCeiling(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("x", 4096)
	page, e := decodeLogStream(strings.NewReader(payload), 4096, true)
	if e != nil {
		t.Fatal(e)
	}
	if page.Truncated {
		t.Fatal("a complete response at the exact ceiling must not be reported truncated")
	}
	page, e = decodeLogStream(strings.NewReader(payload+"!"), 4096, true)
	if e != nil {
		t.Fatal(e)
	}
	if !page.Truncated {
		t.Fatal("content beyond the ceiling must be reported truncated")
	}
}
