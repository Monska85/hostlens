package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BenchmarkGateway measures one bounded authenticated MCP observation over
// loopback with the same fixed host payload on every iteration.
func BenchmarkGateway(b *testing.B) {
	ctx := context.Background()
	store := auth.Store{Path: filepath.Join(b.TempDir(), "tokens.json")}
	_, secret, err := store.Create(ctx, "benchmark", []string{"observe"}, time.Time{})
	if err != nil {
		b.Fatal(err)
	}
	policy, err := core.Compile(map[string]core.Profile{"host": {Allow: []core.Rule{{Effect: core.EffectObserve, Tool: "host_status", Resource: "host"}}}}, []string{"host"}, ToolEffects())
	if err != nil {
		b.Fatal(err)
	}
	app := New("test", store, policy, true, fakeHost{}, nil, nil)
	httpServer := httptest.NewServer(app.Transport)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "bench", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: httpServer.URL, HTTPClient: &http.Client{Transport: bearerTransport{base: http.DefaultTransport, secret: secret}}}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer session.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "host_status"}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGatewayList uses the same bounded tools/list HTTP request and
// recorder shape as the previous gateway benchmark.
func BenchmarkGatewayList(b *testing.B) {
	ctx := context.Background()
	store := auth.Store{Path: filepath.Join(b.TempDir(), "tokens.json")}
	_, secret, err := store.Create(ctx, "benchmark", []string{"observe"}, time.Time{})
	if err != nil {
		b.Fatal(err)
	}
	policy, err := core.Compile(map[string]core.Profile{"host": {Allow: []core.Rule{{Effect: core.EffectObserve, Tool: "host_status", Resource: "host"}}}}, []string{"host"}, ToolEffects())
	if err != nil {
		b.Fatal(err)
	}
	app := New("test", store, policy, true, fakeHost{}, nil, nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
		request.Header.Set("Authorization", "Bearer "+secret)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		response := httptest.NewRecorder()
		app.Transport.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			b.Fatal(response.Code)
		}
	}
}
