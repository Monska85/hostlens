package gateway

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/Monska85/hostlens/internal/backend"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/token"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyChains(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ peer, header, want string }{{"192.0.2.1", "8.8.8.8", "192.0.2.1"}, {"127.0.0.1", "198.51.100.2, 127.0.0.2", "198.51.100.2"}, {"127.0.0.1", "garbage, 198.51.100.2", "127.0.0.1"}, {"::1", "2001:db8::2", "2001:db8::2"}, {"127.0.0.1", "127.0.0.2", "127.0.0.1"}} {
		if got := ClientIP(tt.peer, tt.header, []string{"127.0.0.0/8", "::1"}); got != tt.want {
			t.Errorf("%s -> %s want %s", tt.header, got, tt.want)
		}
	}
}
func TestListenerCleanupAndTLSFailure(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.Server.Bind = []string{"127.0.0.1", "192.0.2.254"}
	cfg.Server.Port = 32149
	cfg.Server.AllowInsecureHTTP = true
	if ls, e := Listen(cfg); e == nil {
		for _, l := range ls {
			l.Close()
		}
		t.Fatal("unavailable bind succeeded")
	}
	cfg.Server.Bind = []string{"127.0.0.1"}
	ls, e := Listen(cfg)
	if e != nil {
		t.Fatal("leaked earlier listener", e)
	}
	for _, l := range ls {
		l.Close()
	}
	cfg.Server.TLS.Enabled = true
	cfg.Server.TLS.CertFile = "/absent"
	cfg.Server.TLS.KeyFile = "/absent"
	if _, e = Listen(cfg); e == nil {
		t.Fatal("TLS downgraded")
	}
}

type observedCollector struct{}

