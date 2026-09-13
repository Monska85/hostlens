package linux

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}
type Commands struct {
	Limit int
	Root  string
}
type limitedBuffer struct {
	bytes.Buffer
	n        int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.n {
		remaining := b.n - b.Len()
		if remaining > 0 {
			b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
func commandPath(root, name string) (string, error) {
	allowed := map[string]bool{"systemctl": true, "journalctl": true, "dpkg-query": true, "pacman": true}
	if !allowed[name] {
		return "", errors.New("unsupported executable")
	}
	var exe string
	for _, dir := range []string{"/usr/bin", "/bin"} {
		p := filepath.Join(root, dir, name)
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			exe = p
			break
		}
	}
	if exe == "" {
		return "", errors.New("collector unavailable")
	}
	return exe, nil
}
func (r Commands) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	exe, err := commandPath(r.Root, name)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "SYSTEMD_PAGER=cat", "SYSTEMD_COLORS=0"}
	cmd.WaitDelay = time.Second
	b := &limitedBuffer{n: r.Limit}
	cmd.Stdout = b
	cmd.Stderr = io.Discard
	e := cmd.Run()
	if b.exceeded {
		return nil, errors.New("inspection limit exceeded")
	}
	return b.Bytes(), e
}

type Collector struct {
	auditRemaining *int
	Config         config.Config
	Policy         *policy.Policy
	Root           string
	Runner         Runner
	Docker         DockerObserver
}

