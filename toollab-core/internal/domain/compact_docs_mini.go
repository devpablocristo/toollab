package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	docsMiniSchemaVersion = "docs-mini-v3"
	maxHighlights         = 3
)

// TargetMeta holds minimal target info to enrich the dossier.
type TargetMeta struct {
	Name        string
	Description string
	SourcePath  string
}

// CompactForDocsMini builds a curated, minimal dossier for narrative-only documentation.
// This dossier does NOT include per-endpoint details or DTOs — those are served by
// EndpointIntelligence in the Endpoints tab.
func CompactForDocsMini(full *DossierV2Full, meta TargetMeta) DocsMiniDossier {
	d := DocsMiniDossier{
		SchemaVersion: docsMiniSchemaVersion,
		RunID:         full.RunID,
		RunMode:       full.RunMode,
	}

	d.Service = buildService(full, meta)
	d.Domains = buildDomainsFromCatalog(full)

	authClass := classifyAuth(full)
	d.AuthSummary = buildAuthSummary(authClass, full)
	d.AuthObserved = buildAuthObserved(full.Runtime.EvidenceSamples)

	d.RouteSummary = buildRouteSummary(full)
	d.ResponseShapes = buildResponseShapes(full)
	d.CommonErrors = buildCommonErrors(full.Runtime.EvidenceSamples)
	d.Findings = buildFindingsSummary(full.FindingsRaw)
	d.Metrics = buildMetricsSummary(full)

	confirmed, catalog := countEndpointsByEvidence(full)
	d.Stats = DocsMiniStats{
		EndpointsConfirmed: confirmed,
		EndpointsCatalog:   catalog,
		DomainsCount:       len(d.Domains),
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
	}
	svc.Description = meta.Description
	if svc.Name == "" && full.TargetProfile.BaseURL != "" {
		parts := strings.Split(full.TargetProfile.BaseURL, "/")
		if len(parts) >= 3 {
			svc.Name = parts[2]
		}
	}
	svc.SourcePath = meta.SourcePath
	return svc
}

// buildDomainsFromCatalog extracts domain/package groupings from the endpoint catalog.
func buildDomainsFromCatalog(full *DossierV2Full) []DocsMiniDomain {
	domainMap := map[string]*DocsMiniDomain{}
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		if ep.HandlerRef == nil {
			continue
		}
		pkg := ep.HandlerRef.Location.Package
		if pkg == "" {
			continue
		}
		if _, ok := domainMap[pkg]; !ok {
			domainMap[pkg] = &DocsMiniDomain{Package: pkg}
		}
		domainMap[pkg].EndpointCount++
		domainMap[pkg].Handlers = appendUnique(domainMap[pkg].Handlers, ep.HandlerRef.Label)
	}

	domains := make([]DocsMiniDomain, 0, len(domainMap))
	for _, d := range domainMap {
		domains = append(domains, *d)
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Package < domains[j].Package })
	return domains
}

// countEndpointsByEvidence counts how many endpoints have runtime evidence vs AST-only.
func countEndpointsByEvidence(full *DossierV2Full) (confirmed, catalog int) {
	samplesByEP := map[string]bool{}
	for _, s := range full.Runtime.EvidenceSamples {
		if s.EndpointID != "" {
			samplesByEP[s.EndpointID] = true
		}
	}
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		if samplesByEP[ep.EndpointID] {
			confirmed++
		} else {
			catalog++
		}
	}
	return
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

var authHeaderNames = map[string]bool{
	"authorization":    true,
	"x-api-key":        true,
	"x-nexus-core-key": true,
	"cookie":           true,
}

var authHeaderCanonical = map[string]string{
	"authorization":    "Authorization",
	"x-api-key":        "X-API-Key",
	"x-nexus-core-key": "X-Nexus-Core-Key",
	"cookie":           "Cookie",
}

func hasAuthHeader(headers map[string]string) bool {
	for k := range headers {
		if authHeaderNames[strings.ToLower(k)] {
			return true
		}
	}
	return false
}

const maxDiscrepancyExamples = 5

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
	allDisc := full.Runtime.Discrepancies.ASTvsRuntime
	summary.DiscrepancyCount = len(allDisc)
	limit := maxDiscrepancyExamples
	if len(allDisc) < limit {
		limit = len(allDisc)
	}
	for _, disc := range allDisc[:limit] {
		summary.DiscrepancyExamples = append(summary.DiscrepancyExamples, DocsMiniDiscrepancy{
			EndpointID:  disc.EndpointID,
			Description: disc.Description,
			ASTSays:     disc.ASTSays,
			RuntimeSays: disc.RuntimeSays,
		})
	}
	return summary
}

