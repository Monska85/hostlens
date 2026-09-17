package dockerobs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// payloadKeys marshals one payload and returns its wire keys. The typed
// constructors are verified at the JSON boundary because that is the contract
// the MCP gateway publishes; omitted optional members and always-present
// required members must match keys-before.txt.
func payloadKeys(t *testing.T, v any) map[string]any {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	return m
}

func TestContainerPayloadBranchTable(t *testing.T) {
	when := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	empty := payloadKeys(t, ToContainer(ContainerSummary{ID: "abc", Names: []string{"web"}, ImageID: "sha256:x", State: "running"}))
	for _, optional := range []string{
		"image", "status", "health", "created", "started_at", "finished_at", "restart_count",
		"ports", "mounts", "networks", "log_driver", "restart_policy", "pids_limit",
		"nano_cpus", "cpu_shares", "cpu_period", "cpu_quota", "memory_limit", "memory_swap",
	} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if empty["id"] != "abc" || empty["image_id"] != "sha256:x" || empty["state"] != "running" {
		t.Fatalf("core fields lost: %v", empty)
	}
	names, ok := empty["names"].([]any)
	if !ok || len(names) != 1 || names[0] != "web" {
		t.Fatalf("names must survive the projection: %v", empty["names"])
	}

	zero := 0
	zeroed := payloadKeys(t, ToContainer(ContainerSummary{ID: "abc", Names: []string{"web"}, ImageID: "sha256:x", State: "running", RestartCount: &zero}))
	if zeroed["restart_count"] != float64(0) {
		t.Fatalf("a supplied zero count must still be projected: %v", zeroed["restart_count"])
	}

	created, started, finished := when, when.Add(time.Second), when.Add(2*time.Second)
	ports := uint16(8080)
	mounts := 1
	populated := payloadKeys(t, ToContainer(ContainerSummary{
		ID: "abc", Names: []string{"web"}, ImageID: "sha256:x", State: "running",
		Image: "nginx:1", Status: "Up", Health: "healthy",
		Created: &created, StartedAt: &started, FinishedAt: &finished,
		RestartCount: &mounts,
		Ports:        []PortMapping{{PrivatePort: ports, Type: "tcp"}},
		Mounts:       []MountRef{{Type: "bind", Destination: "/data", Source: "/srv", RW: true}},
		Networks:     []string{"bridge"},
		LogDriver:    "json-file", RestartPolicy: "no",
	}))
	for _, optional := range []string{
		"image", "status", "health", "created", "started_at", "finished_at", "restart_count",
		"ports", "mounts", "networks", "log_driver", "restart_policy",
	} {
		if _, ok := populated[optional]; !ok {
			t.Fatalf("populated field %s must be projected: %v", optional, populated)
		}
	}
	if populated["created"] != "2026-09-13T12:00:00Z" {
		t.Fatalf("created projection = %v", populated["created"])
	}
	portRow, ok := populated["ports"].([]any)[0].(map[string]any)
	if !ok || portRow["private_port"] != float64(ports) || portRow["type"] != "tcp" {
		t.Fatalf("port row lost: %v", populated["ports"])
	}
	mountRow, ok := populated["mounts"].([]any)[0].(map[string]any)
	if !ok || mountRow["destination"] != "/data" || mountRow["rw"] != true {
		t.Fatalf("mount row lost: %v", populated["mounts"])
	}
}

