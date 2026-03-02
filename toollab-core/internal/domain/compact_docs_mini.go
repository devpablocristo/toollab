package domain

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

const (
	docsSnippetMaxHappy = 1500
	docsSnippetMaxError = 500
	docsSnippetMaxReq   = 500
	maxHighlights       = 3
)

// TargetMeta holds minimal target info to enrich the dossier.
type TargetMeta struct {
	Name       string
	SourcePath string
}

// CompactForDocsMini builds a curated, minimal dossier for documentation generation.
// Deterministic: same input → same output (sorted, stable tie-breaking).
func CompactForDocsMini(full *DossierV2Full, meta TargetMeta) DocsMiniDossier {
	d := DocsMiniDossier{
		SchemaVersion: "docs-mini-v1",
		RunID:         full.RunID,
		RunMode:       full.RunMode,
	}

	d.Service = buildService(full, meta)
	d.Middlewares = buildMiddlewareIndex(&full.AST.RouterGraph)
	mwByEndpoint := mapMiddlewaresToEndpoints(&full.AST.RouterGraph)
	groupLabels := buildGroupLabels(&full.AST.RouterGraph)

	authClass := classifyAuth(full)
	d.AuthSummary = buildAuthSummary(authClass, full)

	samplesByEP := indexSamplesByEndpoint(full.Runtime.EvidenceSamples)
	statusesByEP := indexStatusesByEndpoint(full.Runtime.EvidenceSamples)

	domainMap := map[string]*DocsMiniDomain{}
	samplesCount := 0
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		mep := DocsMiniEndpoint{
			EndpointID:      ep.EndpointID,
			Method:          ep.Method,
			Path:            ep.Path,
			MiddlewareChain: mwByEndpoint[ep.EndpointID],
			Auth:            authClass[ep.EndpointID],
			StatusesSeen:    statusesByEP[ep.EndpointID],
		}
		if ep.HandlerRef != nil {
			mep.HandlerSymbol = ep.HandlerRef.Label
			mep.HandlerFile = ep.HandlerRef.Location.File
			mep.HandlerLine = ep.HandlerRef.Location.Line
			mep.HandlerPackage = ep.HandlerRef.Location.Package

			pkg := ep.HandlerRef.Location.Package
			if pkg != "" {
				if _, ok := domainMap[pkg]; !ok {
					domainMap[pkg] = &DocsMiniDomain{Package: pkg}
				}
				domainMap[pkg].EndpointCount++
				domainMap[pkg].Handlers = appendUnique(domainMap[pkg].Handlers, ep.HandlerRef.Label)
			}
		}
		if ep.GroupRef != nil {
			mep.GroupPrefix = ep.GroupRef.Extra.GroupPrefix
			mep.GroupLabel = groupLabels[ep.EndpointID]
		}

		samples := samplesByEP[ep.EndpointID]
		happy := selectHappySample(samples)
		errSample := selectErrorSample(samples)
		if happy != nil {
			s := toMiniSample(*happy, true)
			mep.HappySample = &s
			samplesCount++
		}
		if errSample != nil {
			s := toMiniSample(*errSample, false)
			mep.ErrorSample = &s
			samplesCount++
		}

		d.Endpoints = append(d.Endpoints, mep)
	}

	d.Domains = buildDomains(domainMap)
	d.DTOs = buildDTOs(full.AST.ASTEntities)
	d.Findings = buildFindingsSummary(full.FindingsRaw)
	d.Metrics = buildMetricsSummary(full)
	d.Stats = DocsMiniStats{
		EndpointsCount:  len(d.Endpoints),
		SamplesCount:    samplesCount,
		MiddlewareCount: len(d.Middlewares),
	}

	return d
}

func buildService(full *DossierV2Full, meta TargetMeta) DocsMiniService {
	svc := DocsMiniService{
		Framework:       full.AST.EndpointCatalog.Framework,
		ContentTypes:    full.TargetProfile.ObservedContentTypes,
		BaseURL:         full.TargetProfile.BaseURL,
		BasePaths:       full.TargetProfile.BasePaths,
		VersioningHint:  full.TargetProfile.VersioningHint,
		HealthEndpoints: full.TargetProfile.HealthEndpoints,
	}
	if meta.Name != "" {
		svc.Name = meta.Name
	} else if full.TargetProfile.BaseURL != "" {
		parts := strings.Split(full.TargetProfile.BaseURL, "/")
		if len(parts) >= 3 {
			svc.Name = parts[2]
		}
	}
	svc.SourcePath = meta.SourcePath
	return svc
}

