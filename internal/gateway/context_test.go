package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/token"
)

func TestCallPreservesContextFailure(t *testing.T) {
	for _, stage := range []string{"/status", "/call"} {
		for _, cancelled := range []bool{false, true} {
			t.Run(stage+"/"+map[bool]string{false: "timeout", true: "cancelled"}[cancelled], func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				c := Coordinator{Active: backend.Snapshot{Generation: "one"}, HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == stage {
						if cancelled {
							cancel()
						}
						<-r.Context().Done()
						return nil, r.Context().Err()
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"generation":"one"}`)), Header: make(http.Header)}, nil
				})}}
				result, err := c.Call(ctx, "get_os_info", contract.Args{}, "test")
				want, code := context.DeadlineExceeded, "timeout"
				if cancelled {
					want, code = context.Canceled, "cancelled"
				}
				if !errors.Is(err, want) || len(result.Issues) != 1 || result.Issues[0].Code != code {
					t.Fatalf("lost cause: %+v %v", result, err)
				}
			})
		}
	}
}

func TestMCPDeadlineIsNotBackendFailure(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ToolTimeout = 20 * time.Millisecond
	c := Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "one"}, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) { return token.Record{ID: "test", Roles: []string{"health"}}, nil }), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/call" {
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-time.After(time.Second):
				return nil, errors.New("request deadline was not propagated")
			}
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"generation":"one","capabilities":{"get_os_info":true}}`)), Header: make(http.Header)}, nil
	})}}
	r := httptest.NewRequest("POST", "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-06-18")
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"code":"timeout"`) || strings.Contains(w.Body.String(), "backend_unavailable") {
		t.Fatalf("wrong deadline classification: %d %s", w.Code, w.Body.String())
	}
}

type cancelledCollector struct{ started, cancelled chan struct{} }

func (c cancelledCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"get_os_info": true}
}
func (c cancelledCollector) Collect(ctx context.Context, _ string, _ contract.Args) contract.Result {
	close(c.started)
	<-ctx.Done()
	close(c.cancelled)
	return contract.Failure("cancelled")
}

func TestHTTPMCPTimeoutResponseAndDisconnectCancellation(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout", true: "disconnect"}[disconnect], func(t *testing.T) {
			cfg := config.DefaultsLinux(false)
			cfg.Limits.ToolTimeout = 200 * time.Millisecond
			snap := backend.Snapshot{Config: cfg, Generation: "one"}
			collector := cancelledCollector{make(chan struct{}), make(chan struct{})}
			be := backend.New(snap, nil, func(backend.Snapshot) contract.Collector { return collector }, "backend")
			bs := httptest.NewServer(be.Handler())
			defer bs.Close()
			client := bs.Client()
			client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
				return http.DefaultTransport.RoundTrip(r)
			})
			c := Coordinator{Active: snap, HTTP: client, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) { return token.Record{ID: "test", Roles: []string{"health"}}, nil })}
			gs := httptest.NewServer(c.Handler())
			defer gs.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "POST", gs.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer test")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			req.Header.Set("MCP-Protocol-Version", "2025-06-18")
			type outcome struct {
				body string
				err  error
			}
			result := make(chan outcome, 1)
			go func() {
				res, err := gs.Client().Do(req)
				if err != nil {
					result <- outcome{err: err}
					return
				}
				defer res.Body.Close()
				body, err := io.ReadAll(res.Body)
				result <- outcome{string(body), err}
			}()
			select {
			case <-collector.started:
			case <-time.After(time.Second):
				t.Fatal("MCP call did not reach backend")
			}
			if disconnect {
				cancel()
			}
			select {
			case <-collector.cancelled:
			case <-time.After(time.Second):
				t.Fatal("request cancellation did not reach backend")
			}
			select {
			case out := <-result:
				if disconnect {
					if out.err == nil {
						t.Fatal("disconnected request unexpectedly completed")
					}
				} else if out.err != nil || !strings.Contains(out.body, `"code":"timeout"`) {
					t.Fatalf("lost timeout response: %+v", out)
				}
			case <-time.After(time.Second):
				t.Fatal("MCP request did not finish")
			}
		})
	}
}
