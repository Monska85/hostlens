package linux

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
	"golang.org/x/sys/unix"
)

const auditMaxEntries = 8192

var errProcessStatMalformed = errors.New("malformed process stat")

// Directory observations use the same rooted, resolved policy boundary as files.
func (c *Collector) auditDirectory(path string) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || !c.Policy.Allowed("files", path, true) {
		return nil, errors.New("source denied")
	}
	root := c.Root
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
	fd, err := unix.Openat2(rfd, strings.TrimPrefix(path, "/"), &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NONBLOCK, Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS})
	if err != nil {
		return nil, err
	}
	resolved, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	rel, err := filepath.Rel(rootPath, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasSuffix(resolved, " (deleted)") || !c.Policy.Allowed("files", "/"+rel, true) {
		unix.Close(fd)
		return nil, errors.New("resolved source denied")
	}
	return os.NewFile(uintptr(fd), "/"+rel), nil
}

func parseProcessStat(b []byte, pid int) (map[string]any, error) {
	s := strings.TrimSpace(string(b))
	begin := strings.IndexByte(s, '(')
	end := strings.LastIndexByte(s, ')')
	if begin < 1 || end <= begin {
		return nil, errors.New("malformed process stat")
	}
	n, e := strconv.Atoi(strings.TrimSpace(s[:begin]))
	if e != nil || n != pid {
		return nil, errors.New("process identity changed")
	}
	fields := strings.Fields(s[end+1:])
	if len(fields) < 22 || len(fields[0]) != 1 {
		return nil, errors.New("short process stat")
	}
	out := map[string]any{"pid": pid, "name": s[begin+1 : end], "state": fields[0]}
	for _, field := range []struct {
		name  string
		index int
	}{{"parent_pid", 1}, {"user_cpu_ticks", 11}, {"system_cpu_ticks", 12}, {"threads", 17}, {"start_ticks", 19}, {"virtual_bytes", 20}, {"resident_pages", 21}} {
		value, e := strconv.ParseUint(fields[field.index], 10, 64)
		if e != nil {
			return nil, fmt.Errorf("invalid process %s", field.name)
		}
		out[field.name] = value
	}
	return out, nil
}

func (c *Collector) processStat(pid int) (map[string]any, error) {
	b, err := c.file("/proc/"+strconv.Itoa(pid)+"/stat", true)
	if err != nil {
		return nil, err
	}
	result, err := parseProcessStat(b, pid)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errProcessStatMalformed, err)
	}
	return result, nil
}

func (c *Collector) auditProcesses(ctx context.Context, r *contract.Result, a contract.Args) {
	r.Source = "procfs"
	r.Data["scope"] = "visible PID namespace; processes hidden by procfs permissions may be absent"
	r.Data["snapshot_consistent"] = false
	limit := a.Limit
	if limit == 0 {
		limit = c.Config.Limits.PageSize
	}
	if limit < 1 || limit > c.Config.Limits.PageSize || a.Offset < 0 {
		auditIssue(r, "invalid_bounds", "/proc", "page bounds exceed ceiling")
		return
	}
	dir, err := c.auditDirectory("/proc")
	if err != nil {
		code := auditErrorCode(err)
		auditIssue(r, code, "/proc", "cannot enumerate processes under current policy or OS permissions")
		return
	}
	defer dir.Close()
	entries, err := dir.ReadDir(auditMaxEntries + 1)
	if err != nil && err != io.EOF {
		auditIssue(r, "collection_failed", "/proc", err.Error())
		return
	}
	if len(entries) > auditMaxEntries {
		entries = entries[:auditMaxEntries]
		r.Truncated = true
		auditIssue(r, "inspection_limit", "/proc", "process directory entry limit reached")
	}
	pids := []int{}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	start := min(a.Offset, len(pids))
	end := min(start+limit, len(pids))
	items := []map[string]any{}
	failures := map[string]int{}
	for _, pid := range pids[start:end] {
		if ctx.Err() != nil {
			break
		}
		if c.auditBudgetExhausted() {
			r.Truncated = true
			auditIssue(r, "inspection_limit", "/proc", "aggregate process read limit reached")
			break
		}
		item, e := c.processStat(pid)
		if e != nil {
			failures[processErrorCode(e)]++
			continue
		}
		items = append(items, item)
	}
	for _, code := range []string{"policy_denied", "permission_denied", "source_unavailable", "malformed_source", "inspection_limit", "collection_failed"} {
		if count := failures[code]; count > 0 {
			auditIssue(r, code, "/proc", fmt.Sprintf("%d selected process observations unavailable", count))
			if code == "inspection_limit" {
				r.Truncated = true
			}
		}
	}
	r.Data["enumerated_processes"] = len(pids)
	r.Data["observed_processes"] = len(items)
	r.Data["items"] = items
	if end < len(pids) {
		r.NextOffset = &end
	}

}