func (c *Collector) file(path string, builtin bool) ([]byte, error) {
	if c.auditBudgetExhausted() {
		return nil, errAuditLimit
	}
	f, e := openObservation(c.Root, path, c.Policy, builtin)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	if c.auditRemaining != nil {
		return c.readAuditBounded(f)
	}
	return ReadBounded(f, c.Config.Limits.InspectionBytes)
}
func (c *Collector) Capabilities(ctx context.Context) map[string]bool {
	if ctx.Err() != nil {
		return nil
	}
	m := map[string]bool{}
	for _, t := range contract.ToolNames() {
		if _, dockerTool := dockerToolKinds[t]; dockerTool {
			// Docker tool availability is decided exclusively by the
			// observer, engine, and grant state below; the preset must not
			// leak Docker tools on disabled or ungranted installations.
			continue
		}
		m[t] = true
		if domain := contract.AuditDomain(t); domain != "" {
			m[t] = c.Policy.Allowed("audit", domain, false)
		}
	}
	_, e := os.Stat(filepath.Join(c.Root, "/run/systemd/system"))
	if ctx.Err() != nil {
		return nil
	}
	_, commandErr := commandPath(c.Root, "systemctl")
	svc := e == nil && commandErr == nil
	m["list_services"] = svc
	m["get_service_status"] = svc
	m["inspect_service"] = svc && m["inspect_service"]
	if ctx.Err() != nil {
		return nil
	}
	_, _, _, packageErr := c.packageCommand()
	m["list_packages"] = packageErr == nil
	for tool, value := range c.dockerCapabilities(ctx) {
		m[tool] = value
	}
	return m
}
func (c *Collector) result() contract.Result {
	r := contract.Result{ObservedAt: time.Now().UTC(), Data: map[string]any{}}
	if b, e := c.file("/proc/sys/kernel/hostname", true); e == nil {
		r.Host = strings.TrimSpace(string(b))
	}
	return r
}
func (c *Collector) Collect(ctx context.Context, tool string, a contract.Args) contract.Result {
	if ctx.Err() != nil {
		r := contract.Failure("cancelled_or_timeout")
		r.Truncated = true
		return r
	}
	if _, dockerTool := dockerToolKinds[tool]; dockerTool {
		return c.docker(ctx, tool, a)
	}
	r := c.result()
	if domain := contract.AuditDomain(tool); domain != "" {
		return c.collectAudit(ctx, tool, domain, a, r)
	}
	switch tool {
	case "get_os_info":
		r.Data = c.osInfo(&r)
	case "get_inventory":
		r.Data["os"] = c.osInfo(&r)
		for key, p := range map[string]string{"machine_id": "/etc/machine-id", "product_name": "/sys/class/dmi/id/product_name", "system_vendor": "/sys/class/dmi/id/sys_vendor"} {
			if ctx.Err() != nil {
				break
			}
			if b, e := c.file(p, true); e == nil && strings.TrimSpace(string(b)) != "" {
				r.Data[key] = strings.TrimSpace(string(b))
			}
		}
		r.Data["scope"] = "process mount and PID namespaces"
	case "read_config":
		c.readConfig(&r, a)
	case "query_logs":
		c.logs(ctx, &r, a)
	case "list_services", "get_service_status":
		c.services(ctx, &r, tool, a)
	case "list_packages":
		c.packages(ctx, &r, a)
	case "get_health_snapshot":
		c.health(ctx, &r)
	default:
		return contract.Failure("unsupported_operation")
	}
	if len(r.Issues) > 0 {
		switch tool {
		case "read_config":
			_, ok := r.Data["content"]
			r.Error = !ok
		case "query_logs":
			_, entries := r.Data["entries"]
			_, lines := r.Data["lines"]
			r.Error = !entries && !lines
		case "list_services", "list_packages":
			_, ok := r.Data["items"]
			r.Error = !ok
		case "get_service_status":
			r.Error = len(r.Data) == 0 || r.Data["LoadState"] == "not-found"
		}
	}
	if e := ctx.Err(); e != nil {
		r.Error = true
		r.Issue("cancelled_or_timeout", "", e.Error())
	}
	return r
}
func (c *Collector) osInfo(r *contract.Result) map[string]any {
	out := map[string]any{"family": "linux", "architecture": runtime.GOARCH}
	r.Source = "os-release; kernel interfaces"
	b, e := c.file("/etc/os-release", true)
	if e != nil {
		r.Issue("collection_failed", "/etc/os-release", e.Error())
	} else {
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			v = strings.Trim(v, "\"'")
			key := map[string]string{"ID": "distribution", "NAME": "product", "VERSION_ID": "version", "VERSION_CODENAME": "codename"}[k]
			if key != "" && v != "" {
				out[key] = v
			}
		}
	}
	if b, e := c.file("/proc/sys/kernel/osrelease", true); e == nil {
		out["kernel"] = strings.TrimSpace(string(b))
	} else {
		r.Issue("collection_failed", "kernel", e.Error())
	}
	return out
}
func (c *Collector) readConfig(r *contract.Result, a contract.Args) {
	f, e := OpenRegular(a.Path, c.Policy)
	if e != nil {
		r.Issue("source_denied_or_unavailable", a.Path, e.Error())
		return
	}
	defer f.Close()
	b, e := ReadBounded(f, c.Config.Limits.ConfigBytes)
	if e != nil {
		r.Issue("size_or_read_failure", a.Path, e.Error())
		return
	}
	if !utf8.Valid(b) {
		r.Issue("unsupported_encoding", a.Path, "UTF-8 required")
		return
	}
	r.Source = a.Path
	r.Data["content"] = string(b)
}
func page(r *contract.Result, items []map[string]any, a contract.Args, maxPage int) {
	limit := a.Limit
	if limit == 0 {
		limit = maxPage
	}
	if limit < 1 || limit > maxPage || a.Offset < 0 {
		r.Issue("invalid_bounds", "", "page bounds exceed ceiling")
		return
	}
	start := min(a.Offset, len(items))
	end := min(start+limit, len(items))
	r.Data["items"] = items[start:end]
	r.Data["snapshot_consistent"] = false
	if end < len(items) {
		r.NextOffset = &end
	}
}
func (c *Collector) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.Runner != nil {
		return c.Runner.Run(ctx, name, args...)
	}
	return Commands{Limit: c.Config.Limits.InspectionBytes, Root: c.Root}.Run(ctx, name, args...)
}
func (c *Collector) services(ctx context.Context, r *contract.Result, tool string, a contract.Args) {
	r.Source = "systemd"
	if tool == "get_service_status" {
		if !c.Policy.Allowed("files", "/run/systemd/system", true) {
			r.Issue("policy_denied", r.Source, "systemd collector source denied")
			return
		}
		if !policy.ValidLinuxUnit(a.Unit) {
			r.Issue("invalid_unit", "", "concrete systemd unit required")
			return
		}
		if !c.Policy.Allowed("journal", a.Unit, true) {
			r.Issue("policy_denied", a.Unit, "unit explicitly denied")
			return
		}
		b, e := c.run(ctx, "systemctl", "show", "--no-pager", "--property=Id,LoadState,ActiveState,SubState,UnitFileState,MainPID,Result", "--", a.Unit)
		if e != nil {
			r.Issue("collection_failed", r.Source, "service query failed")
			return
		}
		for _, line := range strings.Split(string(b), "\n") {
			k, v, ok := strings.Cut(line, "=")
			if ok && v != "" && strings.Contains("|Id|LoadState|ActiveState|SubState|UnitFileState|MainPID|Result|", "|"+k+"|") {
				r.Data[k] = v
			}
		}
		if r.Data["LoadState"] == "not-found" {
			r.Issue("not_found", a.Unit, "unit not found")
		}
		return
	}
	items := c.serviceObservations(ctx, r)
	if items != nil {
		page(r, items, a, c.Config.Limits.PageSize)
	}
}

