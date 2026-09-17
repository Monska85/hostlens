package contract

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// The tool input schemas are inferred from the argument types and completed
// here, in this one post-processor, because jsonschema-go v0.4.3 struct tags
// carry only descriptions (see the change notes.md): every other constraint of
// the published contract is applied here, never in a second table.

func mustInputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("input schema for %T: %v", *new(T), err))
	}
	finalizeInput[T](schema)
	return schema
}

// finalizeInput completes one inferred input schema: it keeps the empty-object
// property map, collapses nullable inferred fields to their plain type, and
// applies the argument constraints the tags cannot express.
func finalizeInput[T any](schema *jsonschema.Schema) {
	if schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	for name, property := range schema.Properties {
		if len(property.Types) == 2 && property.Types[0] == "null" {
			// Optional pointer arguments keep their wire type from the
			// pre-schema era: present values must match one plain type.
			property.Type = property.Types[1]
			property.Types = nil
		}
		constrainArg(name, property)
	}
	var zero T
	if _, logs := any(zero).(LogsArgs); logs {
		schema.OneOf = []*jsonschema.Schema{
			{Required: []string{"path"}},
			{Required: []string{"unit"}},
		}
	}
}

// constrainArg applies the constraints the argument contract has always
// published, by argument name.
func constrainArg(name string, property *jsonschema.Schema) {
	floatPtr := func(v float64) *float64 { return &v }
	intPtr := func(v int) *int { return &v }
	switch name {
	case "path", "unit", "container":
		property.MinLength = intPtr(1)
	case "pid":
		property.Minimum = floatPtr(1)
		property.Maximum = floatPtr(4194304)
	case "offset", "limit":
		property.Minimum = floatPtr(0)
	case "priority":
		property.Minimum = floatPtr(0)
		property.Maximum = floatPtr(7)
	}
}
