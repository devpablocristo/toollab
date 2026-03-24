package report

import (
	"context"
	"fmt"
	"log"
	"time"

	artifactUC "toollab-core/internal/artifact"
	artDomain "toollab-core/internal/artifact/usecases/domain"
	"toollab-core/internal/exports"
	"toollab-core/internal/pipeline"
	d "toollab-core/internal/pipeline/usecases/domain"
)

// Step is the report builder (Step 9).
type Step struct {
	artifactSvc *artifactUC.Service
}

func New(artifactSvc *artifactUC.Service) *Step {
	return &Step{artifactSvc: artifactSvc}
}

func (s *Step) Name() d.PipelineStep { return d.StepReport }

func (s *Step) Run(ctx context.Context, state *pipeline.PipelineState) d.StepResult {
	start := time.Now()

	state.Emit(pipeline.ProgressEvent{Step: d.StepReport, Phase: "start", Message: "Generating exports..."})

	exportsIndex := d.ExportsIndex{}

	// Generate exports
	if state.Catalog != nil {
		data := exports.GenerateEndpointCatalogCSV(state.Catalog)
		if s.saveExport(state.RunID, artDomain.ArtifactEndpointCatalog, "endpoint_catalog.csv", data) {
			exportsIndex.EndpointCSV = "endpoint_catalog.csv"
		}
	}

	if state.AuthMatrix != nil {
		data := exports.GenerateAuthMatrixCSV(state.AuthMatrix)
		if s.saveExport(state.RunID, artDomain.ArtifactAuthMatrix, "auth_matrix.csv", data) {
			exportsIndex.AuthMatrixCSV = "auth_matrix.csv"
		}
	}

	if len(state.Contracts) > 0 {
		data := exports.GenerateContractMatrixCSV(state.Contracts)
		if s.saveExport(state.RunID, artDomain.ArtifactInferredContracts, "contract_matrix.csv", data) {
			exportsIndex.ContractMatrixCSV = "contract_matrix.csv"
		}
	}

	state.Emit(pipeline.ProgressEvent{Step: d.StepReport, Phase: "progress", Message: "CSV exports generated"})

	// Observability findings from evidence
	s.detectObservabilityFindings(state)

	// Performance findings
	s.detectPerformanceFindings(state)

	// Contract findings
	s.detectContractFindings(state)

	// Info leak findings
	s.detectInfoLeakFindings(state)

	state.Emit(pipeline.ProgressEvent{
		Step: d.StepReport, Phase: "results",
		Message: fmt.Sprintf("Report: %d total findings, %d evidence samples", len(state.FindingsRaw), state.Evidence.Count()),
	})

	return d.StepResult{Step: d.StepReport, Status: "ok", DurationMs: ms(start)}
}

func (s *Step) saveExport(runID string, artType artDomain.ArtifactType, name string, data []byte) bool {
	if _, err := s.artifactSvc.PutRaw(runID, artType, data); err != nil {
		log.Printf("save export %s: %v", name, err)
		return false
	}
	return true
}

func (s *Step) detectObservabilityFindings(state *pipeline.PipelineState) {
	hasCorrelation := false
	hasRequestID := false
	structuredErrors := 0
	unstructuredErrors := 0

	for _, sample := range state.Evidence.Samples() {
		if sample.Response == nil {
			continue
		}
		headers := sample.Response.Headers
		for k := range headers {
			kl := fmt.Sprintf("%s", k)
			if contains(kl, "correlation") || contains(kl, "trace") {
				hasCorrelation = true
			}
			if contains(kl, "request-id") || contains(kl, "x-request-id") {
				hasRequestID = true
			}
		}

		if sample.Response.Status >= 400 {
			if isJSON(sample.Response.BodySnippet) {
				structuredErrors++
			} else {
				unstructuredErrors++
			}
		}
	}

	if !hasCorrelation {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxObsNoCorrelationID), "_global", nil),
			TaxonomyID:     d.TaxObsNoCorrelationID,
			Severity:       d.SeverityLow,
			Category:       d.FindCatObs,
			Title:          "No correlation ID headers observed",
			Description:    "No se detectaron headers de correlation/trace ID en ninguna respuesta.",
			Confidence:     0.8,
			Classification: d.ClassCandidate,
		})
	}

	if !hasRequestID {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxObsNoRequestID), "_global", nil),
			TaxonomyID:     d.TaxObsNoRequestID,
			Severity:       d.SeverityLow,
			Category:       d.FindCatObs,
			Title:          "No request ID headers observed",
			Description:    "No se detectaron headers X-Request-ID en respuestas.",
			Confidence:     0.8,
			Classification: d.ClassCandidate,
		})
	}

	if unstructuredErrors > structuredErrors && unstructuredErrors > 3 {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxObsNoStructuredErr), "_global", nil),
			TaxonomyID:     d.TaxObsNoStructuredErr,
			Severity:       d.SeverityLow,
			Category:       d.FindCatObs,
			Title:          "Error responses lack structured format",
			Description:    fmt.Sprintf("%d errores sin estructura JSON vs %d con estructura.", unstructuredErrors, structuredErrors),
			Confidence:     0.7,
			Classification: d.ClassCandidate,
		})
	}
}