func TestContainerStatsPayloadBranchTable(t *testing.T) {
	read := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	nilOnly := payloadKeys(t, ToContainerStats(&ContainerStats{Read: read}))
	if nilOnly["read"] != "2026-09-13T12:00:00Z" {
		t.Fatalf("read projection = %v", nilOnly["read"])
	}
	if _, ok := nilOnly["preread"]; ok {
		t.Fatal("zero preread must be omitted")
	}
	for _, optional := range []string{
		"online_cpus", "cpu_usage_total_ns", "precpu_usage_total_ns", "system_cpu_usage_ns",
		"pre_system_cpu_usage_ns", "cpu_percent", "memory_used", "memory_limit", "memory_cache",
		"block_read", "block_write", "networks", "pids_current", "pids_limit",
	} {
		if _, ok := nilOnly[optional]; ok {
			t.Fatalf("nil field %s must be omitted", optional)
		}
	}

	online, pidsCurrent, pidsLimit := 4, 7, 64
	cpu, preCPU, sys, preSys := uint64(100), uint64(40), uint64(1000), uint64(500)
	percent := 12.0
	used, limit, cache := uint64(900), uint64(2048), uint64(64)
	blockRead, blockWrite := uint64(11), uint64(22)
	populated := payloadKeys(t, ToContainerStats(&ContainerStats{
		Read: read, Preread: read.Add(-time.Second),
		OnlineCPUs: &online, CPUUsage: &cpu, PreCPUUsage: &preCPU,
		SystemUsage: &sys, PreSystem: &preSys, CPUPercent: &percent,
		MemoryUsed: &used, MemoryLimit: &limit, MemoryCache: &cache,
		BlockRead: &blockRead, BlockWrite: &blockWrite,
		Networks:    map[string]NetIO{"eth0": {RxBytes: &blockRead, TxBytes: &blockWrite}},
		PidsCurrent: &pidsCurrent, PidsLimit: &pidsLimit,
	}))
	for _, optional := range []string{
		"preread", "online_cpus", "cpu_usage_total_ns", "precpu_usage_total_ns", "system_cpu_usage_ns",
		"pre_system_cpu_usage_ns", "cpu_percent", "memory_used", "memory_limit", "memory_cache",
		"block_read", "block_write", "networks", "pids_current", "pids_limit",
	} {
		if _, ok := populated[optional]; !ok {
			t.Fatalf("populated field %s must be projected: %v", optional, populated)
		}
	}
	if populated["cpu_percent"] != percent || populated["memory_used"] != float64(used) {
		t.Fatalf("populated values lost: %v", populated)
	}
	if populated["sample_scope"] == "" {
		t.Fatal("sample scope must always be present")
	}
}

func TestNetworkPayloadBranchTable(t *testing.T) {
	empty := payloadKeys(t, ToNetwork(NetworkSummary{ID: "n1", Name: "bridge"}))
	for _, optional := range []string{"driver", "scope", "internal", "enable_ipv6", "subnets", "container_references"} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if empty["id"] != "n1" || empty["name"] != "bridge" {
		t.Fatalf("identity lost: %v", empty)
	}
	refs := 2
	full := payloadKeys(t, ToNetwork(NetworkSummary{
		ID: "n2", Name: "net", Driver: "bridge", Scope: "local",
		Internal: true, EnableIPv6: true, Subnets: []string{"10.0.0.0/24"}, ContainerRefs: &refs,
	}))
	for _, optional := range []string{"driver", "scope", "internal", "enable_ipv6", "subnets", "container_references"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["internal"] != true || full["container_references"] != float64(refs) {
		t.Fatalf("populated values lost: %v", full)
	}
}

func TestReclaimablePayloadBranchTable(t *testing.T) {
	empty := payloadKeys(t, ToReclaimable(&Reclaimable{}))
	if empty["observation_stable"] != false {
		t.Fatal("unstable observation must be reported false")
	}
	for _, optional := range []string{
		"unused_image_ids", "unused_image_unique_bytes", "unused_volume_names",
		"unused_volume_bytes", "stopped_container_ids", "dangling_image_ids",
	} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("empty field %s must be omitted", optional)
		}
	}
	if empty["criteria"] == "" || empty["advisory"] == "" {
		t.Fatal("criteria and advisory must always be present")
	}
	imageBytes, volumeBytes := int64(1200), int64(300)
	full := payloadKeys(t, ToReclaimable(&Reclaimable{
		ObservationStable: true,
		UnusedImages:      []string{"sha256:a"},
		UnusedImageBytes:  &imageBytes,
		UnusedVolumes:     []string{"vol1"},
		UnusedVolumeBytes: &volumeBytes,
		StoppedContainers: []string{"6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"},
		DanglingImages:    []string{"sha256:b"},
	}))
	for _, optional := range []string{
		"unused_image_ids", "unused_image_unique_bytes", "unused_volume_names",
		"unused_volume_bytes", "stopped_container_ids", "dangling_image_ids",
	} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("populated field %s must be projected", optional)
		}
	}
	if full["observation_stable"] != true || full["unused_image_unique_bytes"] != float64(imageBytes) {
		t.Fatalf("populated values lost: %v", full)
	}
}

