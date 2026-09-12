package gateway

import "github.com/Monska85/hostlens/internal/contract"

// inputSchema describes the public tool contract. Args remains the internal IPC
// representation; unrelated fields must not silently reach collectors.
func inputSchema(tool contract.ToolDefinition) map[string]any {
	properties := map[string]any{}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	var fields, required []string
	switch tool.Schema {
	case contract.SchemaPath:
		fields, required = []string{"path"}, []string{"path"}
	case contract.SchemaUnit:
		fields, required = []string{"unit"}, []string{"unit"}
	case contract.SchemaPID:
		fields, required = []string{"pid"}, []string{"pid"}
	case contract.SchemaPage:
		fields = []string{"offset", "limit"}
	case contract.SchemaLogs:
		fields = []string{"path", "unit", "format", "raw_tail", "since", "until", "priority", "limit"}
		schema["oneOf"] = []any{map[string]any{"required": []string{"path"}}, map[string]any{"required": []string{"unit"}}}
	}
	for _, name := range fields {
		field := map[string]any{"type": "string"}
		switch name {
		case "path", "unit":
			field["minLength"] = 1
		case "pid":
			field["type"] = "integer"
			field["minimum"] = 1
			field["maximum"] = 4194304
		case "offset", "limit":
			field["type"] = "integer"
			field["minimum"] = 0
		case "priority":
			field["type"] = "integer"
			field["minimum"] = 0
			field["maximum"] = 7
		case "raw_tail":
			field["type"] = "boolean"
		case "since", "until":
			field["description"] = "Inclusive RFC3339 event timestamp"
		case "format":
			field["description"] = "File parser: jsonl, or raw with raw_tail"
		}
		properties[name] = field
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
