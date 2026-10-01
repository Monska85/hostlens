package gateway

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/policy"
)

func TestReloadRequiresPositiveBackendAcknowledgements(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	cfg.MCP.ReadOnly = !cfg.MCP.ReadOnly
	candidate := backend.NewSnapshot(cfg, p)
	restart, err := RestartFingerprint(active)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, prepared, activated string
		activateCalls             int
	}{
		{"preparation false", `{"prepared":false}`, `{"active":true}`, 0},
		{"preparation missing", `{}`, `{"active":true}`, 0},
		{"preparation null", `null`, `{"active":true}`, 0},
		{"activation false", `{"prepared":true}`, `{"active":false}`, 1},
		{"activation missing", `{"prepared":true}`, `{}`, 1},
		{"activation null", `{"prepared":true}`, `null`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := &Coordinator{Active: active, Load: func() (backend.Snapshot, error) { return candidate, nil }, Restart: restart, Log: slog.New(slog.NewJSONHandler(io.Discard, nil))}
			c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := tc.prepared
				if r.URL.Path == "/activate" {
					calls++
					body = tc.activated
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			if err := c.Reload(context.Background()); err == nil {
				t.Fatal("ambiguous acknowledgement activated a generation")
			}
			if c.Status().Generation != active.Generation || calls != tc.activateCalls {
				t.Fatalf("reload changed active generation or crossed rejected phase: calls=%d", calls)
			}
		})
	}
}

func TestBackendSyncChecksFingerprintAlongsideGeneration(t *testing.T) {
	t.Parallel()
	paths := []string{}
	c := &Coordinator{HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"generation":"current","fingerprint":"unexpected"}`)), Header: make(http.Header)}, nil
	})}}
	if err := c.sync(context.Background(), backend.Snapshot{Generation: "current", Fingerprint: "expected"}); err == nil {
		t.Fatal("same-generation fingerprint mismatch accepted")
	}
	if got := strings.Join(paths, ","); got != "/status" {
		t.Fatalf("same-generation fingerprint mismatch crossed activation boundary: %s", got)
	}
}
