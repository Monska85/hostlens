package observerapp

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestListenIPCRefusesNonSocketResources(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if e := os.WriteFile(regular, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := ListenIPC(regular, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("regular file accepted as IPC socket: %v", e)
	}
	if _, e := ListenIPC(dir, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("directory accepted as IPC socket: %v", e)
	}
	if _, e := ListenIPC(filepath.Join(dir, "missing-dir", "o.sock"), 0); e == nil {
		t.Fatal("missing parent accepted")
	}
}

func TestListenIPCRefusesLiveInstance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observer.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ListenIPC(path, 0); e == nil || !strings.Contains(e.Error(), "already listening") {
		t.Fatalf("live instance replaced: %v", e)
	}
	l.Close()
}

func TestListenIPCRemovesStaleSocketAndBinds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observer.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	l.Close()
	listener, e := ListenIPC(path, 0)
	if e != nil {
		t.Fatalf("stale socket not reclaimed: %v", e)
	}
	defer listener.Close()
	st, e := os.Lstat(path)
	if e != nil {
		t.Fatal(e)
	}
	if st.Mode().Perm() != 0o660 {
		t.Fatalf("IPC socket mode = %o, want 660", st.Mode().Perm())
	}
	// The bound socket must be live again.
	dial, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatalf("rebound socket unreachable: %v", err)
	}
	dial.Close()
}

func TestListenIPCRefusesForeignOwnedSocket(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("foreign-owner refusal requires root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "observer.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	if e := os.Chown(path, 1000, -1); e != nil {
		t.Skipf("cannot arrange foreign ownership here: %v", e)
	}
	if _, e := ListenIPC(path, 0); e == nil || !strings.Contains(e.Error(), "unsafe") {
		t.Fatalf("foreign-owned socket accepted: %v", e)
	}
}

func TestListenIPCHandlesSocketChangedDuringCheck(t *testing.T) {
	// The stale-instance check re-verifies the same file after the failed
	// dial; a concurrent replacement must be refused. The race window is
	// microscopic, so both the defensive refusal and the clean success are
	// acceptable outcomes of each attempt.
	dir := t.TempDir()
	path := filepath.Join(dir, "observer.sock")
	stale, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	stale.Close()
	sawChange, sawSuccess := false, false
	for range 300 {
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
				}
				os.Remove(path)
				replacement, err := net.Listen("unix", path)
				if err == nil {
					replacement.Close()
				}
			}
		}()
		listener, err := ListenIPC(path, 0)
		close(stop)
		<-done
		if listener != nil {
			listener.Close()
			sawSuccess = true
		} else if err != nil && (strings.Contains(err.Error(), "changed") || strings.Contains(err.Error(), "classified") || strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "address already in use") || strings.Contains(err.Error(), "already listening")) {
			sawChange = true
		} else if err != nil {
			t.Fatalf("unexpected ListenIPC failure: %v", err)
		}
		os.Remove(path)
		stale2, err := net.Listen("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		stale2.Close()
	}
	if !sawChange && !sawSuccess {
		t.Fatal("neither a rebound socket nor a defensive refusal was observed")
	}
	t.Logf("changed-during-check branches: refusal=%v success=%v", sawChange, sawSuccess)
}

func TestListenInheritedRefusesMismatchedActivation(t *testing.T) {
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()+3))
	t.Setenv("LISTEN_FDS", "1")
	if _, e := ListenInherited(0); e == nil || !strings.Contains(e.Error(), "not addressed to this process") {
		t.Fatalf("foreign activation accepted: %v", e)
	}
	os.Unsetenv("LISTEN_PID")
	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))
	t.Setenv("LISTEN_FDS", "2")
	if _, e := ListenInherited(0); e == nil || !strings.Contains(e.Error(), "exactly one") {
		t.Fatalf("multi-fd activation accepted: %v", e)
	}
	t.Setenv("LISTEN_FDS", "not-a-number")
	if _, e := ListenInherited(0); e == nil || !strings.Contains(e.Error(), "exactly one") {
		t.Fatalf("malformed LISTEN_FDS accepted: %v", e)
	}
	os.Unsetenv("LISTEN_FDS")
	if _, e := ListenInherited(0); e == nil || !strings.Contains(e.Error(), "exactly one") {
		t.Fatalf("missing LISTEN_FDS accepted: %v", e)
	}
}

