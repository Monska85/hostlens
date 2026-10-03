package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/auth"
	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/core"
	"github.com/Monska85/hostlens/internal/status"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HostStatusArgs struct{}
type DockerStatusArgs struct{}
type ListContainersArgs struct{}
type ListServicesArgs struct{}
type ServiceStatusArgs struct {
	Name string `json:"name" jsonschema:"native service identifier"`
}
type RestartServiceArgs struct {
	Name string `json:"name" jsonschema:"native service identifier"`
}
type ServiceLogsArgs struct {
	Name  string `json:"name" jsonschema:"exact native service identifier"`
	Limit int    `json:"limit" jsonschema:"maximum number of entries, 1 to 100"`
}
type ContainerStatusArgs struct {
	ID string `json:"id" jsonschema:"full 64-character hexadecimal container ID"`
}
type ContainerStatsArgs struct {
	ID string `json:"id" jsonschema:"full 64-character hexadecimal container ID"`
}
type ContainerLogsArgs struct {
	ID    string `json:"id" jsonschema:"full 64-character hexadecimal container ID"`
	Limit int    `json:"limit" jsonschema:"maximum number of recent lines, 1 to 100"`
}
type RestartContainerArgs struct {
	ID string `json:"id" jsonschema:"full stable hexadecimal container ID"`
}

type ServiceStatusResult struct {
	Service status.Service `json:"service"`
}
type ContainerStatusResult struct {
	Container containers.Container `json:"container"`
}
type RestartServiceResult struct {
	Service   *status.Service `json:"service,omitempty"`
	Invoked   *bool           `json:"invoked,omitempty"`
	Completed *bool           `json:"completed,omitempty"`
	Issue     string          `json:"issue,omitempty"`
}
type RestartContainerResult struct {
	Container *containers.Container `json:"container,omitempty"`
	Invoked   *bool                 `json:"invoked,omitempty"`
	Completed *bool                 `json:"completed,omitempty"`
	Issue     string                `json:"issue,omitempty"`
}
type DockerStatusResult struct {
	Engine *containers.Engine `json:"engine,omitempty"`
	Issues []status.Issue     `json:"issues,omitempty"`
}

type RepairService interface {
	RestartService(context.Context, string, string) (status.RestartOutcome, error)
	RestartContainer(context.Context, string, string) (containers.RestartOutcome, error)
}

type App struct {
	Policy    core.Policy
	ReadOnly  bool
	Tokens    auth.Store
	Host      status.Reader
	Docker    containers.Reader
	Repair    RepairService
	MCP       *mcp.Server
	Transport http.Handler
}

type identityKey struct{}

type credential struct {
	record auth.Token
	secret string
}

type definition struct {
	name        string
	description string
	effect      core.Effect
	docker      bool
	collection  bool
	target      func(json.RawMessage) (string, error)
	validTarget func(string) bool
	register    func(*App, *mcp.Tool)
}

func fixedTarget(value string) func(json.RawMessage) (string, error) {
	return func(json.RawMessage) (string, error) { return value, nil }
}

func typedTarget[T any](field func(T) string) func(json.RawMessage) (string, error) {
	return func(raw json.RawMessage) (string, error) {
		var args T
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", errors.New("invalid target arguments")
		}
		resource := field(args)
		if resource == "" {
			return "", errors.New("empty target")
		}
		return resource, nil
	}
}

