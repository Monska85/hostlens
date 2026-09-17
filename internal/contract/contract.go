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

type Effect string
type Role string

const (
	EffectDiagnostic  Effect = "diagnostic"
	EffectRemediation Effect = "remediation"

	RoleHealth      Role = "health"
	RoleInspect     Role = "inspect"
	RoleDiagnostics Role = "diagnostics"
)

// Request is the IPC message for one tool call. Args stays raw JSON here; the
// backend decodes it against the tool's typed argument definition.
type Request struct {
	Version    int             `json:"version"`
	ID         string          `json:"id"`
	Generation string          `json:"generation"`
	Tool       string          `json:"tool"`
	Args       json.RawMessage `json:"args"`
}

type Issue struct {
	Code    string `json:"code"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message"`
}

// Result is the common observation envelope. Data carries the tool's typed
// payload struct; its schema is the tool's OutputSchema `data` member.
type Result struct {
	Error      bool      `json:"error,omitempty"`
	Host       string    `json:"host,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	Source     string    `json:"source,omitempty"`
	Data       any       `json:"data,omitempty"`
	Issues     []Issue   `json:"issues,omitempty"`
	Truncated  bool      `json:"truncated,omitempty"`
	NextOffset *int      `json:"next_offset,omitempty"`
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

// Collector is the diagnostics interface the backend calls. The args value is
// the typed argument struct named by the tool's definition.
type Collector interface {
	Capabilities(context.Context) map[string]bool
	Collect(context.Context, string, any) Result
}
