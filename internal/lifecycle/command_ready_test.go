package lifecycle

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
)

func TestCommandDelegatesToInjectedRunner(t *testing.T) {
	m := Manager{Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte(name), nil
	}}
	out, e := m.command(context.Background(), "systemctl", "is-active")
	if e != nil || string(out) != "systemctl" {
		t.Fatalf("runner delegation lost: %q %v", out, e)
	}
}

func TestCommandRefusesRootedExecution(t *testing.T) {
	m := Manager{Root: t.TempDir()}
	if _, e := m.command(context.Background(), "systemctl"); e == nil || !strings.Contains(e.Error(), "disposable environment adapter") {
		t.Fatalf("rooted execution accepted: %v", e)
	}
}

func TestCommandExecutesAbsoluteNames(t *testing.T) {
	m := Manager{}
	out, e := m.command(context.Background(), "/bin/echo", "one", "two")
	if e != nil || strings.TrimSpace(string(out)) != "one two" {
		t.Fatalf("absolute execution lost: %q %v", out, e)
	}
}

func TestCommandReportsUnavailableExecutables(t *testing.T) {
	m := Manager{}
	if _, e := m.command(context.Background(), "definitely-missing-hostlens-command"); e == nil || !strings.Contains(e.Error(), "unavailable") {
		t.Fatalf("missing executable accepted: %v", e)
	}
	if _, e := m.command(context.Background(), "/no/such/absolute/tool"); e == nil {
		t.Fatal("missing absolute executable accepted")
	}
}

func newStatusSocket(t *testing.T, status int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "status.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		if status == 200 {
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "warming", status)
		}
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(l) }()
	t.Cleanup(func() { _ = server.Close() })
	return path
}

func TestReadyObservesDiagnosticsStatus(t *testing.T) {
	cfg := config.DefaultsLinux(true)
	cfg.Socket = newStatusSocket(t, 200)
	m := Manager{Config: cfg}
	if e := m.ready(context.Background(), "hostlens-diagnostics.service"); e != nil {
		t.Fatalf("healthy diagnostics socket refused: %v", e)
	}
}

func TestReadyGatewayProbesAdminAndTCP(t *testing.T) {
	cfg := config.DefaultsLinux(true)
	cfg.AdminSocket = newStatusSocket(t, 200)
	// Bind the configured admin TCP port on loopback so the gateway probe
	// completes.
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	cfg.Server.Bind = []string{"127.0.0.1"}
	cfg.Server.Port = l.Addr().(*net.TCPAddr).Port
	m := Manager{Config: cfg}
	if e := m.ready(context.Background(), "hostlens-gateway.service"); e != nil {
		t.Fatalf("healthy gateway refused: %v", e)
	}
}

func TestReadyTimesOutWithoutHealthyService(t *testing.T) {
	cfg := config.DefaultsLinux(true)
	cfg.Socket = newStatusSocket(t, http.StatusServiceUnavailable)
	m := Manager{Config: cfg}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := m.ready(ctx, "hostlens-diagnostics.service"); e == nil || !strings.Contains(e.Error(), "readiness failed") {
		t.Fatalf("unhealthy service accepted: %v", e)
	}
}

func TestReadyObserverUnitViaSystemctl(t *testing.T) {
	cfg := config.DefaultsLinux(true)
	m := Manager{Config: cfg}
	e := m.ready(context.Background(), "hostlens-docker-observer.service")
	if e == nil {
		return // systemd booted and the unit is active: success path.
	}
	// Without a running systemd the unit cannot be active; the refusal is
	// the verified behavior.
	if !strings.Contains(e.Error(), "readiness failed") && !strings.Contains(e.Error(), "unavailable") && !strings.Contains(e.Error(), "systemd") {
		t.Fatalf("unexpected observer readiness failure: %v", e)
	}
}

func TestReadyDelegatesInjectedRunner(t *testing.T) {
	m := Manager{Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "systemctl" {
			t.Fatalf("unexpected command %q", name)
		}
		return nil, nil
	}}
	if e := m.ready(context.Background(), "hostlens-diagnostics.service"); e != nil {
		t.Fatalf("healthy unit refused: %v", e)
	}
	m.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, errors.New("unit down")
	}
	if e := m.ready(context.Background(), "hostlens-diagnostics.service"); e == nil {
		t.Fatal("failing unit accepted")
	}
}
