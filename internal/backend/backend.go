package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/telemetry"
)

// Lifecycle IPC request ceilings: control requests carry a generation only;
// call requests carry the bounded diagnostic argument set.
const (
	maxControlRequestBytes = 1024
	maxCallRequestBytes    = 65536
)

type Snapshot struct {
	Config      config.Config
	Policy      *policy.Policy
	Generation  string
	Fingerprint string
}

// NewSnapshot is shared by both process loaders so generation identity cannot drift.
func NewSnapshot(c config.Config, p *policy.Policy) Snapshot {
	policyFingerprint := p.Fingerprint()
	b, _ := json.Marshal([]any{policyFingerprint, c.MCP.ReadOnly})
	fingerprintHash := sha256.Sum256(b)
	fingerprint := hex.EncodeToString(fingerprintHash[:])
	b, _ = json.Marshal([]any{c, fingerprint})
	h := sha256.Sum256(b)
	return Snapshot{Config: c, Policy: p, Generation: hex.EncodeToString(h[:]), Fingerprint: fingerprint}
}

type Status struct {
	Instance     string          `json:"instance"`
	Generation   string          `json:"generation"`
	Fingerprint  string          `json:"fingerprint"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	MCPReadOnly  bool            `json:"mcp_read_only"`
}
type Loader func() (Snapshot, error)
type Factory func(Snapshot) contract.Collector
type discovery struct {
	done   chan struct{}
	status Status
	err    error
}
type Server struct {
	metrics   *telemetry.Registry
	scrapes   telemetry.Flight
	mu        sync.RWMutex
	active    Snapshot
	pending   *Snapshot
	load      Loader
	factory   Factory
	running   int
	preparing bool
	discovery *discovery
	Instance  string
	Log       *slog.Logger
}

func New(snap Snapshot, load Loader, factory Factory, instance string) *Server {
	s := &Server{metrics: telemetry.New("backend"), active: snap, load: load, factory: factory, Instance: instance}
	s.Log = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: s}))
	return s
}
func (s *Server) Level() slog.Level {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var l slog.Level
	l.UnmarshalText([]byte(s.active.Config.Logging.Level))
	return l
}

func (s *Server) Status(ctx context.Context) (Status, error) {
	s.mu.Lock()
	snap := s.active
	ctx, cancel := context.WithTimeout(ctx, snap.Config.Limits.ToolTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return Status{}, err
	}
	flight := s.discovery
	if flight == nil {
		flight = &discovery{done: make(chan struct{}), status: Status{Instance: s.Instance, Generation: snap.Generation, Fingerprint: snap.Fingerprint, MCPReadOnly: snap.Config.MCP.ReadOnly}}
		s.discovery = flight
		go func() {
			// Client cancellation must not release capacity while native I/O is stalled.
			work, stop := context.WithTimeout(context.Background(), snap.Config.Limits.ToolTimeout)
			defer stop()
			flight.status.Capabilities = s.factory(snap).Capabilities(work)
			flight.err = work.Err()
			s.mu.Lock()
			s.discovery = nil
			close(flight.done)
			s.mu.Unlock()
		}()
	}
	s.mu.Unlock()
	if flight.status.Generation != snap.Generation {
		return Status{}, errors.New("previous generation discovery is still running")
	}
	select {
	case <-ctx.Done():
		return Status{}, ctx.Err()
	case <-flight.done:
		return flight.status, flight.err
	}
}
func decode(r *http.Request, v any, n int64) error {
	d := json.NewDecoder(io.LimitReader(r.Body, n+1))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var x any
	if e := d.Decode(&x); e != io.EOF {
		return errors.New("one JSON object required")
	}
	return nil
}

// decodeArgs decodes the raw IPC argument object into the typed argument
// struct for one tool. Unknown members and malformed values are rejected;
// absent or null arguments decode to the zero struct, matching tools whose
// schema has no required members.
func decodeArgs(raw json.RawMessage, v any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}
	d := json.NewDecoder(bytes.NewReader(trimmed))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		switch r.URL.Path {
		case "/telemetry":
			s.telemetryHandler(w, r)
		case "/status":
			status, err := s.Status(r.Context())
			if err != nil {
				http.Error(w, "capability discovery unavailable", http.StatusServiceUnavailable)
				return
			}
			respond(w, status)
		case "/prepare":
			var want struct {
				Generation string `json:"generation"`
			}
			if decode(r, &want, maxControlRequestBytes) != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			s.mu.Lock()
			ctx, cancel := context.WithTimeout(r.Context(), s.active.Config.Limits.ToolTimeout)
			defer cancel()
			if s.preparing || ctx.Err() != nil {
				s.mu.Unlock()
				http.Error(w, "preparation unavailable", http.StatusServiceUnavailable)
				return
			}
			s.preparing = true
			s.mu.Unlock()
			type loaded struct {
				snapshot Snapshot
				err      error
			}
			done := make(chan loaded, 1)
			go func() {
				candidate, err := s.load()
				s.mu.Lock()
				s.preparing = false
				s.mu.Unlock()
				done <- loaded{candidate, err}
			}()
			var result loaded
			select {
			case <-ctx.Done():
				http.Error(w, "preparation unavailable", http.StatusServiceUnavailable)
				return
			case result = <-done:
			}
			if result.err != nil || result.snapshot.Generation != want.Generation {
				http.Error(w, "candidate validation or generation mismatch", 409)
				return
			}
			s.mu.Lock()
			if ctx.Err() != nil {
				s.mu.Unlock()
				http.Error(w, "preparation unavailable", http.StatusServiceUnavailable)
				return
			}
			s.pending = &result.snapshot
			s.mu.Unlock()
			respond(w, map[string]bool{"prepared": true})
		case "/activate":
			var want struct {
				Generation string `json:"generation"`
			}
			if decode(r, &want, maxControlRequestBytes) != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			s.mu.Lock()
			if s.active.Generation == want.Generation {
				s.mu.Unlock()
				respond(w, map[string]bool{"active": true})
				return
			}
			if s.pending == nil || s.pending.Generation != want.Generation {
				s.mu.Unlock()
				http.Error(w, "generation not prepared", 409)
				return
			}
			s.active = *s.pending
			s.pending = nil
			s.mu.Unlock()
			respond(w, map[string]bool{"active": true})
		case "/call":
			var req contract.Request
			if e := decode(r, &req, maxCallRequestBytes); e != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			respond(w, s.Call(r.Context(), req))
		default:
			http.NotFound(w, r)
		}
	})
}
func (s *Server) Call(ctx context.Context, req contract.Request) (result contract.Result) {
	s.mu.Lock()
	snap := s.active
	if snap.Config.Metrics.Enabled {
		started := time.Now()
		defer func() { s.metrics.Tool(req.Tool, result, time.Since(started)) }()
	}
	if req.Version != 1 || req.Generation != snap.Generation {
		s.mu.Unlock()
		return contract.Failure("generation_mismatch")
	}
	definition, known := contract.Tool(req.Tool)
	if !known || definition.Effect != contract.EffectDiagnostic {
		s.mu.Unlock()
		return contract.Failure("unsupported_operation")
	}
	// Decode into the definition's argument type and pass the decoded value,
	// not the pointer: collectors type-assert the value type.
	value := reflect.New(definition.In)
	if err := decodeArgs(req.Args, value.Interface()); err != nil {
		s.mu.Unlock()
		return contract.Failure("invalid_arguments")
	}
	args := value.Elem().Interface()
	if s.running >= snap.Config.Limits.Concurrent {
		s.mu.Unlock()
		return contract.Failure("overload")
	}
	s.running++
	s.mu.Unlock()
	if snap.Config.Metrics.Enabled {
		s.metrics.StartTool()
	}
	ctx, cancel := context.WithTimeout(ctx, snap.Config.Limits.ToolTimeout)
	defer cancel()
	started := time.Now()
	done := make(chan contract.Result, 1)
	go func() {
		defer func() {
			s.mu.Lock()
			s.running--
			s.mu.Unlock()
			if snap.Config.Metrics.Enabled {
				s.metrics.EndTool()
			}
		}()
		done <- s.factory(snap).Collect(ctx, req.Tool, args).Bounded(snap.Config.Limits.ResponseBytes)
	}()
	select {
	case result = <-done:
	case <-ctx.Done():
		code := "cancelled"
		if ctx.Err() == context.DeadlineExceeded {
			code = "timeout"
		}
		result = contract.Failure(code)
	}
	outcome := "success"
	if len(result.Issues) > 0 {
		outcome = "issues"
	}
	if outcome != "success" || snap.Config.Logging.AuditSuccessfulCalls {
		level := slog.LevelInfo
		if outcome != "success" {
			level = slog.LevelError
		}
		record := contract.AuditRecord{Component: "diagnostics", RequestID: req.ID, Tool: req.Tool, Outcome: outcome, Duration: time.Since(started)}
		s.Log.Log(ctx, level, "tool_call", record.Attributes()...)
	}
	return result
}
