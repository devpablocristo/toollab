package gen

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type OpenAPIDoc struct {
	OpenAPI    string              `yaml:"openapi"`
	Servers    []Server            `yaml:"servers"`
	Paths      map[string]PathItem `yaml:"paths"`
	Components *Components         `yaml:"components"`
}

type Server struct {
	URL string `yaml:"url"`
}

type PathItem struct {
	Get        *Operation  `yaml:"get"`
	Post       *Operation  `yaml:"post"`
	Put        *Operation  `yaml:"put"`
	Patch      *Operation  `yaml:"patch"`
	Delete     *Operation  `yaml:"delete"`
	Head       *Operation  `yaml:"head"`
	Options    *Operation  `yaml:"options"`
	Parameters []Parameter `yaml:"parameters"`
}

type Operation struct {
	OperationID string                 `yaml:"operationId"`
	Tags        []string               `yaml:"tags"`
	Parameters  []Parameter            `yaml:"parameters"`
	RequestBody *RequestBody           `yaml:"requestBody"`
	Summary     string                 `yaml:"summary"`
	Description string                 `yaml:"description"`
	Responses   map[string]ResponseObj `yaml:"responses"`
	Deprecated  bool                   `yaml:"deprecated"`
}

type ResponseObj struct {
	Description string                  `yaml:"description"`
	Content     map[string]MediaTypeObj `yaml:"content"`
}

type Parameter struct {
	Name        string     `yaml:"name"`
	In          string     `yaml:"in"`
	Required    bool       `yaml:"required"`
	Schema      *SchemaObj `yaml:"schema"`
	Example     any        `yaml:"example"`
	Description string     `yaml:"description"`
}

type RequestBody struct {
	Required    bool                    `yaml:"required"`
	Content     map[string]MediaTypeObj `yaml:"content"`
	Description string                  `yaml:"description"`
}

type MediaTypeObj struct {
	Schema *SchemaObj `yaml:"schema"`
}

type SchemaObj struct {
	Type        string                `yaml:"type"`
	Format      string                `yaml:"format"`
	Properties  map[string]*SchemaObj `yaml:"properties"`
	Required    []string              `yaml:"required"`
	Items       *SchemaObj            `yaml:"items"`
	Example     any                   `yaml:"example"`
	Enum        []any                 `yaml:"enum"`
	Ref         string                `yaml:"$ref"`
	AllOf       []*SchemaObj          `yaml:"allOf"`
	Description string                `yaml:"description"`
	Nullable    bool                  `yaml:"nullable"`
}

type Components struct {
	Schemas         map[string]*SchemaObj     `yaml:"schemas"`
	SecuritySchemes map[string]SecurityScheme `yaml:"securitySchemes"`
}

type SecurityScheme struct {
	Type         string `yaml:"type"`
	Scheme       string `yaml:"scheme"`
	In           string `yaml:"in"`
	Name         string `yaml:"name"`
	BearerFormat string `yaml:"bearerFormat"`
}

func LoadSpec(source string) (*OpenAPIDoc, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err := fetchSpec(source)
		if err != nil {
			return nil, err
		}
		return ParseSpec(data)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("read spec file: %w", err)
	}
	return ParseSpec(data)
}

func fetchSpec(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch spec: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch spec: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return nil, fmt.Errorf("read spec body: %w", err)
	}
	return data, nil
}

func ParseSpec(data []byte) (*OpenAPIDoc, error) {
	var doc OpenAPIDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi spec: %w", err)
	}
	if doc.OpenAPI == "" {
		return nil, fmt.Errorf("not a valid OpenAPI document: missing openapi version field")
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		return nil, fmt.Errorf("unsupported OpenAPI version %q (only 3.x supported)", doc.OpenAPI)
	}
	return &doc, nil
}

func (doc *OpenAPIDoc) ResolveRef(ref string) (*SchemaObj, error) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported $ref %q (only %s... refs supported)", ref, prefix)
	}
	name := strings.TrimPrefix(ref, prefix)
	if doc.Components == nil || doc.Components.Schemas == nil {
		return nil, fmt.Errorf("$ref %q: no components/schemas defined", ref)
	}
	schema, ok := doc.Components.Schemas[name]
	if !ok {
		return nil, fmt.Errorf("$ref %q: schema %q not found", ref, name)
	}
	return schema, nil
}

type MethodOp struct {
	Method string
	Op     *Operation
}

var methodOrder = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

func (pi *PathItem) Operations() []MethodOp {
	candidates := []MethodOp{
		{"GET", pi.Get},
		{"POST", pi.Post},
		{"PUT", pi.Put},
		{"PATCH", pi.Patch},
		{"DELETE", pi.Delete},
		{"HEAD", pi.Head},
		{"OPTIONS", pi.Options},
	}
	var out []MethodOp
	for _, c := range candidates {
		if c.Op != nil {
			out = append(out, c)
		}
	}
	return out
}
