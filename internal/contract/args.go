package contract

// Argument types shared by the MCP tools. One type per argument shape; tools
// with the same shape reuse the same struct. The advertised input schema is
// inferred from these types and completed by finalizeInput; constraints that
// struct tags cannot express (jsonschema-go v0.4.3 tags carry only
// descriptions) are applied exactly once there.

// NoArgs accepts an empty object only.
type NoArgs struct{}

// PageArgs selects one bounded page of a list observation.
type PageArgs struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// PathArgs selects one policy-permitted file path.
type PathArgs struct {
	Path string `json:"path"`
}

// UnitArgs selects one systemd unit.
type UnitArgs struct {
	Unit string `json:"unit"`
}

// PIDArgs selects one visible process identifier.
type PIDArgs struct {
	PID int `json:"pid"`
}

// LogsArgs selects one bounded log window from exactly one approved file path
// or journal unit.
type LogsArgs struct {
	Path     string `json:"path,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Format   string `json:"format,omitempty" jsonschema:"File parser: jsonl, or raw with raw_tail"`
	RawTail  bool   `json:"raw_tail,omitempty"`
	Since    string `json:"since,omitempty" jsonschema:"Inclusive RFC3339 event timestamp"`
	Until    string `json:"until,omitempty" jsonschema:"Inclusive RFC3339 event timestamp"`
	Priority *int   `json:"priority,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ContainerArgs selects one container by name or stable identity prefix.
type ContainerArgs struct {
	Container string `json:"container"`
}

// DockerLogsArgs selects one bounded non-following log window for one container.
type DockerLogsArgs struct {
	Container string `json:"container"`
	Since     string `json:"since,omitempty" jsonschema:"Inclusive RFC3339 event timestamp"`
	Until     string `json:"until,omitempty" jsonschema:"Inclusive RFC3339 event timestamp"`
	Limit     int    `json:"limit,omitempty"`
}
