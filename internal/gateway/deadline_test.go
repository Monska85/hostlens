package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/token"
)

type deadlineCollector struct {
	observedCollector
	delay time.Duration
}

func (c deadlineCollector) Collect(ctx context.Context, tool string, args contract.Args) contract.Result {
	timer := time.NewTimer(c.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return contract.Failure("timeout")
	case <-timer.C:
		return c.observedCollector.Collect(ctx, tool, args)
	}
}

func TestReloadAppliesRequestAndResponseDeadlines(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		grow  bool
		delay time.Duration
	}{
		{"body-grow", true, 0}, {"body-shrink", false, 0}, {"response-grow", true, 520 * time.Millisecond},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			grow := scenario.grow
			cfg := config.DefaultsLinux(false)
			cfg.Health.Sample = time.Millisecond
			cfg.Limits.ToolTimeout = 50 * time.Millisecond
			if !grow {
				cfg.Limits.ToolTimeout = time.Second
			}
			p, err := policy.CompileLinux(cfg, "/configuration", nil)
			if err != nil {
				t.Fatal(err)
			}
			initial := backend.NewSnapshot(cfg, p)
			cfg.Limits.ToolTimeout = time.Second
			if scenario.delay > 0 {
				cfg.Limits.ToolTimeout = 900 * time.Millisecond
			}
			if !grow {
				cfg.Limits.ToolTimeout = 50 * time.Millisecond
			}
			candidate := backend.NewSnapshot(cfg, p)
			be := backend.New(initial, func() (backend.Snapshot, error) { return candidate, nil }, func(backend.Snapshot) contract.Collector { return deadlineCollector{delay: scenario.delay} }, "be")
			bs := httptest.NewServer(be.Handler())
			defer bs.Close()
			client := bs.Client()
			client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
				return http.DefaultTransport.RoundTrip(r)
			})
			restart, err := RestartFingerprint(initial)
			if err != nil {
				t.Fatal(err)
			}
			c := &Coordinator{Active: initial, Load: func() (backend.Snapshot, error) { return candidate, nil }, Restart: restart, HTTP: client, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) { return token.Record{ID: "test", Roles: []string{"health"}}, nil })}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			served := make(chan error, 1)
			go func() { served <- Serve(ctx, []net.Listener{listener}, c.Handler(), initial.Config) }()
			defer func() {
				cancel()
				if err := <-served; err != nil {
					t.Error(err)
				}
			}()
			if err := c.Reload(context.Background()); err != nil {
				t.Fatal(err)
			}
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(4 * time.Second)); err != nil {
				t.Fatal(err)
			}
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
			if scenario.delay > 0 {
				body = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`
			}
			started := time.Now()
			_, err = fmt.Fprintf(conn, "POST /mcp HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer test\r\nContent-Type: application/json\r\nAccept: application/json, text/event-stream\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body[:1])
			if err != nil {
				t.Fatal(err)
			}
			if scenario.delay == 0 {
				time.Sleep(150 * time.Millisecond)
			}
			if grow {
				if _, err := io.WriteString(conn, body[1:]); err != nil {
					t.Fatal(err)
				}
			}
			response, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if grow && response.StatusCode != http.StatusOK {
				t.Fatal("grown deadline rejected valid request", response.StatusCode)
			}
			if scenario.delay > 0 {
				var reply struct {
					Error  json.RawMessage `json:"error"`
					Result struct {
						IsError           bool            `json:"isError"`
						StructuredContent contract.Result `json:"structuredContent"`
					} `json:"result"`
				}
				if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
					t.Fatal("grown deadline truncated the response", err)
				}
				if len(reply.Error) != 0 || reply.Result.IsError || reply.Result.StructuredContent.Error || reply.Result.StructuredContent.Data["observed"] != float64(0) {
					t.Fatalf("grown deadline did not return observations: %+v", reply)
				}
			}
			if !grow && response.StatusCode == http.StatusOK {
				t.Fatal("stalled body accepted after shrunken deadline")
			}
			if !grow && time.Since(started) > time.Second {
				t.Fatal("stalled body retained the startup deadline")
			}
		})
	}
}

func TestIdleTimeoutRequiresRestart(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/configuration", nil)
	if err != nil {
		t.Fatal(err)
	}
	initial := backend.NewSnapshot(cfg, p)
	restart, err := RestartFingerprint(initial)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Limits.IdleTimeout += time.Second
	candidate := backend.NewSnapshot(cfg, p)
	c := &Coordinator{Active: initial, Restart: restart, Load: func() (backend.Snapshot, error) { return candidate, nil }}
	if err := c.Reload(context.Background()); err == nil || !strings.Contains(err.Error(), "restart required") || c.Status().Generation != initial.Generation {
		t.Fatal("idle timeout silently reloaded", err)
	}
}
