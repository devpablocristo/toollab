package usecases

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	analysisDomain "toollab-core/internal/analyze/domain"
	artifactUC "toollab-core/internal/artifact/usecases"
	auditDomain "toollab-core/internal/audit/usecases/domain"
	discoveryDomain "toollab-core/internal/discovery/usecases/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/interpretation/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
)

const (
	defaultTopEndpoints    = 30
	defaultTopFindings     = 20
	defaultMaxSnippetBytes = 4096
)

type DossierBuilder struct {
	artifactSvc *artifactUC.Service
}

func NewDossierBuilder(artifactSvc *artifactUC.Service) *DossierBuilder {
	return &DossierBuilder{artifactSvc: artifactSvc}
}

type DossierOptions struct {
	TopEndpoints    int `json:"top_endpoints,omitempty"`
	TopFindings     int `json:"top_findings,omitempty"`
	MaxSnippetBytes int `json:"max_snippet_bytes,omitempty"`
}

func (o DossierOptions) normalized() DossierOptions {
	if o.TopEndpoints <= 0 {
		o.TopEndpoints = defaultTopEndpoints
	}
	if o.TopFindings <= 0 {
		o.TopFindings = defaultTopFindings
	}
	if o.MaxSnippetBytes <= 0 {
		o.MaxSnippetBytes = defaultMaxSnippetBytes
	}
	return o
}

type loadedArtifacts struct {
	model    *discoveryDomain.ServiceModel
	report   *discoveryDomain.ModelReport
	plan     *scenarioDomain.ScenarioPlan
	pack     *evidenceDomain.EvidencePack
	audit    *auditDomain.AuditReport
	analysis *analysisDomain.Analysis
}

func (b *DossierBuilder) Build(runID string, opts DossierOptions) (domain.Dossier, *loadedArtifacts, error) {
	opts = opts.normalized()

	arts, err := b.loadArtifacts(runID)
	if err != nil {
		return domain.Dossier{}, nil, err
	}

	overview := buildOverview(arts.model, arts.report)
	topEndpoints := selectTopEndpoints(arts.model, arts.audit, arts.pack, opts.TopEndpoints)

	// Build highlights from Analysis findings if no legacy AuditReport
	var highlights []domain.AuditHighlight
	if arts.audit != nil {
		highlights = selectAuditHighlights(arts.audit, opts.TopFindings)
	} else if arts.analysis != nil {
		highlights = highlightsFromAnalysis(arts.analysis, opts.TopFindings)
	}

	evidenceReferenced := collectReferencedEvidenceIDs(highlights)
	samples := selectEvidenceSamples(arts.pack, evidenceReferenced, topEndpoints, opts.MaxSnippetBytes)

	var analysisSummary json.RawMessage
	if arts.analysis != nil {
		analysisSummary = buildCompactAnalysisSummary(arts.analysis)
	}

	return domain.Dossier{
		SchemaVersion:   "v1",
		RunID:           runID,
		GeneratedAt:     shared.Now(),
		ServiceOverview: overview,
		EndpointsTop:    topEndpoints,
		AuditHighlights: highlights,
		EvidenceSamples: samples,
		AnalysisSummary: analysisSummary,
		Constraints: []string{
			"Do not invent facts.",
			"Every fact/inference must reference real evidence_ids from the dossier samples.",
			"Contract anomalies are not confirmed bugs; propose experiments.",
			"If evidence is missing, output open_questions with suggested probes.",
		},
		Knobs: domain.DossierKnobs{
			MaxSnippetBytes: opts.MaxSnippetBytes,
			TopEndpoints:    opts.TopEndpoints,
			TopFindings:     opts.TopFindings,
		},
	}, arts, nil
}

func highlightsFromAnalysis(analysis *analysisDomain.Analysis, limit int) []domain.AuditHighlight {
	findings := analysis.Security.Findings
	if len(findings) > limit {
		findings = findings[:limit]
	}
	var result []domain.AuditHighlight
	for _, f := range findings {
		result = append(result, domain.AuditHighlight{
			FindingID: f.ID,
			RuleID:    f.ID,
			Severity:  f.Severity,
			Category:  f.Category,
			Title:     f.Title,
		})
	}
	return result
}

