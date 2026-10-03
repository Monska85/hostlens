//go:build linux

package observer

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/containers"
	"github.com/Monska85/hostlens/internal/localipc"
)

type fakeDocker struct{ calls atomic.Int32 }

const fixtureID = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func (f *fakeDocker) Engine(context.Context) (containers.Engine, error) {
	f.calls.Add(1)
	return containers.Engine{Version: "fixture"}, nil
}
func (f *fakeDocker) Containers(context.Context) (containers.Inventory, error) {
	f.calls.Add(1)
	return containers.Inventory{Containers: []containers.Container{{ID: fixtureID}}}, nil
}
func (f *fakeDocker) Container(_ context.Context, id string) (containers.Container, error) {
	f.calls.Add(1)
	return containers.Container{ID: id}, nil
}
func (f *fakeDocker) Stats(_ context.Context, id string) (containers.Stats, error) {
	f.calls.Add(1)
	return containers.Stats{ID: id}, nil
}
func (f *fakeDocker) Logs(_ context.Context, id string, limit int) (containers.Logs, error) {
	f.calls.Add(1)
	return containers.Logs{ID: id, Limit: limit}, nil
}

func startObserver(t *testing.T, gatewayUID uint32, docker *fakeDocker) string {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "observer.sock")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (Server{Socket: socket, GatewayUID: gatewayUID, Docker: docker}).Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("observer shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("observer did not stop")
		}
	})
	for range 100 {
		if _, err := os.Stat(socket); err == nil {
			return socket
		}
		select {
		case err := <-done:
			t.Fatalf("observer did not start: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("observer socket unavailable")
	return ""
}

func TestNamedObserverAndPeerBoundary(t *testing.T) {
	if _, err := localipc.PeerUID(&net.TCPConn{}); err == nil {
		t.Fatal("non-Unix connection accepted")
	}
	docker := &fakeDocker{}
	socket := startObserver(t, uint32(os.Geteuid()), docker)
	client := NewClient(socket, uint32(os.Geteuid()))
	engine, err := client.Engine(context.Background())
	if err != nil || engine.Version != "fixture" {
		t.Fatalf("engine: %+v, %v", engine, err)
	}
	inventory, err := client.Containers(context.Background())
	if err != nil || len(inventory.Containers) != 1 {
		t.Fatalf("inventory: %+v, %v", inventory, err)
	}
	container, err := client.Container(context.Background(), fixtureID)
	if err != nil || container.ID != fixtureID {
		t.Fatalf("container: %+v, %v", container, err)
	}
	stats, err := client.Stats(context.Background(), fixtureID)
	if err != nil || stats.ID != fixtureID {
		t.Fatalf("stats: %+v, %v", stats, err)
	}
	logs, err := client.Logs(context.Background(), fixtureID, 5)
	if err != nil || logs.ID != fixtureID || logs.Limit != 5 {
		t.Fatalf("logs: %+v, %v", logs, err)
	}
	if _, err := client.Logs(context.Background(), fixtureID, 101); err == nil {
		t.Fatal("unbounded logs reached observer")
	}
	if _, err := client.Container(context.Background(), "../escape"); err == nil {
		t.Fatal("invalid ID reached observer")
	}
	if _, err := NewClient(socket, uint32(os.Geteuid()+1)).Engine(context.Background()); err == nil {
		t.Fatal("wrong observer peer UID accepted")
	}
	if docker.calls.Load() != 5 {
		t.Fatalf("unexpected Docker calls: %d", docker.calls.Load())
	}
}

func TestObserverRejectsWrongGatewayUID(t *testing.T) {
	docker := &fakeDocker{}
	socket := startObserver(t, uint32(os.Geteuid()+1), docker)
	if _, err := NewClient(socket, uint32(os.Geteuid())).Engine(context.Background()); err == nil {
		t.Fatal("wrong gateway UID accepted")
	}
	if docker.calls.Load() != 0 {
		t.Fatal("Docker contacted before peer check")
	}
}
