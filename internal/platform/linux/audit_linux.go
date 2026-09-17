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
// Each tool's argument struct carries exactly its own selectors, so the
// remaining checks are the policy-dependent bounds the schema cannot know.
func (c *Collector) collectAudit(ctx context.Context, tool, domain string, args any, r contract.Result) contract.Result {
	r.Data = newAuditPayload(tool)
	if !c.Policy.Allowed("audit", domain, false) {
		auditIssue(&r, "policy_denied", "audit:"+domain, "explicit audit domain grant required")
		r.Error = true
		return r
	}
	invalid := func() contract.Result {
		auditIssue(&r, "invalid_arguments", tool, "invalid or unrelated audit selector")
		r.Error = true
		return r
	}
	remaining := c.Config.Limits.InspectionBytes
	local := *c
	local.auditRemaining = &remaining
	c = &local
	var evidence bool
	switch tool {
	case "list_processes":
		a, ok := args.(contract.PageArgs)
		if !ok || a.Offset < 0 || a.Limit < 0 || a.Limit > c.Config.Limits.PageSize {
			return invalid()
		}
		evidence = c.auditProcesses(ctx, &r, a)
	case "get_process_info":
		a, ok := args.(contract.PIDArgs)
		if !ok || a.PID <= 0 || a.PID > 4194304 {
			return invalid()
		}
		evidence = c.auditProcess(ctx, &r, a)
	case "get_network_info":
		if _, ok := args.(contract.NoArgs); !ok {
			return invalid()
		}
		evidence = c.auditNetwork(ctx, &r)
	case "list_accounts":
		a, ok := args.(contract.PageArgs)
		if !ok || a.Offset < 0 || a.Limit < 0 || a.Limit > c.Config.Limits.PageSize {
			return invalid()
		}
		evidence = c.auditAccounts(ctx, &r, a)
	case "get_storage_info":
		if _, ok := args.(contract.NoArgs); !ok {
			return invalid()
		}
		evidence = c.auditStorage(ctx, &r)
	case "get_update_info":
		if _, ok := args.(contract.NoArgs); !ok {
			return invalid()
		}
		evidence = c.auditUpdates(ctx, &r)
	case "get_security_info":
		if _, ok := args.(contract.NoArgs); !ok {
			return invalid()
		}
		evidence = c.auditSecurity(ctx, &r)
	case "get_hostlens_info":
		if _, ok := args.(contract.NoArgs); !ok {
			return invalid()
		}
		evidence = c.auditHostlens(&r)
	case "inspect_service":
		a, ok := args.(contract.UnitArgs)
		if !ok || !policy.ValidLinuxUnit(a.Unit) {
			return invalid()
		}
		evidence = c.auditService(ctx, &r, a)
	case "inspect_path":
		a, ok := args.(contract.PathArgs)
		if !ok || !filepath.IsAbs(a.Path) || filepath.Clean(a.Path) != a.Path {
			return invalid()
		}
		evidence = c.auditPath(&r, a)
	}
	if ctx.Err() != nil {
		auditIssue(&r, "cancelled_or_timeout", tool, "audit collection interrupted")
		r.Error = true
	}
	// Scope and coverage declarations alone are not collected evidence.
	if !evidence && len(r.Issues) > 0 {
		r.Error = true
	}
	return r
}

// newAuditPayload allocates the typed payload for one audit tool with its
// coverage declaration and the collections the output schema requires, so
// denied or invalid calls still produce a schema-valid payload. Sub-collectors
// fill the returned pointer in place.
func newAuditPayload(tool string) any {
	switch tool {
	case "list_processes":
		return &contract.ProcessPage{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}, Items: []contract.ProcessRow{}}
	case "get_process_info":
		return &contract.ProcessInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	case "get_network_info":
		return &contract.NetworkInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	case "list_accounts":
		return &contract.AccountPage{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}, Items: []contract.AccountRow{}}
	case "get_storage_info":
		return &contract.StorageInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	case "get_update_info":
		return &contract.UpdateInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	case "get_security_info":
		return &contract.SecurityInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}, KernelControls: map[string]uint32{}}
	case "get_hostlens_info":
		return &contract.HostlensInfo{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}, Server: contract.HostlensServer{Bind: []string{}}, AuditDomains: map[string]bool{}}
	case "inspect_service":
		return &contract.ServiceInspection{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	case "inspect_path":
		return &contract.PathInspection{AuditCoverage: contract.AuditCoverage{CoverageComplete: true}}
	}
	return nil
}

// auditCoverage records the coverage declaration on whichever typed audit
// payload the result carries; payloads are pointers, so the flip reaches the
// caller's result.
func auditCoverage(r *contract.Result, complete bool) {
	switch p := r.Data.(type) {
	case *contract.ProcessPage:
		p.CoverageComplete = complete
	case *contract.ProcessInfo:
		p.CoverageComplete = complete
	case *contract.NetworkInfo:
		p.CoverageComplete = complete
	case *contract.AccountPage:
		p.CoverageComplete = complete
	case *contract.StorageInfo:
		p.CoverageComplete = complete
	case *contract.UpdateInfo:
		p.CoverageComplete = complete
	case *contract.SecurityInfo:
		p.CoverageComplete = complete
	case *contract.HostlensInfo:
		p.CoverageComplete = complete
	case *contract.ServiceInspection:
		p.CoverageComplete = complete
	case *contract.PathInspection:
		p.CoverageComplete = complete
	}
}

func auditIssue(r *contract.Result, code, source, message string) {
	auditCoverage(r, false)
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

func (c *Collector) auditHostlens(r *contract.Result) bool {
	p := r.Data.(*contract.HostlensInfo)
	r.Source = "active backend configuration; process credentials"
	p.Scope = "active diagnostic backend snapshot; gateway liveness and TLS peer verification require client observation"
	p.Version = contract.Version
	p.Mode = c.Config.Mode
	p.Privilege = c.Config.Privilege
	p.UID = os.Getuid()
	p.GID = os.Getgid()
	p.PolicyFingerprint = c.Policy.Fingerprint()
	bind := c.Config.Server.Bind
	if bind == nil {
		bind = []string{}
	}
	p.Server = contract.HostlensServer{Bind: bind, Port: c.Config.Server.Port, TLSEnabled: c.Config.Server.TLS.Enabled, AllowInsecureHTTP: c.Config.Server.AllowInsecureHTTP, TrustedProxyCount: len(c.Config.Server.TrustedProxies), AllowedOriginCount: len(c.Config.Server.AllowedOrigins)}
	p.Limits = contract.HostlensLimits{ToolTimeoutMS: c.Config.Limits.ToolTimeout.Milliseconds(), MaxConcurrentOperations: c.Config.Limits.Concurrent, MaxResponseBytes: c.Config.Limits.ResponseBytes, MaxInspectionBytes: c.Config.Limits.InspectionBytes, MaxPageSize: c.Config.Limits.PageSize}
	p.AuditSuccessfulCalls = c.Config.Logging.AuditSuccessfulCalls
	domains := map[string]bool{}
	for _, t := range contract.ToolNames() {
		if d := contract.AuditDomain(t); d != "" {
			domains[d] = c.Policy.Allowed("audit", d, false)
		}
	}
	p.AuditDomains = domains
	return true
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
