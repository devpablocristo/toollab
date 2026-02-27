package catalog

import (
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/gen"
)

type Catalog struct {
	Total     int        `json:"total"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	OperationID string      `json:"operation_id,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Deprecated  bool        `json:"deprecated,omitempty"`
	Parameters  []Param     `json:"parameters,omitempty"`
	RequestBody *Body       `json:"request_body,omitempty"`
	Responses   []Response  `json:"responses,omitempty"`
}

type Param struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Format      string `json:"format,omitempty"`
	Description string `json:"description,omitempty"`
	Example     any    `json:"example,omitempty"`
}

type Body struct {
	ContentType string  `json:"content_type"`
	Required    bool    `json:"required"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type Response struct {
	Status      string  `json:"status"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type Schema struct {
	Type       string   `json:"type"`
	Format     string   `json:"format,omitempty"`
	Fields     []Field  `json:"fields,omitempty"`
	Items      *Schema  `json:"items,omitempty"`
	Example    any      `json:"example,omitempty"`
}

type Field struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Format      string  `json:"format,omitempty"`
	Required    bool    `json:"required"`
	Description string  `json:"description,omitempty"`
	Example     any     `json:"example,omitempty"`
	Enum        []any   `json:"enum,omitempty"`
	Nullable    bool    `json:"nullable,omitempty"`
	Items       *Schema `json:"items,omitempty"`
}

func Build(doc *gen.OpenAPIDoc) *Catalog {
	if doc == nil {
		return &Catalog{Endpoints: []Endpoint{}}
	}

	var endpoints []Endpoint

	paths := sortedKeys(doc.Paths)
	for _, path := range paths {
		pi := doc.Paths[path]
		allParams := pi.Parameters

		for _, pair := range pi.Operations() {
			op := pair.Op
			if op == nil {
				continue
			}

			mergedParams := mergeParams(allParams, op.Parameters)

			ep := Endpoint{
				Method:      pair.Method,
				Path:        path,
				OperationID: op.OperationID,
				Summary:     op.Summary,
				Description: op.Description,
				Tags:        op.Tags,
				Deprecated:  op.Deprecated,
				Parameters:  convertParams(mergedParams),
				RequestBody: convertRequestBody(op.RequestBody, doc),
				Responses:   convertResponses(op.Responses, doc),
			}
			endpoints = append(endpoints, ep)
		}
	}

	if endpoints == nil {
		endpoints = []Endpoint{}
	}

	return &Catalog{
		Total:     len(endpoints),
		Endpoints: endpoints,
	}
}

func convertParams(params []gen.Parameter) []Param {
	if len(params) == 0 {
		return nil
	}
	out := make([]Param, 0, len(params))
	for _, p := range params {
		cp := Param{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required,
			Description: p.Description,
			Example:     p.Example,
		}
		if p.Schema != nil {
			cp.Type = p.Schema.Type
			cp.Format = p.Schema.Format
		}
		out = append(out, cp)
	}
	return out
}

func convertRequestBody(rb *gen.RequestBody, doc *gen.OpenAPIDoc) *Body {
	if rb == nil || rb.Content == nil {
		return nil
	}

	ct, mt := pickMediaType(rb.Content)
	if mt.Schema == nil {
		return nil
	}

	return &Body{
		ContentType: ct,
		Required:    rb.Required,
		Description: rb.Description,
		Schema:      flattenSchema(mt.Schema, doc, 0),
	}
}

func convertResponses(responses map[string]gen.ResponseObj, doc *gen.OpenAPIDoc) []Response {
	if len(responses) == 0 {
		return nil
	}

	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	out := make([]Response, 0, len(codes))
	for _, code := range codes {
		r := responses[code]
		resp := Response{
			Status:      code,
			Description: r.Description,
		}
		if r.Content != nil {
			_, mt := pickMediaType(r.Content)
			if mt.Schema != nil {
				resp.Schema = flattenSchema(mt.Schema, doc, 0)
			}
		}
		out = append(out, resp)
	}
	return out
}

const maxDepth = 4

func flattenSchema(s *gen.SchemaObj, doc *gen.OpenAPIDoc, depth int) *Schema {
	if s == nil || depth > maxDepth {
		return nil
	}

	resolved := resolveSchema(s, doc)
	if resolved == nil {
		return nil
	}

	out := &Schema{
		Type:    resolved.Type,
		Format:  resolved.Format,
		Example: resolved.Example,
	}

	if resolved.Type == "array" && resolved.Items != nil {
		out.Items = flattenSchema(resolved.Items, doc, depth+1)
	}

	if resolved.Type == "object" || len(resolved.Properties) > 0 {
		if out.Type == "" {
			out.Type = "object"
		}
		requiredSet := map[string]bool{}
		for _, r := range resolved.Required {
			requiredSet[r] = true
		}

		propNames := sortedSchemaKeys(resolved.Properties)
		for _, name := range propNames {
			prop := resolved.Properties[name]
			rp := resolveSchema(prop, doc)
			if rp == nil {
				continue
			}

			f := Field{
				Name:        name,
				Type:        rp.Type,
				Format:      rp.Format,
				Required:    requiredSet[name],
				Description: rp.Description,
				Example:     rp.Example,
				Enum:        rp.Enum,
				Nullable:    rp.Nullable,
			}

			if rp.Type == "array" && rp.Items != nil {
				f.Items = flattenSchema(rp.Items, doc, depth+1)
			}
			if rp.Type == "object" || len(rp.Properties) > 0 {
				f.Items = flattenSchema(rp, doc, depth+1)
			}

			out.Fields = append(out.Fields, f)
		}
	}

	return out
}

func resolveSchema(s *gen.SchemaObj, doc *gen.OpenAPIDoc) *gen.SchemaObj {
	if s == nil {
		return nil
	}

	if s.Ref != "" {
		resolved, err := doc.ResolveRef(s.Ref)
		if err != nil {
			return nil
		}
		return resolved
	}

	if len(s.AllOf) > 0 {
		merged := &gen.SchemaObj{
			Type:       "object",
			Properties: map[string]*gen.SchemaObj{},
		}
		for _, part := range s.AllOf {
			rp := resolveSchema(part, doc)
			if rp == nil {
				continue
			}
			if rp.Type != "" {
				merged.Type = rp.Type
			}
			if rp.Description != "" {
				merged.Description = rp.Description
			}
			for k, v := range rp.Properties {
				merged.Properties[k] = v
			}
			merged.Required = append(merged.Required, rp.Required...)
		}
		return merged
	}

	return s
}

func pickMediaType(content map[string]gen.MediaTypeObj) (string, gen.MediaTypeObj) {
	for _, preferred := range []string{"application/json", "application/xml", "text/plain"} {
		if mt, ok := content[preferred]; ok {
			return preferred, mt
		}
	}
	for ct, mt := range content {
		if strings.Contains(ct, "json") {
			return ct, mt
		}
	}
	for ct, mt := range content {
		return ct, mt
	}
	return "", gen.MediaTypeObj{}
}

func mergeParams(pathLevel, opLevel []gen.Parameter) []gen.Parameter {
	seen := map[string]bool{}
	out := make([]gen.Parameter, 0, len(pathLevel)+len(opLevel))

	for _, p := range opLevel {
		key := fmt.Sprintf("%s:%s", p.In, p.Name)
		seen[key] = true
		out = append(out, p)
	}
	for _, p := range pathLevel {
		key := fmt.Sprintf("%s:%s", p.In, p.Name)
		if !seen[key] {
			out = append(out, p)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedSchemaKeys(m map[string]*gen.SchemaObj) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
