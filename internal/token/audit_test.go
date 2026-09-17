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
		if tool == "get_hostlens_info" {
			// Server facts drop to the health role; the hostlens audit grant
			// still gates the tool's availability.
			for _, role := range []string{"health", "inspect", "diagnostics"} {
				if !Allows([]string{role}, tool) {
					t.Fatalf("role %s lost server facts", role)
				}
			}
			continue
		}
		for _, role := range []string{"health", "inspect", "diagnostics"} {
			if Allows([]string{role}, tool) != (role == "diagnostics") {
				t.Fatalf("role %s tool %s", role, tool)
			}
		}
	}
}
