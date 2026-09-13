package observerapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// PeerUID returns the effective peer UID of one local Unix connection.
func PeerUID(c net.Conn) (uint32, error) {
	u, ok := c.(*net.UnixConn)
	if !ok {
		return 0, errors.New("Unix connection required")
	}
	raw, e := u.SyscallConn()
	if e != nil {
		return 0, e
	}
	var cred *syscall.Ucred
	var inner error
	e = raw.Control(func(fd uintptr) {
		cred, inner = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	})
	if e != nil {
		return 0, e
	}
	if inner != nil {
		return 0, inner
	}
	return cred.Uid, nil
}

// checkedListener accepts only connections from the diagnostic backend
// identity. Unrelated local processes are rejected before any Docker access.
type checkedListener struct {
	net.Listener
	UID uint32
}

func (l checkedListener) Accept() (net.Conn, error) {
	for {
		c, e := l.Listener.Accept()
		if e != nil {
			return nil, e
		}
		uid, e := PeerUID(c)
		if e == nil && uid == l.UID {
			return c, nil
		}
		c.Close()
		// Rejected peers shed load instead of busy-looping the accept path.
		time.Sleep(time.Millisecond)
	}
}

// ListenIPC prepares the observer IPC endpoint. An existing live instance
// or unsafe socket resource is refused instead of replaced.
func ListenIPC(path string, uid uint32) (net.Listener, error) {
	if st, e := os.Lstat(path); e == nil {
		stat, ok := st.Sys().(*syscall.Stat_t)
		if !ok || st.Mode()&os.ModeSocket == 0 || int(stat.Uid) != os.Geteuid() {
			return nil, errors.New("unsafe existing observer socket resource")
		}
		conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil, errors.New("observer already listening")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return nil, errors.New("existing observer socket cannot be safely classified")
		}
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(st, current) {
			return nil, errors.New("observer socket changed during stale-instance check")
		}
		if err = os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	l, e := net.Listen("unix", path)
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0660); e != nil {
		l.Close()
		return nil, e
	}
	return checkedListener{l, uid}, nil
}

// ListenInherited adopts the socket-activated listener passed by the service
// manager. The observer process never touches the IPC directory itself, so
// it needs no access to the runtime directory of the diagnostic backend.
func ListenInherited(uid uint32) (net.Listener, error) {
	if os.Getenv("LISTEN_PID") != strconv.Itoa(os.Getpid()) {
		return nil, errors.New("socket activation not addressed to this process")
	}
	if fds, e := strconv.Atoi(os.Getenv("LISTEN_FDS")); e != nil || fds != 1 {
		return nil, errors.New("exactly one socket-activated listener expected")
	}
	file := os.NewFile(3, "systemd-socket")
	if file == nil {
		return nil, errors.New("socket-activated listener unavailable")
	}
	l, e := net.FileListener(file)
	if e != nil {
		return nil, e
	}
	return checkedListener{l, uid}, nil
}

// Server dispatches typed observation requests over the observer IPC. Only
// the fixed operation set, protocol version, and bounds are accepted.
type Server struct {
	engine   *client
	general  chan struct{}
	diskRuns chan struct{}
}

func NewServer(socket string, groupGID int) *Server {
	return &Server{
		engine:   newClient(socket, groupGID),
		general:  make(chan struct{}, dockerobs.MaxConcurrent),
		diskRuns: make(chan struct{}, dockerobs.MaxDiskUsageRuns),
	}
}

func (s *Server) Close() { s.engine.close() }

// Handler returns the IPC endpoint. Unknown paths, methods, versions,
// operations, fields, and oversized or malformed requests fail before any
// Docker access.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path != "/observe" {
			http.NotFound(w, r)
			return
		}
		var request dockerobs.Request
		if e := decodeRequest(r, &request); e != nil {
			http.Error(w, "invalid observation request", http.StatusBadRequest)
			return
		}
		if e := request.Validate(); e != nil {
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}
		response := s.run(r.Context(), request)
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.Encode(response)
	})
}

func decodeRequest(r *http.Request, out *dockerobs.Request) error {
	d := json.NewDecoder(io.LimitReader(r.Body, dockerobs.MaxRequestBytes+1))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e == nil {
		return errors.New("one observation request required")
	}
	return nil
}

// run admits the observation through per-operation work bounds. Expensive
// disk accounting shares one slot so it cannot starve all other evidence.
func (s *Server) run(ctx context.Context, request dockerobs.Request) dockerobs.Response {
	slot := s.general
	if request.Operation == dockerobs.OpDiskUsage {
		slot = s.diskRuns
	}
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-ctx.Done():
		return dockerobs.Response{Failed: true, Reason: "observation slot unavailable"}
	}
	return s.engine.observe(ctx, request)
}

// resolveUID maps a service identity name to its numeric UID.
func resolveUID(name string) (uint32, error) {
	if name == "" {
		return uint32(os.Getuid()), nil
	}
	b, e := os.ReadFile("/etc/passwd")
	if e != nil {
		return 0, e
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 2 && fields[0] == name {
			n, e := strconv.ParseUint(fields[2], 10, 32)
			return uint32(n), e
		}
	}
	return 0, errors.New("configured identity not found")
}

// resolveGID maps the configured access group to its numeric GID.
func resolveGID(name string) (int, error) {
	b, e := os.ReadFile("/etc/group")
	if e != nil {
		return 0, e
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 2 && fields[0] == name {
			n, e := strconv.Atoi(fields[2])
			if e != nil || n < 0 {
				return 0, errors.New("invalid group identifier")
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("access group %q not found", name)
}
