package gateway

import (
	"bytes"
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
func (s schemaCollector) Collect(ctx context.Context, tool string, args any) contract.Result {
	s.calls.Add(1)
	return observedCollector{}.Collect(ctx, tool, args)
}

func TestToolArgumentContracts(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	snap := backend.Snapshot{Config: cfg, Generation: "schema-test"}
	var calls atomic.Int32
	var backendRequests atomic.Int32
	var gatewayLogs bytes.Buffer
	be := backend.New(snap, nil, func(backend.Snapshot) contract.Collector { return schemaCollector{&calls} }, "backend")
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		backendRequests.Add(1)
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c := Coordinator{Active: snap, HTTP: client, Log: slog.New(slog.NewJSONHandler(&gatewayLogs, nil)), Tokens: verifyFunc(func(string) (token.Record, error) {
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
				OutputSchema struct {
					Required   []string
					Properties struct {
						Data struct {
							Properties map[string]any
						} `json:"data"`
					}
				} `json:"outputSchema"`
			}
		}
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if len(listing.Result.Tools) != len(contract.ToolNames()) {
		t.Fatal("missing tools", listed.Body.String())
	}
	for _, tool := range listing.Result.Tools {
		if tool.InputSchema.AdditionalProperties {
			t.Fatal("unrelated properties accepted", tool.Name)
		}
		if strings.HasPrefix(tool.Name, "get_") && tool.Name != "get_service_status" && tool.Name != "get_process_info" && tool.Name != "get_docker_container" && tool.Name != "get_docker_container_stats" && len(tool.InputSchema.Properties) != 0 {
			t.Fatal("observation tool advertises irrelevant arguments", tool.Name)
		}
		if len(tool.OutputSchema.Properties.Data.Properties) == 0 {
			t.Fatal("output schema does not enumerate data members", tool.Name)
		}
		for _, member := range tool.OutputSchema.Required {
			if member == "data" {
				t.Fatal("output schema marks data required", tool.Name)
			}
		}
	}
	for _, tc := range []struct {
		name, tool, args string
		accepted         bool
	}{
		{"audit process", "get_process_info", `{"pid":1}`, true},
		{"audit process zero", "get_process_info", `{"pid":0}`, false},
		{"audit process out of range", "get_process_info", `{"pid":4194305}`, false},
		{"audit process missing", "get_process_info", `{}`, false},
		{"audit injection", "get_network_info", `{"command":"id"}`, false},
		{"audit unknown member", "get_process_info", `{"pid":1,"command":"id"}`, false},
		{"audit path missing", "inspect_path", `{}`, false},
		{"audit service missing", "inspect_service", `{}`, false},
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
		{"docker info empty", "get_docker_info", `{}`, true},
		{"docker info extra", "get_docker_info", `{"container":"x"}`, false},
		{"docker container missing", "get_docker_container", `{}`, false},
		{"docker container", "get_docker_container", `{"container":"abc"}`, true},
		{"docker stats empty", "get_docker_container_stats", `{}`, false},
		{"docker disk usage extra", "get_docker_disk_usage", `{"limit":1}`, false},
		{"docker logs missing", "query_docker_logs", `{}`, false},
		{"docker logs", "query_docker_logs", `{"container":"abc","limit":5}`, true},
		{"docker logs format", "query_docker_logs", `{"container":"abc","format":"jsonl"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := calls.Load()
			requestsBefore := backendRequests.Load()
			w := request("tools/call", map[string]any{"name": tc.tool, "arguments": json.RawMessage(tc.args)})
			var response struct {
				Error  any
				Result struct {
					IsError    bool
					Structured struct {
						Issues []struct{ Code string } `json:"issues"`
					} `json:"structuredContent"`
				}
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
				if backendRequests.Load() <= requestsBefore {
					t.Fatal("accepted arguments never reached the backend")
				}
			} else {
				if len(response.Result.Structured.Issues) == 0 || response.Result.Structured.Issues[0].Code != "invalid_arguments" {
					t.Fatal("rejection lost the invalid_arguments issue", w.Body.String())
				}
				if backendRequests.Load() != requestsBefore {
					t.Fatal("invalid arguments reached the backend", backendRequests.Load(), requestsBefore)
				}
			}
			if calls.Load() != expected {
				t.Fatal("collector boundary crossed", calls.Load(), expected)
			}
		})
	}
	before := calls.Load()
	requestsBefore := backendRequests.Load()
	const unclassifiedMarker = "remediate_host_sensitive_marker"
	w := request("tools/call", map[string]any{"name": unclassifiedMarker, "arguments": map[string]any{}})
	var rejected struct{ Error any }
	if err := json.Unmarshal(w.Body.Bytes(), &rejected); err != nil || rejected.Error == nil || calls.Load() != before || backendRequests.Load() != requestsBefore {
		t.Fatal("remembered remediation crossed the effect gate", w.Body.String(), calls.Load(), before, backendRequests.Load(), requestsBefore)
	}
	if strings.Contains(gatewayLogs.String(), unclassifiedMarker) {
		t.Fatal("unclassified tool argument entered audit logs")
	}
	// Non-object argument JSON fails at the gateway with the same
	// invalid_arguments issue and never reaches the backend.
	raw := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":[1,2]}}`))
	raw.Header.Set("Content-Type", "application/json")
	raw.Header.Set("Accept", "application/json, text/event-stream")
	raw.Header.Set("MCP-Protocol-Version", "2025-06-18")
	raw.Header.Set("Authorization", "Bearer test")
	w = httptest.NewRecorder()
	c.Handler().ServeHTTP(w, raw)
	var malformed struct {
		Result struct {
			IsError    bool
			Structured struct {
				Issues []struct{ Code string } `json:"issues"`
			} `json:"structuredContent"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &malformed); err != nil || !malformed.Result.IsError || malformed.Result.Structured.Issues[0].Code != "invalid_arguments" {
		t.Fatal("malformed arguments JSON escaped schema rejection", w.Body.String())
	}
	if backendRequests.Load() != requestsBefore || calls.Load() != before {
		t.Fatal("malformed arguments JSON reached the backend")
	}
}
