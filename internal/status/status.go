package status

import (
	"context"
	"time"
)

// Issue identifies a measurement that could not be collected. A missing value
// is never substituted with zero.
type Issue struct {
	Source string `json:"source"`
	Kind   string `json:"kind"`
}

type Host struct {
	ObservedAt   time.Time `json:"observed_at"`
	OS           string    `json:"os,omitempty"`
	Platform     string    `json:"platform,omitempty"`
	Kernel       string    `json:"kernel,omitempty"`
	Architecture string    `json:"architecture"`
	Uptime       *uint64   `json:"uptime_seconds,omitempty"`
	LogicalCPUs  *int      `json:"logical_cpus,omitempty"`
	LoadOne      *float64  `json:"load_one,omitempty"`
	MemoryTotal  *uint64   `json:"memory_total_bytes,omitempty"`
	MemoryUsed   *uint64   `json:"memory_used_bytes,omitempty"`
	DiskTotal    *uint64   `json:"root_disk_total_bytes,omitempty"`
	DiskUsed     *uint64   `json:"root_disk_used_bytes,omitempty"`
	Issues       []Issue   `json:"issues,omitempty"`
}

type Service struct {
	ObservedAt   time.Time `json:"observed_at"`
	Name         string    `json:"name"`
	Availability string    `json:"availability,omitempty"`
	State        string    `json:"state,omitempty"`
	Detail       string    `json:"detail,omitempty"`
}

type Reader interface {
	Host(context.Context) Host
	Service(context.Context, string) (Service, error)
}

type ServiceRestarter interface {
	RestartService(context.Context, string) (RestartOutcome, error)
}

type RestartOutcome struct {
	Service   *Service `json:"service,omitempty"`
	Invoked   *bool    `json:"invoked,omitempty"`
	Completed *bool    `json:"completed,omitempty"`
	Issue     string   `json:"issue,omitempty"`
}
