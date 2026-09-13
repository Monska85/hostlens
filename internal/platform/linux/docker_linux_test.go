package linux

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Monska85/hostlens/internal/config"
	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
	"github.com/Monska85/hostlens/internal/policy"
)

const (
	runningID   = "6f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"
	stoppedID   = "7f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"
	unhealthyID = "8f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"
	removedID   = "9f9c2f5f0f0e0c1d2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f"
)

func summary(id, name, state string) dockerobs.ContainerSummary {
	return dockerobs.ContainerSummary{
		ID: id, Names: []string{name}, ImageID: "sha256:" + imageID(1), State: state,
		Status: state, Health: "none",
	}
}

func imageID(seed int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = hex[(seed+i+3)%16]
	}
	return string(b)
}

// fakeObserver serves canned typed responses and records requests.
type fakeObserver struct {
	mu         sync.Mutex
	responses  map[string]dockerobs.Response
	requests   []dockerobs.Request
	err        error
	containers func() []dockerobs.ContainerSummary
}

func (f *fakeObserver) Observe(ctx context.Context, request dockerobs.Request) (dockerobs.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, request)
	f.mu.Unlock()
	if f.err != nil {
		return dockerobs.Response{}, f.err
	}
	if request.Operation == dockerobs.OpContainerList && f.containers != nil {
		return dockerobs.Response{Containers: f.containers()}, nil
	}
	if response, ok := f.responses[request.Operation]; ok {
		return response, nil
	}
	return dockerobs.Response{Failed: true, Reason: "canned response missing"}, nil
}

func collectorWith(t *testing.T, allow, deny []string, observer DockerObserver) *Collector {
	t.Helper()
	c := config.DefaultsLinux(true)
	c.Docker.Enabled = true
	c.Docker.DaemonSocket = "/var/run/docker.sock"
	c.Docker.ObserverSocket = "/run/hostlens/docker-observer.sock"
	if e := config.ValidateLinux(c); e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{Profile: config.Profile{Allow: config.Rules{Docker: allow}, Deny: config.Rules{Docker: deny}}}
	p, e := policy.CompileLinux(cfg, "/etc/hostlens/config.yaml", nil)
	if e != nil {
		t.Fatal(e)
	}
	return &Collector{Config: c, Policy: p, Docker: observer}
}

func engineResponse(t *testing.T) *dockerobs.EngineInfo {
	return &dockerobs.EngineInfo{
		ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44",
		OSType: "linux", Architecture: "amd64", StorageDriver: "overlay2",
		LoggingDriver: "json-file", CgroupVersion: "2",
		ContainerCount: 2, RunningCount: 1, StoppedCount: 1, ImageCount: 3,
	}
}

