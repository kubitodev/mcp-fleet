package jellyfin

import (
	"log"

	"github.com/google/jsonschema-go/jsonschema"
)

// WithEnums generates a JSON schema for the given type and adds enum constraints
// to the specified fields. The enums map keys are field names (JSON names) and
// values are the allowed values for that field.
func WithEnums[T any](enums map[string][]any) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		log.Fatalf("schema generation failed: %v", err)
	}
	for field, values := range enums {
		if prop, ok := schema.Properties[field]; ok {
			prop.Enum = values
		}
	}
	return schema
}