func buildMiddlewareIndex(rg *RouterGraph) []DocsMiniMiddleware {
	seen := map[string]DocsMiniMiddleware{}
	var walkGroup func(g RouteGroup)
	walkGroup = func(g RouteGroup) {
		for _, mw := range g.Middlewares {
			if _, ok := seen[mw.ID]; ok {
				continue
			}
			kind := "unknown"
			name := mw.Label
			lbl := strings.ToLower(mw.Label)
			switch {
			case strings.Contains(lbl, "auth") || strings.Contains(lbl, "jwt") || strings.Contains(lbl, "apikey"):
				kind = "auth"
			case strings.Contains(lbl, "log"):
				kind = "logging"
			case strings.Contains(lbl, "rate") || strings.Contains(lbl, "limit") || strings.Contains(lbl, "throttle"):
				kind = "ratelimit"
			case strings.Contains(lbl, "cors"):
				kind = "cors"
			case strings.Contains(lbl, "recover") || strings.Contains(lbl, "panic"):
				kind = "recovery"
			}
			seen[mw.ID] = DocsMiniMiddleware{
				ID:         mw.ID,
				Name:       name,
				Kind:       kind,
				SourceFile: mw.Location.File,
				SourceLine: mw.Location.Line,
			}
		}
		for _, child := range g.Children {
			walkGroup(child)
		}
	}
	for _, g := range rg.Groups {
		walkGroup(g)
	}

	out := make([]DocsMiniMiddleware, 0, len(seen))
	for _, mw := range seen {
		out = append(out, mw)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func mapMiddlewaresToEndpoints(rg *RouterGraph) map[string][]string {
	result := map[string][]string{}
	var walk func(g RouteGroup, chain []string)
	walk = func(g RouteGroup, chain []string) {
		localChain := append([]string{}, chain...)
		for _, mw := range g.Middlewares {
			localChain = append(localChain, mw.ID)
		}
		for _, epID := range g.Endpoints {
			result[epID] = append([]string{}, localChain...)
		}
		for _, child := range g.Children {
			walk(child, localChain)
		}
	}
	for _, g := range rg.Groups {
		walk(g, nil)
	}
	return result
}

// classifyAuth determines PROVEN_REQUIRED / PROVEN_NOT_REQUIRED / UNKNOWN per endpoint.
func classifyAuth(full *DossierV2Full) map[string]AuthClassification {
	result := map[string]AuthClassification{}
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		result[ep.EndpointID] = AuthClassUnknown
	}

	if full.Runtime.AuthMatrix == nil {
		return result
	}

	denied := map[string]bool{}
	for _, e := range full.Runtime.AuthMatrix.Entries {
		if e.NoAuth == AuthDenied {
			denied[e.EndpointID] = true
		}
	}

	has2xxWithAuth := map[string]bool{}
	has2xxNoAuth := map[string]bool{}
	for _, s := range full.Runtime.EvidenceSamples {
		if s.Response == nil || s.Response.Status < 200 || s.Response.Status >= 300 {
			continue
		}
		if hasAuthHeader(s.Request.Headers) {
			has2xxWithAuth[s.EndpointID] = true
		} else {
			has2xxNoAuth[s.EndpointID] = true
		}
	}

	for epID := range result {
		if denied[epID] && has2xxWithAuth[epID] {
			result[epID] = AuthProvenRequired
		} else if has2xxNoAuth[epID] && !denied[epID] {
			result[epID] = AuthProvenNotRequired
		}
	}

	return result
}

func hasAuthHeader(headers map[string]string) bool {
	for k := range headers {
		kl := strings.ToLower(k)
		if kl == "authorization" || kl == "x-api-key" || kl == "x-nexus-core-key" || kl == "cookie" {
			return true
		}
	}
	return false
}

func buildAuthSummary(authClass map[string]AuthClassification, full *DossierV2Full) DocsMiniAuth {
	summary := DocsMiniAuth{
		Mechanisms: full.TargetProfile.AuthHints.Mechanisms,
	}
	for _, c := range authClass {
		switch c {
		case AuthProvenRequired:
			summary.ProvenRequired++
		case AuthProvenNotRequired:
			summary.ProvenNotRequired++
		default:
			summary.Unknown++
		}
	}
	for _, disc := range full.Runtime.Discrepancies.ASTvsRuntime {
		summary.Discrepancies = append(summary.Discrepancies, DocsMiniDiscrepancy{
			EndpointID:  disc.EndpointID,
			Description: disc.Description,
			ASTSays:     disc.ASTSays,
			RuntimeSays: disc.RuntimeSays,
		})
	}
	return summary
}

func indexSamplesByEndpoint(samples []EvidenceSample) map[string][]EvidenceSample {
	m := map[string][]EvidenceSample{}
	for _, s := range samples {
		if s.EndpointID != "" {
			m[s.EndpointID] = append(m[s.EndpointID], s)
		}
	}
	return m
}

func indexStatusesByEndpoint(samples []EvidenceSample) map[string][]int {
	m := map[string]map[int]bool{}
	for _, s := range samples {
		if s.EndpointID == "" || s.Response == nil {
			continue
		}
		if m[s.EndpointID] == nil {
			m[s.EndpointID] = map[int]bool{}
		}
		m[s.EndpointID][s.Response.Status] = true
	}
	result := map[string][]int{}
	for ep, statuses := range m {
		codes := make([]int, 0, len(statuses))
		for code := range statuses {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		result[ep] = codes
	}
	return result
}

// selectHappySample picks the best 2xx sample: lowest latency, then smallest body.
// Tie-break: lexicographic evidence_id for determinism.
func selectHappySample(samples []EvidenceSample) *EvidenceSample {
	var candidates []EvidenceSample
	for _, s := range samples {
		if s.Response != nil && s.Response.Status >= 200 && s.Response.Status < 300 {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Timing.LatencyMs != b.Timing.LatencyMs {
			return a.Timing.LatencyMs < b.Timing.LatencyMs
		}
		if a.Response.Size != b.Response.Size {
			return a.Response.Size < b.Response.Size
		}
		return a.EvidenceID < b.EvidenceID
	})
	return &candidates[0]
}

// selectErrorSample picks the best error sample.
// Priority: 401/403 → 400/422 → 404 → 5xx → other >=300.
// Within same priority: lowest latency, then smallest body.
func selectErrorSample(samples []EvidenceSample) *EvidenceSample {
	var candidates []EvidenceSample
	for _, s := range samples {
		if s.Response != nil && s.Response.Status >= 300 {
			candidates = append(candidates, s)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		pa, pb := errorPriority(a.Response.Status), errorPriority(b.Response.Status)
		if pa != pb {
			return pa < pb
		}
		if a.Timing.LatencyMs != b.Timing.LatencyMs {
			return a.Timing.LatencyMs < b.Timing.LatencyMs
		}
		if a.Response.Size != b.Response.Size {
			return a.Response.Size < b.Response.Size
		}
		return a.EvidenceID < b.EvidenceID
	})
	return &candidates[0]
}

func errorPriority(status int) int {
	switch {
	case status == 401 || status == 403:
		return 0
	case status == 400 || status == 422:
		return 1
	case status == 404:
		return 2
	case status >= 500:
		return 3
	default:
		return 4
	}
}

func toMiniSample(s EvidenceSample, isHappy bool) DocsMiniSample {
	respMax := docsSnippetMaxError
	if isHappy {
		respMax = docsSnippetMaxHappy
	}
	ms := DocsMiniSample{
		EvidenceID: s.EvidenceID,
		Method:     s.Request.Method,
		Path:       s.Request.Path,
		ReqHeaders: redactHeaders(s.Request.Headers),
		LatencyMs:  s.Timing.LatencyMs,
	}
	if s.Request.Body != "" {
		ms.ReqBody = snippetOf(s.Request.Body, docsSnippetMaxReq)
	}
	if s.Response != nil {
		ms.Status = s.Response.Status
		ms.RespSnippet = snippetOf(s.Response.BodySnippet, respMax)
	}
	return ms
}

func redactHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range headers {
		kl := strings.ToLower(k)
		switch {
		case kl == "authorization" || kl == "x-api-key" || kl == "x-nexus-core-key" || kl == "cookie":
			if len(v) > 12 {
				out[k] = v[:8] + "****"
			} else {
				out[k] = "****"
			}
		case kl == "content-type" || kl == "accept" || kl == "user-agent":
			out[k] = v
		}
	}
	return out
}

func snippetOf(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func buildGroupLabels(rg *RouterGraph) map[string]string {
	labels := map[string]string{}
	var walk func(g RouteGroup)
	walk = func(g RouteGroup) {
		label := ""
		if g.ASTRef != nil {
			label = g.ASTRef.Label
		}
		for _, epID := range g.Endpoints {
			if label != "" {
				labels[epID] = label
			}
		}
		for _, child := range g.Children {
			walk(child)
		}
	}
	for _, g := range rg.Groups {
		walk(g)
	}
	return labels
}

func buildDomains(domainMap map[string]*DocsMiniDomain) []DocsMiniDomain {
	domains := make([]DocsMiniDomain, 0, len(domainMap))
	for _, d := range domainMap {
		domains = append(domains, *d)
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Package < domains[j].Package })
	return domains
}

func buildDTOs(entities []ASTEntity) []DocsMiniDTO {
	var dtos []DocsMiniDTO
	for _, e := range entities {
		if e.ASTRef.Type != "dto" && e.Kind != "dto" {
			continue
		}
		dto := DocsMiniDTO{
			Name:    e.Name,
			Fields:  e.Fields,
			Package: e.ASTRef.Location.Package,
			File:    e.ASTRef.Location.File,
		}
		dtos = append(dtos, dto)
	}
	sort.Slice(dtos, func(i, j int) bool { return dtos[i].Name < dtos[j].Name })
	return dtos
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

func buildFindingsSummary(findings []FindingRaw) DocsMiniFindings {
	fs := DocsMiniFindings{
		Total:      len(findings),
		BySeverity: map[FindingSeverity]int{},
		ByCategory: map[FindingCategory]int{},
	}
	for _, f := range findings {
		fs.BySeverity[f.Severity]++
		fs.ByCategory[f.Category]++
	}

	sorted := make([]FindingRaw, len(findings))
	copy(sorted, findings)
	sort.Slice(sorted, func(i, j int) bool {
		pi, pj := severityPriority(sorted[i].Severity), severityPriority(sorted[j].Severity)
		if pi != pj {
			return pi < pj
		}
		return sorted[i].FindingID < sorted[j].FindingID
	})

	limit := maxHighlights
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for _, f := range sorted[:limit] {
		fs.Highlights = append(fs.Highlights, DocsMiniHighlight{
			Title:        f.Title,
			Severity:     f.Severity,
			Category:     f.Category,
			Description:  snippetOf(f.Description, 200),
			EvidenceRefs: f.EvidenceRefs,
		})
	}
	return fs
}

func severityPriority(s FindingSeverity) int {
	switch s {
	case SeverityCritical:
		return 0
	case SeverityHigh:
		return 1
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 3
	default:
		return 4
	}
}

func buildMetricsSummary(full *DossierV2Full) DocsMiniMetrics {
	dm := full.Runtime.DerivedMetrics
	return DocsMiniMetrics{
		TotalRequests:   dm.TotalRequests,
		SuccessRate:     dm.SuccessRate,
		P50Ms:           dm.P50Ms,
		P95Ms:           dm.P95Ms,
		EndpointsTested: dm.EndpointsTested,
		EndpointsTotal:  dm.EndpointsTotal,
		CoveragePct:     dm.CoveragePct,
	}
}

// bodyHash returns a short SHA256 prefix for linkability (unused for now but available).
func bodyHash(body string) string {
	if body == "" {
		return ""
	}
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", h[:8])
}
