package linux

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

// Audit grants are additional authority, never an exemption from source policy.
func (c *Collector) collectAudit(ctx context.Context, tool, domain string, a contract.Args, r contract.Result) contract.Result {
	r.Data["coverage_complete"] = true
	if !c.Policy.Allowed("audit", domain, false) {
		auditIssue(&r, "policy_denied", "audit:"+domain, "explicit audit domain grant required")
		r.Error = true
		return r
	}
	if !validAuditArgs(tool, a) || a.Limit > c.Config.Limits.PageSize {
		auditIssue(&r, "invalid_arguments", tool, "invalid or unrelated audit selector")
		r.Error = true
		return r
	}
	remaining := c.Config.Limits.InspectionBytes
	local := *c
	local.auditRemaining = &remaining
	c = &local
	switch tool {
	case "list_processes":
		c.auditProcesses(ctx, &r, a)
	case "get_process_info":
		c.auditProcess(ctx, &r, a)
	case "get_network_info":
		c.auditNetwork(ctx, &r)
	case "list_accounts":
		c.auditAccounts(ctx, &r, a)
	case "get_storage_info":
		c.auditStorage(ctx, &r)
	case "get_update_info":
		c.auditUpdates(ctx, &r)
	case "get_security_info":
		c.auditSecurity(ctx, &r)
	case "get_hostlens_info":
		c.auditHostlens(&r)
	case "inspect_service":
		c.auditService(ctx, &r, a)
	case "inspect_path":
		c.auditPath(&r, a)
	}
	if ctx.Err() != nil {
		auditIssue(&r, "cancelled_or_timeout", tool, "audit collection interrupted")
		r.Error = true
	}
	// Scope and coverage declarations alone are not collected evidence.
	evidence := false
	for k := range r.Data {
		if k != "coverage_complete" && k != "scope" && k != "snapshot_consistent" {
			evidence = true
			break
		}
	}
	if !evidence && len(r.Issues) > 0 {
		r.Error = true
	}
	return r
}

func validAuditArgs(tool string, a contract.Args) bool {
	if a.Format != "" || a.RawTail || a.Since != "" || a.Until != "" || a.Priority != nil {
		return false
	}
	switch tool {
	case "list_processes", "list_accounts":
		return a.PID == 0 && a.Path == "" && a.Unit == "" && a.Offset >= 0 && a.Limit >= 0
	case "get_process_info":
		return a.PID > 0 && a.PID <= 4194304 && a.Path == "" && a.Unit == "" && a.Offset == 0 && a.Limit == 0
	case "inspect_service":
		return policy.ValidLinuxUnit(a.Unit) && a.PID == 0 && a.Path == "" && a.Offset == 0 && a.Limit == 0
	case "inspect_path":
		return filepath.IsAbs(a.Path) && filepath.Clean(a.Path) == a.Path && a.PID == 0 && a.Unit == "" && a.Offset == 0 && a.Limit == 0
	default:
		return a.PID == 0 && a.Path == "" && a.Unit == "" && a.Offset == 0 && a.Limit == 0
	}
}

func auditIssue(r *contract.Result, code, source, message string) {
	r.Data["coverage_complete"] = false
	// Several missing observations may share the same cause. Bound issue metadata.
	for _, i := range r.Issues {
		if i.Code == code && i.Source == source && i.Message == message {
			return
		}
	}
	if len(r.Issues) < 32 {
		r.Issue(code, source, message)
	} else {
		r.Truncated = true
	}
}

func (c *Collector) auditRead(r *contract.Result, path string) ([]byte, bool) {
	if !c.Policy.Allowed("files", path, true) {
		auditIssue(r, "policy_denied", path, "audit source denied")
		return nil, false
	}
	b, err := c.file(path, true)
	if err == nil {
		return b, true
	}
	code := auditErrorCode(err)
	auditIssue(r, code, path, "cannot read audit source under current policy, OS permissions or inspection limits")
	return nil, false
}

func (c *Collector) auditHostlens(r *contract.Result) {
	r.Source = "active backend configuration; process credentials"
	r.Data["scope"] = "active diagnostic backend snapshot; gateway liveness and TLS peer verification require client observation"
	r.Data["version"] = contract.Version
	r.Data["mode"] = c.Config.Mode
	r.Data["privilege"] = c.Config.Privilege
	r.Data["uid"] = os.Getuid()
	r.Data["gid"] = os.Getgid()
	r.Data["policy_fingerprint"] = c.Policy.Fingerprint()
	r.Data["server"] = map[string]any{"bind": c.Config.Server.Bind, "port": c.Config.Server.Port, "tls_enabled": c.Config.Server.TLS.Enabled, "allow_insecure_http": c.Config.Server.AllowInsecureHTTP, "trusted_proxy_count": len(c.Config.Server.TrustedProxies), "allowed_origin_count": len(c.Config.Server.AllowedOrigins)}
	r.Data["limits"] = map[string]any{"tool_timeout_ms": c.Config.Limits.ToolTimeout.Milliseconds(), "max_concurrent_operations": c.Config.Limits.Concurrent, "max_response_bytes": c.Config.Limits.ResponseBytes, "max_inspection_bytes": c.Config.Limits.InspectionBytes, "max_page_size": c.Config.Limits.PageSize}
	r.Data["audit_successful_calls"] = c.Config.Logging.AuditSuccessfulCalls
	domains := map[string]bool{}
	for _, t := range contract.Tools {
		if d := contract.AuditDomain(t); d != "" {
			domains[d] = c.Policy.Allowed("audit", d, false)
		}
	}
	r.Data["audit_domains"] = domains
}

var errAuditLimit = errors.New("aggregate audit inspection limit exceeded")

func (c *Collector) auditBudgetExhausted() bool {
	return c.auditRemaining != nil && *c.auditRemaining <= 0
}

func (c *Collector) readAuditBounded(f *os.File) ([]byte, error) {
	if c.auditRemaining == nil {
		return ReadBounded(f, c.Config.Limits.InspectionBytes)
	}
	limit := *c.auditRemaining
	if limit <= 0 {
		return nil, errAuditLimit
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	*c.auditRemaining = max(0, limit-len(b))
	if len(b) > limit {
		return nil, errAuditLimit
	}
	return b, err
}

func auditErrorCode(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return "permission_denied"
	case errors.Is(err, os.ErrNotExist):
		return "source_unavailable"
	case errors.Is(err, errAuditLimit):
		return "inspection_limit"
	case strings.Contains(err.Error(), "denied"), strings.Contains(err.Error(), "protected"):
		return "policy_denied"
	default:
		return "collection_failed"
	}
}
