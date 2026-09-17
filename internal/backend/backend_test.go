package backend

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
)

func TestSnapshotFingerprintIncludesMCPReadOnly(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	first := NewSnapshot(cfg, p)
	cfg.MCP.ReadOnly = false
	second := NewSnapshot(cfg, p)
	if first.Fingerprint == second.Fingerprint || first.Generation == second.Generation {
		t.Fatal("read-only setting missing from active identity")
	}
	if first.Fingerprint != NewSnapshot(first.Config, p).Fingerprint {
		t.Fatal("equivalent settings changed fingerprint")
	}
}

type countedCollector struct{ calls *atomic.Int32 }

func (c countedCollector) Capabilities(context.Context) map[string]bool { return nil }
func (c countedCollector) Collect(context.Context, string, any) contract.Result {
	c.calls.Add(1)
	return contract.Result{}
}

// liveValue is the typed payload the test collector returns; the backend must
// deliver it unmodified.
type liveValue struct {
	Value int32 `json:"value"`
}

type liveCollector struct{ value *atomic.Int32 }

func (c liveCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"get_os_info": true}
}
func (c liveCollector) Collect(context.Context, string, any) contract.Result {
	return contract.Result{Data: liveValue{c.value.Load()}}
}

func TestEveryRequestReadsLiveEvidence(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	var source atomic.Int32
	s := New(Snapshot{Config: cfg, Generation: "live"}, nil, func(Snapshot) contract.Collector { return liveCollector{&source} }, "backend")
	for index, definition := range contract.ToolDefinitions() {
		request := contract.Request{Version: 1, Generation: "live", Tool: definition.Name}
		first := s.Call(context.Background(), request)
		want := int32(index + 1)
		source.Store(want)
		second := s.Call(context.Background(), request)
		firstValue, firstOK := first.Data.(liveValue)
		secondValue, secondOK := second.Data.(liveValue)
		if !firstOK || !secondOK || firstValue.Value == secondValue.Value || secondValue.Value != want {
			t.Fatalf("%s retained diagnostic evidence: first=%v second=%v", definition.Name, first, second)
		}
	}
}

func TestBackendRejectsUnknownOperationBeforeCollector(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	var calls atomic.Int32
	s := New(Snapshot{Config: cfg, Generation: "one"}, nil, func(Snapshot) contract.Collector { return countedCollector{&calls} }, "backend")
	result := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "remediate_host"})
	if !result.Error || result.Issues[0].Code != "unsupported_operation" || calls.Load() != 0 {
		t.Fatal("unknown operation crossed collector boundary", result, calls.Load())
	}
}

// typedCollector asserts that the backend decodes arguments into the exact
// typed struct named by the tool definition before admitting the worker.
type typedCollector struct {
	calls    *atomic.Int32
	received any
}

func (c *typedCollector) Capabilities(context.Context) map[string]bool { return nil }
func (c *typedCollector) Collect(_ context.Context, tool string, args any) contract.Result {
	c.calls.Add(1)
	c.received = args
	return contract.Result{}
}

func TestValidArgumentsReachCollectorTyped(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	var calls atomic.Int32
	collector := &typedCollector{calls: &calls}
	s := New(Snapshot{Config: cfg, Generation: "one"}, nil, func(Snapshot) contract.Collector { return collector }, "backend")
	result := s.Call(context.Background(), contract.Request{
		Version: 1, Generation: "one", Tool: "read_config",
		Args: []byte(`{"path":"/etc/hostname"}`),
	})
	if result.Error || calls.Load() != 1 {
		t.Fatal("valid arguments did not reach the collector", result, calls.Load())
	}
	args, ok := collector.received.(contract.PathArgs)
	if !ok || args.Path != "/etc/hostname" {
		t.Fatalf("collector received %T %v, want contract.PathArgs", collector.received, collector.received)
	}
}

// TestTypedPayloadsObeyTheResponseCeiling proves Result.Bounded still fails
// closed when a typed payload exceeds the configured ceiling.
func TestTypedPayloadsObeyTheResponseCeiling(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ResponseBytes = 256
	s := New(Snapshot{Config: cfg, Generation: "one"}, nil, func(Snapshot) contract.Collector {
		return ceilingCollector{data: contract.ConfigFile{Content: strings.Repeat("x", 4096)}}
	}, "backend")
	result := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "read_config", Args: []byte(`{"path":"/etc/hostname"}`)})
	if !result.Error || result.Issues[0].Code != "response_limit" || !result.Truncated || result.Data != nil {
		t.Fatal("oversized typed payload leaked", result)
	}
}

type ceilingCollector struct{ data contract.ConfigFile }

func (c ceilingCollector) Capabilities(context.Context) map[string]bool { return nil }
func (c ceilingCollector) Collect(context.Context, string, any) contract.Result {
	return contract.Result{Data: c.data}
}