func TestDockerInfoAvailableAndUnsupportedModes(t *testing.T) {
	t.Parallel()

	info := dockerobs.EngineInfo{ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44"}
	c := collectorWith(t, []string{"daemon"}, nil, &fakeObserver{responses: map[string]dockerobs.Response{
		dockerobs.OpEngineInfo: {Engine: &info},
	}})
	result := c.Collect(context.Background(), "get_docker_info", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if result.Data["available"] != true || result.Data["server_version"] != "28.0.0" {
		t.Fatal(result.Data)
	}
	for _, banned := range []string{"registry", "proxy", "plugin"} {
		if strings.Contains(strings.ToLower(mustJSON(t, result)), banned) {
			t.Fatalf("daemon info leaked %s", banned)
		}
	}
	rootless := dockerobs.EngineInfo{ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44", UnsupportedReasons: []string{"rootless engine"}}
	c = collectorWith(t, []string{"daemon"}, nil, &fakeObserver{responses: map[string]dockerobs.Response{
		dockerobs.OpEngineInfo: {Engine: &rootless},
	}})
	result = c.Collect(context.Background(), "get_docker_info", contract.Args{})
	if !result.Error || result.Data["available"] != false {
		t.Fatal("rootless engine must stay explicitly unsupported")
	}
}

func TestDockerDisabledAndUnavailable(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(true)
	p, e := policy.CompileLinux(config.Config{}, "/x", nil)
	if e != nil {
		t.Fatal(e)
	}
	c := &Collector{Config: cfg, Policy: p}
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if result.Issues[0].Code != "docker_disabled" {
		t.Fatal(result)
	}
	c = collectorWith(t, []string{"containers"}, nil, &fakeObserver{err: errors.New("observer down")})
	result = c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if !result.Error || result.Issues[0].Code != "docker_unavailable" {
		t.Fatal(result)
	}
}

// TestDockerUngrantedCallsNeverReachTheObserver proves the dispatcher gate:
// a diagnostics client without any docker grant cannot contact the observer,
// and denial of a collection form wins before any observation.
// TestDockerDefaultPageFitsResponseBudget proves a full default-page
// observation is not discarded by the backend response ceiling: the page
// clamps and hands the client a next offset instead.
func TestDockerDefaultPageFitsTheResponseBudget(t *testing.T) {
	t.Parallel()

	items := make([]dockerobs.ContainerSummary, 0, 300)
	for i := range 300 {
		items = append(items, summary(id64For(i+100), fmt.Sprintf("svc-%d", i), "running"))
	}
	observer := &fakeObserver{containers: func() []dockerobs.ContainerSummary { return items }}
	c := collectorWith(t, []string{"containers"}, nil, observer)
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if result.Error {
		t.Fatalf("default page failed: %v", result.Issues)
	}
	page := result.Data["items"].([]map[string]any)
	b := mustJSONBytes(result)
	if len(page) == 0 || len(page) >= 300 {
		t.Fatalf("unexpected page size %d", len(page))
	}
	if len(b) >= c.Config.Limits.ResponseBytes {
		t.Fatalf("page exceeds the response budget: %d bytes", len(b))
	}
	if result.NextOffset == nil || *result.NextOffset != len(page) {
		t.Fatalf("clamped page lost its continuation: %v", result.NextOffset)
	}
	// An explicit over-budget limit clamps with a continuation instead of a
	// wholesale response_limit failure.
	result = c.Collect(context.Background(), "list_docker_containers", contract.Args{Limit: c.Config.Limits.PageSize})
	if result.Error {
		t.Fatal(result.Issues)
	}
}

func mustJSONBytes(r contract.Result) []byte {
	b, _ := json.Marshal(r)
	return b
}

func TestDockerUngrantedCallsNeverReachTheObserver(t *testing.T) {
	t.Parallel()

	for _, tool := range []string{
		"get_docker_info", "list_docker_containers", "get_docker_container",
		"get_docker_container_stats", "list_docker_images", "list_docker_volumes",
		"list_docker_networks", "get_docker_disk_usage", "query_docker_logs",
	} {
		observer := &fakeObserver{responses: map[string]dockerobs.Response{
			dockerobs.OpEngineInfo: {Engine: &dockerobs.EngineInfo{ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44"}},
		}}
		c := collectorWith(t, nil, nil, observer)
		result := c.Collect(context.Background(), tool, contract.Args{Container: "web"})
		if len(observer.requests) != 0 {
			t.Fatalf("%s contacted the observer without any grant: %+v", tool, observer.requests)
		}
		if !result.Error || result.Issues[0].Code != "policy_denied" {
			t.Fatalf("%s ungranted result: %+v", tool, result)
		}
	}
	// Deny precedence at class level: a wildcard allow with an explicit
	// collection denial stays rejected.
	for _, tc := range []struct{ allow, deny []string }{
		{[]string{"*"}, []string{"daemon"}},
		{[]string{"*"}, []string{"disk_usage"}},
		{[]string{"containers"}, []string{"containers"}},
	} {
		observer := &fakeObserver{}
		c := collectorWith(t, tc.allow, tc.deny, observer)
		tool := "get_docker_info"
		if tc.deny[0] == "disk_usage" {
			tool = "get_docker_disk_usage"
		}
		if tc.deny[0] == "containers" {
			tool = "list_docker_containers"
		}
		result := c.Collect(context.Background(), tool, contract.Args{})
		if len(observer.requests) != 0 {
			t.Fatalf("denied %s contacted the observer", tc.deny[0])
		}
		if !result.Error || result.Issues[0].Code != "policy_denied" {
			t.Fatalf("denied %s result: %+v", tool, result)
		}
	}
}

// TestDockerDiskUsageFiltersDeniedContainers proves that denied containers
// contribute neither their inventory row nor their stopped-container ID.
func TestDockerDiskUsageFiltersDeniedContainers(t *testing.T) {
	t.Parallel()

	containers := []dockerobs.ContainerSummary{summary(stoppedID, "db", "exited")}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return containers },
		responses: map[string]dockerobs.Response{
			dockerobs.OpDiskUsage: {Usage: &dockerobs.DiskUsage{
				LayersSize: int64Ptr(1000),
				Containers: []dockerobs.ContainerRef{{ID: stoppedID, SizeRw: int64Ptr(77)}},
			}},
		},
	}
	c := collectorWith(t, []string{"disk_usage"}, []string{"container/db"}, observer)
	result := c.Collect(context.Background(), "get_docker_disk_usage", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if rows := result.Data["containers"].([]map[string]any); len(rows) != 0 {
		t.Fatalf("denied container inventory row leaked: %v", rows)
	}
	if stopped := result.Data["reclaimable"].(map[string]any)["stopped_container_ids"]; stopped != nil {
		t.Fatalf("denied stopped container leaked: %v", stopped)
	}
}

// TestDockerImageTagAndNetworkNameDenialsApply proves that deny rules
// written through practical name forms exclude inventory items.
func TestDockerImageTagAndNetworkNameDenialsApply(t *testing.T) {
	t.Parallel()

	image := dockerobs.ImageSummary{ID: "sha256:" + imageID(5), RepoTags: []string{"secretimg:1"}}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return nil },
		responses: map[string]dockerobs.Response{
			dockerobs.OpImageList: {Images: []dockerobs.ImageSummary{image}},
		},
	}
	c := collectorWith(t, []string{"images"}, []string{"image/secretimg:1"}, observer)
	result := c.Collect(context.Background(), "list_docker_images", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if items := result.Data["items"].([]map[string]any); len(items) != 0 {
		t.Fatalf("tag-denied image listed: %v", items)
	}
	network := dockerobs.NetworkSummary{ID: id64For(7), Name: "secret-net"}
	observer.responses[dockerobs.OpNetworkList] = dockerobs.Response{Networks: []dockerobs.NetworkSummary{network}}
	c = collectorWith(t, []string{"networks"}, []string{"network/secret-net*"}, observer)
	result = c.Collect(context.Background(), "list_docker_networks", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if items := result.Data["items"].([]map[string]any); len(items) != 0 {
		t.Fatalf("name-denied network listed: %v", items)
	}
}

func TestContainerListFiltersDeniedAndPages(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{containers: func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{
			summary(runningID, "web", "running"),
			summary(stoppedID, "db", "exited"),
			summary(unhealthyID, "cache", "running"),
		}
	}}
	c := collectorWith(t, []string{"containers"}, []string{"container/cache"}, observer)
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	items := result.Data["items"].([]map[string]any)
	visible := map[string]bool{}
	for _, item := range items {
		visible[item["id"].(string)] = true
	}
	if len(items) != 2 || !visible[runningID] || !visible[stoppedID] || visible[unhealthyID] {
		t.Fatalf("denied container not filtered: %v", result.Data)
	}
	codes := issueCodes(result)
	if !strings.Contains(strings.Join(codes, ","), "policy_filtered") {
		t.Fatal("filtered coverage must be reported")
	}
	// Inventory without item grant lists metadata but item calls are denied.
	result = c.Collect(context.Background(), "get_docker_container", contract.Args{Container: "web"})
	if !result.Error || result.Issues[0].Code != "policy_denied" {
		t.Fatalf("item observation without grant: %v", result)
	}
}

func TestDockerContainerDetailAndNameReuse(t *testing.T) {
	t.Parallel()

	detail := &dockerobs.ContainerDetail{ContainerSummary: summary(runningID, "web", "running"), HealthCheck: "configured"}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerDetail: {Detail: detail},
		},
	}
	c := collectorWith(t, []string{"container/*"}, nil, observer)
	result := c.Collect(context.Background(), "get_docker_container", contract.Args{Container: "web"})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if result.Data["id"] != runningID {
		t.Fatal(result.Data)
	}
	// Name reuse: the selector resolves to a different identity after the
	// observation, so no evidence may be released.
	observer.containers = func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{summary(removedID, "web", "running")}
	}
	result = c.Collect(context.Background(), "get_docker_container", contract.Args{Container: "web"})
	if !result.Error || result.Issues[len(result.Issues)-1].Code != "identity_changed" {
		t.Fatalf("name reuse released evidence: %v", result.Issues)
	}
	// Ambiguous prefix is rejected without choosing a resource.
	prefix := "6f9c2f5f0f0e"
	first := summary(prefix+strings.Repeat("a", 52), "web", "running")
	second := summary(prefix+strings.Repeat("b", 52), "web2", "running")
	observer.containers = func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{first, second}
	}
	result = c.Collect(context.Background(), "get_docker_container", contract.Args{Container: prefix})
	if !result.Error || result.Issues[0].Code != "ambiguous_selector" {
		t.Fatalf("ambiguous prefix resolved: %v", result.Issues)
	}
}

