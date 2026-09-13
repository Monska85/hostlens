// Package dockerobs defines the portable, versioned observation contract between
// the diagnostic backend and the isolated Docker observer. It carries Docker
// observation semantics only: no Unix paths, numeric Unix identities, systemd
// units, or other native transport and lifecycle details. Linux composition for
// this contract lives outside portable build targets.
package dockerobs

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ProtocolVersion identifies the typed observation contract revision. A change
// that removes, renames, or reshapes a shared structure increments this value.
const ProtocolVersion = 1

// Engine API compatibility range, verified against official Docker
// documentation and representative engines before activation:
//   - Floor 1.41 supplies one-shot stats with precpu samples, CgroupVersion in
//     daemon info, and the observation endpoints used by this integration.
//   - Ceiling 1.51 keeps the legacy system disk-usage response fields and the
//     images shared-size query stable while remaining negotiable with newer
//     compatible daemons.
const (
	APIFloor   = "1.41"
	APICeiling = "1.51"
)

// Operations are the closed set of typed observation requests. Callers cannot
// express HTTP methods, paths, headers, or bodies through this surface.
const (
	OpEngineInfo      = "engine_info"
	OpContainerList   = "container_list"
	OpContainerDetail = "container_detail"
	OpContainerStats  = "container_stats"
	OpContainerLogs   = "container_logs"
	OpImageList       = "image_list"
	OpVolumeList      = "volume_list"
	OpNetworkList     = "network_list"
	OpDiskUsage       = "disk_usage"
)

// Fixed observation and work ceilings. They bound observer work regardless of
// daemon inventory size; truncation is reported, never silently dropped.
const (
	MaxContainerItems = 5000
	MaxImageItems     = 5000
	MaxVolumeItems    = 5000
	MaxNetworkItems   = 1000
	MaxDiskUsageBytes = 16 << 20
	MaxInfoBytes      = 256 << 10
	MaxStatsBytes     = 256 << 10
	MaxInspectBytes   = 256 << 10
	MaxListBytes      = 16 << 20
	MaxRequestBytes   = 64 << 10
	MaxLogBytes       = 8 << 20
	MaxLogRecords     = 10000
	MaxConcurrent     = 4
	MaxDiskUsageRuns  = 1
)

// Request is one typed observation. Every field is fixed by this contract;
// unknown operation names, protocol versions, and fields fail before any
// Docker access.
type Request struct {
	Version   int        `json:"version"`
	Operation string     `json:"operation"`
	Selector  string     `json:"selector,omitempty"`
	Logs      LogOptions `json:"logs,omitempty"`
}

// LogOptions bounds one log observation. The window and record ceiling are
// part of the typed request; the observer never follows live streams.
type LogOptions struct {
	Since    time.Time `json:"since,omitempty"`
	Until    time.Time `json:"until,omitempty"`
	Records  int       `json:"records,omitempty"`
	MaxBytes int       `json:"max_bytes,omitempty"`
}

// Validate rejects requests that do not match the typed contract.
func (r Request) Validate() error {
	if r.Version != ProtocolVersion {
		return fmt.Errorf("unsupported observation protocol version %d", r.Version)
	}
	switch r.Operation {
	case OpEngineInfo, OpContainerList, OpImageList, OpVolumeList, OpNetworkList, OpDiskUsage:
		if r.Selector != "" || !r.Logs.Since.IsZero() || !r.Logs.Until.IsZero() || r.Logs.Records != 0 || r.Logs.MaxBytes != 0 {
			return errors.New("observation operation received unsupported fields")
		}
	case OpContainerDetail, OpContainerStats, OpContainerLogs:
		// Item operations cross the boundary by full stable identity only;
		// the backend resolves names against the live list beforehand.
		if !validID(r.Selector) {
			return errors.New("full 64-character container identity required")
		}
		if r.Logs.Records < 0 || r.Logs.Records > MaxLogRecords {
			return errors.New("log record ceiling exceeded")
		}
		if r.Logs.MaxBytes < 0 || r.Logs.MaxBytes > MaxLogBytes {
			return errors.New("log byte ceiling exceeded")
		}
		if !r.Logs.Since.IsZero() && !r.Logs.Until.IsZero() && r.Logs.Until.Before(r.Logs.Since) {
			return errors.New("log window ends before it starts")
		}
	default:
		return fmt.Errorf("unknown observation operation %q", r.Operation)
	}
	return nil
}

var idRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidID reports whether s is a full 64-character hexadecimal resource ID.
func ValidID(s string) bool { return validID(s) }

func validID(s string) bool { return idRE.MatchString(s) }