func TestInvalidArgumentsRejectedWithoutCollectorOrAdmission(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	s := New(Snapshot{Config: cfg, Policy: p, Generation: "one"}, nil, func(Snapshot) contract.Collector { return countedCollector{&calls} }, "backend")
	for name, raw := range map[string]string{
		"unknown member":  `{"path":"/etc/hostname","command":"id"}`,
		"malformed type":  `{"path":5}`,
		"malformed deep":  `{"unit":"nginx","priority":"high"}`,
		"wrong json type": `["path"]`,
	} {
		tool := "read_config"
		if name == "malformed deep" {
			tool = "query_logs"
		}
		result := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: tool, Args: []byte(raw)})
		if !result.Error || result.Issues[0].Code != "invalid_arguments" {
			t.Fatalf("%s: want invalid_arguments, got %v", name, result)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("rejected arguments crossed collector boundary")
	}
}

type blockedCollector struct {
	started chan struct{}
	once    *sync.Once
}

func (b blockedCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"get_os_info": true}
}
func (b blockedCollector) Collect(ctx context.Context, _ string, _ any) contract.Result {
	if b.once != nil {
		b.once.Do(func() { close(b.started) })
	}
	<-ctx.Done()
	return contract.Failure("timeout")
}

func TestInvalidArgumentsConsumeNoAdmissionSlot(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	cfg.Limits.ToolTimeout = 20 * time.Millisecond
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	s := New(Snapshot{Config: cfg, Policy: p, Generation: "one"}, nil, func(Snapshot) contract.Collector { return blockedCollector{started, &sync.Once{}} }, "backend")
	done := make(chan contract.Result)
	go func() {
		done <- s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "get_os_info"})
	}()
	<-started
	// The single admission slot is held by the blocked call; the invalid call
	// must fail on decode instead of waiting on admission.
	if r := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "read_config", Args: []byte(`{"path":5}`)}); r.Issues[0].Code != "invalid_arguments" {
		t.Fatal(r)
	}
	// A valid call still sees the slot held, proving the invalid call consumed nothing.
	if r := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "read_config", Args: []byte(`{"path":"/etc/hostname"}`)}); r.Issues[0].Code != "overload" {
		t.Fatal("rejected arguments consumed an admission slot")
	}
	<-done
}

func TestAdmissionGenerationAndCancellation(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	cfg.Limits.ToolTimeout = 20 * time.Millisecond
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	snap := Snapshot{Config: cfg, Policy: p, Generation: "one"}
	started := make(chan struct{})
	s := New(snap, nil, func(Snapshot) contract.Collector { return blockedCollector{started, &sync.Once{}} }, "backend")
	if r := s.Call(context.Background(), contract.Request{Version: 1, Generation: "wrong"}); r.Issues[0].Code != "generation_mismatch" {
		t.Fatal(r)
	}
	done := make(chan contract.Result)
	go func() {
		done <- s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "get_os_info"})
	}()
	<-started
	r := s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "get_os_info"})
	if r.Issues[0].Code != "overload" {
		t.Fatal(r)
	}
	<-done
}
func TestMalformedIPC(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Snapshot{Config: cfg, Policy: p, Generation: "one"}, nil, nil, "instance")
	for _, body := range []string{`{"shell":"cat /etc/shadow"}`, `{} {}`} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/call", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("accepted %s", body)
		}
	}
	// Args are raw at the transport layer; argument-level garbage is rejected
	// as a typed failure result, not an HTTP error.
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/call",
		strings.NewReader(`{"version":1,"generation":"one","tool":"read_config","args":{"command":"id"}}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "invalid_arguments") {
		t.Fatalf("injection-bearing arguments crossed decode: %d %s", w.Code, w.Body.String())
	}
}

type uncancellableCollector struct{ release chan struct{} }

func (c uncancellableCollector) Capabilities(context.Context) map[string]bool { return nil }
func (c uncancellableCollector) Collect(context.Context, string, any) contract.Result {
	<-c.release
	return contract.Result{ObservedAt: time.Now()}
}
func TestTimeoutKeepsAdmissionUntilBlockedIOEnds(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.Concurrent = 1
	cfg.Limits.ToolTimeout = 10 * time.Millisecond
	p, err := policy.CompileLinux(cfg, "/config", nil)
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	s := New(Snapshot{Config: cfg, Policy: p, Generation: "generation"}, nil, func(Snapshot) contract.Collector { return uncancellableCollector{release} }, "test")
	req := contract.Request{Version: 1, Generation: "generation", Tool: "read_config"}
	start := time.Now()
	r := s.Call(context.Background(), req)
	if r.Issues[0].Code != "timeout" || time.Since(start) > time.Second {
		t.Fatal("blocking I/O bypasses response deadline")
	}
	if r = s.Call(context.Background(), req); r.Issues[0].Code != "overload" {
		t.Fatal("timed-out I/O released shared budget prematurely")
	}
	close(release)
}

func BenchmarkStatus(b *testing.B) {
	for _, rules := range []int{0, 10000} {
		b.Run(strconv.Itoa(rules)+"-rules", func(b *testing.B) {
			cfg := config.DefaultsLinux(false)
			for i := range rules {
				cfg.Allow.Files = append(cfg.Allow.Files, "/approved/"+strconv.Itoa(i))
			}
			p, err := policy.CompileLinux(cfg, "/config", nil)
			if err != nil {
				b.Fatal(err)
			}
			snap := NewSnapshot(cfg, p)
			s := New(snap, nil, func(Snapshot) contract.Collector { return blockedCollector{} }, "benchmark")
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.Status(context.Background())
			}
		})
	}
}
