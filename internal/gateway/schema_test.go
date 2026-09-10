package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
)

type schemaCollector struct{ calls *atomic.Int32 }

func (s schemaCollector) Capabilities(ctx context.Context) map[string]bool {
	return observedCollector{}.Capabilities(ctx)
}
func (s schemaCollector) Collect(ctx context.Context, tool string, args contract.Args) contract.Result {
	s.calls.Add(1)
	return observedCollector{}.Collect(ctx, tool, args)
}

func TestToolArgumentContracts(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	snap := backend.Snapshot{Config: cfg, Generation: "schema-test"}
	var calls atomic.Int32
	be := backend.New(snap, nil, func(backend.Snapshot) contract.Collector { return schemaCollector{&calls} }, "backend")
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c := Coordinator{Active: snap, HTTP: client, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "schema-test", Roles: []string{"diagnostics"}}, nil
	})}
	request := func(method string, params any) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(string(body)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-06-18")
		r.Header.Set("Authorization", "Bearer test")
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected transport status: %d %s", w.Code, w.Body.String())
		}
		return w
	}
	listed := request("tools/list", map[string]any{})
	var listing struct {
		Result struct {
			Tools []struct {
				Name        string
				InputSchema struct {
					Properties           map[string]any
					AdditionalProperties bool
				}
			}
		}
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Result.Tools) != len(contract.Tools) {
		t.Fatal("missing tools", listed.Body.String())
	}
	for _, tool := range listing.Result.Tools {
		if tool.InputSchema.AdditionalProperties {
			t.Fatal("unrelated properties accepted", tool.Name)
		}
		if strings.HasPrefix(tool.Name, "get_") && tool.Name != "get_service_status" && len(tool.InputSchema.Properties) != 0 {
			t.Fatal("observation tool advertises irrelevant arguments", tool.Name)
		}
	}
	for _, tc := range []struct {
		name, tool, args string
		accepted         bool
	}{
		{"empty observations", "get_os_info", `{}`, true},
		{"ignored source", "get_inventory", `{"path":"/etc/example"}`, false},
		{"ignored pagination", "get_health_snapshot", `{"limit":1}`, false},
		{"required file", "read_config", `{}`, false},
		{"empty file", "read_config", `{"path":""}`, false},
		{"file", "read_config", `{"path":"/etc/example"}`, true},
		{"irrelevant unit", "read_config", `{"path":"/etc/example","unit":"example.service"}`, false},
		{"required service", "get_service_status", `{}`, false},
		{"service", "get_service_status", `{"unit":"example.service"}`, true},
		{"pagination", "list_services", `{"offset":1,"limit":2}`, true},
		{"irrelevant package source", "list_packages", `{"path":"/etc/example"}`, false},
		{"missing log source", "query_logs", `{}`, false},
		{"ambiguous log source", "query_logs", `{"path":"/var/log/example","unit":"example.service"}`, false},
		{"raw log", "query_logs", `{"path":"/var/log/example","raw_tail":true}`, true},
		{"journal", "query_logs", `{"unit":"example.service","priority":3}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls.Load()
			w := request("tools/call", map[string]any{"name": tc.tool, "arguments": json.RawMessage(tc.args)})
			var response struct {
				Error  any
				Result struct{ IsError bool }
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error != nil || response.Result.IsError == tc.accepted {
				t.Fatal("wrong schema outcome", w.Body.String())
			}
			expected := before
			if tc.accepted {
				expected++
			}
			if calls.Load() != expected {
				t.Fatal("invalid arguments reached collector", calls.Load(), expected)
			}
		})
	}
}
