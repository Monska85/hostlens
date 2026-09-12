package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToolRegistryIsClosedAndComplete(t *testing.T) {
	definitions := ToolDefinitions()
	if err := ValidateRegistry(definitions); err != nil || len(definitions) != len(ToolNames()) {
		t.Fatalf("production registry: %v", err)
	}
	for _, tc := range []struct {
		name string
		defs []ToolDefinition
	}{
		{"missing fields", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic}}},
		{"unknown effect", []ToolDefinition{{Name: "broken", Effect: Effect("write"), RequiredRole: RoleDiagnostics, Capability: "broken", Schema: SchemaEmpty, Description: "broken"}}},
		{"unknown role", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic, RequiredRole: Role("typo"), Capability: "broken", Schema: SchemaEmpty, Description: "broken"}}},
		{"unknown schema", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic, RequiredRole: RoleDiagnostics, Capability: "broken", Schema: Schema("typo"), Description: "broken"}}},
		{"duplicate", []ToolDefinition{{Name: "same", Effect: EffectDiagnostic, RequiredRole: RoleHealth, Capability: "same", Schema: SchemaEmpty, Description: "same"}, {Name: "same", Effect: EffectDiagnostic, RequiredRole: RoleHealth, Capability: "same", Schema: SchemaEmpty, Description: "same"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ValidateRegistry(tc.defs) == nil {
				t.Fatal("invalid registry accepted")
			}
		})
	}
	definitions[0].Name = "mutated"
	names := ToolNames()
	names[0] = "mutated"
	if _, ok := Tool("mutated"); ok || ToolNames()[0] == "mutated" {
		t.Fatal("registry accessors exposed mutable state")
	}
	if !EffectAllowed(EffectDiagnostic, true) || EffectAllowed(EffectRemediation, true) || !EffectAllowed(EffectRemediation, false) || EffectAllowed(Effect("unknown"), false) {
		t.Fatal("effect gate is not fail closed")
	}
}

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
