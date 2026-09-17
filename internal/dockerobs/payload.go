package dockerobs

import (
	"sort"
	"time"
)

// MCP result payloads. These types back the Docker tools' `data` members;
// every JSON name equals the member the map projections emitted before the
// typed registry (see keys-before.txt in the change directory). Secret-bearing
// metadata stays excluded by construction: the types have no environment,
// command, label, or health output fields.

// EngineInfoPayload is the engine identity projection for get_docker_info.
type EngineInfoPayload struct {
	ServerVersion       string   `json:"server_version"`
	NegotiatedAPI       string   `json:"negotiated_api"`
	MinAPI              string   `json:"min_api"`
	OperatingSystem     string   `json:"operating_system"`
	OSType              string   `json:"os_type"`
	Architecture        string   `json:"architecture"`
	KernelVersion       string   `json:"kernel_version"`
	StorageDriver       string   `json:"storage_driver"`
	LoggingDriver       string   `json:"logging_driver"`
	Containers          int      `json:"containers"`
	Running             int      `json:"running"`
	Paused              int      `json:"paused"`
	Stopped             int      `json:"stopped"`
	Images              int      `json:"images"`
	CgroupDriver        string   `json:"cgroup_driver,omitempty"`
	CgroupVersion       string   `json:"cgroup_version,omitempty"`
	Experimental        bool     `json:"experimental,omitempty"`
	SupportedLogDrivers []string `json:"supported_log_drivers,omitempty"`
	Available           bool     `json:"available"`
}

// PortPayload is one published port.
type PortPayload struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
	HostIP      string `json:"host_ip,omitempty"`
}

// MountPayload is one mount's type, destination, and policy-safe source.
// Mounted content is never read.
type MountPayload struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Source      string `json:"source,omitempty"`
	RW          bool   `json:"rw,omitempty"`
	VolumeName  string `json:"volume_name,omitempty"`
}

// ContainerPayload is one container's identity and lifecycle evidence.
// Environment, commands, labels, and health output are excluded by
// construction.
type ContainerPayload struct {
	ID            string         `json:"id"`
	Names         []string       `json:"names"`
	ImageID       string         `json:"image_id"`
	State         string         `json:"state"`
	Image         string         `json:"image,omitempty"`
	Status        string         `json:"status,omitempty"`
	Health        string         `json:"health,omitempty"`
	Created       string         `json:"created,omitempty"`
	Ports         []PortPayload  `json:"ports,omitempty"`
	Mounts        []MountPayload `json:"mounts,omitempty"`
	Networks      []string       `json:"networks,omitempty"`
	RestartCount  *int           `json:"restart_count,omitempty"`
	StartedAt     string         `json:"started_at,omitempty"`
	FinishedAt    string         `json:"finished_at,omitempty"`
	LogDriver     string         `json:"log_driver,omitempty"`
	RestartPolicy string         `json:"restart_policy,omitempty"`
	PidsLimit     *int64         `json:"pids_limit,omitempty"`
	NanoCPUs      *int64         `json:"nano_cpus,omitempty"`
	CPUShares     *int64         `json:"cpu_shares,omitempty"`
	CPUPeriod     *int64         `json:"cpu_period,omitempty"`
	CPUQuota      *int64         `json:"cpu_quota,omitempty"`
	MemoryLimit   *int64         `json:"memory_limit,omitempty"`
	MemorySwap    *int64         `json:"memory_swap,omitempty"`
}

// EndpointPayload is one network attachment without secret-bearing metadata.
type EndpointPayload struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address,omitempty"`
	MACAddress string `json:"mac_address,omitempty"`
}

// ContainerDetailPayload extends the container projection with inspect-only
// selected fields. Health check output is never included.
type ContainerDetailPayload struct {
	ContainerPayload
	HealthCheck         string            `json:"health_check,omitempty"`
	HealthFailingStreak *int              `json:"health_failing_streak,omitempty"`
	OomKilled           bool              `json:"oom_killed,omitempty"`
	Pid                 *int              `json:"pid,omitempty"`
	NetworkMode         string            `json:"network_mode,omitempty"`
	Endpoints           []EndpointPayload `json:"endpoints,omitempty"`
}

