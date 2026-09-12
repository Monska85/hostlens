package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/telemetry"
	"github.com/Monska85/hostlens/internal/token"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

func metricsCoordinator(t *testing.T) (*Coordinator, *backend.Server, token.Store) {
	t.Helper()
	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.NewSnapshot(cfg, p)
	be := backend.New(snap, func() (backend.Snapshot, error) { return snap, nil }, func(backend.Snapshot) contract.Collector { return observedCollector{} }, "be")
	bs := httptest.NewServer(be.Handler())
	t.Cleanup(bs.Close)
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	restart, _ := RestartFingerprint(snap)
	c := &Coordinator{Active: snap, HTTP: client, Tokens: token.Store{Path: cfg.TokenStore}, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Restart: restart}
	return c, be, token.Store{Path: cfg.TokenStore, AdminUID: os.Geteuid()}
}
func scrapeRequest(c *Coordinator, method, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/metrics", nil)
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	c.Handler().ServeHTTP(w, r)
	// The worker retains the slot through response writing and context cancellation.
	return w
}
func waitScrape(t *testing.T, c *Coordinator) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for !c.scrapes.Acquire() {
		if time.Now().After(until) {
			t.Fatal("scrape slot retained after completion")
		}
		time.Sleep(time.Millisecond)
	}
	c.scrapes.Release()
}
func parseMetrics(t *testing.T, w *httptest.ResponseRecorder) map[string]*dto.MetricFamily {
	t.Helper()
	if w.Code != 200 || w.Header().Get("Content-Type") != telemetry.ContentType {
		t.Fatalf("scrape: %d %s", w.Code, w.Body.String())
	}
	p := expfmt.NewTextParser(model.LegacyValidation)
	f, err := p.TextToMetricFamilies(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func TestMetricsHTTPAuthorizationAndLifecycle(t *testing.T) {
	c, _, store := metricsCoordinator(t)
	for _, tc := range []struct {
		method, auth string
		status       int
	}{{"GET", "", 401}, {"GET", "Basic nope", 401}, {"GET", "Bearer invalid", 401}, {"POST", "", 405}, {"HEAD", "", 405}} {
		w := scrapeRequest(c, tc.method, tc.auth)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
		if tc.status == 405 && w.Header().Get("Allow") != "GET" {
			t.Fatal("missing Allow")
		}
		waitScrape(t, c)
	}
	for _, role := range []string{"health", "inspect", "diagnostics", "metrics"} {
		row, secret, err := store.Create(role, []string{role}, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		w := scrapeRequest(c, "GET", "Bearer "+secret)
		want := 403
		if role == "metrics" {
			want = 200
		}
		if w.Code != want {
			t.Fatal(role, w.Code, w.Body.String())
		}
		waitScrape(t, c)
		if err = store.Update(row.ID, []string{role, "metrics"}, false); err != nil {
			t.Fatal(err)
		}
		f := parseMetrics(t, scrapeRequest(c, "GET", "Bearer "+secret))
		if f["hostlens_backend_up"].Metric[0].Gauge.GetValue() != 1 {
			t.Fatal("backend missing")
		}
		waitScrape(t, c)
		if err = store.Update(row.ID, []string{"health"}, false); err != nil {
			t.Fatal(err)
		}
		if w = scrapeRequest(c, "GET", "Bearer "+secret); w.Code != 403 {
			t.Fatal("role removal ignored")
		}
		waitScrape(t, c)
		if err = store.Update(row.ID, nil, true); err != nil {
			t.Fatal(err)
		}
		if w = scrapeRequest(c, "GET", "Bearer "+secret); w.Code != 401 {
			t.Fatal("revocation ignored")
		}
		waitScrape(t, c)
	}
	row, old, err := store.Create("rotate", []string{"metrics"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	overlap := time.Now().Add(100 * time.Millisecond)
	_, replacement, err := store.Rotate(row.ID, time.Now().Add(time.Hour), overlap)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{old, replacement} {
		parseMetrics(t, scrapeRequest(c, "GET", "Bearer "+secret))
		waitScrape(t, c)
	}
	time.Sleep(time.Until(overlap) + time.Millisecond)
	if w := scrapeRequest(c, "GET", "Bearer "+old); w.Code != 401 {
		t.Fatal("expired overlap accepted")
	}
	waitScrape(t, c)
	parseMetrics(t, scrapeRequest(c, "GET", "Bearer "+replacement))
	waitScrape(t, c)
	c.Active.Config.Metrics.AllowAnonymous = true
	for _, auth := range []string{"", "Bearer invalid", "Basic invalid"} {
		parseMetrics(t, scrapeRequest(c, "GET", auth))
		waitScrape(t, c)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/mcp", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "192.0.2.1")
	c.Active.Config.Server.TrustedProxies = []string{"127.0.0.1"}
	c.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("anonymous metrics changed MCP")
	}
	c.Active.Config.Metrics.Enabled = false
	for _, method := range []string{"GET", "POST"} {
		if w := scrapeRequest(c, method, "Bearer invalid"); w.Code != 404 {
			t.Fatal("disabled route precedence")
		}
	}
}
func TestMetricsOnlyCannotDiscoverOrExecuteTools(t *testing.T) {
	c, _, store := metricsCoordinator(t)
	_, secret, err := store.Create("scraper", []string{"metrics"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"tools/list", "tools/call"} {
		params := `{}`
		if method == "tools/call" {
			params = `{"name":"get_os_info","arguments":{}}`
		}
		r := diagnosticRequest()
		r.Body = io.NopCloser(strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q,"params":%s}`, method, params)))
		r.ContentLength = -1
		r.Header.Set("Authorization", "Bearer "+secret)
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		if method == "tools/list" && strings.Contains(w.Body.String(), "get_os_info") {
			t.Fatal("metrics discovered tool")
		}
		if method == "tools/call" && !strings.Contains(w.Body.String(), "authorization denied") {
			t.Fatal("metrics executed tool", w.Body.String())
		}
	}
	for _, path := range []string{"/reload", "/status", "/telemetry"} {
		r := httptest.NewRequest("POST", path, nil)
		r.Header.Set("Authorization", "Bearer "+secret)
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		if w.Code != 404 {
			t.Fatal("private endpoint exposed")
		}
	}
}
func TestMetricsAdmissionCancellationAndSnapshot(t *testing.T) {
	c, _, _ := metricsCoordinator(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var reads atomic.Int32
	c.Tokens = verifyFunc(func(string) (token.Record, error) {
		reads.Add(1)
		close(entered)
		<-release
		return token.Record{Roles: []string{"metrics"}}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("GET", "/metrics", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer valid")
	done := make(chan int, 1)
	go func() { w := httptest.NewRecorder(); c.Handler().ServeHTTP(w, r); done <- w.Code }()
	<-entered
	for range 20 {
		if w := scrapeRequest(c, "GET", "Bearer invalid"); w.Code != 503 {
			t.Fatal("overload", w.Code)
		}
	}
	if reads.Load() != 1 {
		t.Fatal("overload read credentials")
	}
	c.mu.Lock()
	c.Active.Config.Metrics.Enabled = false
	c.mu.Unlock()
	if w := scrapeRequest(c, "GET", ""); w.Code != 404 {
		t.Fatal("new snapshot not active")
	}
	cancel()
	select {
	case status := <-done:
		if status != 503 {
			t.Fatal(status)
		}
	case <-time.After(time.Second):
		t.Fatal("stalled auth blocked response cancellation")
	}
	if c.scrapes.Acquire() {
		t.Fatal("stalled credential I/O released admission")
	}
	close(release)
	waitScrape(t, c)
}
func TestMetricsBackendFailuresAndGenerationIsolation(t *testing.T) {
	c, _, _ := metricsCoordinator(t)
	c.Active.Config.Metrics.AllowAnonymous = true
	// A generation mismatch still prevents diagnostics, but telemetry reads no status.
	c.Active.Generation = "different"
	f := parseMetrics(t, scrapeRequest(c, "GET", ""))
	if f["hostlens_backend_up"].Metric[0].Gauge.GetValue() != 1 {
		t.Fatal("generation blocked telemetry")
	}
	waitScrape(t, c)
	if result, err := c.Call(context.Background(), "get_os_info", contract.Args{}, "id"); err == nil || !result.Error {
		t.Fatal("diagnostics bypassed generation")
	}
	for _, body := range []string{`[{"name":"hostlens_process_start_time_seconds","type":1,"metric":[{"label":[{"name":"component","value":"backend"}],"gauge":{}}]}]`, "invalid", "[]", strings.Repeat("x", telemetry.MaxBytes+1)} {
		c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/telemetry" || r.Header.Get("Authorization") != "" {
				t.Error("telemetry crossed boundary")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		f = parseMetrics(t, scrapeRequest(c, "GET", ""))
		if f["hostlens_backend_up"].Metric[0].Gauge.GetValue() != 0 {
			t.Fatal("bad backend marked up")
		}
		for _, family := range f {
			for _, m := range family.Metric {
				for _, l := range m.Label {
					if l.GetName() == "component" && l.GetValue() == "backend" {
						t.Fatal("stale backend observations")
					}
				}
			}
		}
		waitScrape(t, c)
	}
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	start := time.Now()
	f = parseMetrics(t, scrapeRequest(c, "GET", ""))
	if time.Since(start) > 3*time.Second || f["hostlens_backend_up"].Metric[0].Gauge.GetValue() != 0 {
		t.Fatal("backend timeout not bounded")
	}
	waitScrape(t, c)
}
func TestMetricsReloadAtomicity(t *testing.T) {
	c, _, _ := metricsCoordinator(t)
	candidate := c.Active
	c.Load = func() (backend.Snapshot, error) { return candidate, nil }
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	candidate.Config.Metrics.AllowAnonymous = true
	if err := c.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	parseMetrics(t, scrapeRequest(c, "GET", ""))
	waitScrape(t, c)
	candidate.Config.Metrics.AllowAnonymous = false
	if err := c.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if w := scrapeRequest(c, "GET", ""); w.Code != 401 {
		t.Fatal("access transition failed")
	}
	waitScrape(t, c)
	c.Load = func() (backend.Snapshot, error) { return backend.Snapshot{}, errors.New("invalid config") }
	if c.Reload(context.Background()) == nil {
		t.Fatal("invalid reload accepted")
	}
	if w := scrapeRequest(c, "GET", ""); w.Code != 401 {
		t.Fatal("failed reload changed access")
	}
	waitScrape(t, c)
}
func TestScrapeStormAllowsMCPProgress(t *testing.T) {
	c, _, _ := metricsCoordinator(t)
	c.Active.Config.Metrics.AllowAnonymous = true
	c.Tokens = verifyFunc(func(string) (token.Record, error) { return token.Record{Roles: []string{"health"}}, nil })
	server := httptest.NewServer(c.Handler())
	defer server.Close()
	client := server.Client()
	client.Timeout = 5 * time.Second
	var wg sync.WaitGroup
	defer wg.Wait()
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 15 {
				resp, err := client.Get(server.URL + "/metrics")
				if err != nil {
					t.Error(err)
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != 200 && resp.StatusCode != 503 {
					t.Error(resp.StatusCode)
				}
			}
		}()
	}
	for range 10 {
		r := diagnosticRequest()
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(server.URL, "http://")
		r.RequestURI = ""
		r.Host = r.URL.Host
		resp, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || result["error"] != nil {
			t.Fatal("MCP lost progress", err, result)
		}
	}
	wg.Wait()
}

func TestAdmittedScrapeFinishesOriginalSnapshot(t *testing.T) {
	c, _, _ := metricsCoordinator(t)
	c.Active.Config.Metrics.AllowAnonymous = true
	entered := make(chan struct{})
	release := make(chan struct{})
	c.HTTP = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return nil, errors.New("backend offline")
	})}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- scrapeRequest(c, "GET", "") }()
	<-entered
	c.mu.Lock()
	c.Active.Config.Metrics.AllowAnonymous = false
	c.mu.Unlock()
	close(release)
	parseMetrics(t, <-done)
	waitScrape(t, c)
	if w := scrapeRequest(c, "GET", ""); w.Code != 401 {
		t.Fatal("next scrape retained anonymous access")
	}
	waitScrape(t, c)
}

func TestMetricsUnreadBodyHasScrapeDeadline(t *testing.T) {
	for _, anonymous := range []bool{false, true} {
		t.Run(fmt.Sprint(anonymous), func(t *testing.T) {
			c, _, _ := metricsCoordinator(t)
			c.Active.Config.Metrics.AllowAnonymous = anonymous
			completed := make(chan struct{})
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { defer close(completed); c.Handler().ServeHTTP(w, r) }))
			server.Config.ReadTimeout = 65 * time.Second
			server.Start()
			defer server.Close()
			conn, err := net.Dial("tcp", server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if _, err = io.WriteString(conn, "GET /metrics HTTP/1.1\r\nHost: localhost\r\nContent-Length: 1\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			select {
			case <-completed:
			case <-time.After(telemetry.ScrapeTimeout + time.Second):
				t.Fatal("unread body exceeded scrape deadline")
			}
			waitScrape(t, c)
		})
	}
}

func TestMetricsAuditExcludesCredentials(t *testing.T) {
	c, _, store := metricsCoordinator(t)
	var logs bytes.Buffer
	c.Log = slog.New(slog.NewJSONHandler(&logs, nil))
	row, secret, err := store.Create("scraper", []string{"health"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, auth := range []string{"Bearer secret-invalid-marker", "Bearer " + secret} {
		scrapeRequest(c, "GET", auth)
		waitScrape(t, c)
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), "secret-invalid-marker") {
		t.Fatal("credential leaked into audit")
	}
	decoder := json.NewDecoder(&logs)
	for _, want := range []string{"authentication_failed", "authorization_denied"} {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		if record["msg"] != want || record["route"] != "metrics" || record["request_id"] == "" {
			t.Fatal("incomplete audit", record)
		}
		if want == "authentication_failed" && record["token_id"] != "" {
			t.Fatal("invented identity")
		}
		if want == "authorization_denied" && record["token_id"] != row.ID {
			t.Fatal("known identity omitted")
		}
	}
}