// Response carries one typed observation or an explicit failure. Raw daemon
// payloads never cross this boundary; every structure lists maintained fields.
type Response struct {
	Engine   *EngineInfo      `json:"engine,omitempty"`
	Detail   *ContainerDetail `json:"detail,omitempty"`
	Stats    *ContainerStats  `json:"stats,omitempty"`
	Logs     *LogPage         `json:"logs,omitempty"`
	Usage    *DiskUsage       `json:"usage,omitempty"`
	Images   []ImageSummary   `json:"images,omitempty"`
	Volumes  []VolumeSummary  `json:"volumes,omitempty"`
	Networks []NetworkSummary `json:"networks,omitempty"`
	// Containers serves both list (summaries) and detail (single-element)
	// results; detail responses populate Detail instead.
	Containers []ContainerSummary `json:"containers,omitempty"`
	// Truncated reports that a maintained ceiling removed content.
	Truncated bool `json:"truncated,omitempty"`
	// Issue explains a bounded gap, such as ceiling truncation or an
	// unsupported daemon capability. Failures use Failed with Reason.
	Issue string `json:"issue,omitempty"`
	// Failed marks the whole observation unsuccessful; no partial Docker
	// evidence is released with a failure.
	Failed bool   `json:"failed,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// EngineInfo projects selected daemon facts without registry configuration,
// proxy values, plugin settings, or raw daemon configuration.
type EngineInfo struct {
	ServerVersion      string   `json:"server_version"`
	NegotiatedAPI      string   `json:"negotiated_api"`
	MinAPI             string   `json:"min_api"`
	OperatingSystem    string   `json:"operating_system"`
	OSType             string   `json:"os_type"`
	Architecture       string   `json:"architecture"`
	KernelVersion      string   `json:"kernel_version"`
	StorageDriver      string   `json:"storage_driver"`
	CgroupDriver       string   `json:"cgroup_driver,omitempty"`
	CgroupVersion      string   `json:"cgroup_version,omitempty"`
	LoggingDriver      string   `json:"logging_driver"`
	Experimental       bool     `json:"experimental,omitempty"`
	ContainerCount     int      `json:"container_count"`
	RunningCount       int      `json:"running_count"`
	PausedCount        int      `json:"paused_count"`
	StoppedCount       int      `json:"stopped_count"`
	ImageCount         int      `json:"image_count"`
	SupportedDrivers   []string `json:"supported_log_drivers,omitempty"`
	UnsupportedReasons []string `json:"unsupported_reasons,omitempty"`
}

// PortMapping is one published port projection.
type PortMapping struct {
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
	HostIP      string `json:"host_ip,omitempty"`
}

// MountRef projects mount type, destination, and policy-safe source identity.
// Mounted content is never read.
type MountRef struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Source      string `json:"source,omitempty"`
	RW          bool   `json:"rw,omitempty"`
	VolumeName  string `json:"volume_name,omitempty"`
}

// ContainerSummary projects current identity and lifecycle evidence from the
// all-container list. It omits environment, commands, labels, and raw config.
type ContainerSummary struct {
	ID            string        `json:"id"`
	Names         []string      `json:"names"`
	Image         string        `json:"image,omitempty"`
	ImageID       string        `json:"image_id"`
	State         string        `json:"state"`
	Status        string        `json:"status,omitempty"`
	Health        string        `json:"health,omitempty"`
	Created       *time.Time    `json:"created,omitempty"`
	Ports         []PortMapping `json:"ports,omitempty"`
	Mounts        []MountRef    `json:"mounts,omitempty"`
	Networks      []string      `json:"networks,omitempty"`
	RestartCount  *int          `json:"restart_count,omitempty"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	FinishedAt    *time.Time    `json:"finished_at,omitempty"`
	LogDriver     string        `json:"log_driver,omitempty"`
	RestartPolicy string        `json:"restart_policy,omitempty"`
	PidsLimit     *int64        `json:"pids_limit,omitempty"`
	NanoCPUs      *int64        `json:"nano_cpus,omitempty"`
	CPUShares     *int64        `json:"cpu_shares,omitempty"`
	CPUPeriod     *int64        `json:"cpu_period,omitempty"`
	CPUQuota      *int64        `json:"cpu_quota,omitempty"`
	Memory        *int64        `json:"memory_limit,omitempty"`
	MemorySwap    *int64        `json:"memory_swap,omitempty"`
}

// ContainerDetail extends the summary with inspect-only selected fields.
// Health check command output is never included.
type ContainerDetail struct {
	ContainerSummary
	HealthCheck   string     `json:"health_check,omitempty"`
	HealthFailing *int       `json:"health_failing_streak,omitempty"`
	OomKilled     bool       `json:"oom_killed,omitempty"`
	Pid           *int       `json:"pid,omitempty"`
	NetworkMode   string     `json:"network_mode,omitempty"`
	Endpoints     []Endpoint `json:"endpoints,omitempty"`
}

