package dockerobs

import (
	"sort"
	"time"
)

// Shared MCP projections. These helpers turn typed observation structures
// into the diagnostic result payloads. They are portable: no Linux transport,
// identity, or supervisor detail may appear here. Every projection excludes
// secret-bearing metadata by construction and reports honest gaps.

func EngineInfoData(e *EngineInfo) map[string]any {
	data := map[string]any{
		"server_version":   e.ServerVersion,
		"negotiated_api":   e.NegotiatedAPI,
		"min_api":          e.MinAPI,
		"operating_system": e.OperatingSystem,
		"os_type":          e.OSType,
		"architecture":     e.Architecture,
		"kernel_version":   e.KernelVersion,
		"storage_driver":   e.StorageDriver,
		"logging_driver":   e.LoggingDriver,
		"containers":       e.ContainerCount,
		"running":          e.RunningCount,
		"paused":           e.PausedCount,
		"stopped":          e.StoppedCount,
		"images":           e.ImageCount,
	}
	if e.CgroupDriver != "" {
		data["cgroup_driver"] = e.CgroupDriver
	}
	if e.CgroupVersion != "" {
		data["cgroup_version"] = e.CgroupVersion
	}
	if e.Experimental {
		data["experimental"] = true
	}
	if len(e.SupportedDrivers) > 0 {
		data["supported_log_drivers"] = e.SupportedDrivers
	}
	return data
}

