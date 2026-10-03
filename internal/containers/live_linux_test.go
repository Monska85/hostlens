//go:build linux

package containers

import (
	"context"
	"github.com/moby/moby/client"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestPrivateLiveDocker(t *testing.T) {
	if os.Getenv("HOSTLENS_LIVE_DOCKER_FIXTURES") != "1" {
		t.Skip("private Docker fixture not requested")
	}
	if _, err := os.Stat("/run/hostlens-isolated-docker-test"); err != nil {
		t.Fatal("private nested-engine marker missing")
	}
	socket := os.Getenv("HOSTLENS_LIVE_DOCKER_SOCKET")
	if socket != "/dind/docker.sock" || os.Getenv("DOCKER_HOST") != "unix:///dind/docker.sock" {
		t.Fatal("refusing a non-private Docker endpoint")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := NewReader(socket)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := reader.Engine(ctx)
	if err != nil || engine.Version == "" {
		t.Fatalf("engine: %+v, %v", engine, err)
	}
	name := "hostlens-private-acceptance"
	command := func(args ...string) string {
		output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	id := command("create", "--name", name, "busybox:1.37.0-musl", "sh", "-c", "echo hostlens-private-log; sleep 120")
	t.Cleanup(func() { exec.Command("docker", "rm", "-f", id).Run() })
	if len(id) != 64 {
		t.Fatalf("unexpected container ID %q", id)
	}
	command("start", id)
	listed, err := reader.Containers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if listed.Limit != 500 {
		t.Fatalf("unexpected fixed inventory limit: %d", listed.Limit)
	}
	found := false
	for _, item := range listed.Containers {
		if item.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("running private fixture missing from inventory")
	}
	before, err := reader.Container(ctx, id)
	if err != nil || before.ID != id {
		t.Fatalf("inspect: %+v, %v", before, err)
	}
	stats, err := reader.Stats(ctx, id)
	if err != nil || stats.ID != id || stats.MemoryUsageBytes == nil {
		t.Fatalf("stats: %+v, %v", stats, err)
	}
	var logs Logs
	for attempt := 0; attempt < 10; attempt++ {
		logs, err = reader.Logs(ctx, id, 10)
		if err == nil && len(logs.Chunks) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || logs.ID != id || len(logs.Chunks) == 0 {
		t.Fatalf("logs: %+v, %v", logs, err)
	}
	if _, err := reader.Logs(ctx, id, 101); err == nil {
		t.Fatal("unbounded log request accepted")
	}
	if _, err := reader.(dockerClient).api.ContainerRestart(ctx, id, client.ContainerRestartOptions{}); err == nil {
		t.Fatal("read-only observer transport allowed restart")
	}
	writer, err := NewRestarter(socket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Restart(ctx, id[:12]); err == nil {
		t.Fatal("short container ID bypassed stable identity requirement")
	}
	if _, err := writer.Restart(ctx, strings.Repeat("0", 64)); err == nil {
		t.Fatal("missing container was restarted")
	}
	after, err := writer.Restart(ctx, id)
	if err != nil || after.Invoked == nil || !*after.Invoked || after.Completed == nil || !*after.Completed || after.Container == nil || after.Container.ID != id {
		t.Fatalf("restart: %+v, %v", after, err)
	}
}
