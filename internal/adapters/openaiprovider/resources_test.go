package openaiprovider

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTaskDefinitionsUseIndependentStrictSchemas(t *testing.T) {
	seenNames := make(map[string]struct{})
	for _, task := range Tasks() {
		definition, err := Definition(task)
		if err != nil {
			t.Fatalf("load %s definition: %v", task, err)
		}
		if strings.TrimSpace(definition.Prompt) == "" {
			t.Fatalf("%s prompt is empty", task)
		}
		if definition.PromptVersion != "v1" {
			t.Fatalf("%s has unexpected prompt version %q", task, definition.PromptVersion)
		}
		if _, exists := seenNames[definition.SchemaName]; exists {
			t.Fatalf("duplicate schema name %q", definition.SchemaName)
		}
		seenNames[definition.SchemaName] = struct{}{}

		var schema map[string]any
		if err := json.Unmarshal(definition.Schema, &schema); err != nil {
			t.Fatalf("decode %s schema: %v", task, err)
		}
		assertStrictObjectSchema(t, string(task), schema)
	}
}

func TestDefinitionRejectsUnknownTask(t *testing.T) {
	if _, err := Definition("other"); err == nil {
		t.Fatal("expected unknown task error")
	}
}

func assertStrictObjectSchema(
	t *testing.T,
	path string,
	schema map[string]any,
) {
	t.Helper()
	schemaType, _ := schema["type"].(string)
	if schemaType == "object" {
		if additional, exists := schema["additionalProperties"]; !exists ||
			additional != false {
			t.Fatalf("%s object must set additionalProperties=false", path)
		}
		properties, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		if len(properties) != len(required) {
			t.Fatalf(
				"%s must require every property: properties=%d required=%d",
				path,
				len(properties),
				len(required),
			)
		}
		for name, child := range properties {
			childSchema, ok := child.(map[string]any)
			if !ok {
				t.Fatalf("%s.%s is not an object schema", path, name)
			}
			assertStrictObjectSchema(t, path+"."+name, childSchema)
		}
	}
	if schemaType == "array" {
		items, ok := schema["items"].(map[string]any)
		if !ok {
			t.Fatalf("%s array has no item schema", path)
		}
		assertStrictObjectSchema(t, path+"[]", items)
	}
}
