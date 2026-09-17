package contract

import "time"

// Result payloads for the host, audit, and log tools. Each type backs exactly
// one tool's `data` member; every JSON name equals the member emitted before
// the typed registry (see keys-before.txt in the change directory). Optional
// observations are pointers or omitempty fields, never placeholders; payload
// fields are never `any`.

// OSInfo is the OS release identity, architecture, and kernel release.
type OSInfo struct {
	Family       string `json:"family"`
	Architecture string `json:"architecture"`
	Distribution string `json:"distribution,omitempty"`
	Product      string `json:"product,omitempty"`
	Version      string `json:"version,omitempty"`
	Codename     string `json:"codename,omitempty"`
	Kernel       string `json:"kernel"`
}

// Inventory is the machine identity observed from os-release, machine-id, and
// DMI product strings.
type Inventory struct {
	OS           OSInfo `json:"os"`
	MachineID    string `json:"machine_id,omitempty"`
	ProductName  string `json:"product_name,omitempty"`
	SystemVendor string `json:"system_vendor,omitempty"`
	Scope        string `json:"scope"`
}

// HealthThreshold preserves the wire member names of the configured warning
// and critical limits; they are uppercase because the legacy payload embedded
// the untagged configuration struct.
type HealthThreshold struct {
	Warning  float64 `json:"Warning"`
	Critical float64 `json:"Critical"`
}

// HealthCheck is one named health check row. Value and threshold are absent
// when the check could not be observed ("status": "unknown").
type HealthCheck struct {
	Status    string           `json:"status"`
	Value     *float64         `json:"value,omitempty"`
	Threshold *HealthThreshold `json:"threshold,omitempty"`
}

// HealthFilesystem is one observed filesystem capacity row.
type HealthFilesystem struct {
	Mount            string   `json:"mount"`
	Type             string   `json:"type"`
	TotalBytes       uint64   `json:"total_bytes"`
	AvailableBytes   uint64   `json:"available_bytes"`
	UsedPercent      float64  `json:"used_percent"`
	InodeUsedPercent *float64 `json:"inode_used_percent,omitempty"`
}

// HealthSnapshot is one point-in-time host resource and systemd health sample.
type HealthSnapshot struct {
	MemoryTotalBytes     *float64               `json:"memory_total_bytes,omitempty"`
	MemoryAvailableBytes *float64               `json:"memory_available_bytes,omitempty"`
	SwapTotalBytes       *float64               `json:"swap_total_bytes,omitempty"`
	SwapUsedBytes        *float64               `json:"swap_used_bytes,omitempty"`
	LogicalCPUsAvailable *int                   `json:"logical_cpus_available,omitempty"`
	CPUAvailabilityScope string                 `json:"cpu_availability_scope,omitempty"`
	Load15               []float64              `json:"load_1_5_15,omitempty"`
	Filesystems          []HealthFilesystem     `json:"filesystems,omitempty"`
	ExcludedFilesystems  []string               `json:"excluded_filesystems"`
	FailedServices       []string               `json:"failed_services,omitempty"`
	CPUUtilization       *float64               `json:"cpu_utilization_percent,omitempty"`
	CPUSampleStart       *time.Time             `json:"cpu_sample_start,omitempty"`
	CPUSampleEnd         *time.Time             `json:"cpu_sample_end,omitempty"`
	CPUUtilizationScope  string                 `json:"cpu_utilization_scope,omitempty"`
	Checks               map[string]HealthCheck `json:"checks"`
	Severity             string                 `json:"severity"`
	Complete             bool                   `json:"complete"`
	MissingRequired      []string               `json:"missing_required"`
	Scope                string                 `json:"scope"`
}

// ServiceRow is one systemd unit's load, active, and sub state.
type ServiceRow struct {
	Unit   string `json:"unit"`
	Load   string `json:"load"`
	Active string `json:"active"`
	Sub    string `json:"sub"`
}

// Page is one bounded page of a list observation.
type Page[T any] struct {
	Items              []T  `json:"items"`
	SnapshotConsistent bool `json:"snapshot_consistent"`
}