func TestDockerStatsHonestUnavailableFields(t *testing.T) {
	t.Parallel()

	stats := &dockerobs.ContainerStats{Read: time.Now().UTC(), CPUPercent: floatPtr(12.5)}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerStats: {Stats: stats},
		},
	}
	c := collectorWith(t, []string{"stats/*"}, nil, observer)
	result := c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "web"})
	if result.Error {
		t.Fatal(result.Issues)
	}
	if result.Data["cpu_percent"] != 12.5 {
		t.Fatal(result.Data)
	}
	// Stopped container: stats evidence exists but is explicitly not live.
	observer.containers = func() []dockerobs.ContainerSummary {
		return []dockerobs.ContainerSummary{summary(stoppedID, "db", "exited")}
	}
	observer.responses[dockerobs.OpContainerStats] = dockerobs.Response{Stats: &dockerobs.ContainerStats{Read: time.Now().UTC()}}
	result = c.Collect(context.Background(), "get_docker_container_stats", contract.Args{Container: "db"})
	if result.Error {
		t.Fatal(result.Issues)
	}
	found := false
	for _, code := range issueCodes(result) {
		if code == "container_not_running" {
			found = true
		}
	}
	if !found {
		t.Fatal("stopped container stats must report the measurement scope")
	}
}

func TestUnusedAnalysisRespectsStoppedReferences(t *testing.T) {
	t.Parallel()

	image := dockerobs.ImageSummary{ID: "sha256:" + imageID(1), Size: int64Ptr(100), SharedSize: int64Ptr(40)}
	volume := dockerobs.VolumeSummary{Name: "data", Driver: "local"}
	// The stopped container still references both resources.
	containers := []dockerobs.ContainerSummary{summary(stoppedID, "old", "exited")}
	containers[0].ImageID = "sha256:" + imageID(1)
	containers[0].Mounts = []dockerobs.MountRef{{Type: "volume", VolumeName: "data", Destination: "/data"}}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return containers },
		responses: map[string]dockerobs.Response{
			dockerobs.OpImageList:  {Images: []dockerobs.ImageSummary{image}},
			dockerobs.OpVolumeList: {Volumes: []dockerobs.VolumeSummary{volume}},
		},
	}
	c := collectorWith(t, []string{"images", "volumes"}, nil, observer)
	result := c.Collect(context.Background(), "list_docker_images", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	items := result.Data["items"].([]map[string]any)
	if items[0]["currently_unused"] == true {
		t.Fatal("stopped container reference must prevent unused classification")
	}
	if items[0]["container_references"] != 1 {
		t.Fatalf("reference count missing: %v", items[0])
	}
	result = c.Collect(context.Background(), "list_docker_volumes", contract.Args{})
	items = result.Data["items"].([]map[string]any)
	if items[0]["currently_unused"] == true {
		t.Fatal("referenced volume must not be unused")
	}
	// With no referencing containers, the same inventory is unused.
	observer.containers = func() []dockerobs.ContainerSummary { return nil }
	result = c.Collect(context.Background(), "list_docker_images", contract.Args{})
	items = result.Data["items"].([]map[string]any)
	if items[0]["currently_unused"] != true {
		t.Fatal("unreferenced image must be classified unused at observation time")
	}
	if _, ok := result.Data["unused_since"]; ok {
		t.Fatal("creation time must never become unused duration")
	}
	// The daemon's dangling fact must stay visible next to the unused fact.
	observer = &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return nil },
		responses: map[string]dockerobs.Response{
			dockerobs.OpImageList: {Images: []dockerobs.ImageSummary{{ID: "sha256:" + imageID(9), Dangling: true}}},
		},
	}
	c = collectorWith(t, []string{"images"}, nil, observer)
	result = c.Collect(context.Background(), "list_docker_images", contract.Args{})
	items = result.Data["items"].([]map[string]any)
	if items[0]["dangling"] != true || items[0]["currently_unused"] != true {
		t.Fatal("dangling and unused facts must both be visible")
	}
}

