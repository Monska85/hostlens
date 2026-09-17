package gateway

import (
	"bytes"
	"encoding/json"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/google/jsonschema-go/jsonschema"
)

// Resolved schemas for every registered tool, computed once at init from the
// registry definitions. Input validation runs before IPC and token
// re-verification; output validation runs after the backend call.
var (
	resolvedInputs  = map[string]*jsonschema.Resolved{}
	resolvedOutputs = map[string]*jsonschema.Resolved{}
)

func init() {
	for _, definition := range contract.ToolDefinitions() {
		in, err := definition.InputSchema.Resolve(nil)
		if err != nil {
			panic("resolve input schema for " + definition.Name + ": " + err.Error())
		}
		out, err := definition.OutputSchema.Resolve(nil)
		if err != nil {
			panic("resolve output schema for " + definition.Name + ": " + err.Error())
		}
		resolvedInputs[definition.Name] = in
		resolvedOutputs[definition.Name] = out
	}
}

// validateInput checks raw tool-call arguments against the tool's resolved
// input schema. Absent arguments validate as an empty object, matching tools
// whose schema has no required members.
func validateInput(tool string, arguments json.RawMessage) error {
	resolved, ok := resolvedInputs[tool]
	if !ok {
		return nil
	}
	instance := map[string]any{}
	if trimmed := bytes.TrimSpace(arguments); len(trimmed) > 0 {
		if err := json.Unmarshal(trimmed, &instance); err != nil {
			return err
		}
	}
	var unmarshaled any = instance
	return resolved.Validate(&unmarshaled)
}

// validateOutput checks a decoded backend result envelope against the tool's
// resolved output schema. The envelope is round-tripped through JSON first so
// time.Time and custom marshallers validate exactly as they serialize.
func validateOutput(definition contract.ToolDefinition, result contract.Result) error {
	resolved, ok := resolvedOutputs[definition.Name]
	if !ok {
		return nil
	}
	b, err := json.Marshal(result)
	if err != nil {
		return err
	}
	var unmarshaled any
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		return err
	}
	return resolved.Validate(&unmarshaled)
}
