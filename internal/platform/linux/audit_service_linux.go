package linux

import (
	"context"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

var auditServiceProperties = []string{"Id", "LoadState", "ActiveState", "SubState", "UnitFileState", "MainPID", "Result", "ExecMainCode", "ExecMainStatus", "NRestarts", "User", "Group", "DynamicUser", "NoNewPrivileges", "ProtectSystem", "ProtectHome", "PrivateTmp", "PrivateDevices", "ProtectKernelTunables", "ProtectKernelModules", "ProtectControlGroups", "RestrictSUIDSGID", "RestrictRealtime", "RestrictNamespaces", "LockPersonality", "MemoryCurrent", "MemoryMax", "TasksCurrent", "TasksMax", "CPUUsageNSec", "CapabilityBoundingSet", "AmbientCapabilities", "RestrictAddressFamilies", "Requires", "Wants", "After", "Before", "ActiveEnterTimestamp", "InactiveEnterTimestamp", "FragmentPath", "DropInPaths"}

func (c *Collector) auditService(ctx context.Context, r *contract.Result, a contract.UnitArgs) bool {
	p := r.Data.(*contract.ServiceInspection)
	r.Source = "systemd selected properties"
	p.Scope = "selected effective service properties; no complete sandbox score or application health assertion"
	if !c.Policy.Allowed("files", "/run/systemd/system", true) || !c.Policy.Allowed("journal", a.Unit, true) {
		auditIssue(r, "policy_denied", a.Unit, "service source denied")
		return false
	}
	b, err := c.run(ctx, "systemctl", "show", "--no-pager", "--property="+strings.Join(auditServiceProperties, ","), "--", a.Unit)
	if err != nil {
		auditIssue(r, "collection_failed", a.Unit, "selected service query unavailable under current OS permissions and limits")
		return false
	}
	allowed := map[string]bool{}
	for _, prop := range auditServiceProperties {
		allowed[prop] = true
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok || !allowed[k] {
			continue
		}
		if v != "" {
			values[k] = v
		}
	}
	if values["LoadState"] == "not-found" {
		auditIssue(r, "source_unavailable", a.Unit, "service not found")
		return false
	}
	if values["Id"] == "" || values["LoadState"] == "" {
		auditIssue(r, "invalid_observation", a.Unit, "service response lacks identity or load state")
		return false
	}
	if !policy.ValidLinuxUnit(values["Id"]) || !c.Policy.Allowed("journal", values["Id"], true) {
		auditIssue(r, "policy_denied", a.Unit, "canonical service identity denied or invalid")
		return false
	}
	// Unit-file locations are metadata, but remain subject to explicit path denials.
	for _, key := range []string{"FragmentPath", "DropInPaths"} {
		for _, path := range strings.Fields(values[key]) {
			if !c.Policy.Allowed("files", path, true) {
				delete(values, key)
				auditIssue(r, "policy_denied", a.Unit, "unit file location denied")
				break
			}
		}
	}
	for _, key := range []string{"Requires", "Wants", "After", "Before"} {
		names := strings.Fields(values[key])
		kept := make([]string, 0, len(names))
		for _, name := range names {
			if policy.ValidLinuxUnit(name) && c.Policy.Allowed("journal", name, true) {
				kept = append(kept, name)
			} else {
				auditIssue(r, "policy_denied", a.Unit, "some dependency identities are denied or invalid")
			}
		}
		if len(names) > 0 {
			if len(kept) > 0 {
				values[key] = strings.Join(kept, " ")
			} else {
				delete(values, key)
			}
		}
	}
	p.Service = values
	return true
}