func TestRaceSuppressesUnusedCertainty(t *testing.T) {
	t.Parallel()

	sequence := 0
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			sequence++
			if sequence%2 == 1 {
				return nil
			}
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpImageList: {Images: []dockerobs.ImageSummary{{ID: "sha256:" + imageID(9)}}},
		},
	}
	c := collectorWith(t, []string{"images"}, nil, observer)
	result := c.Collect(context.Background(), "list_docker_images", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	items := result.Data["items"].([]map[string]any)
	if items[0]["currently_unused"] == true {
		t.Fatal("unstable observation must not classify certainty")
	}
	found := false
	for _, code := range issueCodes(result) {
		if code == "non_atomic_observation" {
			found = true
		}
	}
	if !found {
		t.Fatal("non-atomic observation must be reported")
	}
}

func TestDiskUsageReclaimableAdvisory(t *testing.T) {
	t.Parallel()

	containers := []dockerobs.ContainerSummary{summary(stoppedID, "old", "exited")}
	containers[0].ImageID = "sha256:" + imageID(1)
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary { return containers },
		responses: map[string]dockerobs.Response{
			dockerobs.OpDiskUsage: {Usage: &dockerobs.DiskUsage{
				LayersSize: int64Ptr(1000),
				Images:     []dockerobs.ImageSummary{{ID: "sha256:" + imageID(1), Size: int64Ptr(100), SharedSize: int64Ptr(20)}, {ID: "sha256:" + imageID(9), Size: int64Ptr(100), SharedSize: int64Ptr(20)}},
				Volumes:    []dockerobs.VolumeSummary{{Name: "spare", Size: int64Ptr(50)}},
			}},
		},
	}
	c := collectorWith(t, []string{"disk_usage", "images", "volumes"}, nil, observer)
	result := c.Collect(context.Background(), "get_docker_disk_usage", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	reclaimable := result.Data["reclaimable"].(map[string]any)
	if reclaimable["observation_stable"] != true {
		t.Fatal("stable observation expected")
	}
	ids := reclaimable["unused_image_ids"].([]string)
	if len(ids) != 1 || ids[0] != "sha256:"+imageID(9) {
		t.Fatalf("reclaimable candidates wrong: %v", ids)
	}
	// Break the reference basis: the accounting must report non-atomic.
	lists := 0
	observer.containers = func() []dockerobs.ContainerSummary {
		lists++
		switch lists {
		case 1, 3:
			return containers
		default:
			return nil
		}
	}
	result = c.Collect(context.Background(), "get_docker_disk_usage", contract.Args{})
	if result.Error {
		t.Fatal(result.Issues)
	}
	reclaimable = result.Data["reclaimable"].(map[string]any)
	if reclaimable["observation_stable"] != false {
		t.Fatal("unstable accounting must report non-atomic coverage")
	}
}

