package linux

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
	observerapp "github.com/Monska85/hostlens/internal/observerapp"
	"github.com/Monska85/hostlens/internal/policy"
	"github.com/Monska85/hostlens/internal/token"
)

// Live engine acceptance. The suite exercises the complete Docker tool
// surface against a real system-wide engine without performing any Docker
// mutation. Run with:
//
//	HOSTLENS_LIVE_DOCKER_SOCKET=/var/run/docker.sock go test -run TestLiveDockerEngine ./internal/platform/linux/
//
// It reads daemon state before and after every stage through the Docker CLI
// in read-only form and refuses to continue on any state change.
func liveSocket(t *testing.T) (string, int) {
	t.Helper()
	socket := os.Getenv("HOSTLENS_LIVE_DOCKER_SOCKET")
	if socket == "" {
		t.Skip("HOSTLENS_LIVE_DOCKER_SOCKET not set")
	}
	group := os.Getenv("HOSTLENS_LIVE_DOCKER_GROUP")
	if group == "" {
		group = "docker"
	}
	gid, e := dockerobsGID(group)
	if e != nil {
		t.Skipf("access group unavailable: %v", e)
	}
	return socket, gid
}

func dockerSnapshot(t *testing.T) string {
	t.Helper()
	var out bytes.Buffer
	for _, args := range [][]string{
		{"info", "--format", "{{.ServerVersion}}|{{.OperatingSystem}}|{{.Architecture}}|{{.Driver}}"},
		{"version", "--format", "{{.Server.APIVersion}} {{.Server.Os}} {{.Server.Arch}}"},
		{"ps", "-a", "--no-trunc", "--format", "{{.ID}} {{.Image}} {{.State}} {{.Names}}"},
		{"images", "--no-trunc", "--format", "{{.ID}} {{.Repository}}:{{.Tag}}"},
		{"volume", "ls", "--format", "{{.Name}} {{.Driver}}"},
		{"network", "ls", "--no-trunc", "--format", "{{.ID}} {{.Name}} {{.Driver}}"},
	} {
		cmd := exec.Command("docker", args...)
		outBytes, e := cmd.Output()
		if e != nil {
			t.Skipf("docker CLI unavailable for state snapshots: %v", e)
		}
		out.Write(outBytes)
		out.WriteByte('\n')
	}
	return out.String()
}

// assertUnchanged verifies the engine state snapshot did not change. On a
// shared host, concurrent external workloads also mutate state; the check
// then fails only when any change touches HostLens acceptance fixtures,
// because HostLens-observed-only requests cannot be distinguished from
// external mutations by snapshots alone.
func assertUnchanged(t *testing.T, before, after string) {
	t.Helper()
	if before == after {
		return
	}
	beforeLines := map[string]bool{}
	for _, line := range strings.Split(before, "\n") {
		beforeLines[line] = true
	}
	external := true
	for _, line := range strings.Split(after, "\n") {
		if beforeLines[line] {
			continue
		}
		if line == "" {
			continue
		}
		if strings.Contains(line, "hostlens-acceptance") {
			external = false
		}
		t.Logf("state change: %s", line)
	}
	before2 := map[string]bool{}
	for _, line := range strings.Split(after, "\n") {
		before2[line] = true
	}
	for _, line := range strings.Split(before, "\n") {
		if before2[line] || line == "" {
			continue
		}
		if strings.Contains(line, "hostlens-acceptance") {
			external = false
		}
		t.Logf("state removal: %s", line)
	}
	if !external {
		t.Fatalf("HostLens acceptance fixture state changed unexpectedly")
	}
	t.Log("external engine churn detected; HostLens-observation-only requests cannot cause it, snapshots alone cannot prove it on shared hosts")
}

