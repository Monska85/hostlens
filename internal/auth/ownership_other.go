//go:build !linux

package auth

import "os"

// Native adapters must provide their own ownership enforcement before this
// store is enabled on another platform.
func ownerTrusted(os.FileInfo) bool { return false }