// This is the only tool catalog. The SDK derives both schemas and validates
// inputs and outputs from the distinct typed handler signatures below.
var catalog = []definition{
	{
		name: "host_status", description: "Observe current host identity and resource status.", effect: core.EffectObserve,
		target: fixedTarget("host"),
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ HostStatusArgs) (*mcp.CallToolResult, status.Host, error) {
				var permitted func(string) bool
				if a.Policy.Advertises(core.EffectObserve, "service_status") {
					permitted = func(name string) bool { return a.Policy.Allows(core.EffectObserve, "service_status", name) }
				}
				value := a.Host.Host(ctx, permitted)
				return nil, value, nil
			})
		},
	},
	{
		name: "list_services", description: "List permitted native services within a fixed limit.", effect: core.EffectObserve,
		target: fixedTarget("*"), collection: true,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListServicesArgs) (*mcp.CallToolResult, status.ServiceInventory, error) {
				inventory, err := a.Host.Services(ctx, func(name string) bool {
					return a.Policy.Allows(core.EffectObserve, tool.Name, name)
				})
				if err != nil {
					return nil, status.ServiceInventory{}, err
				}
				return nil, inventory, nil
			})
		},
	},
	{
		name: "service_status", description: "Observe one permitted native service.", effect: core.EffectObserve,
		target: typedTarget(func(args ServiceStatusArgs) string { return args.Name }),
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ServiceStatusArgs) (*mcp.CallToolResult, ServiceStatusResult, error) {
				value, err := a.Host.Service(ctx, args.Name)
				return nil, ServiceStatusResult{Service: value}, err
			})
		},
	},
	{
		name: "service_logs", description: "Read bounded recent journal entries for one permitted service.", effect: core.EffectObserve,
		target: typedTarget(func(args ServiceLogsArgs) string { return args.Name }),
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ServiceLogsArgs) (*mcp.CallToolResult, status.ServiceLog, error) {
				value, err := a.Host.ServiceLogs(ctx, args.Name, args.Limit)
				return nil, value, err
			})
		},
	},
	{
		name: "docker_status", description: "Observe the configured local Docker Engine.", effect: core.EffectObserve, docker: true,
		target: fixedTarget("engine"),
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ DockerStatusArgs) (*mcp.CallToolResult, DockerStatusResult, error) {
				value, err := a.Docker.Engine(ctx)
				if err != nil {
					return nil, DockerStatusResult{Issues: []status.Issue{{Source: "docker", Kind: "unavailable"}}}, nil
				}
				return nil, DockerStatusResult{Engine: &value}, nil
			})
		},
	},
	{
		name: "list_docker_containers", description: "Observe permitted current Docker container states.", effect: core.EffectObserve, docker: true,
		target: fixedTarget("*"), collection: true,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListContainersArgs) (*mcp.CallToolResult, containers.Inventory, error) {
				inventory, err := a.Docker.Containers(ctx)
				if err != nil {
					return nil, containers.Inventory{}, err
				}
				filtered := inventory.Containers[:0]
				for _, item := range inventory.Containers {
					if a.Policy.Allows(core.EffectObserve, tool.Name, item.ID) {
						filtered = append(filtered, item)
					}
				}
				inventory.Containers = filtered
				return nil, inventory, nil
			})
		},
	},
	{
		name: "container_status", description: "Observe one permitted Docker container by stable ID.", effect: core.EffectObserve, docker: true,
		target: typedTarget(func(args ContainerStatusArgs) string { return args.ID }), validTarget: containers.ValidID,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ContainerStatusArgs) (*mcp.CallToolResult, ContainerStatusResult, error) {
				value, err := a.Docker.Container(ctx, args.ID)
				return nil, ContainerStatusResult{Container: value}, err
			})
		},
	},
	{
		name: "container_stats", description: "Observe one permitted Docker container resource sample.", effect: core.EffectObserve, docker: true,
		target: typedTarget(func(args ContainerStatsArgs) string { return args.ID }), validTarget: containers.ValidID,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ContainerStatsArgs) (*mcp.CallToolResult, containers.Stats, error) {
				value, err := a.Docker.Stats(ctx, args.ID)
				return nil, value, err
			})
		},
	},
	{
		name: "container_logs", description: "Read bounded recent logs for one permitted Docker container.", effect: core.EffectObserve, docker: true,
		target: typedTarget(func(args ContainerLogsArgs) string { return args.ID }), validTarget: containers.ValidID,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args ContainerLogsArgs) (*mcp.CallToolResult, containers.Logs, error) {
				value, err := a.Docker.Logs(ctx, args.ID, args.Limit)
				return nil, value, err
			})
		},
	},
	{
		name: "restart_service", description: "Restart one explicitly permitted native service.", effect: core.EffectRepair,
		target: typedTarget(func(args RestartServiceArgs) string { return args.Name }),
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args RestartServiceArgs) (*mcp.CallToolResult, RestartServiceResult, error) {
				identity, err := a.identity(ctx)
				if err != nil {
					return nil, RestartServiceResult{}, err
				}
				value, err := a.Repair.RestartService(ctx, identity.secret, args.Name)
				return nil, RestartServiceResult{Service: value.Service, Invoked: value.Invoked, Completed: value.Completed, Issue: value.Issue}, err
			})
		},
	},
	{
		name: "restart_container", description: "Restart one explicitly permitted Docker container.", effect: core.EffectRepair, docker: true,
		target: typedTarget(func(args RestartContainerArgs) string { return args.ID }), validTarget: containers.ValidID,
		register: func(a *App, tool *mcp.Tool) {
			mcp.AddTool(a.MCP, tool, func(ctx context.Context, _ *mcp.CallToolRequest, args RestartContainerArgs) (*mcp.CallToolResult, RestartContainerResult, error) {
				identity, err := a.identity(ctx)
				if err != nil {
					return nil, RestartContainerResult{}, err
				}
				value, err := a.Repair.RestartContainer(ctx, identity.secret, args.ID)
				return nil, RestartContainerResult{Container: value.Container, Invoked: value.Invoked, Completed: value.Completed, Issue: value.Issue}, err
			})
		},
	},
}

