package policy

import (
	"fmt"
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
