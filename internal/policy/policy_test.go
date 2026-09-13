package policy

import (
	"fmt"
	"github.com/Monska85/hostlens/internal/config"
	"strings"
	"testing"
	"time"
)

func TestNativeSemanticsBindGrammarAndFingerprint(t *testing.T) {
	profile := config.Profile{Allow: config.Rules{Files: []string{"event:health"}}}
	native := semantics{
		identity:   "test-events-v1",
		categories: func(r config.Rules) map[string][]string { return map[string][]string{"events": r.Files} },
		validPattern: func(category, pattern string) bool {
			return category == "events" && strings.HasPrefix(pattern, "event:")
		},
		matches:       func(r Rule, target string) bool { return r.Pattern == target },
		builtinExempt: func(Rule) bool { return false },
	}
	p, err := compile(profile, "fixture", nil, native, nil)
	if err != nil || !p.Allowed("events", "event:health", false) || p.Allowed("events", "event:other", false) {
		t.Fatal("native rule grammar not applied", err)
	}
	c := config.DefaultsLinux(false)
	c.Profile = profile
	if _, err := CompileLinux(c, "/config", nil); err == nil {
		t.Fatal("Linux accepted another evaluator's source grammar")
	}
	native.identity = "test-events-v2"
	q, err := compile(profile, "fixture", nil, native, nil)
	if err != nil || p.Fingerprint() == q.Fingerprint() {
		t.Fatal("evaluator identity missing from fingerprint", err)
	}
	profile.Deny = profile.Allow
	q, err = compile(profile, "fixture", nil, native, nil)
	if err != nil || q.Allowed("events", "event:health", false) {
		t.Fatal("native grammar bypassed shared denial precedence", err)
	}
}

func TestPolicyBoundaries(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Allow.Files = []string{"/etc/app/*.conf", "/var/log/**", "/directory"}
	c.Deny.Files = []string{"/var/log/private/**"}
	p, e := CompileLinux(c, "/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		p    string
		want bool
	}{{"/etc/app/a.conf", true}, {"/etc/app/nested/a.conf", false}, {"/var/log/a/b", true}, {"/var/log/private/secret", false}, {"/directory/file", false}, {c.TokenStore, false}, {"/config.yaml", false}} {
		if p.Allowed("files", tt.p, false) != tt.want {
			t.Errorf("%s", tt.p)
		}
	}
	if p.Allowed("files", "/etc/os-release", false) || !p.Allowed("files", "/etc/os-release", true) {
		t.Fatal("collector grants leaked or absent")
	}
}
func TestIncludesCycleProvenanceAndFingerprint(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Profiles = []string{"a", "b"}
	defs := map[string]Definition{"a": {Profile: config.Profile{Profiles: []string{"base"}, Allow: config.Rules{Files: []string{"/etc/**"}}}, Source: "/a.yaml"}, "b": {Profile: config.Profile{Profiles: []string{"base"}}, Source: "/b.yaml"}, "base": {Profile: config.Profile{Deny: config.Rules{Files: []string{"/etc/secret"}}}, Source: "/base.yaml"}, "allow-all": {Profile: config.Profile{Allow: config.Rules{Files: []string{"/**"}}}, Source: "/broad.yaml"}}
	p, e := CompileLinux(c, "/config", defs)
	if e != nil {
		t.Fatal(e)
	}
	if p.Allowed("files", "/etc/secret", false) || p.Allowed("files", "/opt/secret", false) {
		t.Fatal("denial or activation failure")
	}
	matches := p.Matches("files", "/etc/secret")
	chains := 0
	for _, m := range matches {
		if m.Profile == "base" {
			chains++
		}
	}
	if chains != 2 {
		t.Fatalf("lost provenance %d", chains)
	}
	c.Profiles = []string{"b", "a"}
	q, e := CompileLinux(c, "/config", defs)
	if e != nil || p.Fingerprint() != q.Fingerprint() {
		t.Fatal("ordering affects fingerprint")
	}
	defs["base"] = Definition{Profile: config.Profile{Profiles: []string{"a"}}, Source: "/base.yaml"}
	if _, e = CompileLinux(c, "/config", defs); e == nil {
		t.Fatal("cycle accepted")
	}
}
func TestInvalidInputs(t *testing.T) {
	for _, pattern := range []string{"relative", "/foo/[", "/a/../b"} {
		c := config.DefaultsLinux(false)
		c.Allow.Files = []string{pattern}
		if _, e := CompileLinux(c, "/config", nil); e == nil {
			t.Errorf("accepted %q", pattern)
		}
	}
	for _, name := range []string{"../foo", "https://example.com/x", "missing"} {
		c := config.DefaultsLinux(false)
		c.Profiles = []string{name}
		if _, e := CompileLinux(c, "/config", nil); e == nil {
			t.Errorf("accepted %q", name)
		}
	}
	if ValidLinuxUnit("--system") || ValidLinuxUnit("foo.service OTHER=secret") || !ValidLinuxUnit("nginx.service") {
		t.Fatal("unit validation")
	}
}