// ServiceStatus is the selected property set of one systemd unit. Members are
// absent when systemctl does not report a value.
type ServiceStatus struct {
	ID            string `json:"Id,omitempty"`
	LoadState     string `json:"LoadState,omitempty"`
	ActiveState   string `json:"ActiveState,omitempty"`
	SubState      string `json:"SubState,omitempty"`
	UnitFileState string `json:"UnitFileState,omitempty"`
	MainPID       string `json:"MainPID,omitempty"`
	Result        string `json:"Result,omitempty"`
}

// PackageRow is one installed package identity.
type PackageRow struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// LogEntry is one decoded log record with a verified timestamp.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Priority  *int      `json:"priority,omitempty"`
}

// LogPage is one bounded log window from a file or journal unit.
type LogPage struct {
	RequestedSince        time.Time  `json:"requested_since"`
	RequestedUntil        time.Time  `json:"requested_until"`
	CoverageComplete      bool       `json:"coverage_complete"`
	RotationScope         string     `json:"rotation_scope,omitempty"`
	Ordering              string     `json:"ordering,omitempty"`
	Parser                string     `json:"parser,omitempty"`
	Entries               []LogEntry `json:"entries,omitempty"`
	Lines                 []string   `json:"lines,omitempty"`
	EarliestReturnedEvent *time.Time `json:"earliest_returned_event,omitempty"`
	LatestReturnedEvent   *time.Time `json:"latest_returned_event,omitempty"`
}

// ConfigFile is the content of one approved configuration file.
type ConfigFile struct {
	Content string `json:"content"`
}

// AuditCoverage is embedded by every audit tool payload to carry the
// coverage_complete declaration that the legacy collectors emitted at the top
// level of their data maps.
type AuditCoverage struct {
	CoverageComplete bool `json:"coverage_complete"`
}

// ProcessRow is one visible process's identity and resource counters.
type ProcessRow struct {
	PID            int    `json:"pid"`
	Name           string `json:"name"`
	State          string `json:"state"`
	ParentPID      uint64 `json:"parent_pid"`
	UserCPUTicks   uint64 `json:"user_cpu_ticks"`
	SystemCPUTicks uint64 `json:"system_cpu_ticks"`
	Threads        uint64 `json:"threads"`
	StartTicks     uint64 `json:"start_ticks"`
	VirtualBytes   uint64 `json:"virtual_bytes"`
	ResidentPages  uint64 `json:"resident_pages"`
}

// ProcessPage is one page of the process inventory with its coverage counts.
type ProcessPage struct {
	AuditCoverage
	Scope               string       `json:"scope"`
	SnapshotConsistent  bool         `json:"snapshot_consistent"`
	EnumeratedProcesses int          `json:"enumerated_processes"`
	ObservedProcesses   int          `json:"observed_processes"`
	Items               []ProcessRow `json:"items"`
}

// ProcessDetail is one process's full procfs observation. The credential and
// capability members keep the exact key casing of the /proc status file.
type ProcessDetail struct {
	PID               int      `json:"pid"`
	Name              string   `json:"name"`
	State             string   `json:"state"`
	ParentPID         uint64   `json:"parent_pid"`
	UserCPUTicks      uint64   `json:"user_cpu_ticks"`
	SystemCPUTicks    uint64   `json:"system_cpu_ticks"`
	Threads           uint64   `json:"threads"`
	StartTicks        uint64   `json:"start_ticks"`
	VirtualBytes      uint64   `json:"virtual_bytes"`
	ResidentPages     uint64   `json:"resident_pages"`
	UID               []uint64 `json:"uid,omitempty"`
	GID               []uint64 `json:"gid,omitempty"`
	NoNewPrivs        string   `json:"NoNewPrivs,omitempty"`
	Seccomp           string   `json:"Seccomp,omitempty"`
	CapEff            string   `json:"CapEff,omitempty"`
	Executable        string   `json:"executable,omitempty"`
	SocketInodes      []uint64 `json:"socket_inodes,omitempty"`
	IdentityRechecked bool     `json:"identity_rechecked"`
}

// ProcessInfo is one process observation with its namespace scope. The
// process member is present only when the observation was collected.
type ProcessInfo struct {
	AuditCoverage
	Scope              string         `json:"scope"`
	SnapshotConsistent bool           `json:"snapshot_consistent"`
	Process            *ProcessDetail `json:"process,omitempty"`
}

