package backend

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/telemetry"
)

type metricsStalledCollector struct{ entered, release chan struct{} }

func (c metricsStalledCollector) Capabilities(context.Context) map[string]bool {
	panic("telemetry must not discover capabilities")
}
func (c metricsStalledCollector) Collect(context.Context, string, any) contract.Result {
	close(c.entered)
	<-c.release
	return contract.Result{}
}
func TestTelemetrySeparateAdmissionAndNoDiscovery(t *testing.T) {
	cfg := config.DefaultsLinux(false)
	cfg.Limits.ToolTimeout = 20 * time.Millisecond
	collector := metricsStalledCollector{make(chan struct{}), make(chan struct{})}
	s := New(Snapshot{Config: cfg, Generation: "g"}, nil, func(Snapshot) contract.Collector { return collector }, "test")
	s.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	done := make(chan contract.Result, 1)
	go func() {
		done <- s.Call(context.Background(), contract.Request{Version: 1, Generation: "g", Tool: "get_os_info"})
	}()
	<-collector.entered
	result := <-done
	if result.Issues[0].Code != "timeout" {
		t.Fatal(result)
	}
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/telemetry", nil))
		return w
	}
	w := request()
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	families, err := telemetry.DecodeBackend(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatal(err)
	}
	active := 0.
	for _, f := range families {
		if f.GetName() == "hostlens_tool_work_active" {
			active = f.Metric[0].Gauge.GetValue()
		}
	}
	if active != 1 {
		t.Fatal("native timeout lost active work", active)
	}
	until := time.Now().Add(time.Second)
	for !s.scrapes.Acquire() {
		if time.Now().After(until) {
			t.Fatal("slot stuck")
		}
		time.Sleep(time.Millisecond)
	}
	if w = request(); w.Code != 503 {
		t.Fatal("backend telemetry queued")
	}
	s.scrapes.Release()
	close(collector.release)
	until = time.Now().Add(time.Second)
	for {
		s.mu.RLock()
		running := s.running
		s.mu.RUnlock()
		if running == 0 {
			break
		}
		if time.Now().After(until) {
			t.Fatal("call remained active")
		}
		time.Sleep(time.Millisecond)
	}
}
