package cmd

import "testing"

func TestNormalizeListFieldsNativeColumnIDSkipsMetadata(t *testing.T) {
	input := []interface{}{map[string]interface{}{
		"column_id": "Col123",
		"select":    []string{"in_progress"},
	}}

	got, err := normalizeListFields(nil, "F123", input)
	if err != nil {
		t.Fatalf("normalizeListFields() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("normalizeListFields() returned %d fields, want 1", len(got))
	}
	field, ok := got[0].(map[string]interface{})
	if !ok || field["column_id"] != "Col123" {
		t.Fatalf("native field changed unexpectedly: %#v", got[0])
	}
}

func TestListSchemaDuplicateNameIsAmbiguousButIDsResolve(t *testing.T) {
	schema, err := listSchema(map[string]interface{}{
		"list_metadata": map[string]interface{}{
			"schema": []interface{}{
				map[string]interface{}{"id": "ColA", "key": "title", "name": "Title", "type": "text"},
				map[string]interface{}{"id": "ColB", "key": "status", "name": "Title", "type": "select"},
			},
		},
	})
	if err != nil {
		t.Fatalf("listSchema() error = %v", err)
	}
	if definition := schema["title"]; definition.ID != "" {
		t.Fatalf("duplicate name resolved to %#v, want ambiguity marker", definition)
	}
	if definition := schema["title"]; definition.Type != "" {
		t.Fatalf("duplicate name retained type %#v, want empty ambiguity marker", definition)
	}
	if definition := schema["cola"]; definition.ID != "ColA" || definition.Type != "text" {
		t.Fatalf("ID lookup for ColA = %#v, want ColA/text", definition)
	}
	if definition := schema["status"]; definition.ID != "ColB" || definition.Type != "select" {
		t.Fatalf("key lookup for status = %#v, want ColB/select", definition)
	}
}

func TestTypedListValuePreservesStringSlices(t *testing.T) {
	got, err := typedListValue("select", []string{"todo", "done"})
	if err != nil {
		t.Fatalf("typedListValue() error = %v", err)
	}
	values, ok := got["select"].([]string)
	if !ok {
		t.Fatalf("typedListValue() returned %T, want []string", got["select"])
	}
	if len(values) != 2 || values[0] != "todo" || values[1] != "done" {
		t.Fatalf("typedListValue() values = %#v, want todo/done", values)
	}
}

func TestValidateListSchemaRejectsMissingRequiredKey(t *testing.T) {
	err := validateListSchema([]interface{}{map[string]interface{}{
		"name": "Title",
		"type": "text",
	}})
	if err == nil {
		t.Fatal("validateListSchema() error = nil, want missing key error")
	}
}
