package contract

import "time"

// AuditRecord is the complete payload-free vocabulary retained for a tool call.
// Diagnostic arguments, source identities, results, errors, and content never
// enter this record.
type AuditRecord struct {
	Component string
	RequestID string
	TokenID   string
	Tool      string
	Outcome   string
	PeerIP    string
	ClientIP  string
	Duration  time.Duration
}

func (r AuditRecord) Attributes() []any {
	attributes := []any{
		"component", r.Component,
		"request_id", r.RequestID,
		"tool", r.Tool,
		"duration_ms", r.Duration.Milliseconds(),
		"outcome", r.Outcome,
	}
	if r.TokenID != "" {
		attributes = append(attributes, "token_id", r.TokenID)
	}
	if r.PeerIP != "" {
		attributes = append(attributes, "peer_ip", r.PeerIP)
	}
	if r.ClientIP != "" {
		attributes = append(attributes, "client_ip", r.ClientIP)
	}
	return attributes
}