func find(name string) (definition, bool) {
	for _, item := range catalog {
		if item.name == name {
			return item, true
		}
	}
	return definition{}, false
}

func ToolEffects() map[string]core.Effect {
	effects := make(map[string]core.Effect, len(catalog))
	for _, item := range catalog {
		if _, duplicate := effects[item.name]; duplicate {
			panic("duplicate MCP tool: " + item.name)
		}
		effects[item.name] = item.effect
	}
	return effects
}

func hasRole(record auth.Token, role core.Role) bool {
	for _, candidate := range record.Roles {
		if candidate == string(role) {
			return true
		}
	}
	return false
}

func (a *App) authorize(record auth.Token, tool definition, resource string, discovery bool) bool {
	if tool.effect == core.EffectRepair && a.ReadOnly {
		return false
	}
	if !discovery && tool.validTarget != nil && !tool.validTarget(resource) {
		return false
	}
	role := core.RoleObserve
	if tool.effect == core.EffectRepair {
		role = core.RoleRepair
	}
	if !hasRole(record, role) || tool.docker && a.Docker == nil || tool.effect == core.EffectRepair && a.Repair == nil {
		return false
	}
	if discovery || tool.collection {
		return a.Policy.Advertises(tool.effect, tool.name)
	}
	return a.Policy.Allows(tool.effect, tool.name, resource)
}

func bearer(header string) (string, bool) {
	scheme, secret, found := strings.Cut(header, " ")
	return secret, found && strings.EqualFold(scheme, "Bearer") && secret != "" && !strings.ContainsAny(secret, " \t\r\n")
}

func (a *App) identity(ctx context.Context) (credential, error) {
	identity, ok := ctx.Value(identityKey{}).(credential)
	if !ok {
		return credential{}, errors.New("missing request authentication")
	}
	return identity, nil
}

func (a *App) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
		if method != "tools/list" && method != "tools/call" {
			return next(ctx, method, request)
		}
		identity, err := a.identity(ctx)
		if err != nil {
			return nil, errors.New("unauthorized")
		}
		if method == "tools/call" {
			call, ok := request.(*mcp.CallToolRequest)
			if !ok || call.Params == nil {
				return nil, errors.New("invalid tool request")
			}
			tool, known := find(call.Params.Name)
			if !known {
				return nil, errors.New("unknown tool")
			}
			resource, err := tool.target(call.Params.Arguments)
			if err != nil || !a.authorize(identity.record, tool, resource, false) {
				return nil, errors.New("tool denied")
			}
			return next(ctx, method, request)
		}
		result, err := next(ctx, method, request)
		if err != nil {
			return nil, err
		}
		listing, ok := result.(*mcp.ListToolsResult)
		if !ok {
			return nil, errors.New("unexpected tool listing")
		}
		filtered := make([]*mcp.Tool, 0, len(listing.Tools))
		for _, offered := range listing.Tools {
			tool, known := find(offered.Name)
			if known && a.authorize(identity.record, tool, "", true) {
				filtered = append(filtered, offered)
			}
		}
		listing.Tools = filtered
		return listing, nil
	}
}

func New(version string, tokens auth.Store, policy core.Policy, readOnly bool, host status.Reader, docker containers.Reader, repair RepairService) *App {
	a := &App{Policy: policy, ReadOnly: readOnly, Tokens: tokens, Host: host, Docker: docker, Repair: repair}
	a.MCP = mcp.NewServer(&mcp.Implementation{Name: "hostlens", Version: version}, &mcp.ServerOptions{SetCacheable: func(_ context.Context, _ mcp.Request, value *mcp.Cacheable) {
		value.TTLMs, value.CacheScope = 0, "private"
	}})
	a.MCP.AddReceivingMiddleware(a.middleware)
	for _, item := range catalog {
		if item.effect == core.EffectRepair && (readOnly || repair == nil) || item.docker && docker == nil || !policy.Advertises(item.effect, item.name) {
			continue
		}
		item.register(a, &mcp.Tool{Name: item.name, Description: item.description})
	}
	transport := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return a.MCP }, &mcp.StreamableHTTPOptions{Stateless: true, MaxRequestBodyBytes: 128 << 10, PropagateRequestCancellation: true})
	slots := make(chan struct{}, 16)
	a.Transport = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(writer, "busy", http.StatusServiceUnavailable)
			return
		}
		secret, ok := bearer(request.Header.Get("Authorization"))
		if !ok {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		record, err := a.Tokens.Verify(secret)
		if err != nil {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx, cancel := context.WithTimeout(context.WithValue(request.Context(), identityKey{}, credential{record: record, secret: secret}), 45*time.Second)
		defer cancel()
		transport.ServeHTTP(writer, request.WithContext(ctx))
	})
	return a
}