func TestDockerLogsBoundedAndDriverGap(t *testing.T) {
	t.Parallel()

	page := &dockerobs.LogPage{Records: []dockerobs.LogRecord{{Stream: "stdout", Message: "line"}}, Truncated: true}
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpContainerLogs: {Logs: page},
		},
	}
	c := collectorWith(t, []string{"logs/*"}, nil, observer)
	result := c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web", Limit: 10})
	if result.Error {
		t.Fatal(result.Issues)
	}
	entries := result.Data["entries"].([]dockerobs.LogRecord)
	if len(entries) != 1 || entries[0].Message != "line" {
		t.Fatal("records lost")
	}
	if !result.Truncated {
		t.Fatal("truncation must surface")
	}
	// Unsupported driver: an interface gap, not an empty log claim.
	observer.responses[dockerobs.OpContainerLogs] = dockerobs.Response{Failed: true, Reason: "engine operation unsupported by this daemon: driver does not support reading", Issue: "unsupported_driver"}
	result = c.Collect(context.Background(), "query_docker_logs", contract.Args{Container: "web"})
	if !result.Error {
		t.Fatal(result)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "driver_gap" {
			found = true
		}
	}
	if !found {
		t.Fatal("driver gap must be explicit")
	}
}

func TestDockerMutationShapedArgumentsRejected(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{}
	c := collectorWith(t, []string{"*"}, nil, observer)
	before := len(observer.requests)
	for _, args := range []contract.Args{
		{Container: "web", Format: "jsonl"},
		{Container: "web", RawTail: true},
		{Container: "web", Priority: intPtr(3)},
	} {
		result := c.Collect(context.Background(), "query_docker_logs", args)
		if !result.Error || result.Issues[0].Code != "invalid_bounds" {
			t.Fatalf("mutation-shaped argument accepted: %+v", args)
		}
	}
	if len(observer.requests) != before {
		t.Fatal("rejected arguments reached the observer")
	}
}

