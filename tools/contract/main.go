package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/server"
	"github.com/Monska85/hostlens/internal/status"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type host struct{}

func (host) Host(context.Context, func(string) bool) status.Host     { return status.Host{} }
func (host) Service(context.Context, string) (status.Service, error) { return status.Service{}, nil }
func (host) Services(context.Context, func(string) bool) (status.ServiceInventory, error) {
	return status.ServiceInventory{}, nil
}
func (host) ServiceLogs(context.Context, string, int) (status.ServiceLog, error) {
	return status.ServiceLog{}, nil
}

type docker struct{}

func (docker) Engine(context.Context) (containers.Engine, error) { return containers.Engine{}, nil }
func (docker) Containers(context.Context) (containers.Inventory, error) {
	return containers.Inventory{}, nil
}
func (docker) Container(context.Context, string) (containers.Container, error) {
	return containers.Container{}, nil
}
func (docker) Stats(context.Context, string) (containers.Stats, error) {
	return containers.Stats{}, nil
}
func (docker) Logs(context.Context, string, int) (containers.Logs, error) {
	return containers.Logs{}, nil
}

type repair struct{}

func (repair) RestartService(context.Context, string, string) (status.RestartOutcome, error) {
	return status.RestartOutcome{}, nil
}
func (repair) RestartContainer(context.Context, string, string) (containers.RestartOutcome, error) {
	return containers.RestartOutcome{}, nil
}

type handlerTransport struct {
	handler http.Handler
	secret  string
}

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.Header.Set("Authorization", "Bearer "+transport.secret)
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, copy)
	return recorder.Result(), nil
}

func catalog(ctx context.Context) ([]*mcp.Tool, error) {
	directory, err := os.MkdirTemp("", "hostlens-contract-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(directory)
	store := auth.Store{Path: filepath.Join(directory, "tokens.json")}
	_, secret, err := store.Create(ctx, "snapshot", []string{"observe", "repair"}, time.Time{})
	if err != nil {
		return nil, err
	}
	var rules []core.Rule
	for name, effect := range server.ToolEffects() {
		resource := "*"
		if name == "restart_service" {
			resource = "example.service"
		}
		if name == "restart_container" {
			resource = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		}
		rules = append(rules, core.Rule{Effect: effect, Tool: name, Resource: resource})
	}
	policy, err := core.Compile(map[string]core.Profile{"all": {Allow: rules}}, []string{"all"}, server.ToolEffects())
	if err != nil {
		return nil, err
	}
	app := server.New("0.1.0-dev", store, policy, false, host{}, docker{}, repair{})
	client := mcp.NewClient(&mcp.Implementation{Name: "snapshot", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: "http://hostlens.invalid/mcp", HTTPClient: &http.Client{Transport: handlerTransport{handler: app.Transport, secret: secret}}}, nil)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	sort.Slice(result.Tools, func(i, j int) bool { return result.Tools[i].Name < result.Tools[j].Name })
	if len(result.Tools) != len(server.ToolEffects()) {
		return nil, fmt.Errorf("registered %d tools, expected %d", len(result.Tools), len(server.ToolEffects()))
	}
	return result.Tools, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tools, err := catalog(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tools); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