func buildCompactAnalysisSummary(a *analysisDomain.Analysis) json.RawMessage {
	compact := map[string]any{
		"score": a.Score,
		"grade": a.Grade,
		"discovery": map[string]any{
			"framework":       a.Discovery.Framework,
			"endpoints_count": a.Discovery.EndpointsCount,
			"confidence":      a.Discovery.Confidence,
		},
		"performance": map[string]any{
			"total_requests": a.Performance.TotalRequests,
			"success_rate":   a.Performance.SuccessRate,
			"p50_ms":         a.Performance.P50Ms,
			"p95_ms":         a.Performance.P95Ms,
			"p99_ms":         a.Performance.P99Ms,
		},
		"security": map[string]any{
			"score":    a.Security.Score,
			"grade":    a.Security.Grade,
			"summary":  a.Security.Summary,
			"findings": a.Security.Findings,
		},
		"contract": map[string]any{
			"compliance_rate":  a.Contract.ComplianceRate,
			"total_violations": a.Contract.TotalViolations,
			"violations":       a.Contract.Violations,
		},
		"coverage": map[string]any{
			"coverage_rate":    a.Coverage.CoverageRate,
			"total_endpoints":  a.Coverage.TotalEndpoints,
			"tested_endpoints": a.Coverage.TestedEndpoints,
		},
		"behavior": map[string]any{
			"invalid_input":     a.Behavior.InvalidInput,
			"missing_auth":      a.Behavior.MissingAuth,
			"not_found":         a.Behavior.NotFound,
			"error_consistency": a.Behavior.ErrorConsistency,
			"inferred_models":   a.Behavior.InferredModels,
		},
		"probes_summary": a.ProbesSummary,
	}
	data, _ := json.Marshal(compact)
	return data
}

func (b *DossierBuilder) loadArtifacts(runID string) (*loadedArtifacts, error) {
	arts := &loadedArtifacts{}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactEvidencePack); err == nil {
		var pack evidenceDomain.EvidencePack
		if json.Unmarshal(data, &pack) == nil {
			arts.pack = &pack
		}
	}
	if arts.pack == nil {
		return nil, fmt.Errorf("%w: evidence_pack required for interpretation", shared.ErrInvalidInput)
	}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactAnalysis); err == nil {
		var a analysisDomain.Analysis
		if json.Unmarshal(data, &a) == nil {
			arts.analysis = &a
		}
	}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactAuditReport); err == nil {
		var audit auditDomain.AuditReport
		if json.Unmarshal(data, &audit) == nil {
			arts.audit = &audit
		}
	}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactServiceModel); err == nil {
		var m discoveryDomain.ServiceModel
		if json.Unmarshal(data, &m) == nil {
			arts.model = &m
		}
	}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactModelReport); err == nil {
		var r discoveryDomain.ModelReport
		if json.Unmarshal(data, &r) == nil {
			arts.report = &r
		}
	}

	if data, _, err := b.artifactSvc.GetLatest(runID, shared.ArtifactScenarioPlan); err == nil {
		var p scenarioDomain.ScenarioPlan
		if json.Unmarshal(data, &p) == nil {
			arts.plan = &p
		}
	}

	return arts, nil
}

func buildOverview(model *discoveryDomain.ServiceModel, report *discoveryDomain.ModelReport) domain.ServiceOverview {
	ov := domain.ServiceOverview{}
	if model != nil {
		ov.Framework = model.Framework
		ov.EndpointsCount = len(model.Endpoints)
	}
	if report != nil {
		ov.Confidence = report.Confidence
		ov.Gaps = report.Gaps
	}
	return ov
}

func selectTopEndpoints(
	model *discoveryDomain.ServiceModel,
	audit *auditDomain.AuditReport,
	pack *evidenceDomain.EvidencePack,
	limit int,
) []domain.EndpointTop {
	if model == nil {
		return nil
	}

	score := make(map[string]int)
	if audit != nil {
		for _, f := range audit.Findings {
			for _, mr := range f.ModelRefs {
				score[mr.Kind+":"+mr.ID]++
			}
			for _, er := range f.EvidenceRefs {
				score["evidence:"+er]++
			}
		}
	}

	type scored struct {
		ep    discoveryDomain.Endpoint
		key   string
		score int
	}
	var items []scored
	for _, ep := range model.Endpoints {
		key := ep.Method + " " + ep.Path
		s := score["endpoint:"+key]
		if ep.Ref != nil {
			s += score["endpoint:"+ep.Ref.ID]
		}
		items = append(items, scored{ep: ep, key: key, score: s})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].key < items[j].key
	})

	if len(items) > limit {
		items = items[:limit]
	}

	var result []domain.EndpointTop
	for _, it := range items {
		result = append(result, domain.EndpointTop{
			EndpointKey: it.key,
			Method:      it.ep.Method,
			Path:        it.ep.Path,
			HandlerName: it.ep.HandlerName,
			ModelRef:    it.ep.Ref,
		})
	}
	return result
}

func selectAuditHighlights(audit *auditDomain.AuditReport, limit int) []domain.AuditHighlight {
	if audit == nil {
		return nil
	}
	findings := make([]auditDomain.Finding, len(audit.Findings))
	copy(findings, audit.Findings)

	if len(findings) > limit {
		findings = findings[:limit]
	}

	var result []domain.AuditHighlight
	for _, f := range findings {
		anomaly := string(f.Category) == string(auditDomain.CategoryContractAnomaly) || f.RuleID == "CONTRACT_ANOMALY_BASIC"
		result = append(result, domain.AuditHighlight{
			FindingID:    f.FindingID,
			RuleID:       f.RuleID,
			Severity:     string(f.Severity),
			Category:     string(f.Category),
			Title:        f.Title,
			Anomaly:      anomaly,
			EvidenceRefs: f.EvidenceRefs,
			ModelRefs:    f.ModelRefs,
		})
	}
	return result
}

