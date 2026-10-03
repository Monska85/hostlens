//go:build linux

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// OpenTrusted rejects path substitution by untrusted local users before
// reading configuration, credentials, or migration input.
func OpenTrusted(path string) (*os.File, error) {
	if err := trustParents(path); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || !trustedOwner(info) {
		file.Close()
		return nil, errors.New("protected regular file required")
	}
	return file, nil
}

func TrustedDirectory(path string) error {
	if err := trustParents(path); err != nil {
		return err
	}
	var info unix.Stat_t
	if err := unix.Lstat(path, &info); err != nil {
		return err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFDIR || (info.Uid != 0 && int(info.Uid) != os.Geteuid()) || info.Mode&0022 != 0 {
		return errors.New("protected directory required")
	}
	return nil
}

func trustParents(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("trusted path must be absolute and clean")
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		var info unix.Stat_t
		if err := unix.Lstat(parent, &info); err != nil {
			return err
		}
		if info.Mode&unix.S_IFMT != unix.S_IFDIR || (info.Uid != 0 && int(info.Uid) != os.Geteuid()) ||
			(info.Mode&0022 != 0 && !(info.Uid == 0 && info.Mode&unix.S_ISVTX != 0)) {
			return fmt.Errorf("untrusted parent directory %s", parent)
		}
		if parent == "/" {
			break
		}
	}
	return nil
}
