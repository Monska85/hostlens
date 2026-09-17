package contract

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The expectations below reproduce, verbatim, the schemas that
// internal/gateway/schema.go produced for each argument shape before the
// typed registry replaced it. The generated schema must be semantically
// identical: same properties, same constraints, same required sets.
func TestInputSchemasMatchLegacyConstraints(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want string
	}{
		{
			name: "empty",
			got:  mustInputSchema[NoArgs](),
			want: `{"type":"object","properties":{},"additionalProperties":false}`,
		},
		{
			name: "page",
			got:  mustInputSchema[PageArgs](),
			want: `{"type":"object","properties":{"limit":{"type":"integer","minimum":0},"offset":{"type":"integer","minimum":0}},"additionalProperties":false}`,
		},
		{
			name: "path",
			got:  mustInputSchema[PathArgs](),
			want: `{"type":"object","properties":{"path":{"type":"string","minLength":1}},"required":["path"],"additionalProperties":false}`,
		},
		{
			name: "unit",
			got:  mustInputSchema[UnitArgs](),
			want: `{"type":"object","properties":{"unit":{"type":"string","minLength":1}},"required":["unit"],"additionalProperties":false}`,
		},
		{
			name: "pid",
			got:  mustInputSchema[PIDArgs](),
			want: `{"type":"object","properties":{"pid":{"type":"integer","minimum":1,"maximum":4194304}},"required":["pid"],"additionalProperties":false}`,
		},
		{
			name: "logs",
			got:  mustInputSchema[LogsArgs](),
			want: `{"type":"object","properties":{"format":{"type":"string","description":"File parser: jsonl, or raw with raw_tail"},"limit":{"type":"integer","minimum":0},"path":{"type":"string","minLength":1},"priority":{"type":"integer","minimum":0,"maximum":7},"raw_tail":{"type":"boolean"},"since":{"type":"string","description":"Inclusive RFC3339 event timestamp"},"unit":{"type":"string","minLength":1},"until":{"type":"string","description":"Inclusive RFC3339 event timestamp"}},"oneOf":[{"required":["path"]},{"required":["unit"]}],"additionalProperties":false}`,
		},
		{
			name: "container",
			got:  mustInputSchema[ContainerArgs](),
			want: `{"type":"object","properties":{"container":{"type":"string","minLength":1}},"required":["container"],"additionalProperties":false}`,
		},
		{
			name: "docker_logs",
			got:  mustInputSchema[DockerLogsArgs](),
			want: `{"type":"object","properties":{"container":{"type":"string","minLength":1},"limit":{"type":"integer","minimum":0},"since":{"type":"string","description":"Inclusive RFC3339 event timestamp"},"until":{"type":"string","description":"Inclusive RFC3339 event timestamp"}},"required":["container"],"additionalProperties":false}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.got)
			if err != nil {
				t.Fatalf("marshal generated schema: %v", err)
			}
			var generated any
			if err := json.Unmarshal(raw, &generated); err != nil {
				t.Fatalf("unmarshal generated schema: %v", err)
			}
			var legacy any
			if err := json.Unmarshal([]byte(tc.want), &legacy); err != nil {
				t.Fatalf("unmarshal legacy expectation: %v", err)
			}
			if !reflect.DeepEqual(generated, legacy) {
				t.Fatalf("schema diverged from the legacy contract\nlegacy:    %s\ngenerated: %s", tc.want, raw)
			}
		})
	}
}
