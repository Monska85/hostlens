package linux

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
	"github.com/Monska85/hostlens/internal/policy"
)

// transportObserver is a minimal HTTP observer over one Unix socket. It
// stands in for the real observerapp server so transport branches are
// exercised end to end without an engine.
type transportObserver struct {
	listener net.Listener
	path     string
	mu       sync.Mutex
	mode     string
	server   *http.Server
}

func newTransportObserver(t *testing.T) *transportObserver {
	t.Helper()
	path := filepath.Join(t.TempDir(), "observer.sock")
	l, e := net.Listen("unix", path)
	if e != nil {
		t.Fatal(e)
	}
	o := &transportObserver{listener: l, path: path}
	mux := http.NewServeMux()
	mux.HandleFunc("/observe", func(w http.ResponseWriter, r *http.Request) {
		o.mu.Lock()
		mode := o.mode
		o.mu.Unlock()
		switch mode {
		case "badrequest":
			http.Error(w, "observation rejected", http.StatusBadRequest)
		case "malformed":
			fmt.Fprint(w, "{not json")
		case "wrongversion":
			fmt.Fprint(w, `{"version":999,"operation":"container_list"}`)
		case "slow":
			<-r.Context().Done()
		default:
			info := dockerobs.EngineInfo{ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44", OSType: "linux"}
			fmt.Fprintf(w, `{"engine":{"server_version":"28.0.0","negotiated_api":"1.51","min_api":"1.44","os_type":"linux"},"containers":[{"id":"%s","names":["web"],"image_id":"sha256:%s","state":"running"}]}`, runningID, imageID(1))
			_ = info
		}
	})
	o.server = &http.Server{Handler: mux}
	go func() { _ = o.server.Serve(l) }()
	t.Cleanup(func() { _ = o.server.Close() })
	return o
}

func (o *transportObserver) set(mode string) {
	o.mu.Lock()
	o.mode = mode
	o.mu.Unlock()
}

func transportCollector(t *testing.T, observer DockerObserver) *Collector {
	t.Helper()
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = "/var/run/docker.sock"
	c.Docker.ObserverSocket = "/run/hostlens/docker-observer.sock"
	if e := config.ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{Profile: config.Profile{Allow: config.Rules{Docker: []string{"containers"}}}}
	p, e := policy.CompileLinux(cfg, "/etc/hostlens/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	return &Collector{Config: c, Policy: p, Docker: observer}
}

func TestObserverTransportSuccess(t *testing.T) {
	o := newTransportObserver(t)
	c := transportCollector(t, NewObserverClient(o.path))
	response, e := c.observe(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if e != nil {
		t.Fatal(e)
	}
	if len(response.Containers) != 1 || response.Containers[0].Names[0] != "web" {
		t.Fatalf("decoded response lost content: %+v", response)
	}
	// The whole collection path must work through the same transport.
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
}

func TestObserverTransportHTTPRefusal(t *testing.T) {
	o := newTransportObserver(t)
	c := transportCollector(t, NewObserverClient(o.path))
	o.set("badrequest")
	_, e := c.observe(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if e == nil || !strings.Contains(e.Error(), "observer rejected observation (400)") {
		t.Fatalf("HTTP refusal lost: %v", e)
	}
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if !result.Error || result.Issues[0].Code != "docker_unavailable" {
		t.Fatalf("HTTP refusal must surface as unavailable: %+v", result.Issues)
	}
}

func TestObserverTransportConnectionRefused(t *testing.T) {
	dead := filepath.Join(t.TempDir(), "dead.sock")
	c := transportCollector(t, NewObserverClient(dead))
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if !result.Error || result.Issues[0].Code != "docker_unavailable" {
		t.Fatalf("dead socket must surface as docker_unavailable: %+v", result.Issues)
	}
	_, e := c.observe(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if e == nil || !strings.Contains(e.Error(), "unavailable") {
		t.Fatalf("dead socket error lost: %v", e)
	}
}

func TestObserverTransportMalformedBody(t *testing.T) {
	o := newTransportObserver(t)
	c := transportCollector(t, NewObserverClient(o.path))
	o.set("malformed")
	_, e := c.observe(context.Background(), dockerobs.Request{Version: 1, Operation: dockerobs.OpContainerList})
	if e == nil || !strings.Contains(e.Error(), "observer response unavailable") {
		t.Fatalf("malformed body error lost: %v", e)
	}
}

func TestObserverTransportContextCancellation(t *testing.T) {
	o := newTransportObserver(t)
	c := transportCollector(t, NewObserverClient(o.path))
	o.set("slow")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	result := c.Collect(ctx, "list_docker_containers", contract.Args{})
	if !result.Error || result.Issues[0].Code != "cancelled_or_timeout" {
		t.Fatalf("cancellation must surface as cancelled_or_timeout: %+v", result.Issues)
	}
}

func TestDockerUnavailableClassifiesTimeoutAndPlainFailure(t *testing.T) {
	c := transportCollector(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := c.dockerUnavailable(ctx, errors.New("plain"))
	if result.Issues[0].Code != "docker_unavailable" {
		t.Fatalf("plain failure code: %v", result.Issues[0].Code)
	}
	cancel()
	result = c.dockerUnavailable(ctx, context.Cause(ctx))
	if result.Issues[0].Code != "cancelled_or_timeout" {
		t.Fatalf("cancelled failure code: %v", result.Issues[0].Code)
	}
}

func TestDockerStatsBranches(t *testing.T) {
	stopped := summary(runningID, "web", "exited")
	stats := &dockerobs.ContainerStats{Read: time.Now().UTC()}
	c := collectorWith(t, []string{"stats/*"}, nil, &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return []dockerobs.ContainerSummary{stopped} },
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerStats: {Stats: stats},
		},
	})
	result := c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "web"})
	if result.Error {
		t.Fatal(result.Issues)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "container_not_running" {
			found = true
		}
	}
	if !found {
		t.Fatalf("stopped container must report the gap: %+v", result.Issues)
	}

	// A stats projection that never arrives must fail closed.
	c = collectorWith(t, []string{"stats/*"}, nil, &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerStats: {},
		},
	})
	result = c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "docker_unavailable" {
		t.Fatalf("nil stats must fail closed: %+v", result.Issues)
	}

	// A name that re-resolves to a different identity after the observation
	// must not release the sample.
	var mu sync.Mutex
	var calls int
	c = collectorWith(t, []string{"stats/*"}, nil, &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
			}
			return []dockerobs.ContainerSummary{summary(removedID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerStats: {Stats: stats},
		},
	})
	result = c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "identity_changed" {
		t.Fatalf("identity recheck failure lost: %+v", result.Issues)
	}

	// A granted class with a denied specific item stays denied.
	c = collectorWith(t, []string{"stats/*"}, []string{"stats/web"}, &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
	})
	result = c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "policy_denied" {
		t.Fatalf("specific denial must win: %+v", result.Issues)
	}
}

