package gateway

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/token"
)

// dockerCapabilityCollector serves Docker evidence availability states.
type dockerCapabilityCollector struct{ caps map[string]bool }

func (c dockerCapabilityCollector) Capabilities(context.Context) map[string]bool { return c.caps }
func (c dockerCapabilityCollector) Collect(context.Context, string, contract.Args) contract.Result {
	return contract.Result{Data: map[string]any{"observed": true}}
}

func TestDockerToolDiscoveryIsRoleAndCapabilityAware(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(config.Config{}, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.TokenStore = ""
	newGateway := func(caps map[string]bool, roles []string) (*Coordinator, string, func()) {
		snap := backend.NewSnapshot(cfg, p)
		be := backend.New(snap, nil, func(backend.Snapshot) contract.Collector { return dockerCapabilityCollector{caps} }, "be")
		bs := newBackendServer(be.Handler())
		transport := newUnixlessTransport(bs)
		restart, e := RestartFingerprint(snap)
		if e != nil {
			t.Fatal(e)
		}
		c := &Coordinator{Active: snap, Tokens: verifyFunc(func(string) (token.Record, error) {
			return token.Record{ID: "docker-test", Roles: roles}, nil
		}), HTTP: &http.Client{Transport: transport}, Log: newDiscardSlog(), Instance: "gw", Restart: restart}
		return c, bs.URL, bs.Close
	}
	for _, mode := range []bool{true, false} {
		cfg.MCP.ReadOnly = mode
		// The observer is unavailable: no Docker tool is discoverable while
		// non-Docker diagnostics remain.
		c, _, closeServer := newGateway(map[string]bool{"get_os_info": true}, []string{"diagnostics"})
		w := request(c, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if strings.Contains(w.Body.String(), "get_docker_info") {
			t.Fatal("unavailable observer discovered Docker tools")
		}
		if !strings.Contains(w.Body.String(), "get_os_info") {
			t.Fatal("Docker isolation harmed other discovery")
		}
		closeServer()
		// With the observer up and grants active, all nine tools discover.
		caps := map[string]bool{}
		for _, tool := range contract.ToolNames() {
			caps[tool] = true
		}
		c, _, closeServer = newGateway(caps, []string{"diagnostics"})
		w = request(c, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
		for _, name := range []string{"get_docker_info", "list_docker_containers", "get_docker_container", "get_docker_container_stats", "list_docker_images", "list_docker_volumes", "list_docker_networks", "get_docker_disk_usage", "query_docker_logs"} {
			if !strings.Contains(w.Body.String(), name) {
				t.Fatalf("%s missing in read_only=%v discovery", name, mode)
			}
		}
		closeServer()
	}
}

// TestDockerTopologyIsRestartOnly proves the restart fingerprint reacts to
// every Docker topology setting while shared reloadable settings stay free.
func TestDockerTopologyIsRestartOnly(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	base, err := RestartFingerprint(backend.NewSnapshot(cfg, mustPolicy(t, cfg)))
	if err != nil {
		t.Fatal(err)
	}
	changed := func(mutate func(*config.Config)) string {
		candidate := cfg
		mutate(&candidate)
		f, e := RestartFingerprint(backend.NewSnapshot(candidate, mustPolicy(t, candidate)))
		if e != nil {
			t.Fatal(e)
		}
		return f
	}
	if changed(func(c *config.Config) {
		c.Docker.Enabled = true
		c.Docker.DaemonSocket = "/var/run/docker.sock"
		c.Docker.ObserverSocket = "/run/hostlens/docker-observer.sock"
	}) == base {
		t.Fatal("docker enablement must require a restart")
	}
	if changed(func(c *config.Config) { c.Docker.DaemonSocket = "/run/docker-alt.sock" }) == base {
		t.Fatal("daemon socket change must require a restart")
	}
	if changed(func(c *config.Config) { c.Docker.Group = "other" }) == base {
		t.Fatal("group change must require a restart")
	}
	// Shared reloadable settings never change the restart fingerprint.
	if changed(func(c *config.Config) { c.Profile.Allow.Docker = []string{"containers"} }) != base {
		t.Fatal("docker policy must stay reloadable")
	}
	if changed(func(c *config.Config) { c.Limits.PageSize = 199 }) != base {
		t.Fatal("operation limits must stay reloadable")
	}
}

func mustPolicy(t *testing.T, cfg config.Config) *policy.Policy {
	t.Helper()
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newBackendServer(h http.Handler) *httptest.Server {
	bs := httptest.NewServer(h)
	return bs
}

func newDiscardSlog() *slog.Logger { return slog.New(slog.NewJSONHandler(io.Discard, nil)) }

func newUnixlessTransport(bs *httptest.Server) roundTripFunc {
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
}

func request(c *Coordinator, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-06-18")
	r.Header.Set("Authorization", "Bearer test")
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, r)
	return w
}
