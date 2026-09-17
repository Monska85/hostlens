package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/telemetry"
	"github.com/Monska85/hostlens/internal/token"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Coordinator struct {
	metricsOnce sync.Once
	metrics     *telemetry.Registry
	scrapes     telemetry.Flight
	mu          sync.RWMutex
	operations  sync.RWMutex
	running     int
	Tokens      token.Verifier
	Active      backend.Snapshot
	Load        backend.Loader
	HTTP        *http.Client
	Instance    string
	Log         *slog.Logger
	Restart     string
	Effects     EffectAdmission
}

type EffectAdmission interface {
	Prepare(context.Context, bool) (EffectTransition, error)
}

// EffectTransition stages an admission change. Prepare closes and drains
// remediation when entering read-only mode, but opening remains invisible
// until Commit. Commit and Rollback are bounded, in-memory state changes.
type EffectTransition interface {
	Commit()
	Rollback()
}

func (c *Coordinator) Level() slog.Level {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var l slog.Level
	l.UnmarshalText([]byte(c.Active.Config.Logging.Level))
	return l
}
func RestartFingerprint(s backend.Snapshot) (string, error) {
	c := s.Config
	values := []any{c.Server, c.Mode, c.Privilege, c.TokenStore, c.Socket, c.AdminSocket, c.GatewayUser, c.DiagnosticsUser, c.Limits.IdleTimeout, c.Docker}
	if c.Server.TLS.Enabled {
		for _, p := range []string{c.Server.TLS.KeyFile, c.Server.TLS.CertFile} {
			b, e := os.ReadFile(p)
			if e != nil {
				return "", e
			}
			h := sha256.Sum256(b)
			values = append(values, hex.EncodeToString(h[:]))
		}
	}
	b, _ := json.Marshal(values)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// maxBackendResponseBytes bounds any diagnostic IPC response the gateway decodes.
const maxBackendResponseBytes = 16 << 20

func (c *Coordinator) rpc(ctx context.Context, path string, in, out any) error {
	b, e := json.Marshal(in)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", "http://unix"+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		return errors.New("diagnostic backend unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("backend rejected %s (%d)", path, resp.StatusCode)
	}
	err := json.NewDecoder(io.LimitReader(resp.Body, maxBackendResponseBytes)).Decode(out)
	if err != nil && ctx.Err() != nil {
		return context.Cause(ctx)
	}
	return err
}
func backendFailure(err error) contract.Result {
	code := "backend_unavailable"
	if errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
	} else if errors.Is(err, context.Canceled) {
		code = "cancelled"
	}
	return contract.Failure(code)
}

