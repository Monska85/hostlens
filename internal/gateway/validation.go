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

// validatedOutput returns the serialized result after checking its schema.
// Validation uses the wire representation, including custom marshalers.
func validatedOutput(definition contract.ToolDefinition, result contract.Result) ([]byte, error) {
	b, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	resolved, ok := resolvedOutputs[definition.Name]
	if !ok {
		return b, nil
	}
	var unmarshaled any
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		return nil, err
	}
	if err := resolved.Validate(&unmarshaled); err != nil {
		return nil, err
	}
	return b, nil
}
