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
func auditTestResult() contract.Result {
	return contract.Result{Data: map[string]any{"coverage_complete": true}}
}
func TestProcessStatParsing(t *testing.T) {
	stat := processFixtureStat(42, "worker ) (odd)")
	out, e := parseProcessStat([]byte(stat), 42)
	if e != nil || out["name"] != "worker ) (odd)" || out["start_ticks"] != uint64(123) {
		t.Fatalf("%v %v", out, e)
	}
	for _, bad := range []string{"", strings.Replace(stat, "123", "invalid", 1), strings.Replace(stat, "4096", "-1", 1), "42 (worker) S 1", strings.Replace(stat, "42 ", "43 ", 1)} {
		if _, e := parseProcessStat([]byte(bad), 42); e == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
func TestAuditProcessEvidenceAndDeniedSecrets(t *testing.T) {
	c, dir := fixture(t)
	c.Root = dir
	writeAuditFixture(t, c, "/proc/42/stat", processFixtureStat(42, "worker"))
	writeAuditFixture(t, c, "/proc/42/status", "Uid:\t1000 1000 1000 1000\nGid:\t100 100 100 100\nNoNewPrivs:\t1\nSeccomp:\t2\nCapEff:\t0000000000000000\n")
	writeAuditFixture(t, c, "/proc/42/environ", "SECRET_ENV")
	writeAuditFixture(t, c, "/proc/42/cmdline", "SECRET_ARGS")
	fixtureOK(t, os.Symlink("/usr/bin/worker", filepath.Join(dir, "proc/42/exe")))
	fixtureOK(t, os.Mkdir(filepath.Join(dir, "proc/42/fd"), 0700))
	fixtureOK(t, os.Symlink("socket:[678]", filepath.Join(dir, "proc/42/fd/3")))
	r := auditTestResult()
	c.auditProcess(context.Background(), &r, contract.Args{PID: 42})
	p, ok := r.Data["process"].(map[string]any)
	if !ok || p["executable"] != "/usr/bin/worker" {
		t.Fatal(r)
	}
	inodes := p["socket_inodes"].([]uint64)
	if len(inodes) != 1 || inodes[0] != 678 {
		t.Fatal(p)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "SECRET") {
		t.Fatal("secret exposed")
	}
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/usr/bin/worker", Deny: true})
	r = auditTestResult()
	c.auditProcess(context.Background(), &r, contract.Args{PID: 42})
	p = r.Data["process"].(map[string]any)
	if _, ok := p["executable"]; ok || r.Data["coverage_complete"] != false {
		t.Fatal(r)
	}
}
func TestAuditProcessesPagingAndBudget(t *testing.T) {
	c, dir := fixture(t)
	c.Root = dir
	for _, pid := range []int{100, 2, 40} {
		writeAuditFixture(t, c, "/proc/"+strconv.Itoa(pid)+"/stat", processFixtureStat(pid, "worker"))
	}
	writeAuditFixture(t, c, "/proc/50/stat", "malformed")
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/100/stat", Deny: true})
	r := auditTestResult()
	c.auditProcesses(context.Background(), &r, contract.Args{Limit: 1, Offset: 1})
	items := r.Data["items"].([]map[string]any)
	if len(items) != 1 || items[0]["pid"] != 40 || r.NextOffset == nil || *r.NextOffset != 2 || len(r.Issues) != 0 || r.Data["enumerated_processes"] != 4 || r.Data["observed_processes"] != 1 {
		t.Fatal(r)
	}
	// Offsets follow the PID inventory even when the selected process is malformed.
	r = auditTestResult()
	c.auditProcesses(context.Background(), &r, contract.Args{Limit: 1, Offset: 2})
	if len(r.Data["items"].([]map[string]any)) != 0 || r.NextOffset == nil || *r.NextOffset != 3 || r.Data["coverage_complete"] != false {
		t.Fatal(r)
	}
	r = auditTestResult()
	c.auditProcesses(context.Background(), &r, contract.Args{Limit: 1, Offset: 3})
	if len(r.Data["items"].([]map[string]any)) != 0 || r.NextOffset != nil || r.Data["coverage_complete"] != false {
		t.Fatal(r)
	}
	budget := len(processFixtureStat(2, "worker")) + 2
	c.auditRemaining = &budget
	r = auditTestResult()
	c.auditProcesses(context.Background(), &r, contract.Args{})
	if !r.Truncated || r.Data["observed_processes"] != 1 {
		t.Fatal(r)
	}
}
func TestAuditDirectoryResolvedDenial(t *testing.T) {
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
	c, dir := fixture(t)
	c.Root = dir
	r := auditTestResult()
	c.auditProcess(context.Background(), &r, contract.Args{PID: 42})
	if r.Data["process"] != nil || r.Data["coverage_complete"] != false {
		t.Fatal(r)
	}
	writeAuditFixture(t, c, "/proc/42/stat", processFixtureStat(42, "worker"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = auditTestResult()
	c.auditProcess(ctx, &r, contract.Args{PID: 42})
	if r.Data["process"] != nil {
		t.Fatal(r)
	}
}

func TestAuditLinkResolvedChildDenial(t *testing.T) {
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

func TestProcessFailuresKeepSourceClassification(t *testing.T) {
	c, root := fixture(t)
	c.Root = root
	writeAuditFixture(t, c, "/proc/1/stat", "malformed")
	writeAuditFixture(t, c, "/proc/2/stat", processFixtureStat(2, "worker"))
	fixtureOK(t, os.MkdirAll(filepath.Join(root, "proc/3"), 0700))
	c.Policy.Rules = append(c.Policy.Rules, policy.Rule{Category: "files", Pattern: "/proc/2/stat", Deny: true})
	r := auditTestResult()
	c.auditProcesses(context.Background(), &r, contract.Args{})
	for _, code := range []string{"malformed_source", "policy_denied", "source_unavailable"} {
		if !auditHasIssue(r, code) {
			t.Fatalf("missing %s classification: %+v", code, r)
		}
	}
	if auditHasIssue(r, "partial_observation") {
		t.Fatal("source failures collapsed")
	}
}

func TestProcessLinkReadsConsumeAggregateBudget(t *testing.T) {
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
	c, root := fixture(t)
	c.Root = root
	stat := processFixtureStat(42, "worker")
	writeAuditFixture(t, c, "/proc/42/stat", stat)
	writeAuditFixture(t, c, "/proc/42/status", "Uid:\t1000 1000 1000 1000\n")
	budget := len(stat)
	c.auditRemaining = &budget
	r := auditTestResult()
	c.auditProcess(context.Background(), &r, contract.Args{PID: 42})
	process, ok := r.Data["process"].(map[string]any)
	if !ok || process["pid"] != 42 || process["identity_rechecked"] != false || !r.Truncated || !auditHasIssue(r, "inspection_limit") || !auditHasIssue(r, "identity_unverified") || auditHasIssue(r, "process_changed") {
		t.Fatalf("limit masqueraded as identity race: %+v", r)
	}
}
