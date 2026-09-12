package contract

import (
	"testing"
	"time"
)

func TestAuditRecordUsesClosedVocabulary(t *testing.T) {
	record := AuditRecord{Component: "gateway", RequestID: "request", TokenID: "token", Tool: "read_config", Outcome: "issues", PeerIP: "127.0.0.1", ClientIP: "127.0.0.1", Duration: time.Second}
	for index := 0; index < len(record.Attributes()); index += 2 {
		key := record.Attributes()[index]
		switch key {
		case "component", "request_id", "token_id", "tool", "duration_ms", "outcome", "peer_ip", "client_ip":
		default:
			t.Fatalf("unbounded audit field %q", key)
		}
	}
}
