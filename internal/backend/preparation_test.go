package backend

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
)

func TestPreparationCancellationRetainsOneWorkerAndDiscardsCandidate(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ToolTimeout = 20 * time.Millisecond
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	s := New(Snapshot{Config: cfg, Generation: "old"}, func() (Snapshot, error) {
		calls.Add(1)
		started <- struct{}{}
		<-release
		return Snapshot{Config: cfg, Generation: "new"}, nil
	}, nil, "test")
	var workers sync.WaitGroup
	var closeRelease sync.Once
	defer func() { closeRelease.Do(func() { close(release) }); workers.Wait() }()
	request := func(ctx context.Context) chan int {
		done := make(chan int, 1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/prepare", strings.NewReader(`{"generation":"new"}`)).WithContext(ctx))
			done <- w.Code
		}()
		return done
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := request(ctx)
	<-started
	cancel()
	second := request(context.Background())
	for _, done := range []chan int{first, second} {
		select {
		case code := <-done:
			if code != 503 {
				t.Errorf("cancelled/busy preparation returned %d", code)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("preparation ignored caller cancellation or queued behind stalled I/O")
		}
	}
	if calls.Load() != 1 {
		t.Errorf("stalled preparation created %d workers", calls.Load())
	}
	closeRelease.Do(func() { close(release) })
	workers.Wait()
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.RLock()
		busy, staged := s.preparing, s.pending
		s.mu.RUnlock()
		if staged != nil {
			t.Fatal("abandoned preparation staged a candidate")
		}
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("released loader retained preparation admission")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case code := <-request(context.Background()):
		if code != 200 || calls.Load() != 2 {
			t.Fatalf("fresh preparation did not recover: status=%d workers=%d", code, calls.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("fresh preparation did not finish")
	}
}

func TestPreparationValidatesBeforeActivation(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	s := New(Snapshot{Config: cfg, Generation: "old"}, func() (Snapshot, error) { return Snapshot{Config: cfg, Generation: "new"}, nil }, nil, "test")
	for _, tc := range []struct {
		path, generation string
		want             int
	}{
		{"/prepare", "wrong", 409}, {"/activate", "new", 409}, {"/prepare", "new", 200}, {"/activate", "new", 200},
	} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", tc.path, strings.NewReader(`{"generation":"`+tc.generation+`"}`)))
		if w.Code != tc.want {
			t.Fatalf("%s %s: got%d want%d", tc.path, tc.generation, w.Code, tc.want)
		}
	}
}