func TestDockerGrantCannotMutate(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{containers: func() []dockerobs.ContainerSummary { return nil }}
	c := collectorWith(t, []string{"*"}, nil, observer)
	// Every registered Docker tool is a diagnostic effect; the registry has
	// no mutation, exec, or generic API operation.
	for _, definition := range contract.ToolDefinitions() {
		if strings.HasPrefix(definition.Name, "get_docker") || strings.Contains(definition.Name, "docker") {
			if definition.Effect != contract.EffectDiagnostic {
				t.Fatalf("docker tool carries non-diagnostic effect: %s", definition.Name)
			}
		}
	}
	result := c.Collect(context.Background(), "unknown_docker_mutation", contract.Args{})
	if result.Issues[0].Code != "unsupported_operation" {
		t.Fatal("unknown docker operation reached the collector")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

func floatPtr(v float64) *float64 { return &v }

func id64For(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 64)
	for i := range b {
		b[i] = hex[(n+i)%16]
	}
	return string(b)
}

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }

func TestDockerCollectorAdmitsBoundedConcurrentWork(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	observer := &fakeObserver{}
	observer.containers = func() []dockerobs.ContainerSummary {
		mu.Lock()
		defer mu.Unlock()
		return []dockerobs.ContainerSummary{summary(runningID, "web", "running")}
	}
	observer.responses = map[string]dockerobs.Response{
		dockerobs.OpImageList: {Images: []dockerobs.ImageSummary{{ID: "sha256:" + imageID(3)}}},
	}
	c := collectorWith(t, []string{"*"}, nil, observer)
	done := make(chan struct{})
	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()
			c.Collect(context.Background(), "list_docker_containers", contract.Args{})
			c.Collect(context.Background(), "list_docker_images", contract.Args{})
		}()
	}
	for range 8 {
		<-done
	}
}

// TestCollectorCapabilitiesOmitDockerWhenDisabled proves the end-to-end
// discovery contract with the real collector: Docker tools never appear in
// capabilities on disabled, ungranted, or observer-unreachable states while
// unrelated tools stay discoverable.
func TestCollectorCapabilitiesOmitDockerWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultsLinux(true)
	p, e := policy.CompileLinux(config.Config{}, "/x", nil)
	if e != nil {
		t.Fatal(e)
	}
	c := &Collector{Config: cfg, Policy: p}
	caps := c.Capabilities(context.Background())
	if caps == nil {
		t.Fatal("capabilities unavailable")
	}
	for _, tool := range []string{"get_docker_info", "list_docker_containers", "get_docker_container", "get_docker_container_stats", "list_docker_images", "list_docker_volumes", "list_docker_networks", "get_docker_disk_usage", "query_docker_logs"} {
		if caps[tool] {
			t.Fatalf("%s discoverable with docker disabled", tool)
		}
	}
	if !caps["get_os_info"] {
		t.Fatal("non-Docker discovery harmed")
	}
	// Enabled with no docker grant: still omitted.
	c = collectorWith(t, nil, nil, &fakeObserver{err: errors.New("down")})
	caps = c.Capabilities(context.Background())
	if caps["get_docker_info"] {
		t.Fatal("docker tool discoverable without grants")
	}
	// Enabled with grants but unreachable observer: still omitted.
	c = collectorWith(t, []string{"*"}, nil, &fakeObserver{err: errors.New("down")})
	caps = c.Capabilities(context.Background())
	if caps["get_docker_info"] {
		t.Fatal("docker tool discoverable with unreachable observer")
	}
}