func (c *Collector) auditProcess(ctx context.Context, r *contract.Result, a contract.Args) {
	r.Source = "procfs"
	r.Data["scope"] = "visible PID namespace"
	r.Data["snapshot_consistent"] = false
	base := "/proc/" + strconv.Itoa(a.PID)
	stat, ok := c.auditRead(r, base+"/stat")
	if !ok {
		return
	}
	first, err := parseProcessStat(stat, a.PID)
	if err != nil {
		auditIssue(r, "malformed_source", base+"/stat", "invalid process identity or resource fields")
		return
	}
	if ctx.Err() != nil {
		return
	}
	if b, ok := c.auditRead(r, base+"/status"); ok {
		for _, line := range strings.Split(string(b), "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			fields := strings.Fields(value)
			switch key {
			case "Uid", "Gid":
				if len(fields) != 4 {
					auditIssue(r, "malformed_source", base+"/status", "invalid process identity fields")
					continue
				}
				ids := []uint64{}
				for _, f := range fields {
					v, e := strconv.ParseUint(f, 10, 32)
					if e != nil {
						ids = nil
						break
					}
					ids = append(ids, v)
				}
				if ids == nil {
					auditIssue(r, "malformed_source", base+"/status", "invalid process identity values")
				} else {
					first[strings.ToLower(key)] = ids
				}
			case "NoNewPrivs", "Seccomp", "CapEff":
				if len(fields) == 1 {
					first[key] = fields[0]
				}
			}
		}
	}

	for _, key := range []string{"uid", "gid"} {
		if _, ok := first[key]; !ok {
			auditIssue(r, "partial_observation", base+"/status", "process identity fields unavailable")
		}
	}
	if ctx.Err() != nil {
		return
	}
	dir, e := c.auditDirectory(base)
	if e == nil {
		target, e := c.auditLink(dir, "exe", base+"/exe", true)
		if e == nil {
			first["executable"] = target
		} else {
			processObservationIssue(r, base+"/exe", e)
		}
		dir.Close()
	} else {
		processObservationIssue(r, base, e)
	}
	if ctx.Err() != nil {
		return
	}
	fds, e := c.auditDirectory(base + "/fd")
	if e != nil {
		processObservationIssue(r, base+"/fd", e)
	} else {
		entries, e := fds.ReadDir(auditMaxEntries + 1)
		if e != nil && e != io.EOF {
			auditIssue(r, "collection_failed", base+"/fd", e.Error())
		} else {
			if len(entries) > auditMaxEntries {
				entries = entries[:auditMaxEntries]
				r.Truncated = true
				auditIssue(r, "inspection_limit", base+"/fd", "file descriptor limit reached")
			}
			inodes := []uint64{}
			failures := map[string]int{}
			for _, entry := range entries {
				if ctx.Err() != nil {
					break
				}
				if c.auditBudgetExhausted() {
					processObservationIssue(r, base+"/fd", errAuditLimit)
					break
				}
				target, e := c.auditLink(fds, entry.Name(), base+"/fd/"+entry.Name(), false)
				if e != nil {
					failures[processErrorCode(e)]++
					continue
				}
				if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
					n, e := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]"), 10, 64)
					if e == nil {
						inodes = append(inodes, n)
					}
				}
			}
			sort.Slice(inodes, func(i, j int) bool { return inodes[i] < inodes[j] })
			first["socket_inodes"] = inodes
			for _, code := range []string{"policy_denied", "permission_denied", "source_unavailable", "inspection_limit", "collection_failed"} {
				if count := failures[code]; count > 0 {
					auditIssue(r, code, base+"/fd", fmt.Sprintf("%d descriptor observations unavailable", count))
					if code == "inspection_limit" {
						r.Truncated = true
					}
				}
			}
		}
		fds.Close()
	}
	last, err := c.processStat(a.PID)
	if err != nil {
		processObservationIssue(r, base+"/stat", err)
		auditIssue(r, "identity_unverified", base, "final process identity check unavailable; fields may span a PID reuse")
		first["identity_rechecked"] = false
		r.Data["process"] = first
		return
	}
	if first["start_ticks"] != last["start_ticks"] {
		auditIssue(r, "process_changed", base, "process exited or PID identity changed during observation")
		return
	}
	first["identity_rechecked"] = true
	r.Data["process"] = first
}

// Read the link itself, never its target. Executable paths require target policy
// approval; descriptor observations expose only socket inode numbers to callers.
func (c *Collector) auditLink(dir *os.File, name, path string, executable bool) (string, error) {
	if !c.Policy.Allowed("files", path, true) || !c.Policy.Allowed("files", filepath.Join(dir.Name(), name), true) {
		return "", errors.New("source denied")
	}
	if c.auditBudgetExhausted() {
		return "", errAuditLimit
	}
	size := 4097
	if c.auditRemaining != nil {
		size = min(size, *c.auditRemaining+1)
	}
	b := make([]byte, size)
	n, err := unix.Readlinkat(int(dir.Fd()), name, b)
	if c.auditRemaining != nil && n > 0 {
		*c.auditRemaining = max(0, *c.auditRemaining-n)
	}
	if err != nil {
		return "", err
	}
	if n >= len(b) {
		return "", errAuditLimit
	}
	target := string(b[:n])
	if executable {
		clean := strings.TrimSuffix(target, " (deleted)")
		if !filepath.IsAbs(clean) || !c.Policy.Allowed("files", filepath.Clean(clean), true) {
			return "", errors.New("executable target denied")
		}
	}
	return target, nil
}

func processErrorCode(err error) string {
	if errors.Is(err, errProcessStatMalformed) {
		return "malformed_source"
	}
	return auditErrorCode(err)
}
func processObservationIssue(r *contract.Result, source string, err error) {
	code := processErrorCode(err)
	auditIssue(r, code, source, "process source unavailable under current policy, OS permissions or inspection limits")
	if code == "inspection_limit" {
		r.Truncated = true
	}
}