// Endpoint projects one network attachment without secret-bearing metadata.
type Endpoint struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address,omitempty"`
	MACAddress string `json:"mac_address,omitempty"`
}

// ContainerStats projects one bounded point-in-time sample. Rates are derived
// only from the pair of counters and interval this response carries.
type ContainerStats struct {
	Read        time.Time        `json:"read"`
	Preread     time.Time        `json:"preread,omitempty"`
	OnlineCPUs  *int             `json:"online_cpus,omitempty"`
	CPUUsage    *uint64          `json:"cpu_usage_total_ns,omitempty"`
	PreCPUUsage *uint64          `json:"precpu_usage_total_ns,omitempty"`
	SystemUsage *uint64          `json:"system_cpu_usage_ns,omitempty"`
	PreSystem   *uint64          `json:"pre_system_cpu_usage_ns,omitempty"`
	CPUPercent  *float64         `json:"cpu_percent,omitempty"`
	MemoryUsed  *uint64          `json:"memory_used_bytes,omitempty"`
	MemoryLimit *uint64          `json:"memory_limit_bytes,omitempty"`
	MemoryCache *uint64          `json:"memory_cache_bytes,omitempty"`
	BlockRead   *uint64          `json:"block_read_bytes,omitempty"`
	BlockWrite  *uint64          `json:"block_write_bytes,omitempty"`
	Networks    map[string]NetIO `json:"networks,omitempty"`
	PidsCurrent *int             `json:"pids_current,omitempty"`
	PidsLimit   *int             `json:"pids_limit,omitempty"`
	// PreCPU fields belong to this same response; PreRead bounds the interval.
}

// NetIO projects one interface counter pair.
type NetIO struct {
	RxBytes *uint64 `json:"rx_bytes,omitempty"`
	TxBytes *uint64 `json:"tx_bytes,omitempty"`
}

// ImageSummary projects a deduplicated image identity with daemon-supplied
// size semantics. Sizes are never summed into a fabricated physical total.
type ImageSummary struct {
	ID            string     `json:"id"`
	RepoTags      []string   `json:"repo_tags,omitempty"`
	RepoDigests   []string   `json:"repo_digests,omitempty"`
	Created       *time.Time `json:"created,omitempty"`
	Size          *int64     `json:"size_bytes,omitempty"`
	SharedSize    *int64     `json:"shared_size_bytes,omitempty"`
	ContainerRefs *int       `json:"container_references,omitempty"`
	Dangling      bool       `json:"dangling,omitempty"`
	// CurrentlyUnused is true only when the completed observation found no
	// referencing container, including stopped containers.
	CurrentlyUnused bool `json:"currently_unused,omitempty"`
}

// VolumeSummary projects one volume with current references and size only
// when the driver and daemon supply one. A missing size is omitted, never
// substituted with zero.
type VolumeSummary struct {
	Name            string     `json:"name"`
	Driver          string     `json:"driver"`
	Scope           string     `json:"scope,omitempty"`
	Created         *time.Time `json:"created,omitempty"`
	RefCount        *int       `json:"container_references,omitempty"`
	Size            *int64     `json:"size_bytes,omitempty"`
	Anonymous       bool       `json:"anonymous,omitempty"`
	CurrentlyUnused bool       `json:"currently_unused,omitempty"`
}

// NetworkSummary projects identity and selected non-secret configuration.
type NetworkSummary struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Driver          string   `json:"driver,omitempty"`
	Scope           string   `json:"scope,omitempty"`
	Internal        bool     `json:"internal,omitempty"`
	EnableIPv6      bool     `json:"enable_ipv6,omitempty"`
	Subnets         []string `json:"subnets,omitempty"`
	ContainerRefs   *int     `json:"container_references,omitempty"`
	CurrentlyUnused bool     `json:"currently_unused,omitempty"`
}

// DiskUsage projects daemon disk accounting. Physical totals are daemon
// facts; per-resource sizes are never summed into them.
type DiskUsage struct {
	LayersSize      *int64          `json:"layers_size_bytes,omitempty"`
	Images          []ImageSummary  `json:"images,omitempty"`
	Containers      []ContainerRef  `json:"containers,omitempty"`
	Volumes         []VolumeSummary `json:"volumes,omitempty"`
	BuildCacheSize  *int64          `json:"build_cache_size_bytes,omitempty"`
	BuildCacheItems *int            `json:"build_cache_records,omitempty"`
	// Reclaimable summarizes unused evidence at observation time. It is
	// advisory and derived from the same non-atomic observation.
	Reclaimable *Reclaimable `json:"reclaimable,omitempty"`
}

