package gen

import "testing"

func TestGenerateBodyRequiredOnly(t *testing.T) {
	doc := &OpenAPIDoc{
		Components: &Components{
			Schemas: map[string]*SchemaObj{},
		},
	}
	schema := &SchemaObj{
		Type: "object",
		Required: []string{"name", "active"},
		Properties: map[string]*SchemaObj{
			"name":        {Type: "string"},
			"active":      {Type: "boolean"},
			"description": {Type: "string"},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := body["name"]; !ok {
		t.Error("expected 'name' in body")
	}
	if _, ok := body["active"]; !ok {
		t.Error("expected 'active' in body")
	}
	if _, ok := body["description"]; ok {
		t.Error("optional 'description' should not be in body")
	}
}

func TestGenerateBodyUsesExample(t *testing.T) {
	doc := &OpenAPIDoc{Components: &Components{Schemas: map[string]*SchemaObj{}}}
	schema := &SchemaObj{
		Type:     "object",
		Required: []string{"name"},
		Properties: map[string]*SchemaObj{
			"name": {Type: "string", Example: "Buddy"},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["name"] != "Buddy" {
		t.Errorf("expected 'Buddy', got %v", body["name"])
	}
}

func TestGenerateBodyUsesEnum(t *testing.T) {
	doc := &OpenAPIDoc{Components: &Components{Schemas: map[string]*SchemaObj{}}}
	schema := &SchemaObj{
		Type:     "object",
		Required: []string{"species"},
		Properties: map[string]*SchemaObj{
			"species": {Type: "string", Enum: []any{"dog", "cat"}},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["species"] != "dog" {
		t.Errorf("expected 'dog', got %v", body["species"])
	}
}

func TestGenerateBodyRef(t *testing.T) {
	doc := &OpenAPIDoc{
		Components: &Components{
			Schemas: map[string]*SchemaObj{
				"Pet": {
					Type:     "object",
					Required: []string{"name"},
					Properties: map[string]*SchemaObj{
						"name": {Type: "string", Example: "Rex"},
					},
				},
			},
		},
	}
	schema := &SchemaObj{Ref: "#/components/schemas/Pet"}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["name"] != "Rex" {
		t.Errorf("expected 'Rex', got %v", body["name"])
	}
}

func TestGenerateBodyNested(t *testing.T) {
	doc := &OpenAPIDoc{Components: &Components{Schemas: map[string]*SchemaObj{}}}
	schema := &SchemaObj{
		Type:     "object",
		Required: []string{"address"},
		Properties: map[string]*SchemaObj{
			"address": {
				Type:     "object",
				Required: []string{"city"},
				Properties: map[string]*SchemaObj{
					"city":   {Type: "string"},
					"zip":    {Type: "string"},
				},
			},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	addr, ok := body["address"].(map[string]any)
	if !ok {
		t.Fatalf("expected address to be map, got %T", body["address"])
	}
	if _, ok := addr["city"]; !ok {
		t.Error("expected 'city' in nested object")
	}
	if _, ok := addr["zip"]; ok {
		t.Error("optional 'zip' should not be in nested object")
	}
}

func TestGenerateBodyNoRequired(t *testing.T) {
	doc := &OpenAPIDoc{Components: &Components{Schemas: map[string]*SchemaObj{}}}
	schema := &SchemaObj{
		Type:     "object",
		Required: nil,
		Properties: map[string]*SchemaObj{
			"name": {Type: "string"},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) != 0 {
		t.Errorf("expected empty body, got %v", body)
	}
}

func TestSchemaDefaultFormats(t *testing.T) {
	doc := &OpenAPIDoc{Components: &Components{Schemas: map[string]*SchemaObj{}}}

	tests := []struct {
		schema *SchemaObj
		expect any
	}{
		{&SchemaObj{Type: "string", Format: "uuid"}, "00000000-0000-0000-0000-000000000001"},
		{&SchemaObj{Type: "string", Format: "email"}, "test@example.com"},
		{&SchemaObj{Type: "string", Format: "uri"}, "https://example.com"},
		{&SchemaObj{Type: "integer"}, 1},
		{&SchemaObj{Type: "boolean"}, true},
	}

	for _, tt := range tests {
		val, err := schemaDefault(tt.schema, doc, 0)
		if err != nil {
			t.Fatalf("error for %v: %v", tt.schema.Type, err)
		}
		if val != tt.expect {
			t.Errorf("for type=%s format=%s: expected %v, got %v", tt.schema.Type, tt.schema.Format, tt.expect, val)
		}
	}
}

func TestGenerateBodyAllOf(t *testing.T) {
	doc := &OpenAPIDoc{
		Components: &Components{
			Schemas: map[string]*SchemaObj{
				"Base": {
					Type:     "object",
					Required: []string{"id"},
					Properties: map[string]*SchemaObj{
						"id": {Type: "string", Format: "uuid"},
					},
				},
			},
		},
	}
	schema := &SchemaObj{
		AllOf: []*SchemaObj{
			{Ref: "#/components/schemas/Base"},
			{
				Type:     "object",
				Required: []string{"name"},
				Properties: map[string]*SchemaObj{
					"name": {Type: "string", Example: "Test"},
				},
			},
		},
	}
	body, err := GenerateBody(schema, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := body["id"]; !ok {
		t.Error("expected 'id' from allOf base")
	}
	if body["name"] != "Test" {
		t.Errorf("expected 'Test', got %v", body["name"])
	}
}
