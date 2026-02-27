package gen

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParseSpecYAML(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/openapi/petstore.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := ParseSpec(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Errorf("expected openapi 3.0.3, got %q", doc.OpenAPI)
	}
	if len(doc.Servers) == 0 || doc.Servers[0].URL != "http://localhost:3000" {
		t.Error("expected server URL http://localhost:3000")
	}
	if len(doc.Paths) == 0 {
		t.Error("expected paths to be parsed")
	}
}

func TestParseSpecRejectsSwagger2(t *testing.T) {
	data := []byte(`swagger: "2.0"
info:
  title: Old
  version: "1.0"
paths: {}`)
	_, err := ParseSpec(data)
	if err == nil {
		t.Fatal("expected error for swagger 2.0")
	}
}

func TestParseSpecRejectsMissingVersion(t *testing.T) {
	data := []byte(`info:
  title: No Version
paths: {}`)
	_, err := ParseSpec(data)
	if err == nil {
		t.Fatal("expected error for missing openapi field")
	}
}

func TestResolveRef(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/openapi/petstore.yaml")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	doc, err := ParseSpec(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	schema, err := doc.ResolveRef("#/components/schemas/CreatePetRequest")
	if err != nil {
		t.Fatalf("resolveRef: %v", err)
	}
	if schema.Type != "object" {
		t.Errorf("expected object type, got %q", schema.Type)
	}
	if len(schema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(schema.Required))
	}
}

func TestResolveRefInvalid(t *testing.T) {
	doc := &OpenAPIDoc{}
	_, err := doc.ResolveRef("#/components/schemas/Nonexistent")
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}

func TestLoadSpecFromURL(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/openapi/petstore.yaml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	doc, err := LoadSpec(srv.URL + "/openapi.yaml")
	if err != nil {
		t.Fatalf("load from URL: %v", err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Errorf("expected 3.0.3, got %q", doc.OpenAPI)
	}
}

func TestLoadSpecFromFile(t *testing.T) {
	doc, err := LoadSpec("../../../testdata/openapi/petstore.yaml")
	if err != nil {
		t.Fatalf("load from file: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Error("expected paths")
	}
}

func TestPathItemOperations(t *testing.T) {
	pi := PathItem{
		Get:  &Operation{OperationID: "listPets"},
		Post: &Operation{OperationID: "createPet"},
	}
	ops := pi.Operations()
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}
	if ops[0].Method != "GET" {
		t.Errorf("expected GET first, got %s", ops[0].Method)
	}
	if ops[1].Method != "POST" {
		t.Errorf("expected POST second, got %s", ops[1].Method)
	}
}
