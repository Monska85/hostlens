package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestToolRegistryIsClosedAndComplete(t *testing.T) {
	definitions := ToolDefinitions()
	if err := ValidateRegistry(definitions); err != nil || len(definitions) != len(ToolNames()) {
		t.Fatalf("production registry: %v", err)
	}
	if len(definitions) != 27 {
		t.Fatalf("expected 27 tools, got %d", len(definitions))
	}
	typed := Define[NoArgs, OSInfo](ToolDefinition{Name: "broken", Effect: EffectDiagnostic, RequiredRole: RoleDiagnostics, Capability: "broken", Description: "typed"})
	for _, tc := range []struct {
		name string
		defs []ToolDefinition
	}{
		{"missing fields", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic}}},
		{"missing typed contract", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic, RequiredRole: RoleDiagnostics, Capability: "broken", Description: "no schemas"}}},
		{"unknown effect", []ToolDefinition{{Name: "broken", Effect: Effect("write"), RequiredRole: RoleDiagnostics, Capability: "broken", Description: "unknown effect", In: typed.In, Out: typed.Out, InputSchema: typed.InputSchema, OutputSchema: typed.OutputSchema}}},
		{"unknown role", []ToolDefinition{{Name: "broken", Effect: EffectDiagnostic, RequiredRole: Role("typo"), Capability: "broken", Description: "unknown role", In: typed.In, Out: typed.Out, InputSchema: typed.InputSchema, OutputSchema: typed.OutputSchema}}},
		{"duplicate", []ToolDefinition{typed, typed}},
		{"duplicate description", []ToolDefinition{typed, func() ToolDefinition { d := typed; d.Name = "other"; d.Capability = "other"; return d }()}},
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

// TestPayloadTypesHaveNoAnyFields walks every registry Out type and fails on
// any interface{} field: such fields infer to an empty schema and would make
// the published output contract unenforceable.
func TestPayloadTypesHaveNoAnyFields(t *testing.T) {
	var walk func(typ reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		switch typ.Kind() {
		case reflect.Interface:
			t.Fatalf("%s is an interface payload field", path)
		case reflect.Pointer, reflect.Slice, reflect.Array:
			walk(typ.Elem(), path+"[]")
		case reflect.Map:
			if typ.Key().Kind() != reflect.String {
				t.Fatalf("%s has a non-string map key", path)
			}
			walk(typ.Elem(), path+"{}")
		case reflect.Struct:
			if typ == reflect.TypeFor[time.Time]() {
				return
			}
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				if !field.IsExported() {
					continue
				}
				walk(field.Type, path+"."+field.Name)
			}
		}
	}
	for _, definition := range ToolDefinitions() {
		walk(definition.Out, definition.Name)
	}
}

// TestOutputSchemasEnumerateData requires every tool's output schema to carry
// an optional, enumerated `data` object member.
func TestOutputSchemasEnumerateData(t *testing.T) {
	for _, definition := range ToolDefinitions() {
		t.Run(definition.Name, func(t *testing.T) {
			data, ok := definition.OutputSchema.Properties["data"]
			if !ok || data == nil {
				t.Fatal("output schema lacks a data member")
			}
			for _, required := range definition.OutputSchema.Required {
				if required == "data" {
					t.Fatal("data member must stay optional so failure envelopes validate")
				}
			}
			if data.Type != "object" || len(data.Properties) == 0 {
				t.Fatalf("data member is not an enumerated object: %v", data)
			}
			if definition.OutputSchema.AdditionalProperties == nil {
				t.Fatal("envelope schema lost additionalProperties")
			}
		})
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
