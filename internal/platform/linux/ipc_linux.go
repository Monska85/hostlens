package linux

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/identity"
)

func PeerUID(c net.Conn) (uint32, error) {
	return identity.PeerUID(c)
}

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
		if e == nil && (uid == l.UID || uid == 0) {
			return c, nil
		}
		c.Close()
	}
}
func ListenUnix(path string, uid uint32) (net.Listener, error) {
	if st, e := os.Lstat(path); e == nil {
		stat, ok := st.Sys().(*syscall.Stat_t)
		if !ok || st.Mode()&os.ModeSocket == 0 || int(stat.Uid) != os.Geteuid() {
			return nil, errors.New("unsafe existing socket resource")
		}
		conn, err := net.DialTimeout("unix", path, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil, errors.New("instance already listening")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			return nil, errors.New("existing socket cannot be safely classified")
		}
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(st, current) {
			return nil, errors.New("socket changed during stale-instance check")
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
	if e = os.Chmod(path, 0660); e == nil && os.Geteuid() == 0 {
		e = os.Chown(path, int(uid), -1)
	}
	if e != nil {
		l.Close()
		return nil, e
	}
	return checkedListener{l, uid}, nil
}
func UnixClient(path string) *http.Client {
	return &http.Client{Timeout: contract.IPCClientTimeout, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}, DisableKeepAlives: true}}
}
func UID(name string) (uint32, error) {
	if name == "" {
		return uint32(os.Getuid()), nil
	}
	return identity.FileID("/etc/passwd", name)
}
