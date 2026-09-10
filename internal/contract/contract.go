package contract

import (
	"context"
	"encoding/json"
	"time"
)

// Version is set by the archive builder for tagged releases.
var Version = "0.1.0-dev"

const (
	MaxToolTimeout   = time.Minute
	IPCReadTimeout   = 5 * time.Second
	IPCWriteTimeout  = IPCReadTimeout + MaxToolTimeout + 5*time.Second
	IPCClientTimeout = IPCWriteTimeout + 5*time.Second
)

var Tools = []string{"get_os_info", "get_inventory", "get_health_snapshot", "list_services", "get_service_status", "list_packages", "query_logs", "read_config"}

type Args struct {
	Path     string `json:"path,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Format   string `json:"format,omitempty"`
	RawTail  bool   `json:"raw_tail,omitempty"`
	Since    string `json:"since,omitempty"`
	Until    string `json:"until,omitempty"`
	Priority *int   `json:"priority,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}
type Request struct {
	Version    int    `json:"version"`
	ID         string `json:"id"`
	Generation string `json:"generation"`
	Tool       string `json:"tool"`
	Args       Args   `json:"args"`
}
type Issue struct {
	Code    string `json:"code"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message"`
}
type Result struct {
	Error      bool           `json:"error,omitempty"`
	Host       string         `json:"host,omitempty"`
	ObservedAt time.Time      `json:"observed_at"`
	Source     string         `json:"source,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Issues     []Issue        `json:"issues,omitempty"`
	Truncated  bool           `json:"truncated,omitempty"`
	NextOffset *int           `json:"next_offset,omitempty"`
}

func Failure(code string) Result {
	return Result{Error: true, ObservedAt: time.Now().UTC(), Issues: []Issue{{Code: code, Message: code}}}
}
func (r *Result) Issue(code, source, msg string) {
	r.Issues = append(r.Issues, Issue{code, source, msg})
}
func (r Result) Bounded(n int) Result {
	b, e := json.Marshal(r)
	mirror, mirrorErr := json.Marshal(map[string]any{"isError": r.Error, "content": []map[string]string{{"type": "text", "text": string(b)}}, "structuredContent": r})
	if e != nil || mirrorErr != nil || len(mirror) > n {
		f := Failure("response_limit")
		f.Truncated = true
		return f
	}
	return r
}

type Collector interface {
	Capabilities(context.Context) map[string]bool
	Collect(context.Context, string, Args) Result
}
