package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPolicyExplainDockerTargets verifies that Docker resource targets
// explain every matching rule with provenance and the resolved decision
// without claiming any path resolution.
func TestPolicyExplainDockerTargets(t *testing.T) {
	// System-mode configuration reads require trusted root ownership; the
	// disposable container suite runs this as root.
	if os.Geteuid() != 0 {
		t.Skip("requires root-owned system configuration")
	}
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
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
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(dirs, "docker-acceptance.yaml")
	if err := os.WriteFile(profile, []byte("profiles: []\nallow:\n  docker: [container/*]\ndeny:\n  docker: [container/db]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, e := captureExplain(t, configPath, "docker:container/web")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, `"decision": "allowed"`) || !strings.Contains(out, "docker:container/web") {
		t.Fatal(out)
	}
	out, e = captureExplain(t, configPath, "docker:container/db")
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out, `"decision": "denied"`) {
		t.Fatal(out)
	}
	if !strings.Contains(out, "container/db") {
		t.Fatal("explanation must identify the matching deny rule", out)
	}
}

func captureExplain(t *testing.T, configPath, target string) (string, error) {
	t.Helper()
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan error, 1)
	go func() {
		e := Main([]string{"policy", "explain", "--system", "--config", configPath, target})
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
