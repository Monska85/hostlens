package gateway

// inputSchema describes the public tool contract. Args remains the internal IPC
// representation; unrelated fields must not silently reach collectors.
func inputSchema(tool string) map[string]any {
	properties := map[string]any{}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	var fields, required []string
	switch tool {
	case "read_config":
		fields, required = []string{"path"}, []string{"path"}
	case "get_service_status":
		fields, required = []string{"unit"}, []string{"unit"}
	case "list_services", "list_packages":
		fields = []string{"offset", "limit"}
	case "query_logs":
		fields = []string{"path", "unit", "format", "raw_tail", "since", "until", "priority", "limit"}
		schema["oneOf"] = []any{map[string]any{"required": []string{"path"}}, map[string]any{"required": []string{"unit"}}}
	}
	for _, name := range fields {
		field := map[string]any{"type": "string"}
		switch name {
		case "path", "unit":
			field["minLength"] = 1
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
