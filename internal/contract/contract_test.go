package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSerializedMCPResponseCeiling(t *testing.T) {
	r := Result{ObservedAt: time.Now(), Data: map[string]any{"content": strings.Repeat("\"\\\n", 500)}}
	limited := r.Bounded(1024)
	if !limited.Truncated || limited.Data != nil {
		t.Fatal("MCP mirror escaping bypasses ceiling")
	}
	r = Result{ObservedAt: time.Now(), Data: map[string]any{"zero": 0}}
	b, _ := json.Marshal(r.Bounded(1024))
	if !strings.Contains(string(b), `"zero":0`) || strings.Contains(string(b), "null") {
		t.Fatal(string(b))
	}
}
