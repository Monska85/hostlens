package linux

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestListenUnixRefusesUnsafeResources(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if e := os.WriteFile(regular, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := ListenUnix(regular, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("regular file accepted: %v", e)
	}
	if _, e := ListenUnix(dir, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("directory accepted: %v", e)
	}
	if _, e := ListenUnix(filepath.Join(dir, "missing", "o.sock"), 0); e == nil {
		t.Fatal("missing parent accepted")
	}
}

func TestListenUnixRefusesLiveInstance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "o.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ListenUnix(path, uint32(os.Getuid())); e == nil || !strings.Contains(e.Error(), "already listening") {
		t.Fatalf("live instance replaced: %v", e)
	}
	l.Close()
}

func TestListenUnixReclaimsStaleSocket(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "o.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	l.Close()
	listener, e := ListenUnix(path, uint32(os.Getuid()))
	if e != nil {
		t.Fatalf("stale socket not reclaimed: %v", e)
	}
	defer listener.Close()
	st, e := os.Lstat(path)
	if e != nil {
		t.Fatal(e)
	}
	if st.Mode().Perm() != 0o660 {
		t.Fatalf("socket mode = %o, want 660", st.Mode().Perm())
	}
	if os.Geteuid() == 0 {
		// As root the listener must have been chowned to the requested uid.
		info, e := os.Stat(path)
		if e != nil {
			t.Fatal(e)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != 0 {
			t.Fatalf("socket not chowned to the requested identity")
		}
	}
	dial, e := net.DialTimeout("unix", path, time.Second)
	if e != nil {
		t.Fatalf("rebound socket unreachable: %v", e)
	}
	dial.Close()
}

func TestListenUnixRefusesForeignOwnedSocket(t *testing.T) {
	t.Parallel()

	if os.Getuid() != 0 {
		t.Skip("foreign-owner refusal requires root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "o.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e := os.Chown(path, 1000, -1); e != nil {
		t.Skipf("cannot arrange foreign ownership here: %v", e)
	}
	if _, e := ListenUnix(path, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("foreign-owned socket accepted: %v", e)
	}
}

func TestPeerUIDPlatformRejectsNonUnix(t *testing.T) {
	t.Parallel()

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, e := PeerUID(a); e == nil {
		t.Fatal("pipe accepted as unix connection")
	}
}
func TestPlatformUIDResolution(t *testing.T) {
	t.Parallel()

	if uid, e := UID("root"); e != nil || uid != 0 {
		t.Fatalf("root identity = %d, %v", uid, e)
	}
	if uid, e := UID(""); e != nil || uid != uint32(os.Getuid()) {
		t.Fatalf("empty identity = %d, %v", uid, e)
	}
	if _, e := UID("definitely-missing-hostlens-user"); e == nil {
		t.Fatal("bogus identity accepted")
	}
}

func TestUnixClientBuildsLocalTransport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "probe.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })
	resp, e := UnixClient(path).Get("http://unix/probe")
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(resp.Body)
	if e != nil || string(b) != "ok" {
		t.Fatalf("unix client exchange lost: %q %v", b, e)
	}
}