// ContainerStatsPayload is one bounded point-in-time resource sample. Rates
// derive only from the counter pair and interval this response carries.
type ContainerStatsPayload struct {
	Read                string           `json:"read"`
	SampleScope         string           `json:"sample_scope"`
	Preread             string           `json:"preread,omitempty"`
	OnlineCPUs          *int             `json:"online_cpus,omitempty"`
	CPUUsageTotalNS     *uint64          `json:"cpu_usage_total_ns,omitempty"`
	PreCPUUsageTotalNS  *uint64          `json:"precpu_usage_total_ns,omitempty"`
	SystemCPUUsageNS    *uint64          `json:"system_cpu_usage_ns,omitempty"`
	PreSystemCPUUsageNS *uint64          `json:"pre_system_cpu_usage_ns,omitempty"`
	CPUPercent          *float64         `json:"cpu_percent,omitempty"`
	MemoryUsed          *uint64          `json:"memory_used,omitempty"`
	MemoryLimit         *uint64          `json:"memory_limit,omitempty"`
	MemoryCache         *uint64          `json:"memory_cache,omitempty"`
	BlockRead           *uint64          `json:"block_read,omitempty"`
	BlockWrite          *uint64          `json:"block_write,omitempty"`
	Networks            map[string]NetIO `json:"networks,omitempty"`
	PidsCurrent         *int             `json:"pids_current,omitempty"`
	PidsLimit           *int             `json:"pids_limit,omitempty"`
}

// ImagePayload is one deduplicated image identity with daemon-supplied size
// semantics. Sizes are never summed into a fabricated physical total.
type ImagePayload struct {
	ID                 string   `json:"id"`
	RepoTags           []string `json:"repo_tags,omitempty"`
	RepoDigests        []string `json:"repo_digests,omitempty"`
	Created            string   `json:"created,omitempty"`
	Size               *int64   `json:"size,omitempty"`
	SharedSize         *int64   `json:"shared_size,omitempty"`
	ContainerReference *int     `json:"container_references,omitempty"`
	Dangling           bool     `json:"dangling,omitempty"`
	CurrentlyUnused    bool     `json:"currently_unused,omitempty"`
}

// VolumePayload is one volume with current references and size only when the
// driver and daemon supply one. A missing size is omitted, never zero.
type VolumePayload struct {
	Name            string `json:"name"`
	Driver          string `json:"driver"`
	Scope           string `json:"scope,omitempty"`
	Created         string `json:"created,omitempty"`
	ContainerRef    *int   `json:"container_references,omitempty"`
	Size            *int64 `json:"size,omitempty"`
	Anonymous       bool   `json:"anonymous,omitempty"`
	CurrentlyUnused bool   `json:"currently_unused,omitempty"`
}

// NetworkPayload is one network's identity and selected non-secret
// configuration.
type NetworkPayload struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Driver       string   `json:"driver,omitempty"`
	Scope        string   `json:"scope,omitempty"`
	Internal     bool     `json:"internal,omitempty"`
	EnableIPv6   bool     `json:"enable_ipv6,omitempty"`
	Subnets      []string `json:"subnets,omitempty"`
	ContainerRef *int     `json:"container_references,omitempty"`
}

// ContainerRefPayload is one container's storage footprint as the daemon
// reports it.
type ContainerRefPayload struct {
	ID        string `json:"id"`
	SizeRootF *int64 `json:"size_root_fs,omitempty"`
	SizeRw    *int64 `json:"size_rw,omitempty"`
}

// ReclaimablePayload summarizes currently unused resources. It never claims a
// duration and never fabricates a physical total from per-resource sizes.
type ReclaimablePayload struct {
	ObservationStable     bool     `json:"observation_stable"`
	UnusedImageIDs        []string `json:"unused_image_ids,omitempty"`
	UnusedImageUniqueByte *int64   `json:"unused_image_unique_bytes,omitempty"`
	UnusedVolumeNames     []string `json:"unused_volume_names,omitempty"`
	UnusedVolumeBytes     *int64   `json:"unused_volume_bytes,omitempty"`
	StoppedContainerIDs   []string `json:"stopped_container_ids,omitempty"`
	DanglingImageIDs      []string `json:"dangling_image_ids,omitempty"`
	Criteria              string   `json:"criteria"`
	Advisory              string   `json:"advisory"`
}

// DiskUsagePayload is the daemon disk accounting projection.
type DiskUsagePayload struct {
	LayersSize          *int64                `json:"layers_size,omitempty"`
	Images              []ImagePayload        `json:"images"`
	Containers          []ContainerRefPayload `json:"containers"`
	Volumes             []VolumePayload       `json:"volumes"`
	BuildCacheSize      *int64                `json:"build_cache_size,omitempty"`
	BuildCacheRecords   *int                  `json:"build_cache_records,omitempty"`
	BuildCacheSemantics string                `json:"build_cache_semantics,omitempty"`
	Reclaimable         ReclaimablePayload    `json:"reclaimable"`
	SnapshotConsistent  bool                  `json:"snapshot_consistent"`
}

// LogPagePayload is bounded log retrieval; it preserves stream meaning and
// reports truncation and coverage explicitly.
type LogPagePayload struct {
	Entries               []LogRecord `json:"entries"`
	TTY                   bool        `json:"tty"`
	Ordering              string      `json:"ordering"`
	Truncated             bool        `json:"truncated,omitempty"`
	SkippedFrames         int         `json:"skipped_frames,omitempty"`
	MalformedFrames       int         `json:"malformed_frames,omitempty"`
	EarliestReturnedEvent string      `json:"earliest_returned_event,omitempty"`
	LatestReturnedEvent   string      `json:"latest_returned_event,omitempty"`
}

