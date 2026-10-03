package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/status"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fakeHost struct{}

func (fakeHost) Host(_ context.Context, permitted func(string) bool) status.Host {
	result := status.Host{OS: "linux"}
	for _, name := range []string{"example.service", "private.service"} {
		if permitted != nil && permitted(name) {
			result.Failed = append(result.Failed, name)
		}
	}
	return result
}
func (fakeHost) Service(_ context.Context, name string) (status.Service, error) {
	return status.Service{Name: name}, nil
}
func (fakeHost) Services(_ context.Context, permitted func(string) bool) (status.ServiceInventory, error) {
	result := status.ServiceInventory{Services: []status.Service{}, Limit: 500}
	for _, name := range []string{"example.service", "private.service"} {
		if permitted != nil && permitted(name) {
			result.Services = append(result.Services, status.Service{Name: name})
		}
	}
	return result, nil
}
func (fakeHost) ServiceLogs(_ context.Context, name string, limit int) (status.ServiceLog, error) {
	return status.ServiceLog{Name: name, Limit: limit}, nil
}

type fakeDocker struct{}

func (fakeDocker) Engine(context.Context) (containers.Engine, error) {
	return containers.Engine{Version: "test"}, nil
}
func (fakeDocker) Containers(context.Context) (containers.Inventory, error) {
	return containers.Inventory{Containers: []containers.Container{{ID: "aaaaaaaaaaaa"}, {ID: "bbbbbbbbbbbb"}}, Limit: 500}, nil
}
func (fakeDocker) Container(_ context.Context, id string) (containers.Container, error) {
	return containers.Container{ID: id}, nil
}
func (fakeDocker) Stats(_ context.Context, id string) (containers.Stats, error) {
	return containers.Stats{ID: id}, nil
}
func (fakeDocker) Logs(_ context.Context, id string, limit int) (containers.Logs, error) {
	return containers.Logs{ID: id, Limit: limit}, nil
}

type bearerTransport struct {
	base   http.RoundTripper
	secret string
}

type localTransport struct {
	handler http.Handler
	secret  string
}

func (transport localTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+transport.secret)
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, copy)
	return recorder.Result(), nil
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+t.secret)
	return t.base.RoundTrip(copy)
}

func TestMCPAuthorizationAndTypedSchemas(t *testing.T) {
	ctx := context.Background()
	store := auth.Store{Path: filepath.Join(t.TempDir(), "tokens.json")}
	_, secret, err := store.Create(ctx, "test", []string{"observe"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := core.Compile(map[string]core.Profile{"diagnostics": {Allow: []core.Rule{
		{Effect: core.EffectObserve, Tool: "host_status", Resource: "host"},
		{Effect: core.EffectObserve, Tool: "list_docker_containers", Resource: "aaaaaaaaaaaa"},
		{Effect: core.EffectRepair, Tool: "restart_service", Resource: "example.service"},
	}}}, []string{"diagnostics"}, ToolEffects())
	if err != nil {
		t.Fatal(err)
	}
	app := New("test", store, policy, true, fakeHost{}, fakeDocker{}, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://hostlens.invalid/mcp", HTTPClient: &http.Client{Transport: localTransport{handler: app.Transport, secret: secret}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listing, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Tools) != 2 {
		t.Fatalf("visible tools = %d, want 2", len(listing.Tools))
	}
	for _, tool := range listing.Tools {
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("missing typed schema for %s", tool.Name)
		}
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "host_status"}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "restart_service", Arguments: RestartServiceArgs{Name: "example.service"}}); err == nil {
		t.Fatal("read-only repair was allowed")
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_docker_containers"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Fatal("missing filtered inventory")
	}
	payload, err := json.Marshal(result.StructuredContent)
	if err != nil || strings.Contains(string(payload), "bbbbbbbbbbbb") || strings.Contains(string(payload), "truncated") || !strings.Contains(string(payload), `"limit":500`) {
		t.Fatalf("inventory disclosed denied container or omitted fixed limit: %s (%v)", payload, err)
	}
}

func TestContainerStatusCannotBypassFullIDDenialWithPrefix(t *testing.T) {
	ctx := context.Background()
	const denied = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const allowed = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	store := auth.Store{Path: filepath.Join(t.TempDir(), "tokens.json")}
	_, secret, err := store.Create(ctx, "test", []string{"observe"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := core.Compile(map[string]core.Profile{"docker": {
		Allow: []core.Rule{{Effect: core.EffectObserve, Tool: "container_status", Resource: "*"}},
		Deny:  []core.Rule{{Effect: core.EffectObserve, Tool: "container_status", Resource: denied}},
	}}, []string{"docker"}, ToolEffects())
	if err != nil {
		t.Fatal(err)
	}
	app := New("test", store, policy, true, fakeHost{}, fakeDocker{}, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://hostlens.invalid/mcp", HTTPClient: &http.Client{Transport: localTransport{handler: app.Transport, secret: secret}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, id := range []string{denied[:12], denied} {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "container_status", Arguments: ContainerStatusArgs{ID: id}}); err == nil {
			t.Fatalf("denied container ID %q was accepted", id)
		}
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "container_status", Arguments: ContainerStatusArgs{ID: allowed}}); err != nil {
		t.Fatalf("allowed full ID was denied: %v", err)
	}
}

func TestServiceEvidenceRequiresSeparateGrants(t *testing.T) {
	ctx := context.Background()
	store := auth.Store{Path: filepath.Join(t.TempDir(), "tokens.json")}
	_, secret, err := store.Create(ctx, "observe", []string{"observe"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	policy, err := core.Compile(map[string]core.Profile{"services": {Allow: []core.Rule{
		{Effect: core.EffectObserve, Tool: "host_status", Resource: "host"},
		{Effect: core.EffectObserve, Tool: "list_services", Resource: "example.service"},
		{Effect: core.EffectObserve, Tool: "service_status", Resource: "example.service"},
	}}}, []string{"services"}, ToolEffects())
	if err != nil {
		t.Fatal(err)
	}
	app := New("test", store, policy, true, fakeHost{}, nil, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://hostlens.invalid/mcp", HTTPClient: &http.Client{Transport: localTransport{handler: app.Transport, secret: secret}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listing, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range listing.Tools {
		if tool.Name == "service_logs" {
			t.Fatal("ungranted logs advertised")
		}
	}
	for _, name := range []string{"host_status", "list_services"} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		payload, err := json.Marshal(result.StructuredContent)
		if err != nil || strings.Contains(string(payload), "private.service") || !strings.Contains(string(payload), "example.service") {
			t.Fatalf("%s leaked a service: %s (%v)", name, payload, err)
		}
	}
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "service_logs", Arguments: ServiceLogsArgs{Name: "example.service", Limit: 5}}); err == nil {
		t.Fatal("ungranted logs were callable")
	}
}

func TestGatewayResponsesAreNotCacheable(t *testing.T) {
	app := New("test", auth.Store{}, core.Policy{}, true, fakeHost{}, nil, nil)
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0"}`))
	writer := httptest.NewRecorder()
	app.Transport.ServeHTTP(writer, request)
	if writer.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("authenticated tool responses could be cached")
	}
}
