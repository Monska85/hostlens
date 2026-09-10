package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
)

type auditCollector struct{ payload string }

func (auditCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"read_config": true}
}
func (c auditCollector) Collect(context.Context, string, contract.Args) contract.Result {
	return contract.Result{Data: map[string]any{"text": c.payload}}
}

func TestAuditCorrelationAndConfidentiality(t *testing.T) {
	const credential = "private-bearer-fixture"
	const payload = "private-content-fixture\n{\"msg\":\"forged-event\"}"
	cfg := config.DefaultsLinux(false)
	cfg.Logging.AuditSuccessfulCalls = true
	snap := backend.Snapshot{Config: cfg, Generation: "audit"}
	var backendLogs, gatewayLogs bytes.Buffer
	be := backend.New(snap, nil, func(backend.Snapshot) contract.Collector { return auditCollector{payload} }, "backend")
	be.Log = slog.New(slog.NewJSONHandler(&backendLogs, nil))
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c := Coordinator{Active: snap, HTTP: client, Log: slog.New(slog.NewJSONHandler(&gatewayLogs, nil)), Tokens: verifyFunc(func(secret string) (token.Record, error) {
		if secret != credential {
			t.Errorf("credential changed")
		}
		return token.Record{ID: "audit-token", Roles: []string{"diagnostics"}}, nil
	})}
	request := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_config","arguments":{"path":"/private-fixture"}}}`))
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-06-18")
	response := httptest.NewRecorder()
	c.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "private-content-fixture") {
		t.Fatalf("positive read failed: %d %s", response.Code, response.Body.String())
	}
	requestID := ""
	for component, logs := range map[string]*bytes.Buffer{"gateway": &gatewayLogs, "diagnostics": &backendLogs} {
		for _, secret := range []string{credential, "private-content-fixture", "forged-event", "/private-fixture"} {
			if strings.Contains(logs.String(), secret) {
				t.Errorf("%s log disclosed request content", component)
			}
		}
		decoder := json.NewDecoder(logs)
		var event map[string]any
		if err := decoder.Decode(&event); err != nil {
			t.Fatal(err)
		}
		id, ok := event["request_id"].(string)
		if !ok || id == "" || event["msg"] != "tool_call" || event["component"] != component || event["tool"] != "read_config" || event["outcome"] != "success" {
			t.Fatalf("invalid audit event: %+v", event)
		}
		if requestID != "" && requestID != id {
			t.Fatal("gateway and backend request IDs differ")
		}
		requestID = id
		if component == "gateway" && event["token_id"] != "audit-token" {
			t.Fatal("missing token identity")
		}
		if err := decoder.Decode(&event); err != io.EOF {
			t.Fatal("unexpected extra or malformed audit event", err)
		}
	}
}
