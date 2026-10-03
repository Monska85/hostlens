//go:build !linux

package core

import (
	"errors"
	"os"
)

func OpenTrusted(string) (*os.File, error) {
	return nil, errors.New("protected file access unavailable on this platform")
}

func TrustedDirectory(string) error {
	return errors.New("protected directory access unavailable on this platform")
}