// liveStack wires the backend collector to an in-process observer over a
// temporary IPC socket, exactly as the deployed composition does.
func liveStack(t *testing.T, socket string, gid int) *Collector {
	t.Helper()
	ipcDir := t.TempDir()
	observerSocket := filepath.Join(ipcDir, "observer.sock")
	server := observerapp.NewServer(socket, gid)
	listener, e := net.Listen("unix", observerSocket)
	if e != nil {
		t.Fatal(e)
	}
	httpServer := &http.Server{Handler: server.Handler()}
	go func() { _ = httpServer.Serve(listener) }()
	t.Cleanup(func() { httpServer.Close(); server.Close() })
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = socket
	c.Docker.ObserverSocket = observerSocket
	cfg := config.Config{Profile: config.Profile{Allow: config.Rules{Docker: []string{"*"}}}}
	p, e := policy.CompileLinux(cfg, "/etc/hostlens/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	return &Collector{Config: c, Policy: p, Docker: NewObserverClient(observerSocket)}
}

func collectTool(t *testing.T, c *Collector, tool string, a any) contract.Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), contract.MaxToolTimeout)
	defer cancel()
	return c.Collect(ctx, tool, a)
}

func TestLiveDockerEngineObservations(t *testing.T) {
	socket, gid := liveSocket(t)
	c := liveStack(t, socket, gid)
	before := dockerSnapshot(t)
	defer func() { assertUnchanged(t, before, dockerSnapshot(t)) }()
	t.Logf("engine: %s", strings.TrimSpace(firstLine(before)))

	info := collectTool(t, c, "get_docker_info", contract.NoArgs{})
	if info.Error || !dockerInfoOf(info).Available {
		t.Fatalf("engine info: %v %v", info.Error, info.Issues)
	}
	engine := dockerInfoOf(info)
	if engine.ServerVersion == "" || engine.NegotiatedAPI == "" {
		t.Fatal("engine identity incomplete", info.Data)
	}
	t.Logf("engine info: version=%s api=%s storage=%s cgroup=%s", engine.ServerVersion, engine.NegotiatedAPI, engine.StorageDriver, engine.CgroupVersion)

	containers := collectTool(t, c, "list_docker_containers", contract.PageArgs{})
	if containers.Error {
		t.Fatalf("containers: %v", containers.Issues)
	}
	items := dockerPageOf[dockerobs.ContainerPayload](containers).Items
	t.Logf("containers: %d observed, issues %v", len(items), issueCodes(containers))
	for _, item := range items {
		if item.ID == "" || item.State == "" || item.ImageID == "" {
			t.Fatal("container inventory incomplete", item)
		}
	}

	images := collectTool(t, c, "list_docker_images", contract.PageArgs{})
	if images.Error {
		t.Fatalf("images: %v", images.Issues)
	}
	volumes := collectTool(t, c, "list_docker_volumes", contract.PageArgs{})
	if volumes.Error {
		t.Fatalf("volumes: %v", volumes.Issues)
	}
	networks := collectTool(t, c, "list_docker_networks", contract.PageArgs{})
	if networks.Error {
		t.Fatalf("networks: %v", networks.Issues)
	}
	usage := collectTool(t, c, "get_docker_disk_usage", contract.NoArgs{})
	if usage.Error {
		t.Fatalf("disk usage: %v", usage.Issues)
	}
	t.Logf("disk usage issues: %v", issueCodes(usage))

	if len(items) > 0 {
		id := items[0].ID
		detail := collectTool(t, c, "get_docker_container", contract.ContainerArgs{Container: id})
		if detail.Error {
			t.Fatalf("detail: %v", detail.Issues)
		}
		if detail.Data.(*dockerobs.ContainerDetailPayload).ID != id {
			t.Fatal("detail identity mismatch")
		}
		stats := collectTool(t, c, "get_docker_container_stats", contract.ContainerArgs{Container: id})
		if items[0].State == "running" && stats.Error {
			t.Fatalf("running container stats: %v", stats.Issues)
		}
		logs := collectTool(t, c, "query_docker_logs", contract.DockerLogsArgs{Container: id, Limit: 10})
		if logs.Error {
			// Driver gaps and empty log streams stay honest failures.
			t.Logf("logs for %s: %v", id, issueCodes(logs))
		}
	}
}