func TestDockerDiskUsageDeniedClass(t *testing.T) {
	usage := &dockerobs.DiskUsage{LayersSize: &[]int64{10}[0]}
	c := collectorWith(t, []string{"containers"}, nil, &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpDiskUsage: {Usage: usage},
		},
	})
	result := c.Collect(context.Background(), "get_docker_disk_usage", contract.Args{})
	if !result.Error || result.Issues[0].Code != "policy_denied" {
		t.Fatalf("ungranted disk usage must stay denied: %+v", result.Issues)
	}
}
func TestDockerLogsBranches(t *testing.T) {
	logs := &dockerobs.LogPage{Records: []dockerobs.LogRecord{{Stream: "stdout", Message: "line"}}}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerLogs: {Logs: logs},
		},
	}
	c := collectorWith(t, []string{"logs/*"}, nil, observer)
	if result := c.Collect(context.Background(), "query_docker_logs", contract.Args{}); !result.Error || result.Issues[0].Code != "invalid_selector" {
		t.Fatalf("missing selector accepted: %+v", result.Issues)
	}
	if result := c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web", RawTail: true}); !result.Error || result.Issues[0].Code != "invalid_bounds" {
		t.Fatalf("tail-mode logs accepted: %+v", result.Issues)
	}
	if result := c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web", Since: "bogus"}); !result.Error || result.Issues[0].Code != "invalid_window" {
		t.Fatalf("bogus window accepted: %+v", result.Issues)
	}
	result := c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if result.Issues[len(result.Issues)-1].Code != "retention_unverified" {
		t.Fatalf("retention gap lost: %+v", result.Issues)
	}
	if result.Source != "docker-observer:"+runningID {
		t.Fatalf("source identity lost: %q", result.Source)
	}

	// A stopped container labels its records as historical.
	observer.containers = func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{summary(runningID, "web", "exited")}
	}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	historical := false
	for _, issue := range result.Issues {
		if issue.Code == "container_not_running" {
			historical = true
		}
	}
	if !historical {
		t.Fatalf("stopped container gap lost: %+v", result.Issues)
	}

	// Missing projection fails closed.
	observer.responses[dockerobs.OpContainerLogs] = dockerobs.Response{}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "docker_unavailable" {
		t.Fatalf("nil logs accepted: %+v", result.Issues)
	}

	// A name that re-resolves elsewhere after the read fails closed.
	var mu sync.Mutex
	var calls int
	observer.containers = func() []dockerobs.ContainerSummary {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		}
		return []dockerobs.ContainerSummary{summary(removedID, "web", "running")}
	}
	observer.responses[dockerobs.OpContainerLogs] = dockerobs.Response{Logs: logs}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "identity_changed" {
		t.Fatalf("identity recheck lost: %+v", result.Issues)
	}

	// Observer-classified driver gaps keep the bounded gap honest.
	observer.containers = func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
	}
	observer.responses[dockerobs.OpContainerLogs] = dockerobs.Response{Failed: true, Issue: "unsupported_driver", Reason: "driver unsupported"}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "driver_gap" {
		t.Fatalf("driver gap lost: %+v", result.Issues)
	}

	// Not-found observations stay bounded gaps.
	observer.responses[dockerobs.OpContainerLogs] = dockerobs.Response{Failed: true, Issue: "not_found", Reason: "engine resource not found"}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "not_found" {
		t.Fatalf("not-found gap lost: %+v", result.Issues)
	}
}
