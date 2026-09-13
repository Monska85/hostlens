package observerapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"go.yaml.in/yaml/v3"
)

func writeObserverConfig(t *testing.T, mutate func(*config.Config)) string {
	t.Helper()
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = "/var/run/docker.sock"
	c.Docker.ObserverSocket = filepath.Join(t.TempDir(), "run", "docker-observer.sock")
	c.Docker.Group = "docker"
	if mutate != nil {
		mutate(&c)
	}
	b, e := yaml.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func TestMainShowsHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"help"}} {
		if e := Main(args); e != nil {
			t.Fatalf("help %v returned %v", args, e)
		}
	}
	if e := Main([]string{"version"}); e != nil {
		t.Fatalf("version: %v", e)
	}
	if e := Main([]string{"serve", "--help"}); e != nil {
		t.Fatalf("serve --help must stay silent-success: %v", e)
	}
}

func TestMainRefusesUnknownSubcommand(t *testing.T) {
	if e := Main([]string{"reconcile"}); e == nil || !strings.Contains(e.Error(), "serve and version only") {
		t.Fatalf("unknown subcommand accepted: %v", e)
	}
}

func TestMainRefusesBadServeArgs(t *testing.T) {
	if e := Main([]string{"serve", "extra"}); e == nil || !strings.Contains(e.Error(), "unexpected positional") {
		t.Fatalf("positional arguments accepted: %v", e)
	}
	if e := Main([]string{"serve", "--no-such-flag"}); e == nil {
		t.Fatal("unknown flag accepted")
	}
}

func TestMainRefusesUnreadableConfig(t *testing.T) {
	if e := Main([]string{"serve", "--config", filepath.Join(t.TempDir(), "absent.yaml")}); e == nil {
		t.Fatal("missing configuration accepted")
	}
	dir := t.TempDir()
	if e := Main([]string{"serve", "--config", dir}); e == nil {
		t.Fatal("directory configuration accepted")
	}
}

func TestMainRefusesInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if e := os.WriteFile(path, []byte("version: 1\nno_such_key: true\n"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := Main([]string{"serve", "--config", path}); e == nil || !strings.Contains(e.Error(), "observer configuration invalid") {
		t.Fatalf("invalid configuration accepted: %v", e)
	}
	if e := os.WriteFile(path, []byte(": :\n\t- broken"), 0644); e != nil {
		t.Fatal(e)
	}
	if e := Main([]string{"serve", "--config", path}); e == nil {
		t.Fatal("malformed YAML accepted")
	}
}

func TestMainRefusesDockerDisabled(t *testing.T) {
	path := writeObserverConfig(t, func(c *config.Config) { c.Docker.Enabled = false })
	if e := Main([]string{"serve", "--config", path}); e == nil || !strings.Contains(e.Error(), "refuses to start") {
		t.Fatalf("disabled docker accepted: %v", e)
	}
}

func TestMainRefusesUnavailableIdentities(t *testing.T) {
	// The observer endpoint lives under a regular file so, if the system
	// really carries the configured identities, the run stops at the
	// directory creation instead of serving. In the common test container
	// neither identity nor group exists, so the earlier refusals fire.
	blockedParent := filepath.Join(t.TempDir(), "blocked")
	if e := os.WriteFile(blockedParent, []byte("x"), 0644); e != nil {
		t.Fatal(e)
	}
	path := writeObserverConfig(t, func(c *config.Config) {
		c.Docker.ObserverSocket = filepath.Join(blockedParent, "observer.sock")
	})
	e := Main([]string{"serve", "--config", path})
	if e == nil {
		t.Fatal("observer served from a test configuration")
	}
	refused := false
	for _, marker := range []string{
		"identity unavailable", "group unavailable", "not a directory", "refuses to start",
	} {
		if strings.Contains(e.Error(), marker) {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("unexpected observer failure: %v", e)
	}
}
