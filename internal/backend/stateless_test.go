package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/telemetry"
)

type markerCollector struct {
	mode    string
	started chan struct{}
	done    chan struct{}
}

func (markerCollector) Capabilities(context.Context) map[string]bool { return nil }

func (c markerCollector) Collect(ctx context.Context, _ string, args any) contract.Result {
	path := ""
	if typed, ok := args.(contract.PathArgs); ok {
		path = typed.Path
	}
	result := contract.Result{
		Data:   map[string]any{"observed": path},
		Issues: []contract.Issue{{Code: "missing_measurement", Source: path, Message: path}},
	}
	switch c.mode {
	case "success":
		result.Issues = nil
		return result
	case "failure":
		result.Error = true
		return result
	case "timeout", "cancelled":
		close(c.started)
		<-ctx.Done()
		close(c.done)
		return result
	default:
		return result
	}
}

func TestDiagnosticMarkersStayOutOfRetainedBackendSurfaces(t *testing.T) {
	const marker = "sensitive-evidence-marker-4f7c2b"
	for _, mode := range []string{"success", "partial", "failure", "timeout", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.DefaultsLinux(false)
			cfg.Logging.AuditSuccessfulCalls = true
			if mode == "timeout" {
				cfg.Limits.ToolTimeout = 10 * time.Millisecond
			}
			started := make(chan struct{})
			done := make(chan struct{})
			var logs bytes.Buffer
			s := New(Snapshot{Config: cfg, Generation: "marker"}, nil, func(Snapshot) contract.Collector {
				return markerCollector{mode: mode, started: started, done: done}
			}, "backend")
			s.Log = slog.New(slog.NewJSONHandler(&logs, nil))
			ctx := context.Background()
			var cancel context.CancelFunc
			if mode == "cancelled" {
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					<-started
					cancel()
				}()
			}
			rawArgs, err := json.Marshal(contract.PathArgs{Path: marker})
			if err != nil {
				t.Fatal(err)
			}
			result := s.Call(ctx, contract.Request{Version: 1, ID: "marker-request", Generation: "marker", Tool: "read_config", Args: rawArgs})
			if mode == "timeout" || mode == "cancelled" {
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("cancelled collector retained active evidence")
				}
			}
			if !strings.Contains(string(mustJSON(t, result)), marker) && mode != "timeout" && mode != "cancelled" {
				t.Fatal("fixture did not inject marker into the client result")
			}
			families, err := s.metrics.Gather()
			if err != nil {
				t.Fatal(err)
			}
			metrics, err := telemetry.Encode(families)
			if err != nil {
				t.Fatal(err)
			}
			status, err := json.Marshal(s.active)
			if err != nil {
				t.Fatal(err)
			}
			for surface, content := range map[string][]byte{"logs": logs.Bytes(), "metrics": metrics, "state": status} {
				if bytes.Contains(content, []byte(marker)) {
					t.Fatalf("%s retained diagnostic marker", surface)
				}
			}
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
