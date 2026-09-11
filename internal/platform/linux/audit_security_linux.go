package linux

import (
	"context"
	"strconv"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
)

func (c *Collector) auditSecurity(ctx context.Context, r *contract.Result) {
	r.Source = "procfs,securityfs,sysfs"
	r.Data["scope"] = "selected kernel controls visible to diagnostics; namespace and sandbox restrictions apply"
	controls := map[string]any{}
	for _, path := range []string{"/proc/sys/kernel/randomize_va_space", "/proc/sys/kernel/kptr_restrict", "/proc/sys/kernel/dmesg_restrict", "/proc/sys/kernel/unprivileged_bpf_disabled", "/proc/sys/kernel/modules_disabled", "/proc/sys/kernel/yama/ptrace_scope", "/proc/sys/fs/protected_hardlinks", "/proc/sys/fs/protected_symlinks", "/proc/sys/fs/protected_fifos", "/proc/sys/fs/protected_regular"} {
		if !c.auditSystemContinue(ctx, r, path) {
			break
		}
		b, ok := c.auditRead(r, path)
		if !ok {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 32)
		if err != nil {
			auditIssue(r, "malformed_source", path, "expected unsigned kernel control value")
			continue
		}
		controls[path] = n
	}
	r.Data["kernel_controls"] = controls
	if b, ok := c.auditSystemRead(ctx, r, "/sys/kernel/security/lsm"); ok {
		value := strings.TrimSpace(string(b))
		valid := value != ""
		for _, ch := range value {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '_' || ch == ',' || ch == '-') {
				valid = false
			}
		}
		if valid {
			r.Data["active_lsm"] = strings.Split(value, ",")
		} else {
			auditIssue(r, "malformed_source", "/sys/kernel/security/lsm", "invalid security module list")
		}
	}
	if len(controls) == 0 && r.Data["active_lsm"] == nil {
		r.Error = true
	}
	auditIssue(r, "evidence_unavailable", "security", "these controls do not establish workload confinement, effective audit rules, secure boot or overall security compliance")
}
