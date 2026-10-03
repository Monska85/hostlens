//go:build linux

package auth

import (
	"os"
	"syscall"
)

func ownerTrusted(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && (owner.Uid == 0 || int(owner.Uid) == os.Geteuid())
}
