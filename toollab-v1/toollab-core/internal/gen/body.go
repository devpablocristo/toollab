package gen

import (
	"fmt"
	"sort"
)

const maxRecursionDepth = 10

func GenerateBody(schema *SchemaObj, doc *OpenAPIDoc) (map[string]any, error) {
	return generateBodyDepth(schema, doc, 0, false)
}

// GenerateBodyAll generates a body including all properties, not just required ones.
func GenerateBodyAll(schema *SchemaObj, doc *OpenAPIDoc) (map[string]any, error) {
	return generateBodyDepth(schema, doc, 0, true)
}

func generateBodyDepth(schema *SchemaObj, doc *OpenAPIDoc, depth int, includeAll bool) (map[string]any, error) {
	if depth > maxRecursionDepth {
		return map[string]any{}, nil
	}
	if schema == nil {
		return map[string]any{}, nil
	}

	resolved, err := resolveSchema(schema, doc)
	if err != nil {
		return nil, err
	}

	if len(resolved.AllOf) > 0 {
		resolved = mergeAllOf(resolved, doc)
	}

	if resolved.Example != nil {
		if m, ok := resolved.Example.(map[string]any); ok {
			return m, nil
		}
	}

	result := map[string]any{}

	fields := resolved.Required
	if (len(fields) == 0 || includeAll) && len(resolved.Properties) > 0 {
		fields = sortedKeys(resolved.Properties)
	}

	for _, fieldName := range fields {
		prop, ok := resolved.Properties[fieldName]
		if !ok {
			result[fieldName] = "example"
			continue
		}
		val, err := schemaDefault(prop, doc, depth+1)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", fieldName, err)
		}
		result[fieldName] = val
	}

	return result, nil
}

func sortedKeys(m map[string]*SchemaObj) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func schemaDefault(schema *SchemaObj, doc *OpenAPIDoc, depth int) (any, error) {
	if schema == nil {
		return nil, nil
	}
	resolved, err := resolveSchema(schema, doc)
	if err != nil {
		return nil, err
	}

	if resolved.Example != nil {
		return resolved.Example, nil
	}
	if len(resolved.Enum) > 0 {
		return resolved.Enum[0], nil
	}

	switch resolved.Type {
	case "string":
		if resolved.Format == "uuid" {
			return "00000000-0000-0000-0000-000000000001", nil
		}
		if resolved.Format == "date-time" {
			return "2025-01-01T00:00:00Z", nil
		}
		if resolved.Format == "uri" || resolved.Format == "url" {
			return "https://example.com", nil
		}
		if resolved.Format == "email" {
			return "test@example.com", nil
		}
		return "example", nil
	case "integer", "number":
		return 1, nil
	case "boolean":
		return true, nil
	case "array":
		if resolved.Items != nil && depth < maxRecursionDepth {
			item, err := schemaDefault(resolved.Items, doc, depth+1)
			if err != nil {
				return []any{}, nil
			}
			return []any{item}, nil
		}
		return []any{}, nil
	case "object":
		if depth >= maxRecursionDepth {
			return map[string]any{}, nil
		}
		return generateBodyDepth(resolved, doc, depth+1, false)
	default:
		return nil, nil
	}
}

func resolveSchema(schema *SchemaObj, doc *OpenAPIDoc) (*SchemaObj, error) {
	if schema.Ref == "" {
		return schema, nil
	}
	resolved, err := doc.ResolveRef(schema.Ref)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func mergeAllOf(schema *SchemaObj, doc *OpenAPIDoc) *SchemaObj {
	merged := &SchemaObj{
		Type:       schema.Type,
		Properties: map[string]*SchemaObj{},
	}
	if schema.Properties != nil {
		for k, v := range schema.Properties {
			merged.Properties[k] = v
		}
	}

	seenRequired := map[string]struct{}{}
	for _, r := range schema.Required {
		if _, ok := seenRequired[r]; !ok {
			merged.Required = append(merged.Required, r)
			seenRequired[r] = struct{}{}
		}
	}

	for _, sub := range schema.AllOf {
		resolved, err := resolveSchema(sub, doc)
		if err != nil {
			continue
		}
		if resolved.Type != "" && merged.Type == "" {
			merged.Type = resolved.Type
		}
		for k, v := range resolved.Properties {
			merged.Properties[k] = v
		}
		for _, r := range resolved.Required {
			if _, ok := seenRequired[r]; !ok {
				merged.Required = append(merged.Required, r)
				seenRequired[r] = struct{}{}
			}
		}
	}

	if merged.Type == "" {
		merged.Type = "object"
	}
	return merged
}