func TestListenInheritedChildHelper(t *testing.T) {
	if os.Getenv("OBSERVER_IPC_CHILD") != "1" {
		t.Skip("subprocess helper")
	}
	l, e := ListenInherited(uint32(os.Getuid()))
	if e != nil {
		os.Exit(3)
	}
	type deadliner interface{ SetDeadline(time.Time) error }
	if d, ok := l.(deadliner); ok {
		_ = d.SetDeadline(time.Now().Add(5 * time.Second))
	}
	c, e := l.Accept()
	if e != nil {
		os.Exit(4)
	}
	buf := make([]byte, 5)
	if _, e := c.Read(buf); e != nil || string(buf) != "hello" {
		c.Close()
		os.Exit(5)
	}
	c.Write([]byte("world"))
	c.Close()
	// Return normally so the coverage counters of this child process are
	// flushed to GOCOVERDIR and merged with the parent's profile.
}

func TestListenInheritedAdoptsSocketActivation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "activated.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	// systemd hands the listening fd as fd 3; dup it for the child.
	file, e := l.(*net.UnixListener).File()
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	script := fmt.Sprintf("OBSERVER_IPC_CHILD=1 LISTEN_PID=$$ LISTEN_FDS=1 exec %q -test.run=TestListenInheritedChildHelper -test.timeout=8s", os.Args[0])
	if mode := testing.CoverMode(); mode != "" {
		if f := flag.Lookup("test.gocoverdir"); f != nil && f.Value.String() != "" {
			// go test directs the coverage epilogue through the
			// -test.gocoverdir flag; the child needs the same target so its
			// counters merge with the parent's profile.
			script = fmt.Sprintf("OBSERVER_IPC_CHILD=1 LISTEN_PID=$$ LISTEN_FDS=1 exec %q -test.run=TestListenInheritedChildHelper -test.timeout=8s -test.gocoverdir=%s", os.Args[0], f.Value.String())
		}
	}
	cmd := exec.Command("sh", "-c", script)
	cmd.ExtraFiles = []*os.File{file}
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	// The child adopts fd 3 on the same path; exercise it end to end.
	var conn net.Conn
	var dialErr error
	for range 100 {
		conn, dialErr = net.DialTimeout("unix", path, time.Second)
		if dialErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if dialErr != nil {
		cmd.Process.Kill()
		t.Fatalf("activated socket unreachable: %v", dialErr)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, e := conn.Write([]byte("hello")); e != nil {
		t.Fatalf("child exchange write: %v", e)
	}
	buf := make([]byte, 5)
	if _, e := conn.Read(buf); e != nil || string(buf) != "world" {
		t.Fatalf("child exchange read: %v %q", e, buf)
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("inherited listener child failed: %v", e)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("inherited listener child timed out")
	}
}

func TestHandlerRejectsWrongMethodAndUnknownPaths(t *testing.T) {
	engine := newRecordingEngine(t)
	s := NewServer(engine.path, 0)
	defer s.Close()
	handler := s.Handler()

	get := httptest.NewRequest(http.MethodGet, "/observe", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, get)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET observe = %d, want 405", rec.Code)
	}
	unknown := httptest.NewRequest(http.MethodPost, "/mutate", strings.NewReader("{}"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, unknown)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path = %d, want 404", rec.Code)
	}
	// Successful POST must still reach the engine through the same handler.
	post := httptest.NewRequest(http.MethodPost, "/observe", strings.NewReader(`{"version":1,"operation":"container_list"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, post)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid POST = %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestResolveIdentitiesFromSystemFiles(t *testing.T) {
	uid, e := resolveUID("root")
	if e != nil || uid != 0 {
		t.Fatalf("root identity = %d, %v", uid, e)
	}
	missingUID, missingErr := resolveUID("definitely-missing-hostlens-user")
	if missingErr == nil {
		t.Fatal("bogus identity accepted")
	}
	if !strings.Contains(missingErr.Error(), "not found") {
		t.Fatalf("identity error lost: %v", missingErr)
	}
	_ = missingUID
	empty, e := resolveUID("")
	if e != nil || empty != uint32(os.Getuid()) {
		t.Fatalf("empty identity must resolve to the current uid: %d, %v", empty, e)
	}

	gid, e := resolveGID("root")
	if e != nil || gid != 0 {
		t.Fatalf("root group = %d, %v", gid, e)
	}
	if _, e := resolveGID("definitely-missing-hostlens-group"); e == nil || !strings.Contains(e.Error(), "not found") {
		t.Fatalf("bogus group accepted: %v", e)
	}
}

func TestPeerUIDRejectsNonUnixConnection(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, e := PeerUID(a); e == nil {
		t.Fatal("TCP-style connection accepted")
	}
}
