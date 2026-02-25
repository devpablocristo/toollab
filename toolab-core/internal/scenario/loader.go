package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"

	"toolab-core/pkg/utils"
)

func Load(path string) (*Scenario, *Fingerprint, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read scenario: %w", err)
	}

	if err := validateScenarioSchema(raw); err != nil {
		return nil, nil, err
	}

	var parsed rawScenario
	var yamlDoc any
	if err := yaml.Unmarshal(raw, &yamlDoc); err != nil {
		return nil, nil, fmt.Errorf("unmarshal scenario yaml: %w", err)
	}
	jsonRaw, err := json.Marshal(yamlDoc)
	if err != nil {
		return nil, nil, fmt.Errorf("convert scenario yaml to json: %w", err)
	}
	if err := json.Unmarshal(jsonRaw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("unmarshal scenario json model: %w", err)
	}

	normalized, err := normalizeScenario(parsed)
	if err != nil {
		return nil, nil, err
	}

	canonical, err := utils.CanonicalJSON(normalized)
	if err != nil {
		return nil, nil, fmt.Errorf("canonicalize scenario: %w", err)
	}
	fp := &Fingerprint{
		ScenarioPath: path,
		ScenarioSHA:  utils.SHA256Hex(canonical),
	}

	return normalized, fp, nil
}

func validateScenarioSchema(rawYAML []byte) error {
	schemaPath, err := findSchemaPath("scenario.v1.schema.json")
	if err != nil {
		return err
	}

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return fmt.Errorf("compile scenario schema: %w", err)
	}

	var ydoc any
	if err := yaml.Unmarshal(rawYAML, &ydoc); err != nil {
		return fmt.Errorf("parse yaml for schema validation: %w", err)
	}
	jsonRaw, err := json.Marshal(ydoc)
	if err != nil {
		return fmt.Errorf("convert yaml to json for schema validation: %w", err)
	}
	var jdoc any
	if err := json.Unmarshal(jsonRaw, &jdoc); err != nil {
		return fmt.Errorf("parse normalized json doc: %w", err)
	}
	if err := schema.Validate(jdoc); err != nil {
		return fmt.Errorf("scenario schema validation: %w", err)
	}
	return nil
}

func findSchemaPath(fileName string) (string, error) {
	if direct := os.Getenv("TOOLAB_SCENARIO_SCHEMA"); direct != "" {
		if _, err := os.Stat(direct); err == nil {
			return direct, nil
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve cwd for schema: %w", err)
	}
	dir := wd
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "schemas", fileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", fmt.Errorf("schema %s not found from %s", fileName, wd)
}