func ContainerData(s ContainerSummary) map[string]any {
	data := map[string]any{
		"id":       s.ID,
		"names":    s.Names,
		"image_id": s.ImageID,
		"state":    s.State,
	}
	if s.Image != "" {
		data["image"] = s.Image
	}
	if s.Status != "" {
		data["status"] = s.Status
	}
	if s.Health != "" {
		data["health"] = s.Health
	}
	if s.Created != nil {
		data["created"] = s.Created.UTC().Format(time.RFC3339Nano)
	}
	if s.StartedAt != nil {
		data["started_at"] = s.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if s.FinishedAt != nil {
		data["finished_at"] = s.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if s.RestartCount != nil {
		data["restart_count"] = *s.RestartCount
	}
	if len(s.Ports) > 0 {
		data["ports"] = s.Ports
	}
	if len(s.Mounts) > 0 {
		data["mounts"] = s.Mounts
	}
	if len(s.Networks) > 0 {
		data["networks"] = s.Networks
	}
	if s.SizeRootFs != nil {
		data["size_root_fs"] = *s.SizeRootFs
	}
	if s.SizeRw != nil {
		data["size_rw"] = *s.SizeRw
	}
	if s.LogDriver != "" {
		data["log_driver"] = s.LogDriver
	}
	if s.RestartPolicy != "" {
		data["restart_policy"] = s.RestartPolicy
	}
	if s.PidsLimit != nil {
		data["pids_limit"] = *s.PidsLimit
	}
	if s.NanoCPUs != nil {
		data["nano_cpus"] = *s.NanoCPUs
	}
	if s.CPUShares != nil {
		data["cpu_shares"] = *s.CPUShares
	}
	if s.CPUPeriod != nil {
		data["cpu_period"] = *s.CPUPeriod
	}
	if s.CPUQuota != nil {
		data["cpu_quota"] = *s.CPUQuota
	}
	if s.Memory != nil {
		data["memory_limit"] = *s.Memory
	}
	if s.MemorySwap != nil {
		data["memory_swap"] = *s.MemorySwap
	}
	if s.References != nil {
		data["references"] = s.References
	}
	return data
}

func ContainerDetailData(d *ContainerDetail) map[string]any {
	data := ContainerData(d.ContainerSummary)
	if d.HealthCheck != "" {
		data["health_check"] = d.HealthCheck
	}
	if d.HealthFailing != nil {
		data["health_failing_streak"] = *d.HealthFailing
	}
	if d.OomKilled {
		data["oom_killed"] = true
	}
	if d.Pid != nil {
		data["pid"] = *d.Pid
	}
	if d.NetworkMode != "" {
		data["network_mode"] = d.NetworkMode
	}
	if len(d.Endpoints) > 0 {
		data["endpoints"] = d.Endpoints
	}
	return data
}

func ContainerStatsData(s *ContainerStats) map[string]any {
	data := map[string]any{
		"read":         s.Read.UTC().Format(time.RFC3339Nano),
		"sample_scope": "single response; no earlier sample is retained",
	}
	if !s.Preread.IsZero() {
		data["preread"] = s.Preread.UTC().Format(time.RFC3339Nano)
	}
	if s.OnlineCPUs != nil {
		data["online_cpus"] = *s.OnlineCPUs
	}
	if s.CPUUsage != nil {
		data["cpu_usage_total_ns"] = *s.CPUUsage
	}
	if s.PreCPUUsage != nil {
		data["precpu_usage_total_ns"] = *s.PreCPUUsage
	}
	if s.SystemUsage != nil {
		data["system_cpu_usage_ns"] = *s.SystemUsage
	}
	if s.PreSystem != nil {
		data["pre_system_cpu_usage_ns"] = *s.PreSystem
	}
	if s.CPUPercent != nil {
		data["cpu_percent"] = *s.CPUPercent
	}
	if s.MemoryUsed != nil {
		data["memory_used"] = *s.MemoryUsed
	}
	if s.MemoryLimit != nil {
		data["memory_limit"] = *s.MemoryLimit
	}
	if s.MemoryCache != nil {
		data["memory_cache"] = *s.MemoryCache
	}
	if s.BlockRead != nil {
		data["block_read"] = *s.BlockRead
	}
	if s.BlockWrite != nil {
		data["block_write"] = *s.BlockWrite
	}
	if len(s.Networks) > 0 {
		data["networks"] = s.Networks
	}
	if s.PidsCurrent != nil {
		data["pids_current"] = *s.PidsCurrent
	}
	if s.PidsLimit != nil {
		data["pids_limit"] = *s.PidsLimit
	}
	return data
}

func ImageData(i ImageSummary) map[string]any {
	data := map[string]any{"id": i.ID}
	if len(i.RepoTags) > 0 {
		data["repo_tags"] = i.RepoTags
	}
	if len(i.RepoDigests) > 0 {
		data["repo_digests"] = i.RepoDigests
	}
	if i.Created != nil {
		data["created"] = i.Created.UTC().Format(time.RFC3339Nano)
	}
	if i.Size != nil {
		data["size"] = *i.Size
	}
	if i.SharedSize != nil {
		data["shared_size"] = *i.SharedSize
	}
	if i.UniqueSize != nil {
		data["unique_size"] = *i.UniqueSize
	}
	if i.ContainerRefs != nil {
		data["container_references"] = *i.ContainerRefs
	}
	if i.Dangling {
		data["dangling"] = true
	}
	if i.CurrentlyUnused {
		data["currently_unused"] = true
	}
	return data
}

func VolumeData(v VolumeSummary) map[string]any {
	data := map[string]any{"name": v.Name, "driver": v.Driver}
	if v.Scope != "" {
		data["scope"] = v.Scope
	}
	if v.Created != nil {
		data["created"] = v.Created.UTC().Format(time.RFC3339Nano)
	}
	if v.RefCount != nil {
		data["container_references"] = *v.RefCount
	}
	if v.Size != nil {
		data["size"] = *v.Size
	}
	if v.Anonymous {
		data["anonymous"] = true
	}
	if v.CurrentlyUnused {
		data["currently_unused"] = true
	}
	return data
}

func NetworkData(n NetworkSummary) map[string]any {
	data := map[string]any{"id": n.ID, "name": n.Name}
	if n.Driver != "" {
		data["driver"] = n.Driver
	}
	if n.Scope != "" {
		data["scope"] = n.Scope
	}
	if n.Internal {
		data["internal"] = true
	}
	if n.EnableIPv6 {
		data["enable_ipv6"] = true
	}
	if len(n.Subnets) > 0 {
		data["subnets"] = n.Subnets
	}
	if n.ContainerRefs != nil {
		data["container_references"] = *n.ContainerRefs
	}
	return data
}

func DiskUsageData(u *DiskUsage) map[string]any {
	data := map[string]any{}
	if u.LayersSize != nil {
		data["layers_size"] = *u.LayersSize
	}
	images := make([]map[string]any, 0, len(u.Images))
	for _, image := range u.Images {
		images = append(images, ImageData(image))
	}
	data["images"] = images
	containers := make([]map[string]any, 0, len(u.Containers))
	for _, c := range u.Containers {
		ref := map[string]any{"id": c.ID}
		if c.SizeRootF != nil {
			ref["size_root_fs"] = *c.SizeRootF
		}
		if c.SizeRw != nil {
			ref["size_rw"] = *c.SizeRw
		}
		containers = append(containers, ref)
	}
	data["containers"] = containers
	volumes := make([]map[string]any, 0, len(u.Volumes))
	for _, v := range u.Volumes {
		volumes = append(volumes, VolumeData(v))
	}
	data["volumes"] = volumes
	if u.BuildCacheSize != nil {
		data["build_cache_size"] = *u.BuildCacheSize
	}
	if u.BuildCacheItems != nil {
		data["build_cache_records"] = *u.BuildCacheItems
		data["build_cache_semantics"] = "summed per-record sizes are an estimate of reclaimable build cache"
	}
	if u.Reclaimable != nil {
		data["reclaimable"] = ReclaimableData(u.Reclaimable)
	}
	return data
}

func ReclaimableData(r *Reclaimable) map[string]any {
	data := map[string]any{"observation_stable": r.ObservationStable}
	if len(r.UnusedImages) > 0 {
		data["unused_image_ids"] = r.UnusedImages
	}
	if r.UnusedImageBytes != nil {
		data["unused_image_unique_bytes"] = *r.UnusedImageBytes
	}
	if len(r.UnusedVolumes) > 0 {
		data["unused_volume_names"] = r.UnusedVolumes
	}
	if r.UnusedVolumeBytes != nil {
		data["unused_volume_bytes"] = *r.UnusedVolumeBytes
	}
	if len(r.StoppedContainers) > 0 {
		data["stopped_container_ids"] = r.StoppedContainers
	}
	if len(r.DanglingImages) > 0 {
		data["dangling_image_ids"] = r.DanglingImages
	}
	data["criteria"] = "no reference from any container, including stopped containers, in one non-atomic live observation"
	data["advisory"] = "estimates are evidence for review; they never authorize removal"
	return data
}

func LogPageData(l *LogPage) map[string]any {
	data := map[string]any{
		"entries":  l.Records,
		"tty":      l.TTY,
		"ordering": "daemon stream order; not merged chronologically across streams",
	}
	if l.Truncated {
		data["truncated"] = true
	}
	if l.Skipped > 0 {
		data["skipped_frames"] = l.Skipped
	}
	if l.Dropped > 0 {
		data["malformed_frames"] = l.Dropped
	}
	if l.FirstEvent != nil {
		data["earliest_returned_event"] = l.FirstEvent.UTC().Format(time.RFC3339Nano)
	}
	if l.LastEvent != nil {
		data["latest_returned_event"] = l.LastEvent.UTC().Format(time.RFC3339Nano)
	}
	return data
}

// SortByID orders bounded lists by full stable identity so pagination is a
// deterministic function of one observation.
func SortContainerIDs(items []ContainerSummary) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func SortImageIDs(items []ImageSummary) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func SortVolumeNames(items []VolumeSummary) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
}

func SortNetworkIDs(items []NetworkSummary) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
