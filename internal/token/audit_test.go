package token

import (
	"github.com/Monska85/hostlens/internal/contract"
	"testing"
)

func TestAuditToolsRequireDiagnosticsRole(t *testing.T) {
	for _, tool := range contract.ToolNames() {
		if contract.AuditDomain(tool) == "" {
			continue
		}
		for _, role := range []string{"health", "inspect", "diagnostics"} {
			if Allows([]string{role}, tool) != (role == "diagnostics") {
				t.Fatalf("role %s tool %s", role, tool)
			}
		}
	}
}
