package linux

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func auditSystemFixture(t *testing.T, sources map[string]string) *Collector {
	t.Helper()
	c, root := fixture(t)
	c.Root = root
	for path, content := range sources {
		name := filepath.Join(root, path)
		fixtureOK(t, os.MkdirAll(filepath.Dir(name), 0755))
		fixtureOK(t, os.WriteFile(name, []byte(content), 0600))
	}
	return c
}
func auditSystemResult(tool string) contract.Result {
	return contract.Result{ObservedAt: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), Data: newAuditPayload(tool)}
}
func auditHasIssue(r contract.Result, code string) bool {
	for _, issue := range r.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
func TestAuditAccountsExcludeSecretsAndPageGroups(t *testing.T) {
	t.Parallel()

	c := auditSystemFixture(t, map[string]string{"/etc/passwd": "root:HASH_SECRET:0:0:PRIVATE_COMMENT:/root:/bin/sh\nbad:x:no:0::/:/bin/sh\npostgres:x:999:999::/var/lib/postgresql:/bin/sh\n", "/etc/group": "database:GROUP_HASH:999:postgres,operator\n"})
	r := auditSystemResult("list_accounts")
	c.auditAccounts(context.Background(), &r, contract.PageArgs{Limit: 1})
	if r.NextOffset == nil || *r.NextOffset != 1 || !auditHasIssue(r, "malformed_source") {
		t.Fatal(r)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "HASH") || strings.Contains(string(b), "PRIVATE_COMMENT") {
		t.Fatal("sensitive fields returned")
	}
	r = auditSystemResult("list_accounts")
	c.auditAccounts(context.Background(), &r, contract.PageArgs{Offset: 2, Limit: 1})
	items := r.Data.(*contract.AccountPage).Items
	if len(items) != 1 || items[0].Kind != "group" || len(items[0].Members) != 2 {
		t.Fatal(r)
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/etc/passwd", Deny: true})
	r = auditSystemResult("list_accounts")
	c.auditAccounts(context.Background(), &r, contract.PageArgs{Limit: 10})
	if !auditHasIssue(r, "policy_denied") || len(r.Data.(*contract.AccountPage).Items) != 1 {
		t.Fatal(r)
	}
}
func TestAuditStorageOmitsMountCredentialsAndMarksMalformed(t *testing.T) {
	t.Parallel()

	c := auditSystemFixture(t, map[string]string{"/proc/partitions": "major minor  #blocks  name\n8 0 100 sda\n8 1 INVALID sda1\n", "/proc/self/mountinfo": "24 1 8:0 / /data\\040set rw,nosuid,nodev - cifs //user:SECRET@host/share rw,password=SECRET\nbroken\n", "/proc/mdstat": "Personalities : [raid1]\nmd0 : active raid1 sda[0]\n  100 blocks [2/1] [U_]\nunused devices: <none>\n"})
	r := auditSystemResult("get_storage_info")
	c.auditStorage(context.Background(), &r)
	storage, ok := r.Data.(*contract.StorageInfo)
	if !ok || !auditHasIssue(r, "malformed_source") || storage.CoverageComplete {
		t.Fatal(r)
	}
	if len(storage.Mounts) != 1 || storage.Mounts[0].Mount != "/data set" {
		t.Fatal(r)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "SECRET") {
		t.Fatal("mount secrets returned")
	}
}
func TestAuditUpdatesExpiredMetadataAndPolicy(t *testing.T) {
	t.Parallel()

	c := auditSystemFixture(t, map[string]string{"/var/lib/apt/lists/example_InRelease": "Date: Thu, 10 Sep 2026 00:00:00 UTC\nValid-Until: Fri, 11 Sep 2026 00:00:00 UTC\nOrigin: SECRET\n"})
	r := auditSystemResult("get_update_info")
	c.auditUpdates(context.Background(), &r)
	if !auditHasIssue(r, "stale_metadata") || len(r.Data.(*contract.UpdateInfo).RepositoryMetadata) != 1 {
		t.Fatal(r)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "SECRET") {
		t.Fatal("unselected metadata returned")
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/var/lib/apt/lists/example_InRelease", Deny: true})
	r = auditSystemResult("get_update_info")
	c.auditUpdates(context.Background(), &r)
	if !auditHasIssue(r, "policy_denied") || len(r.Data.(*contract.UpdateInfo).RepositoryMetadata) != 0 {
		t.Fatal(r)
	}
}
func TestAuditSecurityMalformedAndMissingControls(t *testing.T) {
	t.Parallel()

	c := auditSystemFixture(t, map[string]string{"/proc/sys/kernel/randomize_va_space": "2\n", "/proc/sys/kernel/kptr_restrict": "invalid\n", "/sys/kernel/security/lsm": "lockdown,capability,landlock\n"})
	r := auditSystemResult("get_security_info")
	c.auditSecurity(context.Background(), &r)
	controls := r.Data.(*contract.SecurityInfo).KernelControls
	if controls["/proc/sys/kernel/randomize_va_space"] != 2 || !auditHasIssue(r, "malformed_source") || !auditHasIssue(r, "source_unavailable") {
		t.Fatal(r)
	}
	if _, ok := controls["/proc/sys/kernel/kptr_restrict"]; ok {
		t.Fatal("malformed control value recorded")
	}
}

func TestAuditSystemUnavailableVersusPartialEvidence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		source, content string
		tool            string
		collect         func(*Collector, *contract.Result)
	}{
		{"accounts", "/etc/passwd", "root:x:0:0::/root:/bin/sh\n", "list_accounts", func(c *Collector, r *contract.Result) {
			c.auditAccounts(context.Background(), r, contract.PageArgs{Limit: 10})
		}},
		{"updates", "/var/lib/apt/lists/example_InRelease", "Date: Thu, 10 Sep 2026 00:00:00 UTC\n", "get_update_info", func(c *Collector, r *contract.Result) { c.auditUpdates(context.Background(), r) }},
		{"security", "/proc/sys/kernel/randomize_va_space", "2\n", "get_security_info", func(c *Collector, r *contract.Result) { c.auditSecurity(context.Background(), r) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := auditSystemFixture(t, map[string]string{tc.source: tc.content})
			r := auditSystemResult(tc.tool)
			tc.collect(c, &r)
			if r.Error || auditCoverageOf(r) {
				t.Fatalf("partial evidence rejected: %+v", r)
			}
			c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/**", Deny: true})
			r = auditSystemResult(tc.tool)
			tc.collect(c, &r)
			if !r.Error || !auditHasIssue(r, "policy_denied") {
				t.Fatalf("denied sources became success: %+v", r)
			}
		})
	}
}

func TestAuditSystemRecordAndReadBudgetLimits(t *testing.T) {
	t.Parallel()

	c := auditSystemFixture(t, map[string]string{
		"/etc/passwd":      strings.Repeat("user:x:1:1::/:/bin/sh\n", auditMaxEntries+1),
		"/etc/group":       "users:x:1:user\n",
		"/proc/partitions": strings.Repeat("8 0 1 sda\n", auditMaxEntries+1),
	})
	r := auditSystemResult("list_accounts")
	c.auditAccounts(context.Background(), &r, contract.PageArgs{Limit: 10})
	if !r.Truncated || !auditHasIssue(r, "inspection_limit") {
		t.Fatalf("accounts unbounded: %+v", r)
	}
	r = auditSystemResult("get_storage_info")
	c.auditStorage(context.Background(), &r)
	if !r.Truncated || len(r.Data.(*contract.StorageInfo).Devices) != auditMaxEntries {
		t.Fatal("devices unbounded")
	}
	zero := 0
	c.auditRemaining = &zero
	r = auditSystemResult("list_accounts")
	c.auditAccounts(context.Background(), &r, contract.PageArgs{Limit: 10})
	if !r.Error || !r.Truncated || !auditHasIssue(r, "inspection_limit") || len(r.Data.(*contract.AccountPage).Items) != 0 {
		t.Fatalf("exhausted budget collected: %+v", r)
	}
}

func TestAuditSystemSourceClassification(t *testing.T) {
	t.Parallel()

	for _, err := range []error{syscall.EACCES, syscall.EPERM} {
		if got := auditErrorCode(&os.PathError{Op: "open", Path: "/restricted", Err: err}); got != "permission_denied" {
			t.Fatalf("OS denial classified %s", got)
		}
	}
	c := auditSystemFixture(t, map[string]string{})
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/denied", Deny: true})
	for _, tc := range []struct{ source, code string }{{"/missing", "source_unavailable"}, {"/denied", "policy_denied"}} {
		r := auditSystemResult("get_storage_info")
		_, ok := c.auditSystemRead(context.Background(), &r, tc.source)
		if ok || !auditHasIssue(r, tc.code) {
			t.Fatalf("%s: %+v", tc.source, r)
		}
	}
}