// buildAuthObserved extracts concrete auth evidence from runtime samples.
func buildAuthObserved(samples []EvidenceSample) DocsMiniAuthObserved {
	headerCounts := map[string]int{}
	for _, s := range samples {
		if s.Response == nil || s.Response.Status < 200 || s.Response.Status >= 300 {
			continue
		}
		for k := range s.Request.Headers {
			kl := strings.ToLower(k)
			if authHeaderNames[kl] {
				headerCounts[authHeaderCanonical[kl]]++
			}
		}
	}

	type fpKey struct {
		status int
		norm   string
	}
	type fpEntry struct {
		body       string
		count      int
		evidenceID string
	}
	fpCounts := map[fpKey]*fpEntry{}
	for _, s := range samples {
		if s.Response == nil {
			continue
		}
		st := s.Response.Status
		if st != 401 && st != 403 {
			continue
		}
		body := snippetOf(s.Response.BodySnippet, 200)
		if body == "" {
			body = fmt.Sprintf("HTTP %d", st)
		}
		code, msg := extractErrorPattern(s.Response.BodySnippet)
		norm := ""
		if code != "" || msg != "" {
			norm = strings.ToLower(fmt.Sprintf("%d|%s|%s", st, code, msg))
		} else {
			norm = fmt.Sprintf("%d|%s", st, NormalizeErrorPattern(s.Response.BodySnippet))
		}
		key := fpKey{status: st, norm: norm}
		entry, ok := fpCounts[key]
		if !ok {
			entry = &fpEntry{
				body:       body,
				evidenceID: s.EvidenceID,
			}
			fpCounts[key] = entry
		}
		entry.count++
	}

	obs := DocsMiniAuthObserved{}
	for name, count := range headerCounts {
		obs.HeadersSeen = append(obs.HeadersSeen, DocsMiniAuthHeader{Name: name, Count: count})
	}
	sort.Slice(obs.HeadersSeen, func(i, j int) bool {
		if obs.HeadersSeen[i].Count != obs.HeadersSeen[j].Count {
			return obs.HeadersSeen[i].Count > obs.HeadersSeen[j].Count
		}
		return obs.HeadersSeen[i].Name < obs.HeadersSeen[j].Name
	})

	for fp, entry := range fpCounts {
		obs.ErrorFingerprints = append(obs.ErrorFingerprints, DocsMiniAuthErrorFingerprint{
			Status:            fp.status,
			Body:              entry.body,
			Count:             entry.count,
			ExampleEvidenceID: entry.evidenceID,
		})
	}
	sort.Slice(obs.ErrorFingerprints, func(i, j int) bool {
		if obs.ErrorFingerprints[i].Count != obs.ErrorFingerprints[j].Count {
			return obs.ErrorFingerprints[i].Count > obs.ErrorFingerprints[j].Count
		}
		if obs.ErrorFingerprints[i].Status != obs.ErrorFingerprints[j].Status {
			return obs.ErrorFingerprints[i].Status < obs.ErrorFingerprints[j].Status
		}
		return obs.ErrorFingerprints[i].Body < obs.ErrorFingerprints[j].Body
	})

	return obs
}

// buildRouteSummary creates a lightweight route listing grouped by handler package.
// Format per route: "METHOD /path" — no samples, no headers, just the API surface.
func buildRouteSummary(full *DossierV2Full) []DocsMiniRouteGroup {
	groups := map[string][]string{}
	var order []string
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		pkg := "other"
		if ep.HandlerRef != nil && ep.HandlerRef.Location.Package != "" {
			pkg = ep.HandlerRef.Location.Package
		}
		route := ep.Method + " " + ep.Path
		if _, ok := groups[pkg]; !ok {
			order = append(order, pkg)
		}
		groups[pkg] = append(groups[pkg], route)
	}
	sort.Strings(order)
	out := make([]DocsMiniRouteGroup, 0, len(order))
	for _, pkg := range order {
		routes := groups[pkg]
		sort.Strings(routes)
		out = append(out, DocsMiniRouteGroup{Domain: pkg, Routes: routes})
	}
	return out
}

