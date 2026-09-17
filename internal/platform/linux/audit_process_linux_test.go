package linux

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func processFixtureStat(pid int, name string) string {
	fields := []string{"S", "1", "0", "0", "0", "0", "0", "0", "0", "0", "0", "10", "20", "0", "0", "0", "0", "2", "0", "123", "4096", "1"}
	return strconv.Itoa(pid) + " (" + name + ") " + strings.Join(fields, " ") + "\n"
}
func writeAuditFixture(t *testing.T, c *Collector, path, content string) {
	t.Helper()
	p := filepath.Join(c.Root, path)
	fixtureOK(t, os.MkdirAll(filepath.Dir(p), 0700))
	fixtureOK(t, os.WriteFile(p, []byte(content), 0600))
}
func auditTestResult(tool string) contract.Result {
	return contract.Result{Data: newAuditPayload(tool)}
}
func TestProcessStatParsing(t *testing.T) {
	t.Parallel()

	stat := processFixtureStat(42, "worker ) (odd)")
	out, e := parseProcessStat([]byte(stat), 42)
	if e != nil || out.Name != "worker ) (odd)" || out.StartTicks != 123 {
		t.Fatalf("%v %v", out, e)
	}
	for _, bad := range []string{"", strings.Replace(stat, "123", "invalid", 1), strings.Replace(stat, "4096", "-1", 1), "42 (worker) S 1", strings.Replace(stat, "42 ", "43 ", 1)} {
		if _, e := parseProcessStat([]byte(bad), 42); e == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestAuditProcessEvidenceAndDeniedSecrets(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	writeAuditFixture(t, c, "/proc/42/stat", processFixtureStat(42, "worker"))
	writeAuditFixture(t, c, "/proc/42/status", "Uid:\t1000 1000 1000 1000\nGid:\t100 100 100 100\nNoNewPrivs:\t1\nSeccomp:\t2\nCapEff:\t0000000000000000\n")
	writeAuditFixture(t, c, "/proc/42/environ", "SECRET_ENV")
	writeAuditFixture(t, c, "/proc/42/cmdline", "SECRET_ARGS")
	fixtureOK(t, os.Symlink("/usr/bin/worker", filepath.Join(dir, "proc/42/exe")))
	fixtureOK(t, os.Mkdir(filepath.Join(dir, "proc/42/fd"), 0700))
	fixtureOK(t, os.Symlink("socket:[678]", filepath.Join(dir, "proc/42/fd/3")))
	r := auditTestResult("get_process_info")
	c.auditProcess(context.Background(), &r, contract.PIDArgs{PID: 42})
	info, ok := r.Data.(*contract.ProcessInfo)
	if !ok || info.Process == nil || info.Process.Executable != "/usr/bin/worker" {
		t.Fatal(r)
	}
	if len(info.Process.SocketInodes) != 1 || info.Process.SocketInodes[0] != 678 {
		t.Fatal(info.Process)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "SECRET") {
		t.Fatal("secret exposed")
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/usr/bin/worker", Deny: true})
	r = auditTestResult("get_process_info")
	c.auditProcess(context.Background(), &r, contract.PIDArgs{PID: 42})
	info = r.Data.(*contract.ProcessInfo)
	if info.Process == nil || info.Process.Executable != "" || auditCoverageOf(r) {
		t.Fatal(r)
	}
}
func TestAuditProcessesPagingAndBudget(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	for _, pid := range []int{100, 2, 40} {
		writeAuditFixture(t, c, "/proc/"+strconv.Itoa(pid)+"/stat", processFixtureStat(pid, "worker"))
	}
	writeAuditFixture(t, c, "/proc/50/stat", "malformed")
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/100/stat", Deny: true})
	r := auditTestResult("list_processes")
	c.auditProcesses(context.Background(), &r, contract.PageArgs{Limit: 1, Offset: 1})
	page := r.Data.(*contract.ProcessPage)
	if len(page.Items) != 1 || page.Items[0].PID != 40 || r.NextOffset == nil || *r.NextOffset != 2 || len(r.Issues) != 0 || page.EnumeratedProcesses != 4 || page.ObservedProcesses != 1 {
		t.Fatal(r)
	}
	// Offsets follow the PID inventory even when the selected process is malformed.
	r = auditTestResult("list_processes")
	c.auditProcesses(context.Background(), &r, contract.PageArgs{Limit: 1, Offset: 2})
	page = r.Data.(*contract.ProcessPage)
	if len(page.Items) != 0 || r.NextOffset == nil || *r.NextOffset != 3 || auditCoverageOf(r) {
		t.Fatal(r)
	}
	r = auditTestResult("list_processes")
	c.auditProcesses(context.Background(), &r, contract.PageArgs{Limit: 1, Offset: 3})
	page = r.Data.(*contract.ProcessPage)
	if len(page.Items) != 0 || r.NextOffset != nil || auditCoverageOf(r) {
		t.Fatal(r)
	}
	budget := len(processFixtureStat(2, "worker")) + 2
	c.auditRemaining = &budget
	r = auditTestResult("list_processes")
	c.auditProcesses(context.Background(), &r, contract.PageArgs{})
	if !r.Truncated || r.Data.(*contract.ProcessPage).ObservedProcesses != 1 {
		t.Fatal(r)
	}
	// Malformed, denied, and absent sources keep their own classification
	// instead of collapsing into one partial-observation issue.
	c2, root2 := fixture(t)
	c2.Root = root2
	writeAuditFixture(t, c2, "/proc/1/stat", "malformed")
	writeAuditFixture(t, c2, "/proc/2/stat", processFixtureStat(2, "worker"))
	fixtureOK(t, os.MkdirAll(filepath.Join(root2, "proc/3"), 0700))
	c2.Policy.Rules = append(c2.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/2/stat", Deny: true})
	r = auditTestResult("list_processes")
	c2.auditProcesses(context.Background(), &r, contract.PageArgs{})
	for _, code := range []string{"malformed_source", "policy_denied", "source_unavailable"} {
		if !auditHasIssue(r, code) {
			t.Fatalf("missing %s classification: %+v", code, r)
		}
	}
	if auditHasIssue(r, "partial_observation") {
		t.Fatal("source failures collapsed")
	}
}
func TestAuditDirectoryResolvedDenial(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	fixtureOK(t, os.Mkdir(filepath.Join(dir, "private"), 0700))
	fixtureOK(t, os.Symlink("/private", filepath.Join(dir, "alias")))
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/private", Deny: true})
	if f, e := c.auditDirectory("/alias"); e == nil {
		f.Close()
		t.Fatal("directory alias escaped denial")
	}
}
func TestAuditProcessAbsentAndCancellation(t *testing.T) {
	t.Parallel()

	c, dir := fixture(t)
	c.Root = dir
	r := auditTestResult("get_process_info")
	c.auditProcess(context.Background(), &r, contract.PIDArgs{PID: 42})
	info := r.Data.(*contract.ProcessInfo)
	if info.Process != nil || auditCoverageOf(r) {
		t.Fatal(r)
	}
	writeAuditFixture(t, c, "/proc/42/stat", processFixtureStat(42, "worker"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = auditTestResult("get_process_info")
	c.auditProcess(ctx, &r, contract.PIDArgs{PID: 42})
	if r.Data.(*contract.ProcessInfo).Process != nil {
		t.Fatal(r)
	}
}

func TestAuditLinkResolvedChildDenial(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	fixtureOK(t, os.Mkdir(filepath.Join(root, "real"), 0700))
	fixtureOK(t, os.Symlink("/real", filepath.Join(root, "alias")))
	fixtureOK(t, os.Symlink("socket:[555]", filepath.Join(root, "real/7")))
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/real/7", Deny: true})
	dir, e := c.auditDirectory("/alias")
	if e != nil {
		t.Fatal(e)
	}
	defer dir.Close()
	if _, e := c.auditLink(dir, "7", "/alias/7", false); e == nil {
		t.Fatal("resolved descriptor source escaped denial")
	}
}

func TestProcessLinkReadsConsumeAggregateBudget(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "proc/42/fd"), 0700))
	fixtureOK(t, os.Symlink("socket:[123]", filepath.Join(root, "proc/42/fd/3")))
	dir, err := c.auditDirectory("/proc/42/fd")
	fixtureOK(t, err)
	defer dir.Close()
	budget := len("socket:[123]")
	c.auditRemaining = &budget
	target, err := c.auditLink(dir, "3", "/proc/42/fd/3", false)
	if err != nil || target != "socket:[123]" || budget != 0 {
		t.Fatalf("uncharged link: %q %d %v", target, budget, err)
	}
	// Missing link must not be inspected after exhausting the budget.
	if _, err = c.auditLink(dir, "missing", "/proc/42/fd/missing", false); !errors.Is(err, errAuditLimit) {
		t.Fatal(err)
	}
	budget = 3
	if target, err = c.auditLink(dir, "3", "/proc/42/fd/3", false); target != "" || !errors.Is(err, errAuditLimit) || budget != 0 {
		t.Fatalf("partial link exposed or uncharged: %q %d %v", target, budget, err)
	}
}

func TestProcessFinalReadLimitPreservesUnverifiedEvidence(t *testing.T) {
	t.Parallel()

	c, root := fixture(t)
	c.Root = root
	stat := processFixtureStat(42, "worker")
	writeAuditFixture(t, c, "/proc/42/stat", stat)
	writeAuditFixture(t, c, "/proc/42/status", "Uid:\t1000 1000 1000 1000\n")
	budget := len(stat)
	c.auditRemaining = &budget
	r := auditTestResult("get_process_info")
	c.auditProcess(context.Background(), &r, contract.PIDArgs{PID: 42})
	info, ok := r.Data.(*contract.ProcessInfo)
	if !ok || info.Process == nil || info.Process.PID != 42 || info.Process.IdentityRechecked || !r.Truncated || !auditHasIssue(r, "inspection_limit") || !auditHasIssue(r, "identity_unverified") || auditHasIssue(r, "process_changed") {
		t.Fatalf("limit masqueraded as identity race: %+v", r)
	}
}
