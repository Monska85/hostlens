//go:build !linux

package core

import "os"

// Native account ownership checks are required before enabling a new OS.
func trustedOwner(os.FileInfo) bool { return false }
