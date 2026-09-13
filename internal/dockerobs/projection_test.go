package dockerobs

import (
	"reflect"
	"testing"
	"time"
)

func TestContainerDataBranchTable(t *testing.T) {
	when := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	empty := ContainerData(ContainerSummary{ID: "abc", Names: []string{"web"}, ImageID: "sha256:x", State: "running"})
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
	if _, ok := empty["names"].([]string); !ok {
		t.Fatal("names must survive projection")
	}

	created := when
	started := when.Add(time.Second)
	finished := when.Add(2 * time.Second)
	restart, pids := 3, int64(128)
	nano, shares, period, quota, mem, swap := int64(1000000), int64(512), int64(100000), int64(90000), int64(1048576), int64(2097152)
	full := ContainerData(ContainerSummary{
		ID: "def", Names: []string{"api"}, ImageID: "sha256:y", State: "exited",
		Image: "app:1", Status: "Exited (0)", Health: "healthy",
		Created: &created, StartedAt: &started, FinishedAt: &finished, RestartCount: &restart,
		Ports:     []PortMapping{{PrivatePort: 80, Type: "tcp"}},
		Mounts:    []MountRef{{Type: "volume", Destination: "/data", VolumeName: "vol"}},
		Networks:  []string{"bridge"},
		LogDriver: "json-file", RestartPolicy: "unless-stopped",
		PidsLimit: &pids, NanoCPUs: &nano, CPUShares: &shares,
		CPUPeriod: &period, CPUQuota: &quota, Memory: &mem, MemorySwap: &swap,
	})
	for _, optional := range []string{
		"image", "status", "health", "created", "started_at", "finished_at", "restart_count",
		"ports", "mounts", "networks", "log_driver", "restart_policy", "pids_limit",
		"nano_cpus", "cpu_shares", "cpu_period", "cpu_quota", "memory_limit", "memory_swap",
	} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["image"] != "app:1" || full["pids_limit"] != pids || full["memory_swap"] != swap {
		t.Fatalf("populated values lost: %v", full)
	}
	if got, ok := full["created"].(string); !ok || got != "2026-09-13T12:00:00Z" {
		t.Fatalf("created projection = %v", full["created"])
	}
}

func TestContainerStatsDataBranchTable(t *testing.T) {
	read := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	nilOnly := ContainerStatsData(&ContainerStats{Read: read})
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
	populated := ContainerStatsData(&ContainerStats{
		Read: read, Preread: read.Add(-time.Second),
		OnlineCPUs: &online, CPUUsage: &cpu, PreCPUUsage: &preCPU,
		SystemUsage: &sys, PreSystem: &preSys, CPUPercent: &percent,
		MemoryUsed: &used, MemoryLimit: &limit, MemoryCache: &cache,
		BlockRead: &blockRead, BlockWrite: &blockWrite,
		Networks:    map[string]NetIO{"eth0": {RxBytes: &blockRead, TxBytes: &blockWrite}},
		PidsCurrent: &pidsCurrent, PidsLimit: &pidsLimit,
	})
	for _, optional := range []string{
		"preread", "online_cpus", "cpu_usage_total_ns", "precpu_usage_total_ns", "system_cpu_usage_ns",
		"pre_system_cpu_usage_ns", "cpu_percent", "memory_used", "memory_limit", "memory_cache",
		"block_read", "block_write", "networks", "pids_current", "pids_limit",
	} {
		if _, ok := populated[optional]; !ok {
			t.Fatalf("populated field %s must be projected", optional)
		}
	}
	if populated["cpu_percent"] != percent || populated["memory_used"] != used {
		t.Fatalf("populated values lost: %v", populated)
	}
}

