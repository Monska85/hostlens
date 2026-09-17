package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
)

// TestHealthRoleServerFactsBehindAuditGrant verifies the health-role
// availability of get_hostlens_info: discovery and execution require the
// hostlens audit grant, other audit tools stay diagnostics-only, and anonymous
// requests receive the status alone.
func TestHealthRoleServerFactsBehindAuditGrant(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	for _, tc := range []struct {
		name         string
		capabilities map[string]bool
	}{
		{"hostlens grant active", map[string]bool{"get_hostlens_info": true}},
		{"hostlens grant inactive", map[string]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "g"}, Log: newDiscardSlog(), Tokens: verifyFunc(func(string) (token.Record, error) {
				return token.Record{ID: "health-client", Roles: []string{"health"}}, nil
			}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"error":true,"observed_at":"2025-01-01T00:00:00Z","issues":[{"code":"unsupported_operation","message":"unsupported_operation"}]}`
				switch r.URL.Path {
				case "/status":
					b, _ := json.Marshal(backend.Status{Generation: "g", Capabilities: tc.capabilities})
					body = string(b)
				case "/call":
					b, _ := json.Marshal(sampleResult("get_hostlens_info"))
					body = string(b)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}}
			w := request(c, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
			if w.Code != 200 {
				t.Fatal(w.Body.String())
			}
			if discovered := strings.Contains(w.Body.String(), "get_hostlens_info"); discovered != (tc.capabilities["get_hostlens_info"] == true) {
				t.Fatalf("unexpected discovery state %v: %s", discovered, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "list_processes") || strings.Contains(w.Body.String(), "get_network_info") || strings.Contains(w.Body.String(), "read_config") {
				t.Fatal("health token discovered audit tools", w.Body.String())
			}
			w = request(c, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_hostlens_info","arguments":{}}}`)
			if tc.capabilities["get_hostlens_info"] {
				if w.Code != 200 || strings.Contains(w.Body.String(), `"isError":true`) || strings.Contains(w.Body.String(), "authorization denied") {
					t.Fatal("hostlens grant did not admit server facts", w.Body.String())
				}
				if !strings.Contains(w.Body.String(), `"policy_fingerprint"`) {
					t.Fatal("server facts payload missing", w.Body.String())
				}
			} else if !strings.Contains(w.Body.String(), "error") {
				t.Fatal("server facts callable without the hostlens audit grant", w.Body.String())
			}
			for _, tool := range []string{"list_processes", "get_network_info", "read_config"} {
				w := request(c, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"`+tool+`","arguments":{}}}`)
				if !strings.Contains(w.Body.String(), "authorization denied") {
					t.Fatal("health token called audit tool", tool, w.Body.String())
				}
			}
		})
	}
	// Anonymous /mcp traffic keeps receiving the status alone.
	c := &Coordinator{Active: backend.Snapshot{Config: cfg}, Log: newDiscardSlog(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "health-client", Roles: []string{"health"}}, nil
	})}
	r := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, r)
	if w.Code != 401 || w.Body.Len() != 0 {
		t.Fatalf("anonymous response changed: %d %q", w.Code, w.Body.String())
	}
}

// sampleResult is one successful backend result for a tool's typed payload.
func sampleResult(tool string) contract.Result {
	return contract.Result{ObservedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Data: sampleData(tool)}
}

// TestResponseShapeFailureIsVisibleAndAudited proves the output-direction
// validation: a collector result that violates the tool's output schema becomes
// a response_shape tool error with no data member, and the audit record names
// the outcome.
func TestResponseShapeFailureIsAudited(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	var logs bytes.Buffer
	c := &Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "g"}, Log: slog.New(slog.NewJSONHandler(&logs, nil)), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "shape-client", Roles: []string{"health"}}, nil
	}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"generation":"g"}`
		if r.URL.Path == "/status" {
			body = `{"generation":"g","capabilities":{"get_os_info":true}}`
		} else if r.URL.Path == "/call" {
			// Wrong-typed member: the schema demands the typed OS identity.
			body = `{"observed_at":"2025-01-01T00:00:00Z","data":{"unexpected":"member"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	w := request(c, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`)
	var response struct {
		Result struct {
			IsError           bool            `json:"isError"`
			StructuredContent contract.Result `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Result.IsError || !response.Result.StructuredContent.Error || len(response.Result.StructuredContent.Issues) != 1 || response.Result.StructuredContent.Issues[0].Code != "response_shape" {
		t.Fatal("shape mismatch was not reported", w.Body.String())
	}
	if response.Result.StructuredContent.Data != nil {
		t.Fatal("mismatched data member leaked", w.Body.String())
	}
	decoder := json.NewDecoder(&logs)
	var record map[string]any
	if err := decoder.Decode(&record); err != nil {
		t.Fatal(err)
	}
	if record["msg"] != "tool_call" || record["tool"] != "get_os_info" || record["outcome"] != "response_shape" {
		t.Fatal("audit outcome missing", record)
	}
}

// TestTextMirrorEqualsStructuredContent verifies the low-level handler packs
// the wire envelope the same way the bounded backend mirror does.
func TestTextMirrorEqualsStructuredContent(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	c := &Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "g"}, Log: newDiscardSlog(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "mirror-client", Roles: []string{"health"}}, nil
	}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"generation":"g"}`
		switch r.URL.Path {
		case "/status":
			body = `{"generation":"g","capabilities":{"get_os_info":true}}`
		case "/call":
			body = `{"observed_at":"2025-01-01T00:00:00Z","data":{"family":"debian","architecture":"amd64","kernel":"6.8.0"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	w := request(c, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`)
	var response struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Structured json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Result.IsError || len(response.Result.Content) != 1 || response.Result.Content[0].Type != "text" {
		t.Fatal("missing text mirror", w.Body.String())
	}
	// The handler packs both members from one marshal: the mirror must equal
	// the wire structuredContent byte for byte.
	if response.Result.Content[0].Text != string(response.Result.Structured) {
		t.Fatalf("text mirror drifted from structured content\nmirror=%q\nstructured=%q",
			response.Result.Content[0].Text, string(response.Result.Structured))
	}
	var structured contract.Result
	if err := json.Unmarshal(response.Result.Structured, &structured); err != nil {
		t.Fatal(err)
	}
	os, ok := structured.Data.(map[string]any)
	if !ok || os["family"] != "debian" {
		t.Fatal("typed payload lost", w.Body.String())
	}
}
