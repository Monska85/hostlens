//go:build !linux

package containers

import "os"

func socketOwnedByRoot(os.FileInfo) bool { return false }
