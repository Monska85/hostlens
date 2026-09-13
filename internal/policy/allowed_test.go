package policy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

func TestAllowedRetainsDenialsAndLiteralSemantics(t *testing.T) {
	c := config.DefaultsLinux(false)
	c.Allow.Files = []string{"/**"}
	p, err := CompileLinux(c, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Rules = append(p.Rules,
		Rule{Category: "files", Pattern: "/ignored", Deny: true, Inactive: true},
		Rule{Category: "journal", Pattern: "/ignored", Deny: true},
		Rule{Category: "files", Pattern: "/literal[1]", Deny: true},
		Rule{Category: "files", Pattern: "/literal[1]", Deny: true, Literal: true},
		Rule{Category: "files", Pattern: "/proc/private/**", Deny: true},
	)
	for _, tt := range []struct {
		target  string
		builtin bool
		want    bool
	}{
		{"/ignored", false, true},
		{"/literal1", false, false},
		{"/literal[1]", false, false},
		{"/proc/stat", true, true},
		{"/proc/stat", false, false},
		{"/proc/private/secret", true, false},
		{"/config", true, false},
	} {
		if got := p.Allowed("files", tt.target, tt.builtin); got != tt.want {
			t.Errorf("Allowed(%q, builtin=%t) = %t, want %t", tt.target, tt.builtin, got, tt.want)
		}
	}
	// Glob boundaries: * never crosses "/", ** does, a bare directory does
	// not grant its children, and the token store plus the configuration
	// source stay protected. Collector grants apply to built-ins only.
	c = config.DefaultsLinux(false)
	c.Allow.Files = []string{"/etc/app/*.conf", "/var/log/**", "/directory"}
	c.Deny.Files = []string{"/var/log/private/**"}
	boundaries, e := CompileLinux(c, "/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		p    string
		want bool
	}{{"/etc/app/a.conf", true}, {"/etc/app/nested/a.conf", false}, {"/var/log/a/b", true}, {"/var/log/private/secret", false}, {"/directory/file", false}, {c.TokenStore, false}, {"/config.yaml", false}} {
		if boundaries.Allowed("files", tt.p, false) != tt.want {
			t.Errorf("%s", tt.p)
		}
	}
	if boundaries.Allowed("files", "/etc/os-release", false) || !boundaries.Allowed("files", "/etc/os-release", true) {
		t.Fatal("collector grants leaked or absent")
	}
	// A native evaluator binds the grammar and identity into compile output,
	// and shared denials still win.
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
	np, err := compile(profile, "fixture", nil, native, nil)
	if err != nil || !np.Allowed("events", "event:health", false) || np.Allowed("events", "event:other", false) {
		t.Fatal("native rule grammar not applied", err)
	}
	cc := config.DefaultsLinux(false)
	cc.Profile = profile
	if _, err := CompileLinux(cc, "/config", nil); err == nil {
		t.Fatal("Linux accepted another evaluator's source grammar")
	}
	native.identity = "test-events-v2"
	q, err := compile(profile, "fixture", nil, native, nil)
	if err != nil || np.Fingerprint() == q.Fingerprint() {
		t.Fatal("evaluator identity missing from fingerprint", err)
	}
	profile.Deny = profile.Allow
	q, err = compile(profile, "fixture", nil, native, nil)
	if err != nil || q.Allowed("events", "event:health", false) {
		t.Fatal("native grammar bypassed shared denial precedence", err)
	}
	// A bracket pattern compiled as a literal must not behave like a glob,
	// and the literal flag participates in the fingerprint.
	c = config.DefaultsLinux(false)
	c.Allow.Files = []string{"/**", "/config[1]"}
	literal, err := CompileLinux(c, "/config[1]", nil)
	if err != nil {
		t.Fatal(err)
	}
	if literal.Allowed("files", "/config[1]", false) || !literal.Allowed("files", "/config1", false) {
		t.Fatal("literal confused with glob")
	}
	before := literal.Fingerprint()
	for i := range literal.Rules {
		if literal.Rules[i].Pattern == "/config[1]" && literal.Rules[i].Mandatory {
			literal.Rules[i].Literal = false
		}
	}
	if before == literal.Fingerprint() {
		t.Fatal("literal semantics missing from fingerprint")
	}
}

func BenchmarkAllowed(b *testing.B) {
	for _, duplicate := range []bool{false, true} {
		b.Run(fmt.Sprintf("duplicate=%t", duplicate), func(b *testing.B) {
			c := config.DefaultsLinux(false)
			for i := 0; i < 10000; i++ {
				pattern := fmt.Sprintf("/some/path/%d/**", i)
				if duplicate {
					pattern = "/some/path/duplicate/**"
				}
				c.Deny.Files = append(c.Deny.Files, pattern)
			}
			p, err := CompileLinux(c, "/config", nil)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if !p.Allowed("files", "/unrelated", true) {
					b.Fatal("built-in observation unexpectedly denied")
				}
			}
		})
	}
}
