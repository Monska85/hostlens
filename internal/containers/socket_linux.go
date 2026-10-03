//go:build linux

package containers

import (
	"os"
	"syscall"
)

func socketOwnedByRoot(info os.FileInfo) bool {
	owner, ok := info.Sys().(*syscall.Stat_t)
	return ok && owner.Uid == 0
}