// InterfaceRow is one network interface's aggregate counters.
type InterfaceRow struct {
	Name      string `json:"name"`
	RxBytes   uint64 `json:"rx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxBytes   uint64 `json:"tx_bytes"`
	TxPackets uint64 `json:"tx_packets"`
	TxErrors  uint64 `json:"tx_errors"`
	TxDropped uint64 `json:"tx_dropped"`
}

// IPv6Address is one IPv6 address on one interface.
type IPv6Address struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Index     uint32 `json:"index"`
	Prefix    uint32 `json:"prefix"`
	Scope     uint32 `json:"scope"`
	Flags     uint32 `json:"flags"`
}

// IPv4LocalAddress is one local IPv4 address from the forwarding trie.
type IPv4LocalAddress struct {
	Address string `json:"address"`
}

// IPv4Route is one main-table IPv4 route.
type IPv4Route struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Netmask     string `json:"netmask"`
	Flags       uint32 `json:"flags"`
	Metric      uint32 `json:"metric"`
}

// IPv6Route is one IPv6 route row.
type IPv6Route struct {
	Interface         string `json:"interface"`
	Destination       string `json:"destination"`
	Source            string `json:"source"`
	Gateway           string `json:"gateway"`
	DestinationPrefix uint32 `json:"destination_prefix"`
	SourcePrefix      uint32 `json:"source_prefix"`
	Metric            uint32 `json:"metric"`
	Flags             uint32 `json:"flags"`
}

// SocketRow is one procfs socket table row; StateHex preserves the raw value.
type SocketRow struct {
	LocalAddress  string `json:"local_address"`
	LocalPort     uint64 `json:"local_port"`
	RemoteAddress string `json:"remote_address"`
	RemotePort    uint64 `json:"remote_port"`
	StateHex      string `json:"state_hex"`
	UID           uint32 `json:"uid"`
	Inode         uint64 `json:"inode"`
}

// NetworkInfo is the procfs network table projection for this namespace.
type NetworkInfo struct {
	AuditCoverage
	Scope              string             `json:"scope"`
	SnapshotConsistent bool               `json:"snapshot_consistent"`
	Interfaces         []InterfaceRow     `json:"interfaces,omitempty"`
	IPv6Addresses      []IPv6Address      `json:"ipv6_addresses,omitempty"`
	IPv4LocalAddresses []IPv4LocalAddress `json:"ipv4_local_addresses,omitempty"`
	IPv4Routes         []IPv4Route        `json:"ipv4_routes,omitempty"`
	IPv6Routes         []IPv6Route        `json:"ipv6_routes,omitempty"`
	TCP4Listeners      []SocketRow        `json:"tcp4_listeners,omitempty"`
	TCP6Listeners      []SocketRow        `json:"tcp6_listeners,omitempty"`
	UDP4Endpoints      []SocketRow        `json:"udp4_endpoints,omitempty"`
	UDP6Endpoints      []SocketRow        `json:"udp6_endpoints,omitempty"`
}

// AccountRow is one local account or group record. Account rows carry uid,
// home, and shell; group rows carry members.
type AccountRow struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	UID     *uint32  `json:"uid,omitempty"`
	GID     *uint32  `json:"gid,omitempty"`
	Home    *string  `json:"home,omitempty"`
	Shell   *string  `json:"shell,omitempty"`
	Members []string `json:"members,omitempty"`
}

// AccountPage is one page of local account records.
type AccountPage struct {
	AuditCoverage
	Scope              string       `json:"scope"`
	SnapshotConsistent bool         `json:"snapshot_consistent"`
	Items              []AccountRow `json:"items"`
}

// DeviceRow is one block device record.
type DeviceRow struct {
	Name       string `json:"name"`
	Major      uint32 `json:"major"`
	Minor      uint32 `json:"minor"`
	Blocks1024 uint64 `json:"blocks_1024"`
}

// MountRow is one mountinfo record. Source superblock options are excluded.
type MountRow struct {
	MountID    uint64   `json:"mount_id"`
	ParentID   uint64   `json:"parent_id"`
	Device     string   `json:"device"`
	Mount      string   `json:"mount"`
	Filesystem string   `json:"filesystem"`
	Flags      []string `json:"flags"`
}

// RaidArray is one software RAID array's identity and state.
type RaidArray struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// StorageInfo is the procfs block device, mount, and RAID projection.
type StorageInfo struct {
	AuditCoverage
	Scope              string      `json:"scope"`
	SnapshotConsistent bool        `json:"snapshot_consistent"`
	Devices            []DeviceRow `json:"devices,omitempty"`
	Mounts             []MountRow  `json:"mounts,omitempty"`
	SoftwareRaidArrays []RaidArray `json:"software_raid_arrays,omitempty"`
}

// RepoMetadata is one cached repository metadata file's timestamp evidence.
type RepoMetadata struct {
	Source      string     `json:"source"`
	ModifiedAt  time.Time  `json:"modified_at"`
	AgeSeconds  float64    `json:"age_seconds"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	ValidUntil  *time.Time `json:"valid_until,omitempty"`
	Expired     *bool      `json:"expired,omitempty"`
}

