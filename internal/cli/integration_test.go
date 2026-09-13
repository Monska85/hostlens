package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"go.yaml.in/yaml/v3"
)

func TestSeparateProcessesAndDocumentedCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess integration")
	}
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	work := t.TempDir()
	bin := filepath.Join(work, "bin")
	buildArgs := []string{"build", "-o", bin + "/"}
	if mode := testing.CoverMode(); mode != "" {
		// Keep child counters beside test counters for the container's covdata merge.
		t.Setenv("GOCOVERDIR", flag.Lookup("test.gocoverdir").Value.String())
		buildArgs = append(buildArgs, "-cover", "-covermode="+mode, "-coverpkg=./internal/...,./cmd/...")
	}
	buildArgs = append(buildArgs, "./cmd/hostlens", "./cmd/hostlens-diagnostics")
	cmd := exec.Command("go", buildArgs...)
	cmd.Dir = root
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(filepath.Join(bin, "hostlens"), args...)
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("CLI %v failed: %v", args, e)
		}
		return b
	}
	for _, args := range [][]string{{"--help"}, {"version"}, {"serve", "--help"}, {"token", "create", "--help"}, {"install", "--help"}, {"upgrade", "--help"}, {"uninstall", "--help"}, {"reconcile", "--help"}} {
		run(args...)
	}
	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(work, "tokens.json")
	cfg.Socket = filepath.Join(work, "diagnostics.sock")
	cfg.AdminSocket = filepath.Join(work, "admin.sock")
	cfg.ProfileDirs = []string{filepath.Join(work, "profiles")}
	cfg.Allow.Files = []string{filepath.Join(work, "allowed.conf")}
	os.WriteFile(cfg.Allow.Files[0], []byte("observed: 0\n"), 0600)
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	cfg.Server.Port = l.Addr().(*net.TCPAddr).Port
	l.Close()
	path := filepath.Join(work, "config.yaml")
	save := func() {
		b, e := yaml.Marshal(cfg)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	save()
	run("config", "validate", "--config", path)
	b := run("token", "create", "--config", path, "--name", "integration", "--roles", "diagnostics", "--expires", time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	var created struct {
		Secret string `json:"secret"`
		Token  struct {
			ID string `json:"id"`
		} `json:"token"`
	}
	if json.Unmarshal(b, &created) != nil || created.Secret == "" {
		t.Fatal("token create returned invalid contract")
	}
	run("token", "list", "--config", path)
	start := func(name string) *exec.Cmd {
		p := exec.Command(filepath.Join(bin, name), "serve", "--config", path)
		p.Stdout = io.Discard
		p.Stderr = io.Discard
		if e = p.Start(); e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { p.Process.Signal(syscall.SIGTERM); p.Wait() })
		return p
	}
	start("hostlens-diagnostics")
	start("hostlens")
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/mcp", cfg.Server.Port)
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Server.Port), 10*time.Millisecond)
		if e == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("gateway did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	call := func(tool string, args map[string]any) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
		req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+created.Secret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	result := call("read_config", map[string]any{"path": cfg.Allow.Files[0]})
	if !strings.Contains(result, "observed: 0") {
		t.Fatalf("real IPC read failed: %s", result)
	}
	status := run("status", "--config", path)
	if !strings.Contains(string(status), `"mcp_read_only":true`) {
		t.Fatal("status omitted default read-only state", string(status))
	}
	cfg.MCP.ReadOnly = false
	save()
	run("reload", "--config", path)
	status = run("status", "--config", path)
	if !strings.Contains(string(status), `"mcp_read_only":false`) || !strings.Contains(call("get_os_info", map[string]any{}), `"result"`) {
		t.Fatal("false eligibility setting changed diagnostic or status behavior", string(status))
	}
	cfg.MCP.ReadOnly = true
	save()
	run("reload", "--config", path)
	b = run("policy", "explain", cfg.Allow.Files[0], "--config", path)
	if !strings.Contains(string(b), "MATCH") {
		t.Fatal("live policy comparison did not match")
	}
	cfg.Deny.Files = []string{cfg.Allow.Files[0]}
	save()
	run("reload", "--config", path)
	if result = call("read_config", map[string]any{"path": cfg.Allow.Files[0]}); strings.Contains(result, "observed: 0") || !strings.Contains(result, "source_denied") {
		t.Fatal("reload did not enforce new denial")
	}
	cfg.Deny.Files = nil
	save()
	run("reload", "--config", path)
	if result = call("read_config", map[string]any{"path": cfg.Allow.Files[0]}); !strings.Contains(result, "observed: 0") {
		t.Fatal("read authority not restored before role update")
	}
	run("token", "update", "--config", path, "--id", created.Token.ID, "--roles", "health")
	result = call("read_config", map[string]any{"path": cfg.Allow.Files[0]})
	var denied struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result), &denied); err != nil || denied.Error.Message != "authorization denied" {
		t.Fatalf("expected role denial, got %s", result)
	}
	if result = call("get_os_info", map[string]any{}); strings.Contains(result, `"error"`) || !strings.Contains(result, `"result"`) {
		t.Fatalf("health role lost permitted access: %s", result)
	}
	run("token", "revoke", "--config", path, "--id", created.Token.ID)
	if result = call("get_os_info", map[string]any{}); !strings.Contains(result, "invalid credential") {
		t.Fatal("revocation not enforced")
	}
	cmd = exec.Command(filepath.Join(bin, "hostlens-diagnostics"), "token", "list")
	if e = cmd.Run(); e == nil {
		t.Fatal("backend exposes token administration")
	}
}
