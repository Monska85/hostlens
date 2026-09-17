package linux

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
	"golang.org/x/sys/unix"
)

func (c *Collector) auditPath(r *contract.Result, a contract.PathArgs) bool {
	p := r.Data.(*contract.PathInspection)
	r.Source = "handle-bound filesystem metadata"
	p.Scope = "selected inode; mode bits do not establish effective access through ACLs, parent directories or security modules"
	if !c.Policy.Allowed("files", a.Path, false) {
		auditIssue(r, "policy_denied", a.Path, "explicit file source grant required")
		return false
	}
	root := c.Root
	if root == "" {
		root = "/"
	}
	rfd, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		auditIssue(r, "collection_failed", a.Path, "cannot open observation root")
		return false
	}
	defer unix.Close(rfd)
	relative := strings.TrimPrefix(a.Path, "/")
	if relative == "" {
		relative = "."
	}
	fd, err := unix.Openat2(rfd, relative, &unix.OpenHow{Flags: unix.O_PATH | unix.O_CLOEXEC, Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		code := "collection_failed"
		switch {
		case errors.Is(err, os.ErrPermission):
			code = "permission_denied"
		case errors.Is(err, os.ErrNotExist):
			code = "source_unavailable"
		case errors.Is(err, unix.ELOOP), errors.Is(err, unix.EXDEV):
			code = "unsafe_source"
		}
		auditIssue(r, code, a.Path, "path metadata unavailable under current OS permissions and safe-resolution rules")
		return false
	}
	defer unix.Close(fd)
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil {
		auditIssue(r, "collection_failed", a.Path, "cannot inspect selected inode")
		return false
	}
	kind := "file"
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		if st.Nlink > 1 {
			auditIssue(r, "policy_denied", a.Path, "hard-linked file metadata denied")
			return false
		}
	case unix.S_IFDIR:
		kind = "directory"
	default:
		auditIssue(r, "unsupported_source", a.Path, "only regular files and directories are supported")
		return false
	}
	rootPath, e1 := os.Readlink("/proc/self/fd/" + strconv.Itoa(rfd))
	resolved, e2 := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
	rel, e3 := filepath.Rel(rootPath, resolved)
	if e1 != nil || e2 != nil || e3 != nil || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasSuffix(resolved, " (deleted)") || !c.Policy.Allowed("files", filepath.Clean("/"+rel), false) {
		auditIssue(r, "policy_denied", a.Path, "resolved metadata source denied or changed")
		return false
	}
	for _, rule := range c.Policy.Rules {
		if rule.Mandatory && rule.Literal {
			var secret unix.Stat_t
			if unix.Stat(filepath.Join(rootPath, rule.Pattern), &secret) == nil && secret.Dev == st.Dev && secret.Ino == st.Ino {
				auditIssue(r, "policy_denied", a.Path, "protected object metadata denied")
				return false
			}
		}
	}
	p.Path = a.Path
	p.Type = kind
	p.UID = st.Uid
	p.GID = st.Gid
	p.Mode = strconv.FormatUint(uint64(st.Mode&07777), 8)
	p.SizeBytes = st.Size
	p.Inode = st.Ino
	p.Links = uint64(st.Nlink)
	p.MtimeUnix = st.Mtim.Sec
	return true
}