func (s *Step) detectPerformanceFindings(state *pipeline.PipelineState) {
	var latencies []int64
	for _, sample := range state.Evidence.Samples() {
		latencies = append(latencies, sample.Timing.LatencyMs)
	}

	if len(latencies) < 10 {
		return
	}

	sortInt64s(latencies)
	p95 := latencies[int(float64(len(latencies))*0.95)]
	p99 := latencies[int(float64(len(latencies))*0.99)]

	if p95 > 2000 {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxPerfP95High), "_global", nil),
			TaxonomyID:     d.TaxPerfP95High,
			Severity:       d.SeverityMedium,
			Category:       d.FindCatPerf,
			Title:          fmt.Sprintf("High P95 latency: %dms", p95),
			Description:    fmt.Sprintf("P95 de latencia es %dms, P99 es %dms. Esto puede indicar problemas de performance.", p95, p99),
			Confidence:     0.8,
			Classification: d.ClassCandidate,
		})
	}
}

func (s *Step) detectContractFindings(state *pipeline.PipelineState) {
	// Check for inconsistent error formats
	errorFormats := make(map[string]int)
	for _, sample := range state.Evidence.Samples() {
		if sample.Response != nil && sample.Response.Status >= 400 {
			ct := sample.Response.ContentType
			errorFormats[ct]++
		}
	}

	if len(errorFormats) > 2 {
		state.AddFindings(d.FindingRaw{
			FindingID:      d.FindingID(string(d.TaxConErrorFormatInconsist), "_global", nil),
			TaxonomyID:     d.TaxConErrorFormatInconsist,
			Severity:       d.SeverityMedium,
			Category:       d.FindCatContract,
			Title:          "Inconsistent error response formats",
			Description:    fmt.Sprintf("Se observaron %d formatos diferentes de error en respuestas.", len(errorFormats)),
			Confidence:     0.7,
			Classification: d.ClassCandidate,
		})
	}
}

func (s *Step) detectInfoLeakFindings(state *pipeline.PipelineState) {
	for _, sample := range state.Evidence.Samples() {
		if sample.Response == nil {
			continue
		}
		body := sample.Response.BodySnippet

		leakMarkers := []struct {
			marker string
			desc   string
		}{
			{"stack trace", "stack trace in response"},
			{"goroutine", "Go goroutine dump"},
			{"panic:", "Go panic"},
			{"SQL syntax", "SQL error details"},
			{"password", "password field in error"},
			{"/usr/local", "server path leak"},
			{"SQLSTATE", "PostgreSQL error details"},
		}

		for _, m := range leakMarkers {
			if contains(body, m.marker) {
				state.AddFindings(d.FindingRaw{
					FindingID:      d.FindingID(string(d.TaxSecInfoLeak), sample.EndpointID, []string{sample.EvidenceID}),
					TaxonomyID:     d.TaxSecInfoLeak,
					Severity:       d.SeverityMedium,
					Category:       d.FindCatInfoLeak,
					EndpointID:     sample.EndpointID,
					Title:          fmt.Sprintf("Information leak: %s in response from %s", m.desc, sample.Request.Path),
					Description:    fmt.Sprintf("Detectado '%s' en respuesta %d de %s %s.", m.marker, sample.Response.Status, sample.Request.Method, sample.Request.Path),
					EvidenceRefs:   []string{sample.EvidenceID},
					Confidence:     0.8,
					Classification: d.ClassCandidate,
				})
				break
			}
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || findStr(s, sub))
}

func findStr(s, sub string) bool {
	sl := fmt.Sprintf("%s", s)
	subl := fmt.Sprintf("%s", sub)
	for i := 0; i <= len(sl)-len(subl); i++ {
		if sl[i:i+len(subl)] == subl {
			return true
		}
	}
	return false
}

func isJSON(s string) bool {
	s = trimSpace(s)
	return (len(s) > 0 && (s[0] == '{' || s[0] == '['))
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	return s
}

func sortInt64s(s []int64) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

func ms(start time.Time) int64 { return time.Since(start).Milliseconds() }