func collectReferencedEvidenceIDs(highlights []domain.AuditHighlight) map[string]bool {
	ids := make(map[string]bool)
	for _, h := range highlights {
		for _, eid := range h.EvidenceRefs {
			ids[eid] = true
		}
	}
	return ids
}

const maxEvidenceSamples = 60

func selectEvidenceSamples(
	pack *evidenceDomain.EvidencePack,
	referenced map[string]bool,
	topEndpoints []domain.EndpointTop,
	maxSnippet int,
) []domain.EvidenceSample {
	if pack == nil {
		return nil
	}

	endpointKeys := make(map[string]bool)
	for _, ep := range topEndpoints {
		endpointKeys[ep.EndpointKey] = true
	}

	seen := make(map[string]bool)
	var samples []domain.EvidenceSample

	add := func(item evidenceDomain.EvidenceItem) {
		if seen[item.EvidenceID] || len(samples) >= maxEvidenceSamples {
			return
		}
		seen[item.EvidenceID] = true
		samples = append(samples, buildSample(item, maxSnippet))
	}

	// Pass 1: audit-referenced evidence (highest priority)
	for _, item := range pack.Items {
		if referenced[item.EvidenceID] {
			add(item)
		}
	}

	// Pass 2: one sample per top endpoint
	for _, item := range pack.Items {
		key := endpointKeyFromURL(item.Request.Method, item.Request.URL)
		if endpointKeys[key] {
			add(item)
		}
	}

	// Pass 3: diverse status codes — ensure 2xx, 4xx, 5xx, errors are represented
	endpointSeen := make(map[string]map[int]bool)
	for _, item := range pack.Items {
		if len(samples) >= maxEvidenceSamples {
			break
		}
		key := endpointKeyFromURL(item.Request.Method, item.Request.URL)
		status := 0
		if item.Response != nil {
			status = item.Response.Status
		}
		if endpointSeen[key] == nil {
			endpointSeen[key] = make(map[int]bool)
		}
		statusBucket := (status / 100) * 100
		if !endpointSeen[key][statusBucket] || item.Error != "" {
			endpointSeen[key][statusBucket] = true
			add(item)
		}
	}

	return samples
}

func buildSample(item evidenceDomain.EvidenceItem, maxSnippet int) domain.EvidenceSample {
	s := domain.EvidenceSample{
		EvidenceID:       item.EvidenceID,
		EndpointKey:      endpointKeyFromURL(item.Request.Method, item.Request.URL),
		RequestSignature: computeRequestSignature(item),
		RequestSummary: domain.RequestSummary{
			Method:        item.Request.Method,
			URL:           item.Request.URL,
			HeadersMasked: reMaskHeaders(item.Request.Headers),
			BodySnippet:   truncate(item.Request.BodyInlineTruncated, maxSnippet),
		},
		TimingMs: item.TimingMs,
		Error:    item.Error,
	}
	if item.Response != nil {
		s.ResponseSummary = &domain.ResponseSummary{
			Status:        item.Response.Status,
			HeadersMasked: reMaskHeaders(item.Response.Headers),
			BodySnippet:   truncate(item.Response.BodyInlineTruncated, maxSnippet),
		}
	}
	return s
}

func computeRequestSignature(item evidenceDomain.EvidenceItem) string {
	bodyHash := ""
	if item.Hashes != nil {
		bodyHash = item.Hashes.SHA256RequestBody
	}

	query := ""
	if u, err := url.Parse(item.Request.URL); err == nil {
		params := u.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			vals := params[k]
			sort.Strings(vals)
			for _, v := range vals {
				parts = append(parts, k+"="+v)
			}
		}
		query = strings.Join(parts, "&")
	}

	stableHeaders := ""
	if ct, ok := item.Request.Headers["Content-Type"]; ok {
		stableHeaders = "Content-Type:" + ct
	}

	raw := item.Request.Method + " " + extractPathFromURL(item.Request.URL) + "\n" +
		bodyHash + "\n" +
		query + "\n" +
		stableHeaders
	return shared.SHA256Bytes([]byte(raw))[:16]
}

func extractPathFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Path
}

func endpointKeyFromURL(method, rawURL string) string {
	return method + " " + extractPathFromURL(rawURL)
}

var sensitiveHeaders = map[string]bool{
	"authorization": true,
	"cookie":        true,
	"set-cookie":    true,
	"x-api-key":     true,
	"x-auth-token":  true,
}

func reMaskHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		if sensitiveHeaders[strings.ToLower(k)] {
			out[k] = "***MASKED***"
		} else {
			out[k] = v
		}
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}