func TestEngineInfoPayloadBranchTable(t *testing.T) {
	base := payloadKeys(t, ToEngineInfo(&EngineInfo{
		ServerVersion: "27.0.1", NegotiatedAPI: "1.51", MinAPI: "1.41",
		OperatingSystem: "Ubuntu", OSType: "linux", Architecture: "amd64",
		KernelVersion: "6.8", StorageDriver: "overlay2", LoggingDriver: "json-file",
		ContainerCount: 5, RunningCount: 3, PausedCount: 0, StoppedCount: 2, ImageCount: 9,
	}))
	for _, optional := range []string{"cgroup_driver", "cgroup_version", "experimental", "supported_log_drivers"} {
		if _, ok := base[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if base["server_version"] != "27.0.1" || base["negotiated_api"] != "1.51" {
		t.Fatalf("core fields lost: %v", base)
	}
	// available is owned by the collector; the constructor leaves it false.
	if base["available"] != false {
		t.Fatalf("availability must default to false: %v", base["available"])
	}
	drivers := []string{"json-file", "syslog"}
	full := payloadKeys(t, ToEngineInfo(&EngineInfo{
		ServerVersion: "27.0.1", NegotiatedAPI: "1.51", MinAPI: "1.41",
		OperatingSystem: "Debian", OSType: "linux", Architecture: "arm64",
		KernelVersion: "6.8", StorageDriver: "overlay2", LoggingDriver: "journald",
		CgroupDriver: "systemd", CgroupVersion: "2", Experimental: true,
		ContainerCount: 1, RunningCount: 1, StoppedCount: 0, ImageCount: 2,
		SupportedDrivers: drivers,
	}))
	for _, optional := range []string{"cgroup_driver", "cgroup_version", "experimental", "supported_log_drivers"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["experimental"] != true {
		t.Fatalf("experimental flag lost: %v", full)
	}
}

func TestSortHelpersOrderAscending(t *testing.T) {
	t.Run("SortContainerIDs", func(t *testing.T) {
		items := []ContainerSummary{{ID: "c2"}, {ID: "c0"}, {ID: "c1"}}
		SortContainerIDs(items)
		if items[0].ID != "c0" || items[1].ID != "c1" || items[2].ID != "c2" {
			t.Fatalf("not ascending: %v", items)
		}
	})
	t.Run("SortImageIDs", func(t *testing.T) {
		items := []ImageSummary{{ID: "i2"}, {ID: "i0"}, {ID: "i1"}}
		SortImageIDs(items)
		if items[0].ID != "i0" || items[1].ID != "i1" || items[2].ID != "i2" {
			t.Fatalf("not ascending: %v", items)
		}
	})
	t.Run("SortVolumeNames", func(t *testing.T) {
		items := []VolumeSummary{{Name: "v2"}, {Name: "v0"}, {Name: "v1"}}
		SortVolumeNames(items)
		if items[0].Name != "v0" || items[1].Name != "v1" || items[2].Name != "v2" {
			t.Fatalf("not ascending: %v", items)
		}
	})
	t.Run("SortNetworkIDs", func(t *testing.T) {
		items := []NetworkSummary{{ID: "n2"}, {ID: "n0"}, {ID: "n1"}}
		SortNetworkIDs(items)
		if items[0].ID != "n0" || items[1].ID != "n1" || items[2].ID != "n2" {
			t.Fatalf("not ascending: %v", items)
		}
	})
	t.Run("empty lists stay usable", func(t *testing.T) {
		containers := []ContainerSummary{}
		SortContainerIDs(containers)
		images := []ImageSummary{}
		SortImageIDs(images)
		volumes := []VolumeSummary{}
		SortVolumeNames(volumes)
		networks := []NetworkSummary{}
		SortNetworkIDs(networks)
		if len(containers)+len(images)+len(volumes)+len(networks) != 0 {
			t.Fatal("lengths changed")
		}
	})
}

func TestVolumeAndImagePayloadPopulated(t *testing.T) {
	created := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	refs, size, sharedSize := 2, int64(512), int64(64)
	volume := payloadKeys(t, ToVolume(VolumeSummary{
		Name: "data", Driver: "local", Scope: "local", Created: &created,
		RefCount: &refs, Size: &size, Anonymous: true, CurrentlyUnused: true,
	}))
	for _, optional := range []string{"scope", "created", "container_references", "size", "anonymous", "currently_unused"} {
		if _, ok := volume[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if volume["size"] != float64(size) || volume["currently_unused"] != true {
		t.Fatalf("populated values lost: %v", volume)
	}
	if volume["created"] != "2026-09-13T12:00:00Z" {
		t.Fatalf("created projection = %v", volume["created"])
	}
	// An absent size stays absent, never zero.
	unsized := payloadKeys(t, ToVolume(VolumeSummary{Name: "v", Driver: "local"}))
	if _, ok := unsized["size"]; ok {
		t.Fatal("missing size must be omitted")
	}

	dangling := payloadKeys(t, ToImage(ImageSummary{ID: "sha256:d", Dangling: true, CurrentlyUnused: true, Size: &size}))
	if dangling["dangling"] != true || dangling["currently_unused"] != true || dangling["size"] != float64(size) {
		t.Fatalf("image flags lost: %v", dangling)
	}

	full := payloadKeys(t, ToImage(ImageSummary{
		ID: "sha256:f", RepoTags: []string{"app:1"}, RepoDigests: []string{"app@sha256:1"},
		Created: &created, Size: &size, SharedSize: &sharedSize, ContainerRefs: &refs,
	}))
	for _, optional := range []string{"repo_tags", "repo_digests", "created", "size", "shared_size", "container_references"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["container_references"] != float64(refs) || full["shared_size"] != float64(sharedSize) {
		t.Fatalf("populated image values lost: %v", full)
	}
}

func TestContainerDetailPayloadBranches(t *testing.T) {
	summaryOnly := payloadKeys(t, ToContainerDetail(&ContainerDetail{ContainerSummary: ContainerSummary{ID: "x", ImageID: "sha256:x", State: "running"}}))
	for _, optional := range []string{"health_check", "health_failing_streak", "oom_killed", "pid", "network_mode", "endpoints"} {
		if _, ok := summaryOnly[optional]; ok {
			t.Fatalf("unset detail field %s must be omitted", optional)
		}
	}
	if summaryOnly["id"] != "x" {
		t.Fatalf("summary fields lost: %v", summaryOnly)
	}
	failing, pid := 3, 4242
	full := payloadKeys(t, ToContainerDetail(&ContainerDetail{
		ContainerSummary: ContainerSummary{ID: "x", ImageID: "sha256:x", State: "running"},
		HealthCheck:      "configured", HealthFailing: &failing, OomKilled: true,
		Pid: &pid, NetworkMode: "bridge",
		Endpoints: []Endpoint{{Name: "eth0", IPAddress: "172.17.0.2"}},
	}))
	for _, optional := range []string{"health_check", "health_failing_streak", "oom_killed", "pid", "network_mode", "endpoints"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set detail field %s must be projected", optional)
		}
	}
	endpoint, ok := full["endpoints"].([]any)[0].(map[string]any)
	if !ok || endpoint["name"] != "eth0" || endpoint["ip_address"] != "172.17.0.2" {
		t.Fatalf("endpoint row lost: %v", full["endpoints"])
	}
}

func TestLogPagePayloadEdges(t *testing.T) {
	first := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	last := time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)
	data := payloadKeys(t, ToLogPage(&LogPage{
		Records:   []LogRecord{{Stream: "stderr", Message: "boom"}},
		Truncated: true, Skipped: 1, Dropped: 2, FirstEvent: &first, LastEvent: &last,
	}))
	if data["malformed_frames"] != float64(2) || data["skipped_frames"] != float64(1) {
		t.Fatalf("frame accounting lost: %v", data)
	}
	if data["earliest_returned_event"] != "2026-09-13T10:00:00Z" || data["latest_returned_event"] != "2026-09-13T11:00:00Z" {
		t.Fatalf("event bounds lost: %v", data)
	}
	if data["truncated"] != true || data["tty"] != false || data["ordering"] == "" {
		t.Fatalf("stream facts lost: %v", data)
	}
	quiet := payloadKeys(t, ToLogPage(&LogPage{}))
	for _, optional := range []string{"truncated", "skipped_frames", "malformed_frames", "earliest_returned_event", "latest_returned_event"} {
		if _, ok := quiet[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if _, ok := quiet["entries"]; !ok {
		t.Fatal("entries must always be present")
	}
}

func TestDiskUsagePayloadBranches(t *testing.T) {
	layers, cacheSize, rootFs, rw := int64(100), int64(11), int64(10), int64(1)
	empty := payloadKeys(t, ToDiskUsage(&DiskUsage{}))
	for _, optional := range []string{"layers_size", "build_cache_size", "build_cache_records", "build_cache_semantics"} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if _, ok := empty["snapshot_consistent"]; !ok || empty["snapshot_consistent"] != false {
		t.Fatal("snapshot_consistent must always be present")
	}
	if empty["reclaimable"] == nil {
		t.Fatal("reclaimable analysis must always be present")
	}
	for _, required := range []string{"images", "containers", "volumes"} {
		rows, ok := empty[required].([]any)
		if !ok || len(rows) != 0 {
			t.Fatalf("%s must always be an array: %v", required, empty[required])
		}
	}
	full := payloadKeys(t, ToDiskUsage(&DiskUsage{
		LayersSize:     &layers,
		Images:         []ImageSummary{{ID: "sha256:a", Size: &rw}},
		Containers:     []ContainerRef{{ID: "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f", SizeRootF: &rootFs, SizeRw: &rw}},
		Volumes:        []VolumeSummary{{Name: "v", Driver: "local"}},
		BuildCacheSize: &cacheSize, BuildCacheItems: &[]int{2}[0],
		Reclaimable: &Reclaimable{ObservationStable: true},
	}))
	if full["layers_size"] != float64(layers) || full["build_cache_size"] != float64(cacheSize) {
		t.Fatalf("disk usage values lost: %v", full)
	}
	if full["build_cache_semantics"] == "" {
		t.Fatal("build cache semantics must accompany records")
	}
	containers, ok := full["containers"].([]any)
	if !ok || len(containers) != 1 {
		t.Fatalf("container refs lost: %v", full["containers"])
	}
	ref, ok := containers[0].(map[string]any)
	if !ok || ref["size_root_fs"] != float64(rootFs) || ref["size_rw"] != float64(rw) {
		t.Fatalf("container ref row lost: %v", containers[0])
	}
	if full["reclaimable"] == nil {
		t.Fatal("reclaimable analysis lost")
	}
}

func TestPagePayloadWire(t *testing.T) {
	page := payloadKeys(t, Page[ImagePayload]{Items: []ImagePayload{{ID: "sha256:a"}}, SnapshotConsistent: false})
	items, ok := page["items"].([]any)
	if !ok || len(items) != 1 || page["snapshot_consistent"] != false {
		t.Fatalf("page wire lost: %v", page)
	}
	empty := payloadKeys(t, Page[ImagePayload]{Items: []ImagePayload{}, SnapshotConsistent: false})
	rows, ok := empty["items"].([]any)
	if !ok || len(rows) != 0 {
		t.Fatalf("empty page must serialize as an array, got %v", empty["items"])
	}
}

func TestPayloadsNeverCarrySecretBearingMembers(t *testing.T) {
	// The projection excludes environment, command, label, and health check
	// output by construction; no payload type may grow such a member.
	b, e := json.Marshal([]any{
		ToEngineInfo(&EngineInfo{ServerVersion: "x"}),
		ToContainer(ContainerSummary{ID: "x"}),
		ToContainerDetail(&ContainerDetail{ContainerSummary: ContainerSummary{ID: "x"}}),
		ToContainerStats(&ContainerStats{}),
		ToImage(ImageSummary{ID: "sha256:x"}),
		ToVolume(VolumeSummary{Name: "v", Driver: "local"}),
		ToNetwork(NetworkSummary{ID: "n", Name: "n"}),
		ToDiskUsage(&DiskUsage{}),
		ToReclaimable(&Reclaimable{}),
		ToLogPage(&LogPage{}),
		DockerLogPage{},
	})
	if e != nil {
		t.Fatal(e)
	}
	for _, banned := range []string{`"env":`, `"command":`, `"labels":`, `"health_output":`, `"health_log":`} {
		if strings.Contains(string(b), banned) {
			t.Fatalf("payload leaked %s member", banned)
		}
	}
}
