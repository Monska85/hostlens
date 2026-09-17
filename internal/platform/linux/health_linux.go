package linux

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"golang.org/x/sys/unix"
)

func nums(b []byte) map[string]float64 {
	m := map[string]float64{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 {
			v, e := strconv.ParseFloat(f[1], 64)
			if e == nil {
				m[strings.TrimSuffix(f[0], ":")] = v
			}
		}
	}
	return m
}
func cpu(b []byte) (float64, float64, error) {
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 5 && f[0] == "cpu" {
			total, idle := float64(0), float64(0)
			for i, s := range f[1:] {
				if i >= 8 {
					break
				}
				v, e := strconv.ParseFloat(s, 64)
				if e != nil {
					return 0, 0, e
				}
				total += v
				if i == 3 || i == 4 {
					idle += v
				}
			}
			return total, idle, nil
		}
	}
	return 0, 0, fmt.Errorf("aggregate CPU counters absent")
}
func (c *Collector) health(ctx context.Context, r *contract.Result) {
	r.Source = "Linux procfs, statfs, systemd"
	snap := contract.HealthSnapshot{
		Checks:              map[string]contract.HealthCheck{},
		MissingRequired:     []string{},
		ExcludedFilesystems: c.Config.Health.ExcludeFilesystems,
		Scope:               "current process-visible host resources; no application or historical health claim",
	}
	completed := map[string]bool{}
	severity := "OK"
	rank := map[string]int{"OK": 0, "warning": 1, "critical": 2}
	add := func(name string, value float64, t config.Threshold) {
		status := "OK"
		if value >= t.Critical {
			status = "critical"
		} else if value >= t.Warning {
			status = "warning"
		}
		v, threshold := value, contract.HealthThreshold{Warning: t.Warning, Critical: t.Critical}
		snap.Checks[name] = contract.HealthCheck{Status: status, Value: &v, Threshold: &threshold}
		if rank[status] > rank[severity] {
			severity = status
		}
	}
	if b, e := c.file("/proc/meminfo", true); e == nil {
		m := nums(b)
		total, tok := m["MemTotal"]
		avail, aok := m["MemAvailable"]
		if tok && aok && total > 0 {
			mt, ma := total*1024, avail*1024
			snap.MemoryTotalBytes = &mt
			snap.MemoryAvailableBytes = &ma
			add("memory", 100*(total-avail)/total, c.Config.Health.Usage)
			completed["memory"] = true
		} else {
			r.Issue("missing_measurement", "/proc/meminfo", "MemTotal or MemAvailable absent")
		}
		total, tok = m["SwapTotal"]
		free, fok := m["SwapFree"]
		if tok && fok {
			used := total - free
			st, su := total*1024, used*1024
			snap.SwapTotalBytes = &st
			snap.SwapUsedBytes = &su
			pct := float64(0)
			if total > 0 {
				pct = 100 * used / total
			}
			add("swap", pct, c.Config.Health.Usage)
			completed["swap"] = true
		}
	} else {
		r.Issue("collection_failed", "/proc/meminfo", e.Error())
	}
	var cpus unix.CPUSet
	if e := unix.SchedGetaffinity(0, &cpus); e == nil {
		n := cpus.Count()
		snap.LogicalCPUsAvailable = &n
		snap.CPUAvailabilityScope = "scheduler affinity; cgroup CPU quotas may further restrict capacity"
	}
	if b, e := c.file("/proc/loadavg", true); e == nil {
		f := strings.Fields(string(b))
		if len(f) >= 3 {
			vals := []float64{}
			for _, s := range f[:3] {
				v, e := strconv.ParseFloat(s, 64)
				if e == nil {
					vals = append(vals, v)
				}
			}
			if len(vals) == 3 {
				snap.Load15 = vals
				if n := cpus.Count(); n > 0 {
					add("load", vals[0]/float64(n), c.Config.Health.Load)
					completed["load"] = true
				}
			}
		}
	} else {
		r.Issue("collection_failed", "/proc/loadavg", e.Error())
	}
	filesystems := []contract.HealthFilesystem{}
	fsOK := true
	if b, e := c.file("/proc/self/mounts", true); e == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(string(b), "\n") {
			if ctx.Err() != nil {
				fsOK = false
				break
			}
			f := strings.Fields(line)
			if len(f) < 3 {
				continue
			}
			mount := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(f[1])
			if seen[mount] {
				continue
			}
			skip := false
			for _, x := range c.Config.Health.ExcludeFilesystems {
				if mount == x {
					skip = true
				}
			}
			switch f[2] {
			case "proc", "sysfs", "devtmpfs", "devpts", "cgroup", "cgroup2", "securityfs", "debugfs", "tracefs", "mqueue", "pstore", "hugetlbfs", "configfs", "fusectl", "autofs", "binfmt_misc":
				skip = true
			}
			if skip {
				continue
			}
			// An autofs control entry may precede mounted storage at this path.
			seen[mount] = true
			if !c.Policy.Allowed("files", mount, true) {
				r.Issue("policy_denied", mount, "filesystem observation denied")
				fsOK = false
				continue
			}
			var st unix.Statfs_t
			if e := unix.Statfs(mount, &st); e != nil {
				r.Issue("collection_failed", mount, "statfs failed")
				fsOK = false
				continue
			}
			if st.Blocks == 0 {
				continue
			}
			space := 100 * float64(st.Blocks-st.Bavail) / float64(st.Blocks)
			item := contract.HealthFilesystem{Mount: mount, Type: f[2], TotalBytes: st.Blocks * uint64(st.Bsize), AvailableBytes: st.Bavail * uint64(st.Bsize), UsedPercent: space}
			add("filesystem:"+mount, space, c.Config.Health.Usage)
			if st.Files > 0 {
				inode := 100 * float64(st.Files-st.Ffree) / float64(st.Files)
				item.InodeUsedPercent = &inode
				add("inodes:"+mount, inode, c.Config.Health.Usage)
			}
			filesystems = append(filesystems, item)
		}
	} else {
		fsOK = false
		r.Issue("collection_failed", "/proc/self/mounts", e.Error())
	}
	if len(filesystems) > 0 {
		snap.Filesystems = filesystems
		completed["filesystem"] = fsOK
	}
	if snap.ExcludedFilesystems == nil {
		snap.ExcludedFilesystems = []string{}
	}
	svc := contract.Result{}
	items, ok := c.serviceObservations(ctx, &svc)
	if ok && (len(items) > 0 || len(svc.Issues) == 0) {
		failed := []string{}
		for _, item := range items {
			if item.Active == "failed" {
				failed = append(failed, item.Unit)
			}
		}
		snap.FailedServices = failed
		add("services", float64(len(failed)), c.Config.Health.Services)
		completed["services"] = len(svc.Issues) == 0
	}
	r.Issues = append(r.Issues, svc.Issues...)
	started := time.Now().UTC()
	var first []byte
	e := ctx.Err()
	if e == nil {
		first, e = c.file("/proc/stat", true)
	}
	if e == nil {
		timer := time.NewTimer(c.Config.Health.Sample)
		select {
		case <-ctx.Done():
			timer.Stop()
			e = ctx.Err()
		case <-timer.C:
		}
		if e == nil {
			second, err := c.file("/proc/stat", true)
			e = err
			if e == nil {
				a, ai, ae := cpu(first)
				b, bi, be := cpu(second)
				if ae == nil && be == nil && b > a && bi >= ai {
					used := 100 * (1 - (bi-ai)/(b-a))
					snap.CPUUtilization = &used
					snap.CPUSampleStart = &started
					end := time.Now().UTC()
					snap.CPUSampleEnd = &end
					snap.CPUUtilizationScope = "aggregate procfs CPUs"
					add("cpu", used, c.Config.Health.Usage)
					completed["cpu"] = true
				} else {
					e = fmt.Errorf("invalid or reset CPU sample counters")
				}
			}
		}
	}
	if e != nil {
		r.Issue("collection_failed", "/proc/stat", e.Error())
	}
	complete := true
	missing := []string{}
	for _, name := range c.Config.Health.Required {
		if !completed[name] {
			complete = false
			missing = append(missing, name)
			if _, observed := snap.Checks[name]; !observed {
				snap.Checks[name] = contract.HealthCheck{Status: "unknown"}
			}
		}
	}
	snap.Severity = severity
	snap.Complete = complete
	snap.MissingRequired = missing
	r.Data = snap
}
