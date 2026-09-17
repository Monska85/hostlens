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

func parseProcessStat(b []byte, pid int) (contract.ProcessDetail, error) {
	s := strings.TrimSpace(string(b))
	begin := strings.IndexByte(s, '(')
	end := strings.LastIndexByte(s, ')')
	if begin < 1 || end <= begin {
		return contract.ProcessDetail{}, errors.New("malformed process stat")
	}
	n, e := strconv.Atoi(strings.TrimSpace(s[:begin]))
	if e != nil || n != pid {
		return contract.ProcessDetail{}, errors.New("process identity changed")
	}
	fields := strings.Fields(s[end+1:])
	if len(fields) < 22 || len(fields[0]) != 1 {
		return contract.ProcessDetail{}, errors.New("short process stat")
	}
	out := contract.ProcessDetail{PID: pid, Name: s[begin+1 : end], State: fields[0]}
	for _, field := range []struct {
		name  string
		index int
	}{{"parent_pid", 1}, {"user_cpu_ticks", 11}, {"system_cpu_ticks", 12}, {"threads", 17}, {"start_ticks", 19}, {"virtual_bytes", 20}, {"resident_pages", 21}} {
		value, e := strconv.ParseUint(fields[field.index], 10, 64)
		if e != nil {
			return contract.ProcessDetail{}, fmt.Errorf("invalid process %s", field.name)
		}
		switch field.name {
		case "parent_pid":
			out.ParentPID = value
		case "user_cpu_ticks":
			out.UserCPUTicks = value
		case "system_cpu_ticks":
			out.SystemCPUTicks = value
		case "threads":
			out.Threads = value
		case "start_ticks":
			out.StartTicks = value
		case "virtual_bytes":
			out.VirtualBytes = value
		case "resident_pages":
			out.ResidentPages = value
		}
	}
	return out, nil
}

func (c *Collector) processStat(pid int) (contract.ProcessDetail, error) {
	b, err := c.file("/proc/"+strconv.Itoa(pid)+"/stat", true)
	if err != nil {
		return contract.ProcessDetail{}, err
	}
	result, err := parseProcessStat(b, pid)
	if err != nil {
		return contract.ProcessDetail{}, fmt.Errorf("%w: %v", errProcessStatMalformed, err)
	}
	return result, nil
}

func (c *Collector) auditProcesses(ctx context.Context, r *contract.Result, a contract.PageArgs) bool {
	p := r.Data.(*contract.ProcessPage)
	r.Source = "procfs"
	p.Scope = "visible PID namespace; processes hidden by procfs permissions may be absent"
	p.SnapshotConsistent = false
	limit := a.Limit
	if limit == 0 {
		limit = c.Config.Limits.PageSize
	}
	if limit < 1 || limit > c.Config.Limits.PageSize || a.Offset < 0 {
		auditIssue(r, "invalid_bounds", "/proc", "page bounds exceed ceiling")
		return false
	}
	dir, err := c.auditDirectory("/proc")
	if err != nil {
		code := auditErrorCode(err)
		auditIssue(r, code, "/proc", "cannot enumerate processes under current policy or OS permissions")
		return false
	}
	defer dir.Close()
	entries, err := dir.ReadDir(auditMaxEntries + 1)
	if err != nil && err != io.EOF {
		auditIssue(r, "collection_failed", "/proc", err.Error())
		return false
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
	rows := []contract.ProcessRow{}
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
		rows = append(rows, contract.ProcessRow{PID: item.PID, Name: item.Name, State: item.State, ParentPID: item.ParentPID, UserCPUTicks: item.UserCPUTicks, SystemCPUTicks: item.SystemCPUTicks, Threads: item.Threads, StartTicks: item.StartTicks, VirtualBytes: item.VirtualBytes, ResidentPages: item.ResidentPages})
	}
	for _, code := range []string{"policy_denied", "permission_denied", "source_unavailable", "malformed_source", "inspection_limit", "collection_failed"} {
		if count := failures[code]; count > 0 {
			auditIssue(r, code, "/proc", fmt.Sprintf("%d selected process observations unavailable", count))
			if code == "inspection_limit" {
				r.Truncated = true
			}
		}
	}
	p.EnumeratedProcesses = len(pids)
	p.ObservedProcesses = len(rows)
	p.Items = rows
	if end < len(pids) {
		r.NextOffset = &end
	}
	return true
}

func (c *Collector) auditProcess(ctx context.Context, r *contract.Result, a contract.PIDArgs) bool {
	p := r.Data.(*contract.ProcessInfo)
	r.Source = "procfs"
	p.Scope = "visible PID namespace"
	p.SnapshotConsistent = false
	base := "/proc/" + strconv.Itoa(a.PID)
	stat, ok := c.auditRead(r, base+"/stat")
	if !ok {
		return false
	}
	first, err := parseProcessStat(stat, a.PID)
	if err != nil {
		auditIssue(r, "malformed_source", base+"/stat", "invalid process identity or resource fields")
		return false
	}
	if ctx.Err() != nil {
		return false
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
				} else if key == "Uid" {
					first.UID = ids
				} else {
					first.GID = ids
				}
			case "NoNewPrivs", "Seccomp", "CapEff":
				if len(fields) == 1 {
					switch key {
					case "NoNewPrivs":
						first.NoNewPrivs = fields[0]
					case "Seccomp":
						first.Seccomp = fields[0]
					case "CapEff":
						first.CapEff = fields[0]
					}
				}
			}
		}
	}

	if first.UID == nil || first.GID == nil {
		auditIssue(r, "partial_observation", base+"/status", "process identity fields unavailable")
	}
	if ctx.Err() != nil {
		return false
	}
	dir, e := c.auditDirectory(base)
	if e == nil {
		target, e := c.auditLink(dir, "exe", base+"/exe", true)
		if e == nil {
			first.Executable = target
		} else {
			processObservationIssue(r, base+"/exe", e)
		}
		dir.Close()
	} else {
		processObservationIssue(r, base, e)
	}
	if ctx.Err() != nil {
		return false
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
			first.SocketInodes = inodes
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
		first.IdentityRechecked = false
		p.Process = &first
		return true
	}
	if first.StartTicks != last.StartTicks {
		auditIssue(r, "process_changed", base, "process exited or PID identity changed during observation")
		return false
	}
	first.IdentityRechecked = true
	p.Process = &first
	return true
}

// Read the link itself, never its target. Executable target paths are
// default-allowed unless denied by policy; only the path string is exposed,
// never link contents. Descriptor observations expose only socket inode
// numbers to callers.
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
