//go:build linux

package localipc

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// PeerUID reads the kernel credential of an already connected local socket.
func PeerUID(conn net.Conn) (uint32, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, errors.New("Unix connection required")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var identity *unix.Ucred
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		identity, socketErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, err
	}
	if socketErr != nil {
		return 0, socketErr
	}
	return identity.Uid, nil
}
