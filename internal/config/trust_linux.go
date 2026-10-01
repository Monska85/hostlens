package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// TrustLinux requires every path component to be owned by root or uid and
// protected from untrusted writes. Root-owned sticky ancestors are permitted.
func TrustLinux(path string, uid int) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("trust path must be absolute and clean")
	}
	for p := path; ; p = filepath.Dir(p) {
		var st unix.Stat_t
		if err := unix.Lstat(p, &st); err != nil {
			return err
		}
		if st.Mode&unix.S_IFMT == unix.S_IFLNK {
			return fmt.Errorf("%s: symlinked policy source", p)
		}
		if (int(st.Uid) != uid && st.Uid != 0) || (st.Mode&0022 != 0 && !(st.Uid == 0 && st.Mode&unix.S_ISVTX != 0 && p != path)) {
			return fmt.Errorf("%s: untrusted ownership or writable permissions", p)
		}
		if p == "/" {
			return nil
		}
	}
}

// ReadTrustedLinux enforces the per-document and remaining aggregate ceilings.
// The opened descriptor cannot be a symlink or a blocking special file.
func ReadTrustedLinux(path string, uid int, remaining *int) ([]byte, error) {
	if err := TrustLinux(path, uid); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("configuration source is not a regular file")
	}
	limit := min(1<<20, *remaining)
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > limit {
		if limit < 1<<20 {
			return nil, errors.New("configuration input exceeds 8 MiB aggregate size limit")
		}
		return nil, errors.New("configuration document exceeds 1 MiB size limit")
	}
	*remaining -= len(b)
	return b, nil
}