func (c *Coordinator) sync(ctx context.Context, s backend.Snapshot) error {
	var status backend.Status
	if e := c.rpc(ctx, "/status", nil, &status); e != nil {
		return e
	}
	if status.Generation == s.Generation {
		return nil
	}
	var ack map[string]bool
	want := map[string]string{"generation": s.Generation}
	if e := c.rpc(ctx, "/prepare", want, &ack); e != nil {
		return e
	}
	return c.rpc(ctx, "/activate", want, &ack)
}
func (c *Coordinator) Call(ctx context.Context, tool string, args json.RawMessage, id string) (result contract.Result, err error) {
	if err := context.Cause(ctx); err != nil {
		return backendFailure(err), err
	}
	// Do not queue remote calls behind reload or its in-flight readers.
	if !c.operations.TryRLock() {
		return contract.Failure("backend_unavailable"), errors.New("configuration reload in progress")
	}
	defer c.operations.RUnlock()
	c.mu.RLock()
	snapshot := c.Active
	c.mu.RUnlock()
	if snapshot.Config.Metrics.Enabled {
		started := time.Now()
		registry := c.telemetry()
		registry.StartTool()
		defer func() { registry.EndTool(); registry.Tool(tool, result, time.Since(started)) }()
	}
	definition, known := contract.Tool(tool)
	if !toolAdmitted(definition, known, snapshot.Config.MCP.ReadOnly) {
		return contract.Failure("operation_denied"), errors.New("operation is not admitted")
	}
	if e := c.sync(ctx, snapshot); e != nil {
		return backendFailure(e), e
	}
	req := contract.Request{Version: 1, ID: id, Generation: snapshot.Generation, Tool: tool, Args: args}
	e := c.rpc(ctx, "/call", req, &result)
	if e != nil {
		return backendFailure(e), e
	}
	return result, e
}
func (c *Coordinator) Reload(ctx context.Context) error {
	c.operations.Lock()
	defer c.operations.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	candidate, e := c.Load()
	if e != nil {
		return e
	}
	restart, e := RestartFingerprint(candidate)
	if e != nil {
		return e
	}
	if restart != c.Restart {
		return errors.New("restart-only settings or TLS material changed; restart required")
	}
	var transition EffectTransition
	if c.Effects != nil {
		transition, e = c.Effects.Prepare(ctx, candidate.Config.MCP.ReadOnly)
		if e != nil {
			if transition != nil {
				transition.Rollback()
			}
			return fmt.Errorf("prepare remediation admission: %w", e)
		}
		if transition == nil {
			return errors.New("prepare remediation admission: missing transition")
		}
	}
	committed := false
	defer func() {
		if transition != nil && !committed {
			transition.Rollback()
		}
	}()
	var ack map[string]bool
	want := map[string]string{"generation": candidate.Generation}
	if e = c.rpc(ctx, "/prepare", want, &ack); e != nil {
		return e
	}
	if e = c.rpc(ctx, "/activate", want, &ack); e != nil {
		return e
	}
	c.mu.Lock()
	if transition != nil {
		transition.Commit()
	}
	c.Active = candidate
	c.mu.Unlock()
	committed = true
	return nil
}

func toolAdmitted(definition contract.ToolDefinition, known, readOnly bool) bool {
	return known && contract.EffectAllowed(definition.Effect, readOnly)
}