func (c *Collector) serviceObservations(ctx context.Context, r *contract.Result) []map[string]any {
	if !c.Policy.Allowed("files", "/run/systemd/system", true) {
		r.Issue("policy_denied", "systemd", "systemd collector source denied")
		return nil
	}
	b, e := c.run(ctx, "systemctl", "list-units", "--type=service", "--all", "--no-pager", "--plain", "--no-legend")
	if e != nil {
		r.Issue("collection_failed", "systemd", "service list failed")
		return nil
	}
	items := []map[string]any{}
	invalid := 0
	for _, line := range strings.Split(string(b), "\n") {
		if ctx.Err() != nil {
			r.Issue("cancelled_or_timeout", "systemd", ctx.Err().Error())
			break
		}
		f := strings.Fields(line)
		if len(f) >= 4 && policy.ValidLinuxUnit(f[0]) {
			if !c.Policy.Allowed("journal", f[0], true) {
				r.Issue("policy_denied", f[0], "unit omitted by explicit source denial")
				continue
			}
			items = append(items, map[string]any{"unit": f[0], "load": f[1], "active": f[2], "sub": f[3]})
		} else if len(f) != 0 {
			invalid++
		}
	}
	if invalid > 0 {
		r.Issue("invalid_service_record", "systemd", fmt.Sprintf("%d malformed service records omitted", invalid))
	}
	return items
}
func (c *Collector) packageCommand() (string, string, []string, error) {
	for _, candidate := range []struct {
		name, source string
		args         []string
	}{
		{"dpkg-query", "/var/lib/dpkg/status", []string{"-W", "-f=${db:Status-Abbrev}\t${Package}\t${Version}\n"}},
		{"pacman", "/var/lib/pacman/local", []string{"-Q"}},
	} {
		if _, err := os.Stat(filepath.Join(c.Root, candidate.source)); err != nil {
			continue
		}
		if _, err := commandPath(c.Root, candidate.name); err != nil {
			continue
		}
		return candidate.name, candidate.source, candidate.args, nil
	}
	return "", "", nil, errors.New("package collector unavailable")
}
func (c *Collector) packages(ctx context.Context, r *contract.Result, a contract.Args) {
	name, source, args, err := c.packageCommand()
	if err != nil {
		r.Issue("collection_failed", "packages", err.Error())
		return
	}
	r.Source = name
	if !c.Policy.Allowed("files", source, true) {
		r.Issue("policy_denied", source, "package source denied")
		return
	}
	b, e := c.run(ctx, name, args...)
	if e != nil {
		r.Issue("collection_failed", source, "package collection failed")
		return
	}
	items := []map[string]any{}
	for _, line := range strings.Split(string(b), "\n") {
		if ctx.Err() != nil {
			r.Issue("cancelled_or_timeout", r.Source, ctx.Err().Error())
			return
		}
		f := strings.Fields(line)
		if name == "dpkg-query" {
			// The first status column is the desired action; the second is
			// the installed state, including held or removal-selected packages.
			if len(f) != 3 || len(f[0]) < 2 || len(f[0]) > 3 || f[0][1] != 'i' {
				continue
			}
			f = f[1:]
		}
		if len(f) == 2 {
			items = append(items, map[string]any{"name": f[0], "version": f[1]})
		}
	}
	page(r, items, a, c.Config.Limits.PageSize)
}
func window(a contract.Args, l config.Limits) (time.Time, time.Time, error) {
	until := time.Now().UTC()
	var e error
	if a.Until != "" {
		until, e = time.Parse(time.RFC3339Nano, a.Until)
		if e != nil {
			return time.Time{}, time.Time{}, e
		}
	}
	since := until.Add(-l.DefaultLogWindow)
	if a.Since != "" {
		since, e = time.Parse(time.RFC3339Nano, a.Since)
		if e != nil {
			return since, until, e
		}
	}
	if until.Before(since) || until.Sub(since) > l.MaxLogWindow {
		return since, until, errors.New("invalid log window")
	}
	return since, until, nil
}
func (c *Collector) logs(ctx context.Context, r *contract.Result, a contract.Args) {
	l := c.Config.Limits
	n := a.Limit
	if n == 0 {
		n = l.LogEntries
	}
	if n < 1 || n > l.LogEntries || a.Offset != 0 || (a.Path == "") == (a.Unit == "") {
		r.Issue("invalid_bounds", "", "select one source within entry ceiling")
		return
	}
	if a.Priority != nil && (*a.Priority < 0 || *a.Priority > 7) {
		r.Issue("unsupported_filter", "", "priority must be 0 through 7")
		return
	}
	since, until, e := window(a, l)
	if e != nil {
		r.Issue("invalid_window", "", e.Error())
		return
	}
	r.Data["requested_since"] = since
	r.Data["requested_until"] = until
	r.Data["coverage_complete"] = false
	if a.Unit != "" {
		c.journal(ctx, r, a, n, since, until)
		return
	}
	r.Source = a.Path
	f, e := OpenRegular(a.Path, c.Policy)
	if e != nil {
		r.Issue("source_denied_or_unavailable", a.Path, e.Error())
		return
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		r.Issue("collection_failed", a.Path, e.Error())
		return
	}
	r.Data["rotation_scope"] = "selected open file only"
	r.Data["ordering"] = "physical file order"
	r.Data["parser"] = a.Format
	if a.RawTail {
		if a.Priority != nil || a.Since != "" || a.Until != "" || (a.Format != "" && a.Format != "raw") {
			r.Issue("unsupported_filter", a.Path, "raw tail has no verified timestamp or severity")
			return
		}
		b, truncated, e := readTailWindow(f, st.Size(), l.InspectionBytes)
		r.Truncated = truncated
		if e != nil {
			r.Issue("collection_failed", a.Path, e.Error())
			return
		}
		if r.Truncated && len(b) > 0 {
			if b[0] == '\n' {
				b = b[1:]
			} else if newline := bytes.IndexByte(b, '\n'); newline >= 0 {
				b = b[newline+1:]
			} else {
				b = nil
			}
		}
		if !utf8.Valid(b) {
			r.Issue("unsupported_encoding", a.Path, "raw tail requires UTF-8")
			return
		}
		lines := []string{}
		if len(b) > 0 {
			lines = strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
		}
		if len(lines) > n {
			lines = lines[len(lines)-n:]
			r.Truncated = true
		}
		r.Data["lines"] = lines
		r.Issue("unverified_time_coverage", a.Path, "raw tail does not establish event times")
		return
	}
	if a.Format != "jsonl" {
		r.Issue("unsupported_parser", a.Path, "time queries require explicit jsonl parser")
		return
	}
	b, e := ReadBounded(f, l.InspectionBytes)
	if e != nil {
		r.Truncated = true
		r.Issue("inspection_limit", a.Path, e.Error())
		return
	}
	entries := []map[string]any{}
	counts := map[string]int{}
	for line := range strings.SplitSeq(string(b), "\n") {
		if ctx.Err() != nil {
			r.Truncated = true
			break
		}
		if line == "" {
			continue
		}
		var row struct {
			Timestamp string  `json:"timestamp"`
			Message   *string `json:"message"`
			Priority  *int    `json:"priority"`
		}
		if !utf8.ValidString(line) || json.Unmarshal([]byte(line), &row) != nil || row.Message == nil {
			counts["malformed JSON line"]++
			continue
		}
		t, e := time.Parse(time.RFC3339Nano, row.Timestamp)
		if e != nil {
			counts["missing or invalid RFC3339 timestamp"]++
			continue
		}
		if a.Priority != nil && (row.Priority == nil || *row.Priority < 0 || *row.Priority > 7) {
			counts["record lacks valid native priority"]++
			continue
		}
		if t.Before(since) || t.After(until) || (a.Priority != nil && *row.Priority > *a.Priority) {
			continue
		}
		if len(entries) >= n {
			r.Truncated = true
			continue
		}
		entry := map[string]any{"timestamp": t, "message": *row.Message}
		if row.Priority != nil && *row.Priority >= 0 && *row.Priority <= 7 {
			entry["priority"] = *row.Priority
		}
		entries = append(entries, entry)
	}
	for _, message := range []string{"malformed JSON line", "missing or invalid RFC3339 timestamp", "record lacks valid native priority"} {
		if count := counts[message]; count > 0 {
			code := "unresolved_window"
			if message == "record lacks valid native priority" {
				code = "unsupported_filter"
			}
			r.Issue(code, a.Path, fmt.Sprintf("%s (%d records)", message, count))
		}
	}
	r.Data["entries"] = entries
	returnedWindow(r, entries)
	r.Issue("retention_unverified", a.Path, "selected file cannot establish full rotation or retention coverage")
}