func TestNetworkDataBranchTable(t *testing.T) {
	empty := NetworkData(NetworkSummary{ID: "n1", Name: "bridge"})
	for _, optional := range []string{"driver", "scope", "internal", "enable_ipv6", "subnets", "container_references"} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if empty["id"] != "n1" || empty["name"] != "bridge" {
		t.Fatalf("identity lost: %v", empty)
	}
	refs := 2
	full := NetworkData(NetworkSummary{
		ID: "n2", Name: "net", Driver: "bridge", Scope: "local",
		Internal: true, EnableIPv6: true, Subnets: []string{"10.0.0.0/24"}, ContainerRefs: &refs,
	})
	for _, optional := range []string{"driver", "scope", "internal", "enable_ipv6", "subnets", "container_references"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["internal"] != true || full["container_references"] != refs {
		t.Fatalf("populated values lost: %v", full)
	}
}

func TestReclaimableDataBranchTable(t *testing.T) {
	empty := ReclaimableData(&Reclaimable{})
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
	if _, ok := empty["criteria"]; !ok {
		t.Fatal("criteria must always be present")
	}
	imageBytes, volumeBytes := int64(1200), int64(300)
	full := ReclaimableData(&Reclaimable{
		ObservationStable: true,
		UnusedImages:      []string{"sha256:a"},
		UnusedImageBytes:  &imageBytes,
		UnusedVolumes:     []string{"vol1"},
		UnusedVolumeBytes: &volumeBytes,
		StoppedContainers: []string{"6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"},
		DanglingImages:    []string{"sha256:b"},
	})
	for _, optional := range []string{
		"unused_image_ids", "unused_image_unique_bytes", "unused_volume_names",
		"unused_volume_bytes", "stopped_container_ids", "dangling_image_ids",
	} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("populated field %s must be projected", optional)
		}
	}
	if full["observation_stable"] != true || full["unused_image_unique_bytes"] != imageBytes {
		t.Fatalf("populated values lost: %v", full)
	}
}