func (observedCollector) Capabilities(context.Context) map[string]bool {
	m := map[string]bool{}
	for _, t := range contract.ToolNames() {
		m[t] = true
	}
	return m
}
func (observedCollector) Collect(context.Context, string, contract.Args) contract.Result {
	return contract.Result{ObservedAt: time.Now(), Data: map[string]any{"observed": 0}}
}
func TestMCPAuthorizationAndReload(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.Snapshot{Config: cfg, Policy: p}
	snap = backend.NewSnapshot(snap.Config, snap.Policy)
	candidate := snap
	factory := func(backend.Snapshot) contract.Collector { return observedCollector{} }
	be := backend.New(snap, func() (backend.Snapshot, error) { return candidate, nil }, factory, "be")
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	restart, err := RestartFingerprint(snap)
	if err != nil {
		t.Fatal(err)
	}
	c := &Coordinator{Active: snap, Tokens: token.Store{Path: cfg.TokenStore}, Load: func() (backend.Snapshot, error) { return candidate, nil }, HTTP: client, Log: slog.Default(), Instance: "gw", Restart: restart}
	store := token.Store{Path: cfg.TokenStore, AdminUID: os.Geteuid()}
	row, secret, e := store.Create("test", []string{"health"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	request := func(body string, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-06-18")
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		return w
	}
	w := request(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`, secret)
	if w.Code != 200 || strings.Contains(w.Body.String(), "query_logs") || !strings.Contains(w.Body.String(), "get_os_info") {
		t.Fatalf("discovery %d %s", w.Code, w.Body.String())
	}
	w = request(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_logs","arguments":{}}}`, secret)
	if !strings.Contains(w.Body.String(), "error") {
		t.Fatal(w.Body.String())
	}
	w = request(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`, secret)
	var response map[string]any
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response["error"] != nil {
		t.Fatal(w.Body.String())
	}
	bs.Close()
	w = request(`{"jsonrpc":"2.0","id":33,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`, secret)
	if !strings.Contains(w.Body.String(), "backend_unavailable") {
		t.Fatal("remembered tool did not report backend failure", w.Body.String())
	}
	if err := store.Update(row.ID, nil, true); err != nil {
		t.Fatal(err)
	}
	w = request(`{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`, secret)
	if w.Code != 401 {
		t.Fatal("revoked bearer accepted")
	}
	candidate.Config.Server.Port++
	candidate = backend.NewSnapshot(candidate.Config, candidate.Policy)
	if c.Reload(context.Background()) == nil || c.Status().Generation != snap.Generation {
		t.Fatal("restart-only reload accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEffectGateRejectsBeforeBackendAccess(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	var backendCalls atomic.Int32
	c := Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "one"}, HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		backendCalls.Add(1)
		return nil, errors.New("unexpected backend access")
	})}}
	result, err := c.Call(context.Background(), "remembered_remediation", contract.Args{}, "request")
	if err == nil || !result.Error || result.Issues[0].Code != "operation_denied" || backendCalls.Load() != 0 {
		t.Fatal("unclassified operation crossed authority boundary", result, err, backendCalls.Load())
	}
	// The known remediation effect obeys the read-only gate: admitted only
	// when the tool is known and the snapshot is writable.
	remediation := contract.ToolDefinition{Effect: contract.EffectRemediation}
	if toolAdmitted(remediation, true, true) || !toolAdmitted(remediation, true, false) || toolAdmitted(remediation, false, false) {
		t.Fatal("known remediation effect bypassed or ignored the read-only gate")
	}
}

func TestBackendLossFallbackAdmitsOnlyKnownTools(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.NewSnapshot(cfg, p)
	be := backend.New(snap, func() (backend.Snapshot, error) { return snap, nil }, func(backend.Snapshot) contract.Collector { return observedCollector{} }, "be")
	bs := httptest.NewServer(be.Handler())
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c := &Coordinator{Active: snap, Tokens: token.Store{Path: cfg.TokenStore}, HTTP: client, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	store := token.Store{Path: cfg.TokenStore, AdminUID: os.Geteuid()}
	_, secret, e := store.Create("test", []string{"health"}, time.Now().Add(time.Hour))
	if e != nil {
		t.Fatal(e)
	}
	request := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-06-18")
		r.Header.Set("Authorization", "Bearer "+secret)
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, r)
		return w
	}
	bs.Close()
	if w := request(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`); !strings.Contains(w.Body.String(), "backend_unavailable") {
		t.Fatal("admitted tool did not report backend failure", w.Body.String())
	}
	// An unknown tool must not receive the backend-failure envelope: fallback
	// admission reuses the fail-closed per-call predicate.
	if w := request(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"unclassified_tool","arguments":{}}}`); strings.Contains(w.Body.String(), "backend_unavailable") {
		t.Fatal("unknown tool received the backend-failure envelope", w.Body.String())
	}
}

type admissionProbe struct {
	values      []bool
	commits     int
	rollbacks   int
	prepareFail bool
	partialFail bool
	nilResult   bool
}

func (p *admissionProbe) Prepare(_ context.Context, value bool) (EffectTransition, error) {
	p.values = append(p.values, value)
	if p.prepareFail {
		return nil, errors.New("mutation still active")
	}
	if p.partialFail {
		return admissionTransition{probe: p}, errors.New("drain failed after admission closed")
	}
	if p.nilResult {
		return nil, nil
	}
	return admissionTransition{probe: p}, nil
}

type admissionTransition struct{ probe *admissionProbe }

func (t admissionTransition) Commit()   { t.probe.commits++ }
func (t admissionTransition) Rollback() { t.probe.rollbacks++ }

func TestReloadClosesRemediationBeforeActivationAndRestoresOnFailure(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.MCP.ReadOnly = false
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	candidateConfig := cfg
	candidateConfig.MCP.ReadOnly = true
	candidate := backend.NewSnapshot(candidateConfig, p)
	probe := &admissionProbe{}
	failActivation := true
	c := Coordinator{Active: active, Load: func() (backend.Snapshot, error) { return candidate, nil }, Effects: probe, Log: slog.Default()}
	restart, err := RestartFingerprint(active)
	if err != nil {
		t.Fatal(err)
	}
	c.Restart = restart
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/prepare" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"prepared":true}`))}, nil
		}
		if failActivation {
			return &http.Response{StatusCode: http.StatusConflict, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"active":true}`))}, nil
	})}
	if err := c.Reload(context.Background()); err == nil || c.Status().MCPReadOnly || len(probe.values) != 1 || !probe.values[0] || probe.commits != 0 || probe.rollbacks != 1 {
		t.Fatal("failed activation did not restore admission", err, probe, c.Status())
	}
	failActivation = false
	probe = &admissionProbe{}
	c.Effects = probe
	if err := c.Reload(context.Background()); err != nil || !c.Status().MCPReadOnly || len(probe.values) != 1 || !probe.values[0] || probe.commits != 1 || probe.rollbacks != 0 {
		t.Fatal("read-only transition was not atomic", err, probe, c.Status())
	}
	candidate = active
	probe = &admissionProbe{}
	c.Effects = probe
	if err := c.Reload(context.Background()); err != nil || c.Status().MCPReadOnly || len(probe.values) != 1 || probe.values[0] || probe.commits != 1 || probe.rollbacks != 0 {
		t.Fatal("writable eligibility transition was not atomic", err, probe, c.Status())
	}
}

func TestReloadRejectsWhenRemediationCannotDrain(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.MCP.ReadOnly = false
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	candidateConfig := cfg
	candidateConfig.MCP.ReadOnly = true
	candidate := backend.NewSnapshot(candidateConfig, p)
	probe := &admissionProbe{prepareFail: true}
	var backendCalls atomic.Int32
	restart, _ := RestartFingerprint(active)
	c := Coordinator{Active: active, Load: func() (backend.Snapshot, error) { return candidate, nil }, Effects: probe, Restart: restart, HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		backendCalls.Add(1)
		return nil, errors.New("unexpected backend call")
	})}}
	if err := c.Reload(context.Background()); err == nil || c.Status().MCPReadOnly || backendCalls.Load() != 0 {
		t.Fatal("undrained remediation activated candidate", err, c.Status(), backendCalls.Load())
	}
}

func TestReloadRollsBackPartiallyPreparedAdmission(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	candidateConfig := cfg
	candidateConfig.MCP.ReadOnly = false
	candidate := backend.NewSnapshot(candidateConfig, p)
	probe := &admissionProbe{partialFail: true}
	restart, _ := RestartFingerprint(active)
	var backendCalls atomic.Int32
	c := Coordinator{Active: active, Load: func() (backend.Snapshot, error) { return candidate, nil }, Effects: probe, Restart: restart, HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		backendCalls.Add(1)
		return nil, errors.New("unexpected backend call")
	})}}
	if err := c.Reload(context.Background()); err == nil || backendCalls.Load() != 0 || probe.rollbacks != 1 || probe.commits != 0 || !c.Status().MCPReadOnly {
		t.Fatal("partial admission preparation was not rolled back", err, backendCalls.Load(), probe, c.Status())
	}
}

func TestReloadRejectsMissingAdmissionTransition(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	candidateConfig := cfg
	candidateConfig.MCP.ReadOnly = false
	candidate := backend.NewSnapshot(candidateConfig, p)
	restart, _ := RestartFingerprint(active)
	var backendCalls atomic.Int32
	c := Coordinator{Active: active, Load: func() (backend.Snapshot, error) { return candidate, nil }, Effects: &admissionProbe{nilResult: true}, Restart: restart, HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		backendCalls.Add(1)
		return nil, errors.New("unexpected backend call")
	})}}
	if err := c.Reload(context.Background()); err == nil || backendCalls.Load() != 0 || !c.Status().MCPReadOnly {
		t.Fatal("nil admission transition activated candidate", err, backendCalls.Load(), c.Status())
	}
}

func TestNativeTLSAndCertificateReload(t *testing.T) {
	t.Parallel()

	template := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer template.Close()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: template.TLS.Certificates[0].Certificate[0]})
	key, e := x509.MarshalPKCS8PrivateKey(template.TLS.Certificates[0].PrivateKey)
	if e != nil {
		t.Fatal(e)
	}
	if err := os.WriteFile(certPath, cert, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultsLinux(false)
	cfg.Server.Port = 0
	cfg.Server.TLS = config.TLS{Enabled: true, CertFile: certPath, KeyFile: keyPath}
	listeners, e := Listen(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer listeners[0].Close()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("TLS transport")) })}
	defer srv.Close()
	go srv.Serve(listeners[0])
	roots := x509.NewCertPool()
	roots.AddCert(template.Certificate())
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots}}, Timeout: time.Second}
	resp, e := client.Get("https://" + listeners[0].Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	snap := backend.Snapshot{Config: cfg}
	first, e := RestartFingerprint(snap)
	if e != nil {
		t.Fatal(e)
	}
	if err := os.WriteFile(certPath, append(cert, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	second, e := RestartFingerprint(snap)
	if e != nil || first == second {
		t.Fatal("TLS material replacement not detected")
	}
}

type resultCollector struct {
	observedCollector
	result contract.Result
}

func (c resultCollector) Collect(context.Context, string, contract.Args) contract.Result {
	return c.result
}

func TestMCPExecutionErrorFlags(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	cfg.TokenStore = filepath.Join(t.TempDir(), "tokens.json")
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.NewSnapshot(cfg, p)
	result := contract.Result{Data: map[string]any{"observed": 0}}
	be := backend.New(snap, func() (backend.Snapshot, error) { return snap, nil }, func(backend.Snapshot) contract.Collector { return resultCollector{result: result} }, "be")
	bs := httptest.NewServer(be.Handler())
	defer bs.Close()
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c := &Coordinator{Active: snap, Tokens: token.Store{Path: cfg.TokenStore}, HTTP: client, Log: slog.Default()}
	store := token.Store{Path: cfg.TokenStore, AdminUID: os.Geteuid()}
	_, secret, err := store.Create("test", []string{"diagnostics"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"success", "coverage", "policy_denied", "source_denied_or_unavailable", "timeout", "overload", "backend_unavailable"} {
		t.Run(code, func(t *testing.T) {
			want := code != "success" && code != "coverage"
			result = contract.Result{Data: map[string]any{"observed": 0}}
			if want {
				result = contract.Failure(code)
			} else if code != "success" {
				result.Issue("retention_unverified", "fixture", "partial coverage")
			}
			r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Accept", "application/json, text/event-stream")
			r.Header.Set("MCP-Protocol-Version", "2025-06-18")
			r.Header.Set("Authorization", "Bearer "+secret)
			w := httptest.NewRecorder()
			c.Handler().ServeHTTP(w, r)
			var response struct {
				Result struct {
					IsError    bool            `json:"isError"`
					Structured contract.Result `json:"structuredContent"`
				} `json:"result"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Result.IsError != want || response.Result.Structured.Error != want {
				t.Fatal(w.Body.String(), err)
			}
			if want && (len(response.Result.Structured.Issues) != 1 || response.Result.Structured.Issues[0].Code != code) {
				t.Fatal("structured failure lost", w.Body.String())
			}
		})
	}
}

