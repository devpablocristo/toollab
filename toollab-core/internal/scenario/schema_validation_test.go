package scenario_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller")
	}
	dir := filepath.Dir(currentFile)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "schemas")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("repository root not found")
	return ""
}

func compileSchema(t *testing.T, root, schemaFile string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	schemaPath := filepath.Join(root, "schemas", schemaFile)
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("compile schema %s: %v", schemaFile, err)
	}
	return schema
}

func mustReadDir(t *testing.T, path string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read dir %s: %v", path, err)
	}
	return entries
}

func TestScenarioSchemaFixtures(t *testing.T) {
	root := repoRoot(t)
	schema := compileSchema(t, root, "scenario.v1.schema.json")

	validDir := filepath.Join(root, "testdata", "scenario", "valid")
	for _, entry := range mustReadDir(t, validDir) {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		t.Run("valid_"+name, func(t *testing.T) {
			var doc any
			raw, err := os.ReadFile(filepath.Join(validDir, name))
			if err != nil {
				t.Fatalf("read valid fixture: %v", err)
			}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("yaml unmarshal: %v", err)
			}
			jsonRaw, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("json marshal: %v", err)
			}
			if err := schema.Validate(bytesReader(jsonRaw)); err != nil {
				t.Fatalf("fixture should be valid: %v", err)
			}
		})
	}

	invalidDir := filepath.Join(root, "testdata", "scenario", "invalid")
	for _, entry := range mustReadDir(t, invalidDir) {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		t.Run("invalid_"+name, func(t *testing.T) {
			var doc any
			raw, err := os.ReadFile(filepath.Join(invalidDir, name))
			if err != nil {
				t.Fatalf("read invalid fixture: %v", err)
			}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("yaml unmarshal: %v", err)
			}
			jsonRaw, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("json marshal: %v", err)
			}
			if err := schema.Validate(bytesReader(jsonRaw)); err == nil {
				t.Fatalf("fixture should be invalid")
			}
		})
	}
}

func TestEvidenceSchemaFixtures(t *testing.T) {
	root := repoRoot(t)
	schema := compileSchema(t, root, "evidence.v1.schema.json")

	validDir := filepath.Join(root, "testdata", "evidence", "valid")
	for _, entry := range mustReadDir(t, validDir) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		t.Run("valid_"+entry.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(validDir, entry.Name()))
			if err != nil {
				t.Fatalf("read valid evidence fixture: %v", err)
			}
			if err := schema.Validate(bytesReader(raw)); err != nil {
				t.Fatalf("fixture should be valid: %v", err)
			}
		})
	}

	invalidDir := filepath.Join(root, "testdata", "evidence", "invalid")
	for _, entry := range mustReadDir(t, invalidDir) {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		t.Run("invalid_"+entry.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(invalidDir, entry.Name()))
			if err != nil {
				t.Fatalf("read invalid evidence fixture: %v", err)
			}
			if err := schema.Validate(bytesReader(raw)); err == nil {
				t.Fatalf("fixture should be invalid")
			}
		})
	}
}

func bytesReader(b []byte) any {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return b
	}
	return v
}