// ContainerRef is one container's storage footprint as the daemon reports it.
type ContainerRef struct {
	ID        string `json:"id"`
	SizeRootF *int64 `json:"size_root_fs_bytes,omitempty"`
	SizeRw    *int64 `json:"size_rw_bytes,omitempty"`
}

// Reclaimable summarizes currently unused resources. It never claims a
// duration and never fabricates a physical total from per-resource sizes.
type Reclaimable struct {
	UnusedImages      []string `json:"unused_image_ids,omitempty"`
	UnusedImageBytes  *int64   `json:"unused_image_unique_bytes,omitempty"`
	UnusedVolumes     []string `json:"unused_volume_names,omitempty"`
	UnusedVolumeBytes *int64   `json:"unused_volume_bytes,omitempty"`
	StoppedContainers []string `json:"stopped_container_ids,omitempty"`
	DanglingImages    []string `json:"dangling_image_ids,omitempty"`
	// ObservationStable is false when the correlation raced with daemon
	// changes; candidates are then evidence only, not removal targets.
	ObservationStable bool `json:"observation_stable"`
}

// LogRecord is one decoded log line. Content is untrusted data.
type LogRecord struct {
	Stream    string     `json:"stream"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	Message   string     `json:"message"`
}

// LogPage projects bounded log retrieval. It preserves stream meaning and
// reports truncation and coverage explicitly.
type LogPage struct {
	Records    []LogRecord `json:"records"`
	TTY        bool        `json:"tty,omitempty"`
	Truncated  bool        `json:"truncated,omitempty"`
	Skipped    int         `json:"skipped_frames,omitempty"`
	Dropped    int         `json:"malformed_frames,omitempty"`
	FirstEvent *time.Time  `json:"first_event,omitempty"`
	LastEvent  *time.Time  `json:"last_event,omitempty"`
}

// StableFingerprint hashes the container reference basis of an unused
// analysis. Equal before-and-after fingerprints mean the correlation did not
// race with a daemon change within that observation's bounds.
type StableFingerprint struct {
	ContainerIDs []string          `json:"container_ids,omitempty"`
	References   map[string]string `json:"references,omitempty"`
}

func (f StableFingerprint) Equal(other StableFingerprint) bool {
	a := append([]string{}, f.ContainerIDs...)
	b := append([]string{}, other.ContainerIDs...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	if len(f.References) != len(other.References) {
		return false
	}
	for k, v := range f.References {
		if other.References[k] != v {
			return false
		}
	}
	return true
}

// APIVersion parses an Engine API version string into a comparable value.
func APIVersion(s string) (int, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid API version %q", s)
	}
	major, e := strconv.Atoi(parts[0])
	if e != nil || major <= 0 {
		return 0, fmt.Errorf("invalid API version %q", s)
	}
	minor, e := strconv.Atoi(parts[1])
	if e != nil || minor < 0 {
		return 0, fmt.Errorf("invalid API version %q", s)
	}
	return major*100 + minor, nil
}

// Negotiate selects the Engine API version the daemon can serve within the
// verified range. A daemon serves versions between its own minimum and
// current API; the client requests the highest version both sides share and
// fails explicitly when that would leave the verified range.
func Negotiate(daemonVersion, daemonMin, clientMax string) (string, error) {
	daemon, e := APIVersion(daemonVersion)
	if e != nil {
		return "", fmt.Errorf("daemon API version %q unavailable: %w", daemonVersion, e)
	}
	floor, e := APIVersion(APIFloor)
	if e != nil {
		return "", e
	}
	ceiling, e := APIVersion(clientMax)
	if e != nil {
		return "", e
	}
	chosen := daemon
	if chosen > ceiling {
		chosen = ceiling
	}
	if chosen < floor {
		return "", fmt.Errorf("daemon API %s is below the verified %s floor", daemonVersion, APIFloor)
	}
	if daemonMin != "" {
		dmin, err := APIVersion(daemonMin)
		if err != nil {
			return "", fmt.Errorf("daemon minimum API %q unavailable: %w", daemonMin, err)
		}
		if dmin > chosen {
			chosen = dmin
		}
	}
	if chosen > ceiling {
		return "", fmt.Errorf("daemon minimum API %s exceeds the tested %s ceiling", daemonMin, clientMax)
	}
	if chosen > daemon {
		return "", fmt.Errorf("daemon cannot serve negotiated API %s", fmt.Sprintf("%d.%d", chosen/100, chosen%100))
	}
	return fmt.Sprintf("%d.%d", chosen/100, chosen%100), nil
}
