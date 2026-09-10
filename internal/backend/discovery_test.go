package backend

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
)

type stalledDiscovery struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *stalledDiscovery) Capabilities(context.Context) map[string]bool {
	if c.calls.Add(1) == 1 {
		close(c.started)
	}
	<-c.release
	return map[string]bool{"get_os_info": true}
}
func (c *stalledDiscovery) Collect(context.Context, string, contract.Args) contract.Result {
	return contract.Result{}
}

func TestStalledDiscoveryBoundsWorkAndLeavesStateResponsive(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ToolTimeout = 50 * time.Millisecond
	collector := &stalledDiscovery{started: make(chan struct{}), release: make(chan struct{})}
	defer close(collector.release)
	s := New(Snapshot{Config: cfg, Generation: "one"}, nil, func(Snapshot) contract.Collector { return collector }, "instance")
	done := make(chan error, 1)
	go func() { _, err := s.Status(context.Background()); done <- err }()
	<-collector.started
	for range 5 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		_, err := s.Status(ctx)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("discovery deadline: %v", err)
		}
	}
	if collector.calls.Load() != 1 {
		t.Fatal("stalled discovery created additional workers")
	}
	callDone := make(chan contract.Result, 1)
	go func() {
		callDone <- s.Call(context.Background(), contract.Request{Version: 1, Generation: "one", Tool: "get_os_info"})
	}()
	select {
	case r := <-callDone:
		if r.Error {
			t.Fatal(r)
		}
	case <-time.After(time.Second):
		t.Fatal("discovery blocked unrelated call")
	}
	s.mu.Lock()
	s.pending = &Snapshot{Config: cfg, Generation: "two"}
	s.mu.Unlock()
	activated := make(chan int, 1)
	go func() {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/activate", strings.NewReader(`{"generation":"two"}`)))
		activated <- w.Code
	}()
	select {
	case code := <-activated:
		if code != 200 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("discovery blocked activation")
	}
	if _, err := s.Status(context.Background()); err == nil {
		t.Fatal("discovery reused previous generation capabilities")
	}
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/status", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "capabilities") {
		t.Fatalf("unavailable discovery pretended success: %d %s", w.Code, w.Body.String())
	}
}

func TestDiscoveryReturnsConsistentSnapshotAndRecovers(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	collector := &stalledDiscovery{started: make(chan struct{}), release: make(chan struct{})}
	close(collector.release)
	s := New(Snapshot{Config: cfg, Generation: "one", Fingerprint: "policy"}, nil, func(Snapshot) contract.Collector { return collector }, "instance")
	for range 2 {
		status, err := s.Status(context.Background())
		if err != nil || status.Generation != "one" || status.Fingerprint != "policy" || !status.Capabilities["get_os_info"] {
			t.Fatalf("%+v %v", status, err)
		}
	}
	if collector.calls.Load() != 2 {
		t.Fatal("completed discovery never refreshed")
	}
}
