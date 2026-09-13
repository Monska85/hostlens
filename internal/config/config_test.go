package config

import (
	"strings"
	"testing"
	"time"
)

func TestStrictConfig(t *testing.T) {
	for _, s := range []string{"server:\n  unknown: true\n", "allow:\n  windows_events: []\n", "version: 1\n---\nversion: 1\n", "version: 1\nversion: 2\n"} {
		c := DefaultsLinux(false)
		if e := Decode([]byte(s), &c); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	c := DefaultsLinux(false)
	if e := Decode([]byte("limits:\n  tool_timeout: 12s\n"), &c); e != nil || c.Limits.ToolTimeout != 12*time.Second {
		t.Fatal(e)
	}
	c.Remediation.Enabled = true
	if e := ValidateLinux(c); e == nil || !strings.Contains(e.Error(), "remediation") {
		t.Fatal(e)
	}
	// Metrics and MCP sections decode with safe defaults and strict values.
	for _, input := range []string{"version: 1\n", "metrics: {}\n"} {
		c := DefaultsLinux(false)
		if err := Decode([]byte(input), &c); err != nil || !c.Metrics.Enabled || c.Metrics.AllowAnonymous {
			t.Fatalf("defaults: %+v %v", c.Metrics, err)
		}
	}
	c = DefaultsLinux(false)
	if err := Decode([]byte("metrics:\n  enabled: false\n  allow_anonymous: true\n"), &c); err != nil || c.Metrics.Enabled || !c.Metrics.AllowAnonymous {
		t.Fatal(c.Metrics, err)
	}
	for _, input := range []string{"metrics:\n  enabled: maybe\n", "metrics:\n  allow_anonymous: 1\n", "metrics:\n  unknown: true\n"} {
		c := DefaultsLinux(false)
		if Decode([]byte(input), &c) == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	for _, input := range []string{"version: 1\n", "mcp: {}\n"} {
		c := DefaultsLinux(false)
		if err := Decode([]byte(input), &c); err != nil || !c.MCP.ReadOnly {
			t.Fatalf("default read-only: %+v %v", c.MCP, err)
		}
	}
	for input, want := range map[string]bool{
		"mcp:\n  read_only: true\n":  true,
		"mcp:\n  read_only: false\n": false,
	} {
		c := DefaultsLinux(false)
		if err := Decode([]byte(input), &c); err != nil || c.MCP.ReadOnly != want {
			t.Fatalf("decoded %q as %+v: %v", input, c.MCP, err)
		}
	}
	for _, input := range []string{"mcp:\n  read_only: maybe\n", "mcp:\n  unknown: true\n"} {
		c := DefaultsLinux(false)
		if Decode([]byte(input), &c) == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
func TestNetworkAndLimits(t *testing.T) {
	for _, bind := range [][]string{{"0.0.0.0"}, {"127.0.0.1", "127.0.0.1"}, {"::ffff:127.0.0.1"}, {"bad"}} {
		c := DefaultsLinux(false)
		c.Server.Bind = bind
		if e := ValidateLinux(c); e == nil {
			t.Errorf("accepted %v", bind)
		}
	}
	c := DefaultsLinux(false)
	c.Server.AllowInsecureHTTP = true
	c.Server.Bind = []string{"0.0.0.0", "127.0.0.1"}
	if ValidateLinux(c) == nil {
		t.Fatal("overlap accepted")
	}
	c.Server.Bind = []string{"0.0.0.0", "::"}
	if e := ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
	c.Health.Sample = c.Limits.ToolTimeout
	if ValidateLinux(c) == nil {
		t.Fatal("unbounded sample")
	}
}
