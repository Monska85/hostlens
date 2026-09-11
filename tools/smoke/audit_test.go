package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/contract"
)

func TestAuditEvidenceRequiresFixtureFacts(t *testing.T) {
	for _, tc := range []struct{ tool, data string }{
		{"list_accounts", `{"items":[{"kind":"account","name":"root","uid":0}]}`},
		{"get_storage_info", `{"mounts":[{"mount":"/"}]}`},
		{"get_security_info", `{"kernel_controls":{"/proc/sys/kernel/randomize_va_space":0}}`},
		{"get_hostlens_info", `{"mode":"system","privilege":"restricted","audit_domains":{"processes":true,"network":true,"accounts":true,"storage":true,"security":true,"hostlens":true,"paths":true}}`},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			var r contract.Result
			if err := json.Unmarshal([]byte(`{"host":"fixture","observed_at":"2026-09-12T00:00:00Z","data":`+tc.data+`}`), &r); err != nil {
				t.Fatal(err)
			}
			if err := checkAuditEvidence(tc.tool, r); err != nil {
				t.Fatal(err)
			}
			r.Data = map[string]any{"coverage_complete": true, "scope": "fixture"}
			if checkAuditEvidence(tc.tool, r) == nil {
				t.Fatal("attribution without evidence passed")
			}
		})
	}
	for _, tc := range []struct{ tool, data string }{
		{"list_accounts", `{"items":[{"kind":"group","name":"root","uid":0}]}`},
		{"list_accounts", `{"items":[{"kind":"account","name":"root","uid":"0"}]}`},
		{"get_storage_info", `{"mounts":[{"mount":"/data"}]}`},
		{"get_security_info", `{"kernel_controls":{"/proc/sys/kernel/randomize_va_space":3}}`},
		{"get_hostlens_info", `{"mode":"system","privilege":"restricted","audit_domains":{}}`},
	} {
		var r contract.Result
		if err := json.Unmarshal([]byte(`{"data":`+tc.data+`}`), &r); err != nil {
			t.Fatal(err)
		}
		if checkAuditEvidence(tc.tool, r) == nil {
			t.Fatalf("accepted wrong fixture evidence for %s", tc.tool)
		}
	}
}

func TestAuditCoverageReportsOnlyKnownCategories(t *testing.T) {
	var r contract.Result
	if err := json.Unmarshal([]byte(`{"issues":[{"code":"permission_denied","source":"secret","message":"secret"},{"code":"permission_denied"},{"code":"secret"}]}`), &r); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(auditCoverageCodes(r), ","); got != "other,permission_denied" {
		t.Fatalf("unsafe or duplicated categories: %q", got)
	}
}