// buildResponseShapes extracts top-level JSON keys from the best 2xx response per endpoint.
// Picks one representative response per endpoint (largest body), extracts keys, deduplicates.
// Limited to ~20 shapes to keep the dossier small.
func buildResponseShapes(full *DossierV2Full) []DocsMiniResponseShape {
	bestByEP := map[string]EvidenceSample{}
	for _, s := range full.Runtime.EvidenceSamples {
		if s.EndpointID == "" || s.Response == nil {
			continue
		}
		if s.Response.Status < 200 || s.Response.Status >= 300 {
			continue
		}
		existing, ok := bestByEP[s.EndpointID]
		if !ok || s.Response.Size > existing.Response.Size {
			bestByEP[s.EndpointID] = s
		}
	}

	epByID := map[string]EndpointEntry{}
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		epByID[ep.EndpointID] = ep
	}

	type shapeKey struct {
		route  string
		status int
		keys   string
	}
	seen := map[shapeKey]bool{}
	var shapes []DocsMiniResponseShape

	for epID, sample := range bestByEP {
		ep, ok := epByID[epID]
		if !ok {
			continue
		}
		keys := extractTopLevelKeys(sample.Response.BodySnippet)
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		sk := shapeKey{
			route:  ep.Method + " " + ep.Path,
			status: sample.Response.Status,
			keys:   strings.Join(keys, ","),
		}
		if seen[sk] {
			continue
		}
		seen[sk] = true
		shapes = append(shapes, DocsMiniResponseShape{
			Route:  sk.route,
			Status: sample.Response.Status,
			Keys:   keys,
		})
	}

	sort.Slice(shapes, func(i, j int) bool { return shapes[i].Route < shapes[j].Route })
	if len(shapes) > 20 {
		shapes = shapes[:20]
	}
	return shapes
}

func extractTopLevelKeys(body string) []string {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	if strings.HasPrefix(body, "[") {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(body), &arr) != nil || len(arr) == 0 {
			return []string{"[]"}
		}
		var firstObj map[string]json.RawMessage
		if json.Unmarshal(arr[0], &firstObj) != nil {
			return []string{"[]"}
		}
		keys := make([]string, 0, len(firstObj))
		for k := range firstObj {
			keys = append(keys, k)
		}
		return append([]string{fmt.Sprintf("[](len=%d)", len(arr))}, keys...)
	}

	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &obj) != nil {
		return nil
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}

func snippetOf(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
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

// buildCommonErrors groups error responses by (status, error_code, message) pattern
// and returns deduplicated, sorted results. Only patterns seen >=2 times are included.
func buildCommonErrors(samples []EvidenceSample) []DocsMiniCommonError {
	type errKey struct {
		status int
		code   string
		msg    string
	}
	type errEntry struct {
		count      int
		evidenceID string
	}

	counts := map[errKey]*errEntry{}
	for _, s := range samples {
		if s.Response == nil || s.Response.Status < 300 {
			continue
		}
		code, msg := extractErrorPattern(s.Response.BodySnippet)
		if msg == "" {
			msg = snippetOf(s.Response.BodySnippet, 100)
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", s.Response.Status)
		}

		key := errKey{s.Response.Status, code, msg}
		entry, ok := counts[key]
		if !ok {
			entry = &errEntry{evidenceID: s.EvidenceID}
			counts[key] = entry
		}
		entry.count++
	}

	var errors []DocsMiniCommonError
	for key, entry := range counts {
		if entry.count < 2 {
			continue
		}
		errors = append(errors, DocsMiniCommonError{
			Status:            key.status,
			ErrorCode:         key.code,
			Message:           key.msg,
			Count:             entry.count,
			ExampleEvidenceID: entry.evidenceID,
		})
	}

	sort.Slice(errors, func(i, j int) bool {
		if errors[i].Count != errors[j].Count {
			return errors[i].Count > errors[j].Count
		}
		if errors[i].Status != errors[j].Status {
			return errors[i].Status < errors[j].Status
		}
		if errors[i].ErrorCode != errors[j].ErrorCode {
			return errors[i].ErrorCode < errors[j].ErrorCode
		}
		return errors[i].Message < errors[j].Message
	})

	if len(errors) > 10 {
		errors = errors[:10]
	}
	return errors
}

// extractErrorPattern attempts to extract a structured error code+message from a JSON response body.
func extractErrorPattern(body string) (code, msg string) {
	var parsed map[string]interface{}
	if json.Unmarshal([]byte(body), &parsed) != nil {
		return "", ""
	}

	if errObj, ok := parsed["error"]; ok {
		switch v := errObj.(type) {
		case map[string]interface{}:
			code, _ = v["code"].(string)
			msg, _ = v["message"].(string)
			return code, msg
		case string:
			return "", v
		}
	}

	code, _ = parsed["code"].(string)
	msg, _ = parsed["message"].(string)
	return code, msg
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
	coveragePct := dm.CoveragePct
	if coveragePct > 100.0 {
		coveragePct = 100.0
	}
	return DocsMiniMetrics{
		TotalRequests:   dm.TotalRequests,
		SuccessRate:     dm.SuccessRate,
		P50Ms:           dm.P50Ms,
		P95Ms:           dm.P95Ms,
		EndpointsTested: dm.EndpointsTested,
		EndpointsTotal:  dm.EndpointsTotal,
		CoveragePct:     coveragePct,
	}
}

// bodyHash returns a short SHA256 prefix for linkability.
func bodyHash(body string) string {
	if body == "" {
		return ""
	}
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", h[:8])
}