func auditToolName(name string) string {
	if definition, ok := contract.Tool(name); ok {
		return definition.Name
	}
	return "unclassified"
}
func (c *Coordinator) Status() backend.Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return backend.Status{Instance: c.Instance, Generation: c.Active.Generation, Fingerprint: c.Active.Fingerprint, MCPReadOnly: c.Active.Config.MCP.ReadOnly}
}
func (c *Coordinator) Admin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/status":
			json.NewEncoder(w).Encode(c.Status())
		case "/reload":
			e := c.Reload(r.Context())
			if e != nil {
				c.Log.Error("reload_failed", "outcome", "rejected")
				http.Error(w, e.Error(), 409)
				return
			}
			c.Log.Info("reload", "outcome", "activated")
			json.NewEncoder(w).Encode(c.Status())
		default:
			http.NotFound(w, r)
		}
	})
}
func (c *Coordinator) MCP(ctx context.Context, identity token.Record) *mcp.Server {
	requestContext := ctx
	server := mcp.NewServer(&mcp.Implementation{Name: "hostlens", Version: contract.Version}, nil)
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/call" {
				// Legacy MCP transports detach HTTP cancellation. Keep both the
				// SDK's cancellation and this stateless request's lifetime.
				var cancel context.CancelCauseFunc
				ctx, cancel = context.WithCancelCause(ctx)
				stop := context.AfterFunc(requestContext, func() { cancel(context.Cause(requestContext)) })
				if cause := context.Cause(requestContext); cause != nil {
					cancel(cause)
				}
				defer stop()
				defer cancel(nil)
				if params, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
					c.mu.RLock()
					readOnly := c.Active.Config.MCP.ReadOnly
					c.mu.RUnlock()
					definition, known := contract.Tool(params.Name)
					if toolAdmitted(definition, known, readOnly) && token.Allows(identity.Roles, params.Name) {
						return next(ctx, method, req)
					}
					c.recordToolDenial()
					c.Log.Error("authorization_denied", "token_id", identity.ID, "tool", auditToolName(params.Name), "request_id", token.Random(12))
					return nil, errors.New("authorization denied")
				}
			}
			return next(ctx, method, req)
		}
	})

	var status backend.Status
	e := c.rpc(ctx, "/status", nil, &status)
	if e != nil {
		failure := backendFailure(e)
		c.Log.Error(failure.Issues[0].Code, "token_id", identity.ID)
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				if method == "tools/call" {
					if params, ok := req.GetParams().(*mcp.CallToolParamsRaw); ok {
						definition, known := contract.Tool(params.Name)
						c.mu.RLock()
						readOnly := c.Active.Config.MCP.ReadOnly
						c.mu.RUnlock()
						// Fallback admission uses the same fail-closed predicate
						// as per-call admission, so a hypothetical non-diagnostic
						// or unknown tool can never receive a result here.
						if toolAdmitted(definition, known, readOnly) && token.Allows(identity.Roles, params.Name) {
							b, _ := json.Marshal(failure)
							return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: failure}, nil
						}
					}
				}
				return next(ctx, method, req)
			}
		})
		return server
	}
	c.mu.RLock()
	readOnly := c.Active.Config.MCP.ReadOnly
	c.mu.RUnlock()
	for _, definition := range contract.ToolDefinitions() {
		name := definition.Name
		if !contract.EffectAllowed(definition.Effect, readOnly) || !token.Allows(identity.Roles, name) || !status.Capabilities[definition.Capability] {
			continue
		}
		// Low-level registration: the registry's schemas are advertised as-is
		// and both directions are validated by this package (input pre-parse,
		// output after the backend call).
		server.AddTool(&mcp.Tool{Name: name, Description: definition.Description, InputSchema: definition.InputSchema, OutputSchema: definition.OutputSchema}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			secret, _ := ctx.Value(secretKey{}).(string)
			current, e := c.Tokens.Verify(secret)
			if e != nil || current.ID != identity.ID || !token.Allows(current.Roles, name) {
				c.recordToolDenial()
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "authorization denied"}}}, nil
			}
			id := token.Random(12)
			start := time.Now()
			result, e := c.Call(ctx, name, req.Params.Arguments, id)
			outcome := "success"
			if e != nil {
				result = backendFailure(e)
				outcome = result.Issues[0].Code
			} else if verr := validateOutput(definition, result); verr != nil {
				result = contract.Failure("response_shape")
				outcome = "response_shape"
			} else if len(result.Issues) > 0 {
				outcome = "issues"
			}
			c.mu.RLock()
			audit := c.Active.Config.Logging.AuditSuccessfulCalls
			c.mu.RUnlock()
			if audit || outcome != "success" {
				peer, _ := ctx.Value(peerKey{}).(string)
				client, _ := ctx.Value(clientKey{}).(string)
				level := slog.LevelInfo
				if outcome != "success" {
					level = slog.LevelError
				}
				record := contract.AuditRecord{Component: "gateway", RequestID: id, TokenID: identity.ID, Tool: name, Outcome: outcome, PeerIP: peer, ClientIP: client, Duration: time.Since(start)}
				c.Log.Log(ctx, level, "tool_call", record.Attributes()...)
			}
			payload, err := json.Marshal(result)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{
				IsError:           result.Error,
				StructuredContent: json.RawMessage(payload),
				Content:           []mcp.Content{&mcp.TextContent{Text: string(payload)}},
			}, nil
		})
	}
	return server
}
func (c *Coordinator) recordToolDenial() {
	c.mu.RLock()
	enabled := c.Active.Config.Metrics.Enabled
	c.mu.RUnlock()
	if enabled {
		c.telemetry().Reject("mcp", "authorization")
	}
}