func TestRejectedTraversalReloadReleasesStatus(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := backend.NewSnapshot(cfg, p)
	c := &Coordinator{Active: snap, Tokens: token.Store{Path: cfg.TokenStore}, Load: func() (backend.Snapshot, error) {
		candidate := cfg
		candidate.Profiles = []string{"a"}
		defs := map[string]policy.Definition{}
		for i := 0; i < 25; i++ {
			name := strings.Repeat("a", i+1)
			next := name + "a"
			includes := []string{}
			if i < 24 {
				includes = []string{next, next}
			}
			defs[name] = policy.Definition{Profile: config.Profile{Profiles: includes}, Source: "/" + name}
		}
		_, err := policy.CompileLinux(candidate, "/config", defs)
		return backend.Snapshot{}, err
	}}
	start := time.Now()
	if err := c.Reload(context.Background()); err == nil {
		t.Fatal("DAG accepted")
	}
	if c.Status().Generation != snap.Generation || time.Since(start) > time.Second {
		t.Fatal("active generation or reload/status availability lost")
	}
}

type verifyFunc func(string) (token.Record, error)

func (f verifyFunc) Verify(secret string) (token.Record, error) { return f(secret) }

func TestAdmissionBeforeAuthenticationAndRelease(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	entered := make(chan struct{})
	release := make(chan struct{})
	var reads atomic.Int32
	c := &Coordinator{Active: backend.Snapshot{Config: cfg}, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
		if reads.Add(1) == 1 {
			close(entered)
			<-release
		}
		return token.Record{}, errors.New("invalid credential")
	})}
	handler := c.Handler()
	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		r.Header.Set("Authorization", "Bearer invalid")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request() }()
	<-entered
	w := request()
	if w.Code != http.StatusServiceUnavailable || reads.Load() != 1 {
		t.Errorf("overload did not precede token I/O: code=%d reads=%d", w.Code, reads.Load())
	}
	close(release)
	if w = <-done; w.Code != http.StatusUnauthorized {
		t.Fatalf("authentication response: %d", w.Code)
	}
	if w = request(); w.Code != http.StatusUnauthorized || reads.Load() != 2 {
		t.Fatalf("authentication failure leaked admission: code=%d reads=%d", w.Code, reads.Load())
	}
	// A slow admitted backend call holds the slot: the next request is
	// rejected immediately instead of queueing behind it.
	cfg = config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	cfg.Limits.ToolTimeout = 2 * time.Second
	backendEntered := make(chan struct{})
	backendRelease := make(chan struct{})
	bs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			json.NewEncoder(w).Encode(backend.Status{Generation: "g", Capabilities: map[string]bool{"get_os_info": true}})
			return
		}
		close(backendEntered)
		select {
		case <-backendRelease:
		case <-r.Context().Done():
			return
		}
		json.NewEncoder(w).Encode(contract.Result{Data: map[string]any{"observed": true}})
	}))
	defer bs.Close()
	defer close(backendRelease)
	client := bs.Client()
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Host = strings.TrimPrefix(bs.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})
	c = &Coordinator{Active: backend.Snapshot{Config: cfg, Generation: "g"}, HTTP: client, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "client", Roles: []string{"health"}}, nil
	})}
	handler = c.Handler()
	firstDone := make(chan struct{})
	go func() { handler.ServeHTTP(httptest.NewRecorder(), diagnosticRequest()); close(firstDone) }()
	select {
	case <-backendEntered:
	case <-time.After(time.Second):
		t.Fatal("diagnostic call did not start")
	}
	overloadDone := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, diagnosticRequest())
		overloadDone <- w.Code
	}()
	select {
	case code := <-overloadDone:
		if code != http.StatusServiceUnavailable {
			t.Fatalf("expected immediate overload, got %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("admission queued behind the running backend call")
	}
	select {
	case <-firstDone:
		t.Fatal("slow call ended before overload assertion")
	default:
	}
}

func TestDiscoveryCancellationReleasesAdmission(t *testing.T) {
	t.Parallel()

	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancel=%t", cancelRequest), func(t *testing.T) {
			cfg := config.DefaultsLinux(false)
			cfg.Limits.Concurrent = 1
			cfg.Limits.ToolTimeout = 50 * time.Millisecond
			entered := make(chan struct{}, 1)
			var reads atomic.Int32
			c := &Coordinator{Active: backend.Snapshot{Config: cfg}, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
				reads.Add(1)
				return token.Record{ID: "client", Roles: []string{"health"}}, nil
			}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				entered <- struct{}{}
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}}
			r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
			r.Header.Set("Authorization", "Bearer valid")
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Accept", "application/json, text/event-stream")
			ctx, cancel := context.WithCancel(r.Context())
			defer cancel()
			done := make(chan struct{})
			handler := c.Handler()
			go func() { handler.ServeHTTP(httptest.NewRecorder(), r.WithContext(ctx)); close(done) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("backend discovery did not start")
			}
			if cancelRequest {
				cancel()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("backend discovery ignored request lifetime")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/mcp", nil))
			if w.Code != http.StatusUnauthorized || reads.Load() != 1 {
				t.Fatalf("discovery cancellation leaked admission: code=%d reads=%d", w.Code, reads.Load())
			}
		})
	}
}

func diagnosticRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_os_info","arguments":{}}}`))
	r.Header.Set("Authorization", "Bearer valid")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	return r
}

func TestReloadDoesNotBlockRequestDeadlineOrStatus(t *testing.T) {
	for _, discoveryStalls := range []bool{false, true} {
		t.Run(fmt.Sprintf("discovery-stalls=%t", discoveryStalls), func(t *testing.T) {
			cfg := config.DefaultsLinux(false)
			cfg.Limits.Concurrent = 1
			cfg.Limits.ToolTimeout = 50 * time.Millisecond
			p, err := policy.CompileLinux(cfg, "/config", nil)
			if err != nil {
				t.Fatal(err)
			}
			snap := backend.NewSnapshot(cfg, p)
			entered := make(chan struct{})
			release := make(chan struct{})
			var calls atomic.Int32
			c := &Coordinator{Active: snap, Log: slog.Default(), Load: func() (backend.Snapshot, error) {
				close(entered)
				<-release
				return backend.Snapshot{}, errors.New("candidate rejected")
			}, Tokens: verifyFunc(func(string) (token.Record, error) {
				return token.Record{ID: "client", Roles: []string{"health"}}, nil
			}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/status" {
					calls.Add(1)
				}
				if discoveryStalls {
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"capabilities":{"get_os_info":true}}`))}, nil
			})}}
			reloadDone := make(chan error, 1)
			go func() { reloadDone <- c.Reload(context.Background()) }()
			<-entered
			defer func() { close(release); <-reloadDone }()
			statusDone := make(chan backend.Status, 1)
			go func() { statusDone <- c.Status() }()
			select {
			case status := <-statusDone:
				if status.Generation != snap.Generation {
					t.Fatal("candidate exposed before activation")
				}
			case <-time.After(time.Second):
				t.Fatal("status blocked behind reload")
			}
			handler := c.Handler()
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, diagnosticRequest())
				done <- w
			}()
			select {
			case w := <-done:
				if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "configuration reload in progress") {
					t.Fatalf("reload admission response: %d %s", w.Code, w.Body.String())
				}
			case <-time.After(time.Second):
				t.Fatal("accepted request blocked behind reload")
			}
			if calls.Load() != 0 {
				t.Fatal("diagnostic operation admitted during reload")
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/mcp", nil))
			if w.Code != http.StatusUnauthorized {
				t.Fatal("reload request leaked admission", w.Code)
			}
		})
	}
	// Tool discovery holds the generation until the response completes:
	// a reload may not cross an in-flight discovery response.
	cfg := config.DefaultsLinux(false)
	cfg.MCP.ReadOnly = false
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	candidateConfig := cfg
	candidateConfig.MCP.ReadOnly = true
	candidate := backend.NewSnapshot(candidateConfig, p)
	restart, err := RestartFingerprint(active)
	if err != nil {
		t.Fatal(err)
	}
	loadEntered := make(chan struct{})
	c := &Coordinator{Active: active, Restart: restart, Load: func() (backend.Snapshot, error) {
		close(loadEntered)
		return candidate, nil
	}, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "client", Roles: []string{"diagnostics"}}, nil
	}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"prepared":true,"active":true}`
		if r.URL.Path == "/status" {
			body = `{"generation":"` + active.Generation + `","capabilities":{"get_os_info":true}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	request := diagnosticRequest()
	request.Body = io.NopCloser(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	w := &blockingRecorder{ResponseRecorder: httptest.NewRecorder(), entered: make(chan struct{}), release: make(chan struct{})}
	handlerDone := make(chan struct{})
	go func() {
		c.Handler().ServeHTTP(w, request)
		close(handlerDone)
	}()
	select {
	case <-w.entered:
	case <-time.After(time.Second):
		t.Fatal("tool discovery did not reach response write")
	}
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- c.Reload(context.Background()) }()
	select {
	case <-loadEntered:
		t.Fatal("reload crossed an in-flight discovery response")
	case <-time.After(50 * time.Millisecond):
	}
	close(w.release)
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("discovery response did not finish")
	}
	select {
	case err := <-reloadDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reload did not proceed after discovery completed")
	}
}

type blockingRecorder struct {
	*httptest.ResponseRecorder
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockingRecorder) Write(payload []byte) (int, error) {
	w.once.Do(func() {
		close(w.entered)
		<-w.release
	})
	return w.ResponseRecorder.Write(payload)
}

func TestQueuedReloadCannotDeadlockAdmittedToolCall(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	active := backend.NewSnapshot(cfg, p)
	restart, err := RestartFingerprint(active)
	if err != nil {
		t.Fatal(err)
	}
	statusEntered := make(chan struct{})
	releaseStatus := make(chan struct{})
	loadEntered := make(chan struct{})
	var statusOnce sync.Once
	c := Coordinator{Active: active, Restart: restart, Load: func() (backend.Snapshot, error) {
		close(loadEntered)
		return active, nil
	}, Log: slog.Default(), Tokens: verifyFunc(func(string) (token.Record, error) {
		return token.Record{ID: "client", Roles: []string{"health"}}, nil
	}), HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body := `{"prepared":true,"active":true}`
		if r.URL.Path == "/status" {
			statusOnce.Do(func() {
				close(statusEntered)
				<-releaseStatus
			})
			body = `{"generation":"` + active.Generation + `","capabilities":{"get_os_info":true}}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	handlerDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		c.Handler().ServeHTTP(w, diagnosticRequest())
		handlerDone <- w
	}()
	select {
	case <-statusEntered:
	case <-time.After(time.Second):
		t.Fatal("admitted call did not begin capability discovery")
	}
	reloadDone := make(chan error, 1)
	go func() { reloadDone <- c.Reload(context.Background()) }()
	select {
	case <-loadEntered:
		t.Fatal("reload acquired the generation during admitted discovery")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseStatus)
	select {
	case response := <-handlerDone:
		if !strings.Contains(response.Body.String(), "backend_unavailable") {
			t.Fatal("queued reload did not fail nested admission promptly", response.Body.String())
		}
	case <-time.After(time.Second):
		t.Fatal("nested read admission deadlocked with queued reload")
	}
	select {
	case err := <-reloadDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("reload did not proceed after admitted call returned")
	}
}