func TestEngineInfoDataBranchTable(t *testing.T) {
	base := EngineInfoData(&EngineInfo{
		ServerVersion: "27.0.1", NegotiatedAPI: "1.51", MinAPI: "1.41",
		OperatingSystem: "Ubuntu", OSType: "linux", Architecture: "amd64",
		KernelVersion: "6.8", StorageDriver: "overlay2", LoggingDriver: "json-file",
		ContainerCount: 5, RunningCount: 3, PausedCount: 0, StoppedCount: 2, ImageCount: 9,
	})
	for _, optional := range []string{"cgroup_driver", "cgroup_version", "experimental", "supported_log_drivers"} {
		if _, ok := base[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	if base["server_version"] != "27.0.1" || base["negotiated_api"] != "1.51" {
		t.Fatalf("core fields lost: %v", base)
	}
	drivers := []string{"json-file", "syslog"}
	full := EngineInfoData(&EngineInfo{
		ServerVersion: "27.0.1", NegotiatedAPI: "1.51", MinAPI: "1.41",
		OperatingSystem: "Debian", OSType: "linux", Architecture: "arm64",
		KernelVersion: "6.8", StorageDriver: "overlay2", LoggingDriver: "journald",
		CgroupDriver: "systemd", CgroupVersion: "2", Experimental: true,
		ContainerCount: 1, RunningCount: 1, StoppedCount: 0, ImageCount: 2,
		SupportedDrivers: drivers,
	})
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

func TestVolumeAndImageProjectionPopulated(t *testing.T) {
	created := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	refs, size, sharedSize := 2, int64(512), int64(64)
	volume := VolumeData(VolumeSummary{
		Name: "data", Driver: "local", Scope: "local", Created: &created,
		RefCount: &refs, Size: &size, Anonymous: true, CurrentlyUnused: true,
	})
	for _, optional := range []string{"scope", "created", "container_references", "size", "anonymous", "currently_unused"} {
		if _, ok := volume[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if volume["size"] != size || volume["currently_unused"] != true {
		t.Fatalf("populated values lost: %v", volume)
	}
	if !reflect.DeepEqual(volume["created"], "2026-09-13T12:00:00Z") {
		t.Fatalf("created projection = %v", volume["created"])
	}

	dangling := ImageData(ImageSummary{ID: "sha256:d", Dangling: true, CurrentlyUnused: true, Size: &size})
	if dangling["dangling"] != true || dangling["currently_unused"] != true || dangling["size"] != size {
		t.Fatalf("image flags lost: %v", dangling)
	}

	full := ImageData(ImageSummary{
		ID: "sha256:f", RepoTags: []string{"app:1"}, RepoDigests: []string{"app@sha256:1"},
		Created: &created, Size: &size, SharedSize: &sharedSize, ContainerRefs: &refs,
	})
	for _, optional := range []string{"repo_tags", "repo_digests", "created", "size", "shared_size", "container_references"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set field %s must be projected", optional)
		}
	}
	if full["container_references"] != refs || full["shared_size"] != sharedSize {
		t.Fatalf("populated image values lost: %v", full)
	}
}

func TestContainerDetailProjectionBranches(t *testing.T) {
	summaryOnly := ContainerDetailData(&ContainerDetail{ContainerSummary: ContainerSummary{ID: "x", ImageID: "sha256:x", State: "running"}})
	for _, optional := range []string{"health_check", "health_failing_streak", "oom_killed", "pid", "network_mode", "endpoints"} {
		if _, ok := summaryOnly[optional]; ok {
			t.Fatalf("unset detail field %s must be omitted", optional)
		}
	}
	failing, pid := 3, 4242
	full := ContainerDetailData(&ContainerDetail{
		ContainerSummary: ContainerSummary{ID: "x", ImageID: "sha256:x", State: "running"},
		HealthCheck:      "configured", HealthFailing: &failing, OomKilled: true,
		Pid: &pid, NetworkMode: "bridge",
		Endpoints: []Endpoint{{Name: "eth0", IPAddress: "172.17.0.2"}},
	})
	for _, optional := range []string{"health_check", "health_failing_streak", "oom_killed", "pid", "network_mode", "endpoints"} {
		if _, ok := full[optional]; !ok {
			t.Fatalf("set detail field %s must be projected", optional)
		}
	}
}

func TestLogPageProjectionEdges(t *testing.T) {
	first := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	last := time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)
	data := LogPageData(&LogPage{
		Records:   []LogRecord{{Stream: "stderr", Message: "boom"}},
		Truncated: true, Skipped: 1, Dropped: 2, FirstEvent: &first, LastEvent: &last,
	})
	if data["malformed_frames"] != 2 {
		t.Fatalf("dropped frames lost: %v", data)
	}
	if data["earliest_returned_event"] != "2026-09-13T10:00:00Z" || data["latest_returned_event"] != "2026-09-13T11:00:00Z" {
		t.Fatalf("event bounds lost: %v", data)
	}
}

func TestDiskUsageProjectionBranches(t *testing.T) {
	layers, cacheSize, rootFs, rw := int64(100), int64(11), int64(10), int64(1)
	empty := DiskUsageData(&DiskUsage{})
	for _, optional := range []string{"layers_size", "build_cache_size", "build_cache_records", "reclaimable"} {
		if _, ok := empty[optional]; ok {
			t.Fatalf("unset field %s must be omitted", optional)
		}
	}
	full := DiskUsageData(&DiskUsage{
		LayersSize:     &layers,
		Images:         []ImageSummary{{ID: "sha256:a", Size: &rw}},
		Containers:     []ContainerRef{{ID: "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f", SizeRootF: &rootFs, SizeRw: &rw}},
		Volumes:        []VolumeSummary{{Name: "v", Driver: "local"}},
		BuildCacheSize: &cacheSize, BuildCacheItems: &[]int{2}[0],
		Reclaimable: &Reclaimable{ObservationStable: true},
	})
	if full["layers_size"] != layers || full["build_cache_size"] != cacheSize {
		t.Fatalf("disk usage values lost: %v", full)
	}
	if _, ok := full["build_cache_semantics"]; !ok {
		t.Fatal("build cache semantics must accompany records")
	}
	containers, ok := full["containers"].([]map[string]any)
	if !ok || len(containers) != 1 || containers[0]["size_root_fs"] != rootFs || containers[0]["size_rw"] != rw {
		t.Fatalf("container refs lost: %v", full["containers"])
	}
}
