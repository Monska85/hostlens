package backend

import (
	"context"
	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/policy"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
func (c countedCollector) Collect(context.Context, string, contract.Args) contract.Result {
	c.calls.Add(1)
	return contract.Result{}
}

type liveCollector struct{ value *atomic.Int32 }

func (c liveCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"get_os_info": true}
}
func (c liveCollector) Collect(context.Context, string, contract.Args) contract.Result {
	return contract.Result{Data: map[string]any{"value": c.value.Load()}}
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
		if first.Data["value"] == second.Data["value"] || second.Data["value"] != want {
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

type blockedCollector struct {
	started chan struct{}
	once    *sync.Once
}

func (b blockedCollector) Capabilities(context.Context) map[string]bool {
	return map[string]bool{"get_os_info": true}
}
func (b blockedCollector) Collect(ctx context.Context, _ string, _ contract.Args) contract.Result {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return contract.Failure("timeout")
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
	for _, body := range []string{`{"shell":"cat /etc/shadow"}`, `{"version":1,"args":{"command":"id"}}`, `{} {}`} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/call", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("accepted %s", body)
		}
	}
}

type uncancellableCollector struct{ release chan struct{} }

func (c uncancellableCollector) Capabilities(context.Context) map[string]bool { return nil }
func (c uncancellableCollector) Collect(context.Context, string, contract.Args) contract.Result {
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
