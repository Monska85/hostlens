package linux

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/policy"
	"golang.org/x/sys/unix"
)

func Trust(path string, uid int) error {
	if !filepath.IsAbs(path) {
		return errors.New("trust path must be absolute")
	}
	for p := path; ; p = filepath.Dir(p) {
		var st unix.Stat_t
		if e := unix.Lstat(p, &st); e != nil {
			return e
		}
		if st.Mode&unix.S_IFMT == unix.S_IFLNK {
			return fmt.Errorf("%s: symlinked policy source", p)
		}
		if (int(st.Uid) != uid && st.Uid != 0) || (st.Mode&0022 != 0 && !(st.Uid == 0 && st.Mode&unix.S_ISVTX != 0 && p != path)) {
			return fmt.Errorf("%s: untrusted ownership or writable permissions", p)
		}
		if p == "/" {
			break
		}
	}
	return nil
}

const (
	maxConfigurationInputBytes = 8 << 20
	maxProfileDefinitions      = 1024
	maxProfileDirectories      = 1024
)

func Load(path string, system bool) (config.Config, *policy.Policy, error) {
	c := config.DefaultsLinux(system)
	uid := os.Getuid()
	if system {
		uid = 0
	}
	if e := Trust(path, uid); e != nil {
		return c, nil, e
	}
	remaining := maxConfigurationInputBytes
	b, e := readConfiguration(path, &remaining)
	if e != nil {
		return c, nil, e
	}
	if e = config.Decode(b, &c); e != nil {
		return c, nil, fmt.Errorf("%s: %w", path, e)
	}
	if (c.Mode == "system") != system {
		return c, nil, errors.New("mode differs from startup --system choice")
	}
	if e = config.ValidateLinux(c); e != nil {
		return c, nil, fmt.Errorf("%s: %w", path, e)
	}
	if len(c.ProfileDirs) > maxProfileDirectories {
		return c, nil, errors.New("configuration exceeds 1024 profile directories")
	}
	defs := map[string]policy.Definition{}
	for _, dir := range c.ProfileDirs {
		if e = Trust(dir, uid); errors.Is(e, os.ErrNotExist) {
			continue
		} else if e != nil {
			return c, nil, e
		}
		d, e := os.OpenFile(dir, os.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return c, nil, e
		}
		entries, e := d.ReadDir(1025)
		d.Close()
		if e != nil && e != io.EOF {
			return c, nil, e
		}
		if len(entries) > 1024 {
			return c, nil, errors.New("profile directory exceeds 1024 entries")
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".yaml")
			file := filepath.Join(dir, entry.Name())
			if !policy.ValidName(name) {
				return c, nil, fmt.Errorf("unsafe profile name %s", file)
			}
			if prev, ok := defs[name]; ok {
				return c, nil, fmt.Errorf("duplicate profile %s: %s and %s", name, prev.Source, file)
			}
			if len(defs) >= maxProfileDefinitions {
				return c, nil, errors.New("configuration exceeds 1024 profile definitions")
			}
			if e = Trust(file, uid); e != nil {
				return c, nil, e
			}
			b, e = readConfiguration(file, &remaining)
			if e != nil {
				return c, nil, e
			}
			var pr config.Profile
			if e = config.Decode(b, &pr); e != nil {
				return c, nil, fmt.Errorf("%s: %w", file, e)
			}
			defs[name] = policy.Definition{Profile: pr, Source: file}
		}
	}
	p, e := policy.CompileLinux(c, path, defs)
	return c, p, e
}

func readConfiguration(path string, remaining *int) ([]byte, error) {
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
func OpenRegular(path string, p *policy.Policy) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("absolute clean path required")
	}
	if !p.Allowed("files", path, false) {
		return nil, errors.New("policy denied")
	}
	fd, e := unix.Openat2(unix.AT_FDCWD, path, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK, Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if e != nil {
		return nil, fmt.Errorf("safe open: %w", e)
	}
	f := os.NewFile(uintptr(fd), path)
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if err := rejectHardlinks(fd); err != nil {
		return nil, err
	}
	resolved, e := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
	if e != nil || strings.HasSuffix(resolved, " (deleted)") || !p.Allowed("files", resolved, false) {
		return nil, errors.New("resolved path denied or changed")
	}
	for _, r := range p.Rules {
		if r.Mandatory && r.Literal {
			secret, e := os.Stat(r.Pattern)
			if e == nil && os.SameFile(st, secret) {
				return nil, errors.New("mandatory protected object")
			}
		}
	}
	ok = true
	return f, nil
}
func ReadBounded(f *os.File, n int) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(f, int64(n)+1))
	if e != nil {
		return nil, e
	}
	if len(b) > n {
		return nil, errors.New("size limit exceeded")
	}
	return b, nil
}

// openObservation binds both policy checks to a descriptor within the selected
// root. IN_ROOT allows normal absolute OS symlinks without escaping fixture roots.
func openObservation(root, path string, p *policy.Policy, builtin bool) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !p.Allowed("files", path, builtin) {
		return nil, errors.New("source denied")
	}
	if root == "" {
		root = "/"
	}
	rfd, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(rfd)
	rootPath, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(rfd))
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat2(rfd, strings.TrimPrefix(path, "/"), &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK, Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, fmt.Errorf("safe observation open: %w", err)
	}
	f := os.NewFile(uintptr(fd), path)
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, errors.New("observation source is not a regular file")
	}
	if err = rejectHardlinks(fd); err != nil {
		return nil, err
	}
	resolved, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
	if err != nil || strings.HasSuffix(resolved, " (deleted)") {
		return nil, errors.New("source changed")
	}
	rel, err := filepath.Rel(rootPath, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || !p.Allowed("files", "/"+rel, builtin) {
		return nil, errors.New("resolved source denied")
	}
	for _, r := range p.Rules {
		if r.Mandatory && r.Literal {
			secret, err := os.Stat(filepath.Join(rootPath, r.Pattern))
			if err == nil && os.SameFile(st, secret) {
				return nil, errors.New("mandatory protected object")
			}
		}
	}
	ok = true
	return f, nil
}

func rejectHardlinks(fd int) error {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Nlink > 1 {
		return errors.New("hard-linked diagnostic sources are unsupported")
	}
	return nil
}
