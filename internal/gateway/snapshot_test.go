package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/token"
)

// snapshotPath is the published tools snapshot inside the repository.
const snapshotPath = "../../docs/v1/tools.json"

// TestToolsSnapshotMatchesThePublishedFile proves discovery equals the
// committed docs/v1/tools.json golden file: a diagnostics identity with all
// capabilities in read-only mode sees every registered tool with the schemas
// the registry derives from the typed argument and payload structs. Run
// `HOSTLENS_UPDATE_SNAPSHOT=1 go test ./internal/gateway/ -run TestToolsSnapshot`
// after an intentional registry change, then commit the diff.
func TestToolsSnapshotMatchesThePublishedFile(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.MCP.ReadOnly = true
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.NewSnapshot(cfg, p)
	candidate := snap
	be := backend.New(snap, func() (backend.Snapshot, error) { return candidate, nil }, func(backend.Snapshot) contract.Collector { return observedCollector{} }, "be")
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	restart, err := RestartFingerprint(snap)
	if err != nil {
		t.Fatal(err)
	}
	c := &Coordinator{Active: snap, Tokens: token.Store{Path: cfg.TokenStore}, Load: func() (backend.Snapshot, error) { return candidate, nil }, HTTP: client, Log: slog.Default(), Instance: "gw", Restart: restart}
	store := token.Store{Path: cfg.TokenStore, AdminUID: os.Geteuid()}
	_, secret, e := store.Create("snapshot", []string{"diagnostics"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-06-18")
	r.Header.Set("Authorization", "Bearer "+secret)
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("tools/list failed: %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
		t.Fatal(e)
	}
	tools := response.Result.Tools
	if len(tools) != len(contract.ToolNames()) {
		t.Fatalf("discovery returned %d tools, registry has %d", len(tools), len(contract.ToolNames()))
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i]["name"].(string) < tools[j]["name"].(string) })
	// Keep the snapshot stable regardless of Go map iteration inside the SDK:
	// re-marshal one tool at a time and re-parse so nested maps sort keys.
	normalized, e := json.MarshalIndent(map[string]any{"tools": tools}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	normalized = append(normalized, '\n')

	path := filepath.Join(snapshotPath)
	if os.Getenv("HOSTLENS_UPDATE_SNAPSHOT") == "1" {
		if e := os.WriteFile(path, normalized, 0o644); e != nil {
			t.Fatal(e)
		}
		return
	}
	existing, e := os.ReadFile(path)
	if e != nil {
		t.Fatalf("published snapshot unreadable (run HOSTLENS_UPDATE_SNAPSHOT=1 go test ./internal/gateway/ -run TestToolsSnapshot): %v", e)
	}
	if string(existing) != string(normalized) {
		t.Fatalf("docs/v1/tools.json is stale: rebuild with HOSTLENS_UPDATE_SNAPSHOT=1\n--- want (committed) ---\n%s\n--- got (registry) ---\n%s", truncate(existing), truncate(normalized))
	}
}

func truncate(b []byte) string {
	const limit = 4000
	if len(b) > limit {
		return string(b[:limit]) + "\n... (truncated)"
	}
	return string(b)
}