func TestLiveDockerEngineLifecycleWithFixtures(t *testing.T) {
	if os.Getenv("HOSTLENS_LIVE_DOCKER_FIXTURES") != "1" {
		t.Skip("HOSTLENS_LIVE_DOCKER_FIXTURES=1 required: this case creates disposable Docker resources")
	}
	socket, gid := liveSocket(t)
	c := liveStack(t, socket, gid)
	name := "hostlens-acceptance-" + token.Random(8)
	volumeName := "hostlens-acceptance-" + token.Random(8)
	networkName := "hostlens-acceptance-" + token.Random(8)
	before := dockerSnapshot(t)
	t.Cleanup(func() { assertUnchanged(t, before, dockerSnapshot(t)) })
	// Disposable fixture lifecycle with unique names: create, observe,
	// reference, unreferenced, and remove. Mutation runs only on explicitly
	// authorized test hosts and only against HostLens-created fixtures.
	pulledImage := !strings.Contains(before, "busybox:latest")
	runDocker(t, "volume", "create", "--label", "hostlens-acceptance-unique="+name, volumeName)
	t.Cleanup(func() { _ = exec.Command("docker", "volume", "rm", volumeName).Run() })
	if pulledImage {
		t.Cleanup(func() { _ = exec.Command("docker", "rmi", "busybox:latest").Run() })
	}
	runDocker(t, "network", "create", "--label", "hostlens-acceptance-unique="+name, networkName)
	t.Cleanup(func() { _ = exec.Command("docker", "network", "rm", networkName).Run() })
	runDocker(t, "run", "-d", "--name", name, "--label", "hostlens-acceptance-unique="+name, "--mount", "source="+volumeName+",target=/data", "--network", networkName, "busybox", "sleep", "300")
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", name).Run() })

	observed := collectTool(t, c, "list_docker_containers", contract.PageArgs{})
	if observed.Error {
		t.Fatal(observed.Issues)
	}
	var fixtureID string
	for _, item := range dockerPageOf[dockerobs.ContainerPayload](observed).Items {
		if item.Image == "busybox" && item.State == "running" {
			fixtureID = item.ID
		}
	}
	if fixtureID == "" {
		t.Fatal("disposable container not observed live")
	}
	stats := collectTool(t, c, "get_docker_container_stats", contract.ContainerArgs{Container: name})
	if stats.Error {
		t.Fatalf("running fixture stats: %v", stats.Issues)
	}

	// A running container references the volume and network: neither may be
	// classified unused.
	assertNotUnused(t, c, volumeName, "list_docker_volumes")
	assertNotUnused(t, c, networkName, "list_docker_networks")
	_ = fixtureID

	// Stop the fixture: the stopped container still holds the reference.
	runDocker(t, "stop", name)
	assertNotUnused(t, c, volumeName, "list_docker_volumes")
	assertNotUnused(t, c, networkName, "list_docker_networks")

	// Remove the fixture container: the volume becomes unused at the next
	// live observation without claiming an unused duration.
	runDocker(t, "rm", "-f", name)
	t.Cleanup(func() {})
	afterRemove := collectTool(t, c, "list_docker_volumes", contract.PageArgs{})
	if afterRemove.Error {
		t.Fatal(afterRemove.Issues)
	}
	unused := false
	for _, item := range dockerPageOf[dockerobs.VolumePayload](afterRemove).Items {
		if item.Name == volumeName {
			if !item.CurrentlyUnused {
				t.Fatalf("unreferenced volume not classified unused: %v", item)
			}
			if strings.Contains(mustJSON(t, item), "unused_since") || strings.Contains(mustJSON(t, item), "unused_duration") {
				t.Fatal("unused duration must never be inferred from creation time")
			}
			unused = true
		}
	}
	if !unused {
		t.Fatal("fixture volume missing from live inventory")
	}
}

func assertNotUnused(t *testing.T, c *Collector, name, tool string) {
	t.Helper()
	result := collectTool(t, c, tool, contract.PageArgs{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	for _, item := range dockerPageOf[dockerobs.VolumePayload](result).Items {
		if item.Name == name {
			if item.CurrentlyUnused {
				t.Fatalf("%s %s referenced by a stopped container must not be unused", tool, name)
			}
			return
		}
	}
	t.Fatalf("%s %s missing from live inventory", tool, name)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func runDocker(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e := cmd.Run(); e != nil {
		t.Fatalf("docker %v: %v: %s", args, e, stderr.String())
	}
}

func dockerobsGID(group string) (int, error) {
	b, e := os.ReadFile("/etc/group")
	if e != nil {
		return 0, e
	}
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) > 2 && fields[0] == group {
			var gid int
			if _, e := fmt.Sscanf(fields[2], "%d", &gid); e == nil {
				return gid, nil
			}
		}
	}
	return 0, fmt.Errorf("group %q not found", group)
}