func TestEmptySharedDAGIsBounded(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Profiles = []string{"root"}
	defs := map[string]Definition{}
	for level := 0; level < 25; level++ {
		for _, side := range []string{"a", "b"} {
			name := fmt.Sprintf("%s%d", side, level)
			children := []string{}
			if level < 24 {
				children = []string{fmt.Sprintf("a%d", level+1), fmt.Sprintf("b%d", level+1)}
			}
			defs[name] = Definition{Profile: config.Profile{Profiles: children}, Source: "/" + name + ".yaml"}
		}
	}
	defs["root"] = Definition{Profile: config.Profile{Profiles: []string{"a0", "b0"}}, Source: "/root.yaml"}
	start := time.Now()
	if _, err := CompileLinux(c, "/config", defs); err == nil || !strings.Contains(err.Error(), "10000 visits") {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("graph rejection did not release promptly")
	}
}

func TestMandatoryLiteralMatchingAndFingerprint(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Allow.Files = []string{"/**", "/config[1]"}
	p, err := CompileLinux(c, "/config[1]", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Allowed("files", "/config[1]", false) || !p.Allowed("files", "/config1", false) {
		t.Fatal("literal confused with glob")
	}
	before := p.Fingerprint()
	for i := range p.Rules {
		if p.Rules[i].Pattern == "/config[1]" && p.Rules[i].Mandatory {
			p.Rules[i].Literal = false
		}
	}
	if before == p.Fingerprint() {
		t.Fatal("literal semantics missing from fingerprint")
	}
}

func TestProvenanceRuleLimit(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	for range 10000 {
		cfg.Allow.Files = append(cfg.Allow.Files, "/fixture")
	}
	if _, err := CompileLinux(cfg, "/config", nil); err != nil {
		t.Fatal("exact rule limit rejected", err)
	}
	cfg.Allow.Files = append(cfg.Allow.Files, "/one-too-many")
	if _, err := CompileLinux(cfg, "/config", nil); err == nil || !strings.Contains(err.Error(), "10000 provenance rules") {
		t.Fatal("rule overflow accepted or misclassified", err)
	}
}

func TestValidLinuxUnitEdgeTable(t *testing.T) {
	valid := []string{"-.mount", "nginx.service", "hostlens-docker-observer.socket", "user@1000.service", "dev-sda1.device", "run-docker.mount", "system.slice", "swap.swap"}
	for _, unit := range valid {
		if !ValidLinuxUnit(unit) {
			t.Fatalf("unit %q must be valid", unit)
		}
	}
	invalid := []string{
		"-system.slice",                       // leading dash is never valid except the literal root mount
		"-.service",                           // only -.mount is special
		"nginx.txt",                           // unsupported suffix
		"no-suffix",                           // missing suffix
		"",                                    // empty
		strings.Repeat("a", 250) + ".service", // > 256 characters
		"nginx.service\x00x",                  // control byte
	}
	for _, unit := range invalid {
		if ValidLinuxUnit(unit) {
			t.Fatalf("unit %q must be invalid", unit)
		}
	}
	// The boundary is inclusive: 256 characters are still valid.
	long := strings.Repeat("a", 256-len(".service")) + ".service"
	if !ValidLinuxUnit(long) {
		t.Fatal("a 256-character unit name must stay valid")
	}
	if ValidLinuxUnit(strings.Repeat("a", 256-len(".service")+1) + ".service") {
		t.Fatal("a 257-character unit name must be invalid")
	}
}