// DockerLogPage adds the requested window to one observer log projection.
type DockerLogPage struct {
	LogPagePayload
	RequestedSince string `json:"requested_since"`
	RequestedUntil string `json:"requested_until"`
}

// Page is one bounded page of a Docker inventory observation.
type Page[T any] struct {
	Items              []T  `json:"items"`
	SnapshotConsistent bool `json:"snapshot_consistent"`
}

// Constructors ---------------------------------------------------------------

func rfc3339Nano(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// ToEngineInfo projects the engine identity for the MCP result.
func ToEngineInfo(e *EngineInfo) EngineInfoPayload {
	p := EngineInfoPayload{
		ServerVersion:   e.ServerVersion,
		NegotiatedAPI:   e.NegotiatedAPI,
		MinAPI:          e.MinAPI,
		OperatingSystem: e.OperatingSystem,
		OSType:          e.OSType,
		Architecture:    e.Architecture,
		KernelVersion:   e.KernelVersion,
		StorageDriver:   e.StorageDriver,
		LoggingDriver:   e.LoggingDriver,
		Containers:      e.ContainerCount,
		Running:         e.RunningCount,
		Paused:          e.PausedCount,
		Stopped:         e.StoppedCount,
		Images:          e.ImageCount,
		CgroupDriver:    e.CgroupDriver,
		CgroupVersion:   e.CgroupVersion,
		Experimental:    e.Experimental,
	}
	if len(e.SupportedDrivers) > 0 {
		p.SupportedLogDrivers = e.SupportedDrivers
	}
	return p
}

// ToContainer projects one container summary. Empty optional values are
// omitted, never zeroed.
func ToContainer(s ContainerSummary) ContainerPayload {
	p := ContainerPayload{
		ID:            s.ID,
		Names:         s.Names,
		ImageID:       s.ImageID,
		State:         s.State,
		Image:         s.Image,
		Status:        s.Status,
		Health:        s.Health,
		Created:       rfc3339Nano(s.Created),
		Networks:      s.Networks,
		RestartCount:  s.RestartCount,
		StartedAt:     rfc3339Nano(s.StartedAt),
		FinishedAt:    rfc3339Nano(s.FinishedAt),
		LogDriver:     s.LogDriver,
		RestartPolicy: s.RestartPolicy,
		PidsLimit:     s.PidsLimit,
		NanoCPUs:      s.NanoCPUs,
		CPUShares:     s.CPUShares,
		CPUPeriod:     s.CPUPeriod,
		CPUQuota:      s.CPUQuota,
		MemoryLimit:   s.Memory,
		MemorySwap:    s.MemorySwap,
	}
	for _, port := range s.Ports {
		p.Ports = append(p.Ports, PortPayload{
			PrivatePort: port.PrivatePort,
			PublicPort:  port.PublicPort,
			Type:        port.Type,
			HostIP:      port.HostIP,
		})
	}
	for _, m := range s.Mounts {
		p.Mounts = append(p.Mounts, MountPayload{
			Type:        m.Type,
			Destination: m.Destination,
			Source:      m.Source,
			RW:          m.RW,
			VolumeName:  m.VolumeName,
		})
	}
	return p
}

// ToContainerDetail projects one container inspection.
func ToContainerDetail(d *ContainerDetail) ContainerDetailPayload {
	p := ContainerDetailPayload{ContainerPayload: ToContainer(d.ContainerSummary)}
	p.HealthCheck = d.HealthCheck
	p.HealthFailingStreak = d.HealthFailing
	p.OomKilled = d.OomKilled
	p.Pid = d.Pid
	p.NetworkMode = d.NetworkMode
	for _, e := range d.Endpoints {
		p.Endpoints = append(p.Endpoints, EndpointPayload{
			Name:       e.Name,
			IPAddress:  e.IPAddress,
			MACAddress: e.MACAddress,
		})
	}
	return p
}

// ToContainerStats projects one stats sample.
func ToContainerStats(s *ContainerStats) ContainerStatsPayload {
	p := ContainerStatsPayload{
		Read:        s.Read.UTC().Format(time.RFC3339Nano),
		SampleScope: "single response; no earlier sample is retained",
	}
	if !s.Preread.IsZero() {
		p.Preread = s.Preread.UTC().Format(time.RFC3339Nano)
	}
	p.OnlineCPUs = s.OnlineCPUs
	p.CPUUsageTotalNS = s.CPUUsage
	p.PreCPUUsageTotalNS = s.PreCPUUsage
	p.SystemCPUUsageNS = s.SystemUsage
	p.PreSystemCPUUsageNS = s.PreSystem
	p.CPUPercent = s.CPUPercent
	p.MemoryUsed = s.MemoryUsed
	p.MemoryLimit = s.MemoryLimit
	p.MemoryCache = s.MemoryCache
	p.BlockRead = s.BlockRead
	p.BlockWrite = s.BlockWrite
	p.Networks = s.Networks
	p.PidsCurrent = s.PidsCurrent
	p.PidsLimit = s.PidsLimit
	return p
}

// ToImage projects one image summary.
func ToImage(i ImageSummary) ImagePayload {
	p := ImagePayload{
		ID:                 i.ID,
		RepoTags:           i.RepoTags,
		RepoDigests:        i.RepoDigests,
		Created:            rfc3339Nano(i.Created),
		ContainerReference: i.ContainerRefs,
		Dangling:           i.Dangling,
		CurrentlyUnused:    i.CurrentlyUnused,
	}
	if i.Size != nil {
		p.Size = i.Size
	}
	if i.SharedSize != nil {
		p.SharedSize = i.SharedSize
	}
	return p
}

// ToVolume projects one volume summary.
func ToVolume(v VolumeSummary) VolumePayload {
	return VolumePayload{
		Name:            v.Name,
		Driver:          v.Driver,
		Scope:           v.Scope,
		Created:         rfc3339Nano(v.Created),
		ContainerRef:    v.RefCount,
		Size:            v.Size,
		Anonymous:       v.Anonymous,
		CurrentlyUnused: v.CurrentlyUnused,
	}
}

// ToNetwork projects one network summary.
func ToNetwork(n NetworkSummary) NetworkPayload {
	return NetworkPayload{
		ID:           n.ID,
		Name:         n.Name,
		Driver:       n.Driver,
		Scope:        n.Scope,
		Internal:     n.Internal,
		EnableIPv6:   n.EnableIPv6,
		Subnets:      n.Subnets,
		ContainerRef: n.ContainerRefs,
	}
}

// ToReclaimable projects one unused-resource analysis.
func ToReclaimable(r *Reclaimable) ReclaimablePayload {
	return ReclaimablePayload{
		ObservationStable:     r.ObservationStable,
		UnusedImageIDs:        r.UnusedImages,
		UnusedImageUniqueByte: r.UnusedImageBytes,
		UnusedVolumeNames:     r.UnusedVolumes,
		UnusedVolumeBytes:     r.UnusedVolumeBytes,
		StoppedContainerIDs:   r.StoppedContainers,
		DanglingImageIDs:      r.DanglingImages,
		Criteria:              "no reference from any container, including stopped containers, in one non-atomic live observation",
		Advisory:              "estimates are evidence for review; they never authorize removal",
	}
}

// ToDiskUsage projects the daemon disk accounting. Per-resource sizes keep
// daemon semantics; nothing is summed into the physical total.
func ToDiskUsage(u *DiskUsage) DiskUsagePayload {
	p := DiskUsagePayload{
		LayersSize:  u.LayersSize,
		Images:      make([]ImagePayload, 0, len(u.Images)),
		Containers:  make([]ContainerRefPayload, 0, len(u.Containers)),
		Volumes:     make([]VolumePayload, 0, len(u.Volumes)),
		Reclaimable: ReclaimablePayload{},
	}
	for _, image := range u.Images {
		p.Images = append(p.Images, ToImage(image))
	}
	for _, c := range u.Containers {
		p.Containers = append(p.Containers, ContainerRefPayload{
			ID:        c.ID,
			SizeRootF: c.SizeRootF,
			SizeRw:    c.SizeRw,
		})
	}
	for _, v := range u.Volumes {
		p.Volumes = append(p.Volumes, ToVolume(v))
	}
	p.BuildCacheSize = u.BuildCacheSize
	p.BuildCacheRecords = u.BuildCacheItems
	if u.BuildCacheItems != nil {
		p.BuildCacheSemantics = "summed per-record sizes are an estimate of reclaimable build cache"
	}
	if u.Reclaimable != nil {
		p.Reclaimable = ToReclaimable(u.Reclaimable)
	}
	return p
}

// ToLogPage projects one observer log page.
func ToLogPage(l *LogPage) LogPagePayload {
	p := LogPagePayload{
		Entries:         l.Records,
		TTY:             l.TTY,
		Ordering:        "daemon stream order; not merged chronologically across streams",
		Truncated:       l.Truncated,
		SkippedFrames:   l.Skipped,
		MalformedFrames: l.Dropped,
	}
	if l.FirstEvent != nil {
		p.EarliestReturnedEvent = l.FirstEvent.UTC().Format(time.RFC3339Nano)
	}
	if l.LastEvent != nil {
		p.LatestReturnedEvent = l.LastEvent.UTC().Format(time.RFC3339Nano)
	}
	return p
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
