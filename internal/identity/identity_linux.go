package identity

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const MaxAccountFileBytes = 16 << 20

var ErrNameNotFound = errors.New("identity not found")

// FileID resolves the numeric third field of a bounded passwd or group file.
func FileID(path, name string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxAccountFileBytes+1))
	if err != nil {
		return 0, err
	}
	if len(b) > MaxAccountFileBytes {
		return 0, errors.New("identity file exceeds size limit")
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		user, rest, ok := strings.Cut(line, ":")
		if !ok || user != name {
			continue
		}
		_, rest, ok = strings.Cut(rest, ":")
		if !ok {
			return 0, errors.New("invalid identity record")
		}
		id, _, ok := strings.Cut(rest, ":")
		if !ok {
			return 0, errors.New("invalid identity record")
		}
		n, err := strconv.ParseUint(id, 10, 32)
		return uint32(n), err
	}
	return 0, fmt.Errorf("%w: %s", ErrNameNotFound, name)
}

// PeerUID reads the kernel credential of a local Unix connection.
func PeerUID(c net.Conn) (uint32, error) {
	u, ok := c.(*net.UnixConn)
	if !ok {
		return 0, errors.New("Unix connection required")
	}
	raw, err := u.SyscallConn()
	if err != nil {
		return 0, err
	}
	var cred *unix.Ucred
	var inner error
	err = raw.Control(func(fd uintptr) { cred, inner = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED) })
	if err != nil {
		return 0, err
	}
	if inner != nil {
		return 0, inner
	}
	return cred.Uid, nil
}
