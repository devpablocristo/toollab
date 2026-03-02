package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	d "toollab-core/internal/pipeline/usecases/domain"
	"toollab-core/internal/pipeline"
)

// Step implements schema inference (Step 2) and semantic annotation (Step 2.5).
type Step struct{}

func New() *Step { return &Step{} }

func (s *Step) Name() d.PipelineStep { return d.StepSchema }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	if state.Catalog == nil || len(state.Catalog.Endpoints) == 0 {
		return d.StepResult{Step: d.StepSchema, Status: "skipped", DurationMs: ms(start), Error: "no endpoints discovered"}
	}

	samples := state.Evidence.Samples()
	byEndpoint := groupByEndpoint(samples)

	var contracts []d.InferredContract
	registry := &d.SchemaRegistry{SchemaVersion: "v2", Schemas: make(map[string]d.Schema)}

	for _, ep := range state.Catalog.Endpoints {
		epSamples := byEndpoint[ep.EndpointID]
		if len(epSamples) == 0 {
			continue
		}

		contract := inferContract(ep, epSamples, registry)
		contracts = append(contracts, contract)
	}

	// Semantic annotation
	var annotations []d.SemanticAnnotation
	for _, c := range contracts {
		annot := annotateFields(c)
		if len(annot.Fields) > 0 {
			annotations = append(annotations, annot)
		}
	}

	state.Contracts = contracts
	state.SchemaRegistry = registry
	state.SemanticAnnotations = annotations

	state.Emit(pipeline.ProgressEvent{
		Step:    d.StepSchema,
		Phase:   "results",
		Message: fmt.Sprintf("Inferred %d contracts, %d semantic annotations", len(contracts), len(annotations)),
	})

	return d.StepResult{
		Step:       d.StepSchema,
		Status:     "ok",
		DurationMs: ms(start),
	}
}

func inferContract(ep d.EndpointEntry, samples []d.EvidenceSample, registry *d.SchemaRegistry) d.InferredContract {
	contract := d.InferredContract{
		EndpointID: ep.EndpointID,
		Method:     ep.Method,
		Path:       ep.Path,
		Confidence: 0.7,
	}

	// Group by status code
	byStatus := make(map[int][]d.EvidenceSample)
	for _, s := range samples {
		if s.Response != nil {
			byStatus[s.Response.Status] = append(byStatus[s.Response.Status], s)
		}
	}

	// Infer request schema from samples with bodies
	for _, s := range samples {
		if s.Request.Body != "" && s.Request.ContentType != "" {
			fields := inferFieldsFromJSON(s.Request.Body)
			if len(fields) > 0 {
				fp := d.SchemaFingerprint(s.Request.ContentType, fields)
				ref := d.SchemaRefID(ep.EndpointID, 0, fp)
				contract.RequestSchema = &d.RequestSchema{
					ContentType: s.Request.ContentType,
					SchemaRef:   ref,
					Fields:      fields,
				}
				registry.Schemas[ref] = d.Schema{
					SchemaRef:   ref,
					ContentType: s.Request.ContentType,
					Fields:      fields,
					Confidence:  0.7,
				}
				contract.EvidenceRefs = append(contract.EvidenceRefs, s.EvidenceID)
				break
			}
		}
	}

	// Infer response schemas per status
	for status, statusSamples := range byStatus {
		if len(statusSamples) == 0 {
			continue
		}
		best := statusSamples[0]
		ct := ""
		if best.Response != nil {
			ct = best.Response.ContentType
		}

		fields := inferFieldsFromJSON(best.Response.BodySnippet)
		fp := d.SchemaFingerprint(ct, fields)
		ref := d.SchemaRefID(ep.EndpointID, status, fp)

		rs := d.ResponseSchema{
			Status:      status,
			ContentType: ct,
			SchemaRef:   ref,
			Fields:      fields,
			ExampleRef:  best.EvidenceID,
		}
		contract.ResponseSchemas = append(contract.ResponseSchemas, rs)
		contract.EvidenceRefs = append(contract.EvidenceRefs, best.EvidenceID)

		if len(fields) > 0 {
			registry.Schemas[ref] = d.Schema{
				SchemaRef:   ref,
				ContentType: ct,
				Fields:      fields,
				Confidence:  0.7,
			}
		}
	}

	return contract
}

func inferFieldsFromJSON(body string) []d.SchemaField {
	if body == "" {
		return nil
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err != nil {
		// Try array
		var arr []any
		if err := json.Unmarshal([]byte(body), &arr); err != nil {
			return nil
		}
		if len(arr) > 0 {
			if m, ok := arr[0].(map[string]any); ok {
				obj = m
			}
		}
	}

	if obj == nil {
		return nil
	}

	var fields []d.SchemaField
	for k, v := range obj {
		f := d.SchemaField{Name: k, Type: jsonType(v)}
		if v != nil {
			f.Example = fmt.Sprintf("%v", v)
			if len(f.Example) > 100 {
				f.Example = f.Example[:100]
			}
		}
		fields = append(fields, f)
	}
	return fields
}

func jsonType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func annotateFields(c d.InferredContract) d.SemanticAnnotation {
	annot := d.SemanticAnnotation{EndpointID: c.EndpointID}

	allFields := collectAllFields(c)
	for _, f := range allFields {
		tag := inferSemanticTag(f)
		if tag != "" {
			annot.Fields = append(annot.Fields, d.FieldAnnotation{
				FieldPath:    f.Name,
				Tag:          tag,
				Confidence:   0.7,
				EvidenceRefs: c.EvidenceRefs,
			})
		}
	}
	return annot
}

func collectAllFields(c d.InferredContract) []d.SchemaField {
	var all []d.SchemaField
	if c.RequestSchema != nil {
		all = append(all, c.RequestSchema.Fields...)
	}
	for _, rs := range c.ResponseSchemas {
		all = append(all, rs.Fields...)
	}
	return all
}

func inferSemanticTag(f d.SchemaField) string {
	name := strings.ToLower(f.Name)

	switch {
	case name == "id" || strings.HasSuffix(name, "_id") || strings.HasSuffix(name, "id"):
		if strings.Contains(name, "user") || strings.Contains(name, "owner") {
			return "owner_field"
		}
		if strings.Contains(name, "tenant") {
			return "tenant_field"
		}
		return "id_field"
	case name == "status" || name == "state":
		return "status_field"
	case strings.Contains(name, "email"):
		return "email_field"
	case strings.Contains(name, "created") || strings.Contains(name, "updated") || strings.Contains(name, "timestamp") || strings.HasSuffix(name, "_at"):
		return "timestamp_field"
	case strings.Contains(name, "amount") || strings.Contains(name, "total") || strings.Contains(name, "price"):
		return "amount_field"
	case strings.Contains(name, "password") || strings.Contains(name, "secret") || strings.Contains(name, "token"):
		return "sensitive_field"
	}
	return ""
}

func groupByEndpoint(samples []d.EvidenceSample) map[string][]d.EvidenceSample {
	m := make(map[string][]d.EvidenceSample)
	for _, s := range samples {
		if s.EndpointID != "" {
			m[s.EndpointID] = append(m[s.EndpointID], s)
		}
	}
	return m
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