func TestDockerCapabilitiesFollowGrantsAndObserver(t *testing.T) {
	t.Parallel()

	observer := &fakeObserver{responses: map[string]dockerobs.Response{
		dockerobs.OpEngineInfo: {Engine: &dockerobs.EngineInfo{ServerVersion: "28.0.0", NegotiatedAPI: "1.51", MinAPI: "1.44"}},
	}}
	// No grants: no Docker tool is discoverable.
	c := collectorWith(t, nil, nil, observer)
	caps := c.dockerCapabilities(context.Background())
	if len(caps) != 0 {
		t.Fatal("inactive grants discovered Docker tools")
	}
	// Full grant with available observer: every tool is discoverable.
	c = collectorWith(t, []string{"*"}, nil, observer)
	caps = c.dockerCapabilities(context.Background())
	if len(caps) != len(dockerToolKinds) {
		t.Fatalf("capabilities missing: %v", caps)
	}
	// Docker policy grant cannot cross into other diagnostic source classes.
	for tool := range caps {
		if !strings.HasPrefix(tool, "get_docker") && !strings.Contains(tool, "docker") {
			t.Fatalf("docker grant exposed %s", tool)
		}
	}
	// Observer unreachable: Docker discovery isolated from other tools.
	c = collectorWith(t, []string{"*"}, nil, &fakeObserver{err: errors.New("down")})
	caps = c.dockerCapabilities(context.Background())
	if len(caps) != 0 {
		t.Fatal("unreachable observer discovered tools")
	}
	// Unrelated discovery still works with Docker unavailable.
	result := c.Collect(context.Background(), "get_os_info", contract.Args{})
	_ = result
}

func TestDockerEvidenceNeverRetained(t *testing.T) {
	t.Parallel()

	const marker = "sensitive-docker-evidence-7d3f2b"
	observer := &fakeObserver{
		containers: func() []dockerobs.ContainerSummary {
			return []dockerobs.ContainerSummary{summary(runningID, marker, "running")}
		},
		responses: map[string]dockerobs.Response{
			dockerobs.OpEngineInfo: {Engine: &dockerobs.EngineInfo{ServerVersion: "x"}},
			dockerobs.OpImageList:  {Images: []dockerobs.ImageSummary{{ID: "sha256:" + imageID(3), RepoTags: []string{"app:" + marker}}}},
		},
	}
	c := collectorWith(t, []string{"*"}, nil, observer)
	for _, tool := range []string{
		"get_docker_info", "list_docker_containers", "list_docker_images", "list_docker_volumes",
		"list_docker_networks", "get_docker_disk_usage", "get_docker_container", "get_docker_container_stats", "query_docker_logs",
	} {
		before := len(observer.requests)
		c.Collect(context.Background(), tool, contract.Args{Container: marker, Path: marker})
		// Every request re-reads the daemon; no retained inventory serves it.
		if len(observer.requests) <= before {
			t.Fatalf("%s did not re-read the daemon", tool)
		}
	}
	// Failure and cancellation paths release observations.
	observer.err = errors.New("down")
	result := c.Collect(context.Background(), "list_docker_containers", contract.Args{})
	if !result.Error {
		t.Fatal("failure hidden")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = c.Collect(ctx, "list_docker_containers", contract.Args{})
	if result.Issues[0].Code != "cancelled_or_timeout" {
		t.Fatal(result)
	}
}

func issueCodes(r contract.Result) []string {
	codes := []string{}
	for _, issue := range r.Issues {
		codes = append(codes, issue.Code)
	}
	return codes
}
