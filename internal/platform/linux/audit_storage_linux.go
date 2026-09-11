package linux

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Monska85/hostlens/internal/contract"
)

func (c *Collector) auditStorage(ctx context.Context, r *contract.Result) {
	r.Source = "/proc/partitions,/proc/self/mountinfo,/proc/mdstat"
	r.Data["scope"] = "kernel-visible block devices and diagnostics mount namespace; mount aliases are not separate capacity"
	r.Data["snapshot_consistent"] = false
	if b, ok := c.auditSystemRead(ctx, r, "/proc/partitions"); ok {
		devices := []map[string]any{}
		bad := false
		for line := range strings.SplitSeq(string(b), "\n") {
			if ctx.Err() != nil {
				r.Truncated = true
				break
			}
			f := strings.Fields(line)
			if len(f) == 0 || len(f) == 4 && f[0] == "major" && f[1] == "minor" && f[2] == "#blocks" && f[3] == "name" {
				continue
			}
			if len(f) != 4 || !utf8.ValidString(line) {
				bad = true
				continue
			}
			major, e1 := strconv.ParseUint(f[0], 10, 32)
			minor, e2 := strconv.ParseUint(f[1], 10, 32)
			blocks, e3 := strconv.ParseUint(f[2], 10, 64)
			if e1 != nil || e2 != nil || e3 != nil {
				bad = true
				continue
			}
			if len(devices) >= auditMaxEntries {
				r.Truncated = true
				auditIssue(r, "inspection_limit", "/proc/partitions", "storage record limit reached")
				break
			}
			devices = append(devices, map[string]any{"name": f[3], "major": major, "minor": minor, "blocks_1024": blocks})
		}
		r.Data["devices"] = devices
		if bad {
			auditIssue(r, "malformed_source", "/proc/partitions", "invalid block device records omitted")
		}
	}
	if b, ok := c.auditSystemRead(ctx, r, "/proc/self/mountinfo"); ok {
		mounts := []map[string]any{}
		bad := false
		for line := range strings.SplitSeq(string(b), "\n") {
			if ctx.Err() != nil {
				r.Truncated = true
				break
			}
			if line == "" {
				continue
			}
			left, right, found := strings.Cut(line, " - ")
			f := strings.Fields(left)
			g := strings.Fields(right)
			if !found || len(f) < 6 || len(g) != 3 || !utf8.ValidString(line) {
				bad = true
				continue
			}
			id, e1 := strconv.ParseUint(f[0], 10, 64)
			parent, e2 := strconv.ParseUint(f[1], 10, 64)
			if e1 != nil || e2 != nil {
				bad = true
				continue
			}
			flags := []string{}
			for _, flag := range strings.Split(f[5], ",") {
				switch flag {
				case "ro", "rw", "nosuid", "nodev", "noexec", "relatime", "noatime", "strictatime":
					flags = append(flags, flag)
				}
			}
			// Superblock options and source strings can contain remote credentials.
			if len(mounts) >= auditMaxEntries {
				r.Truncated = true
				auditIssue(r, "inspection_limit", "/proc/self/mountinfo", "storage record limit reached")
				break
			}
			mounts = append(mounts, map[string]any{"mount_id": id, "parent_id": parent, "device": f[2], "mount": decodeMountField(f[4]), "filesystem": g[0], "flags": flags})
		}
		r.Data["mounts"] = mounts
		if bad {
			auditIssue(r, "malformed_source", "/proc/self/mountinfo", "invalid mount records omitted")
		}
	}
	if b, ok := c.auditSystemRead(ctx, r, "/proc/mdstat"); ok {
		arrays := []map[string]any{}
		bad := false
		for line := range strings.SplitSeq(string(b), "\n") {
			if ctx.Err() != nil {
				r.Truncated = true
				break
			}
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || strings.TrimSpace(line) == "" || strings.HasPrefix(line, "Personalities :") || strings.HasPrefix(line, "unused devices:") {
				continue
			}
			f := strings.Fields(line)
			if len(f) < 3 || f[1] != ":" || !strings.HasPrefix(f[0], "md") || (f[2] != "active" && f[2] != "inactive") {
				bad = true
				continue
			}
			if len(arrays) >= auditMaxEntries {
				r.Truncated = true
				auditIssue(r, "inspection_limit", "/proc/mdstat", "storage record limit reached")
				break
			}
			arrays = append(arrays, map[string]any{"name": f[0], "state": f[2]})
		}
		r.Data["software_raid_arrays"] = arrays
		if bad {
			auditIssue(r, "malformed_source", "/proc/mdstat", "unsupported array records omitted")
		}
	}
	auditIssue(r, "evidence_unavailable", "storage", "device health, LVM topology, RAID redundancy/recovery and underlying virtual storage integrity are not established")
}

func decodeMountField(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}
