package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"go.yaml.in/yaml/v3"
)

func userConfigPath(t *testing.T) string {
	t.Helper()
	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	b, e := yaml.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	saved := os.Stdout
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = saved
	})
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(io.LimitReader(r, 1<<20))
		r.Close()
		done <- string(b)
	}()
	run()
	w.Close()
	return <-done
}

func TestMainShowsHelpAndVersion(t *testing.T) {
	out := captureStdout(t, func() {
		if e := Main(nil); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, "Commands:") {
		t.Fatalf("help text lost: %q", out)
	}
	out = captureStdout(t, func() {
		if e := Main([]string{"version"}); e != nil {
			t.Fatal(e)
		}
	})
	if out == "" {
		t.Fatal("version output missing")
	}
	if e := Main([]string{"serve", "--help"}); e != nil {
		t.Fatal(e)
	}
}

func TestMainRefusesUnknownCommands(t *testing.T) {
	path := userConfigPath(t)
	if e := Main([]string{"mutate", "--config", path}); e == nil || !strings.Contains(e.Error(), "unknown command") {
		t.Fatalf("unknown command accepted: %v", e)
	}
	if e := Main([]string{"token"}); e == nil || !strings.Contains(e.Error(), "subcommand required") {
		t.Fatalf("missing token subcommand accepted: %v", e)
	}
	if e := Main([]string{"token", "mutate", "--config", path}); e == nil || !strings.Contains(e.Error(), "unknown token command") {
		t.Fatalf("unknown token subcommand accepted: %v", e)
	}
	if e := Main([]string{"config", "mutate", "--config", path}); e == nil || !strings.Contains(e.Error(), "unknown config command") {
		t.Fatalf("unknown config subcommand accepted: %v", e)
	}
	if e := Main([]string{"policy"}); e == nil {
		t.Fatal("policy without target accepted")
	}
}

func TestLifecyclePlanWithoutApply(t *testing.T) {
	// Non-apply lifecycle commands print a plan and mutate nothing. The plan
	// source needs the three release binaries, staged as dummies.
	source := t.TempDir()
	for _, name := range []string{"hostlens", "hostlens-diagnostics", "hostlens-docker-observer"} {
		if e := os.WriteFile(filepath.Join(source, name), []byte("stub"), 0755); e != nil {
			t.Fatal(e)
		}
	}
	out := captureStdout(t, func() {
		if e := Main([]string{"install", "--source", source}); e != nil {
			t.Fatalf("install plan refused: %v", e)
		}
	})
	if !strings.Contains(out, `"state": "planned"`) {
		t.Fatalf("plan output lost: %q", out)
	}
	if e := Main([]string{"install", "extra"}); e == nil || !strings.Contains(e.Error(), "unexpected positional") {
		t.Fatalf("positional arguments accepted: %v", e)
	}
	if e := Main([]string{"install", "--privilege", "bogus"}); e == nil || !strings.Contains(e.Error(), "privilege") {
		t.Fatalf("bogus privilege accepted: %v", e)
	}
	if e := Main([]string{"install", "--help"}); e != nil {
		t.Fatal(e)
	}
	// Uninstall previews need an existing installation manifest.
	if _, e := os.Stat("/var/lib/hostlens/install.json"); e == nil {
		t.Skip("host carries an installation manifest")
	}
	if e := Main([]string{"uninstall"}); e == nil {
		t.Fatal("uninstall preview without a manifest accepted")
	}
}

func TestUpgradeRequiresArchiveWithoutApplying(t *testing.T) {
	if e := Main([]string{"upgrade"}); e == nil || !strings.Contains(e.Error(), "--archive required") {
		t.Fatalf("missing archive accepted: %v", e)
	}
	out := captureStdout(t, func() {
		if e := Main([]string{"upgrade", "--archive", filepath.Join(t.TempDir(), "absent.tgz")}); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, "Validate archive") {
		t.Fatalf("upgrade guidance lost: %q", out)
	}
	if e := Main([]string{"upgrade", "--help"}); e != nil {
		t.Fatal(e)
	}
}

func TestReconcileRefusesBadConfiguration(t *testing.T) {
	if e := Main([]string{"reconcile", "--help"}); e != nil {
		t.Fatal(e)
	}
	if e := Main([]string{"reconcile", "extra"}); e == nil || !strings.Contains(e.Error(), "unexpected positional") {
		t.Fatalf("positional arguments accepted: %v", e)
	}
	if e := Main([]string{"reconcile", "--config", filepath.Join(t.TempDir(), "absent.yaml")}); e == nil {
		t.Fatal("missing configuration accepted")
	}
	if e := Main([]string{"reconcile", "--config", t.TempDir()}); e == nil {
		t.Fatal("directory configuration accepted")
	}
}
func TestPolicyExplainExplainsPathsAndDockerTargets(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	cfg.ProfileDirs = []string{filepath.Join(t.TempDir(), "profiles")}
	target := filepath.Join(t.TempDir(), "allowed.conf")
	if e := os.WriteFile(target, []byte("observed: 0\n"), 0600); e != nil {
		t.Fatal(e)
	}
	cfg.Allow.Files = []string{target}
	b, e := yaml.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	out := captureStdout(t, func() {
		if e := Main([]string{"policy", "explain", target, "--config", path}); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, `"decision": "allowed"`) || !strings.Contains(out, `"comparison": "UNKNOWN"`) {
		t.Fatalf("file explanation lost: %q", out)
	}
	out = captureStdout(t, func() {
		if e := Main([]string{"policy", "explain", "docker:container/web", "--config", path}); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, "docker:container/web") {
		t.Fatalf("docker target lost: %q", out)
	}
	// A Docker target denied by a matching rule explains the refusal with
	// provenance and never claims a path resolution.
	cfg.Deny.Docker = []string{"container/db"}
	b, e = yaml.Marshal(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	out = captureStdout(t, func() {
		if e := Main([]string{"policy", "explain", "docker:container/db", "--config", path}); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, `"decision": "denied"`) || !strings.Contains(out, "container/db") {
		t.Fatalf("docker deny explanation lost: %q", out)
	}
	// A target that cannot be resolved stays unevaluable, not denied.
	out = captureStdout(t, func() {
		if e := Main([]string{"policy", "explain", filepath.Join(t.TempDir(), "absent-path"), "--config", path}); e != nil {
			t.Fatal(e)
		}
	})
	if !strings.Contains(out, "unevaluable") {
		t.Fatalf("unresolved target handling lost: %q", out)
	}
	// Missing target argument refuses before any inspection.
	if e := Main([]string{"policy", "explain", "--config", path}); e == nil {
		t.Fatal("missing target accepted")
	}
	// System-mode configuration reads require trusted root ownership; the
	// disposable container suite runs this as root. A Docker target explained
	// through a trusted system configuration resolves every matching rule.
	t.Run("system-mode", func(t *testing.T) {
		if os.Geteuid() != 0 {
			t.Skip("system-mode explain requires root-owned configuration")
		}
		root := t.TempDir()
		systemConfig := filepath.Join(root, "config.yaml")
		dirs := filepath.Join(root, "profiles")
		if err := os.MkdirAll(dirs, 0755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf(`version: 1
mode: system
privilege: standard
profile_dirs: [%s]
profiles: [docker-acceptance]
mcp:
  read_only: true
metrics:
  enabled: true
  allow_anonymous: false
allow:
  files: [/**]
  journal: []
deny:
  files: []
  journal: []
`, dirs)
		if err := os.WriteFile(systemConfig, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		profile := filepath.Join(dirs, "docker-acceptance.yaml")
		if err := os.WriteFile(profile, []byte("profiles: []\nallow:\n  docker: [container/*]\ndeny:\n  docker: [container/db]\n"), 0644); err != nil {
			t.Fatal(err)
		}
		out, e := captureStdoutError(t, func() error {
			return Main([]string{"policy", "explain", "--system", "--config", systemConfig, "docker:container/web"})
		})
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out, `"decision": "allowed"`) || !strings.Contains(out, "docker:container/web") {
			t.Fatal(out)
		}
		out, e = captureStdoutError(t, func() error {
			return Main([]string{"policy", "explain", "--system", "--config", systemConfig, "docker:container/db"})
		})
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out, `"decision": "denied"`) || !strings.Contains(out, "container/db") {
			t.Fatal("explanation must identify the matching deny rule", out)
		}
	})
}

func captureStdoutError(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan error, 1)
	go func() {
		e := fn()
		w.Close()
		done <- e
	}()
	b, readErr := io.ReadAll(r)
	os.Stdout = stdout
	r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(b), <-done
}