func readTailWindow(f *os.File, observedEnd int64, limit int) ([]byte, bool, error) {
	truncated := observedEnd > int64(limit)
	start := int64(0)
	if truncated {
		start = observedEnd - int64(limit) - 1 // Include the preceding record-boundary byte.
	}
	b, err := io.ReadAll(io.NewSectionReader(f, start, observedEnd-start))
	if err == nil && int64(len(b)) != observedEnd-start {
		err = errors.New("log shrank during tail read")
	}
	return b, truncated, err
}
func (c *Collector) journal(ctx context.Context, r *contract.Result, a contract.Args, n int, since, until time.Time) {
	r.Source = "journal:" + a.Unit
	if !policy.ValidLinuxUnit(a.Unit) || !c.Policy.Allowed("journal", a.Unit, false) {
		r.Issue("policy_denied", r.Source, "approved concrete journal unit required")
		return
	}
	if a.RawTail || a.Format != "" {
		r.Issue("invalid_filter", r.Source, "file format options do not apply to journal")
		return
	}
	args := []string{"--no-pager", "--output=json", "--unit=" + a.Unit, "--since=" + since.Format(time.RFC3339Nano), "--until=" + until.Format(time.RFC3339Nano), "--lines=" + strconv.Itoa(n+1)}
	if a.Priority != nil {
		args = append(args, "--priority=0.."+strconv.Itoa(*a.Priority))
	}
	b, e := c.run(ctx, "journalctl", args...)
	if e != nil {
		r.Issue("collection_failed", r.Source, "journal unavailable or inspection limit exceeded")
		return
	}
	entries := []map[string]any{}
	invalid := 0
	for _, line := range strings.Split(string(b), "\n") {
		if ctx.Err() != nil {
			r.Truncated = true
			break
		}
		if line == "" {
			continue
		}
		var row struct {
			Timestamp string  `json:"__REALTIME_TIMESTAMP"`
			Message   *string `json:"MESSAGE"`
			Priority  string  `json:"PRIORITY"`
			Unit      string  `json:"_SYSTEMD_UNIT"`
		}
		if !utf8.ValidString(line) || json.Unmarshal([]byte(line), &row) != nil || row.Message == nil {
			invalid++
			continue
		}
		if row.Unit != "" && row.Unit != a.Unit {
			invalid++
			continue
		}
		micros, e := strconv.ParseInt(row.Timestamp, 10, 64)
		if e != nil || micros < 0 {
			invalid++
			continue
		}
		timestamp := time.UnixMicro(micros)
		if timestamp.Before(since) || timestamp.After(until) {
			invalid++
			continue
		}
		entry := map[string]any{"timestamp": timestamp, "message": *row.Message}
		if p, e := strconv.Atoi(row.Priority); e == nil && p >= 0 && p <= 7 {
			if a.Priority != nil && p > *a.Priority {
				invalid++
				continue
			}
			entry["priority"] = p
		} else if row.Priority != "" || a.Priority != nil {
			invalid++
			continue
		}
		entries = append(entries, entry)
	}
	if invalid > 0 {
		r.Issue("invalid_journal_record", r.Source, fmt.Sprintf("%d records had unsupported fields or did not match the requested source/window", invalid))
	}
	if len(entries) > n {
		entries = entries[len(entries)-n:]
		r.Truncated = true
	}
	r.Data["ordering"] = "oldest to newest in selected journal tail"
	r.Data["entries"] = entries
	returnedWindow(r, entries)
	r.Data["rotation_scope"] = "accessible journal files for selected unit"
	r.Issue("retention_unverified", r.Source, "accessible journal may omit rotated or inaccessible history")
}

func returnedWindow(r *contract.Result, entries []map[string]any) {
	var first, last time.Time
	for _, entry := range entries {
		t, ok := entry["timestamp"].(time.Time)
		if !ok {
			continue
		}
		if first.IsZero() || t.Before(first) {
			first = t
		}
		if last.IsZero() || t.After(last) {
			last = t
		}
	}
	if !first.IsZero() {
		r.Data["earliest_returned_event"] = first
		r.Data["latest_returned_event"] = last
	}
}