// UpdateInfo is the cached package repository metadata projection.
type UpdateInfo struct {
	AuditCoverage
	Scope              string         `json:"scope"`
	RepositoryMetadata []RepoMetadata `json:"repository_metadata,omitempty"`
}

// SecurityInfo is the selected kernel hardening control projection. Control
// paths are the observed keys.
type SecurityInfo struct {
	AuditCoverage
	Scope          string            `json:"scope"`
	KernelControls map[string]uint32 `json:"kernel_controls"`
	ActiveLSM      []string          `json:"active_lsm,omitempty"`
}

// HostlensServer is the server listener section of the active configuration.
type HostlensServer struct {
	Bind               []string `json:"bind"`
	Port               int      `json:"port"`
	TLSEnabled         bool     `json:"tls_enabled"`
	AllowInsecureHTTP  bool     `json:"allow_insecure_http"`
	TrustedProxyCount  int      `json:"trusted_proxy_count"`
	AllowedOriginCount int      `json:"allowed_origin_count"`
}

// HostlensLimits is the active operational ceiling section.
type HostlensLimits struct {
	ToolTimeoutMS           int64 `json:"tool_timeout_ms"`
	MaxConcurrentOperations int   `json:"max_concurrent_operations"`
	MaxResponseBytes        int   `json:"max_response_bytes"`
	MaxInspectionBytes      int   `json:"max_inspection_bytes"`
	MaxPageSize             int   `json:"max_page_size"`
}

// HostlensInfo is HostLens's own active configuration snapshot.
type HostlensInfo struct {
	AuditCoverage
	Scope                string          `json:"scope"`
	Version              string          `json:"version"`
	Mode                 string          `json:"mode"`
	Privilege            string          `json:"privilege"`
	UID                  int             `json:"uid"`
	GID                  int             `json:"gid"`
	PolicyFingerprint    string          `json:"policy_fingerprint"`
	Server               HostlensServer  `json:"server"`
	Limits               HostlensLimits  `json:"limits"`
	AuditSuccessfulCalls bool            `json:"audit_successful_calls"`
	AuditDomains         map[string]bool `json:"audit_domains"`
}

// ServiceInspection is the selected effective property set of one unit. The
// property names are the observed keys, and the member is present only when
// the unit was observed.
type ServiceInspection struct {
	AuditCoverage
	Scope   string            `json:"scope"`
	Service map[string]string `json:"service,omitempty"`
}

// PathInspection is one approved file or directory's metadata. Mode is the
// octal permission string; contents are never read.
type PathInspection struct {
	AuditCoverage
	Scope     string `json:"scope"`
	Path      string `json:"path"`
	Type      string `json:"type"`
	UID       uint32 `json:"uid"`
	GID       uint32 `json:"gid"`
	Mode      string `json:"mode"`
	SizeBytes int64  `json:"size_bytes"`
	Inode     uint64 `json:"inode"`
	Links     uint64 `json:"links"`
	MtimeUnix int64  `json:"mtime_unix"`
}
