// Package domain defines pipeline dossier types and compaction helpers.
package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	docsMiniSchemaVersion = "docs-mini-v5"
	maxContractFields     = 10
	maxResponseKeys       = 8
	maxPrimaryStatuses    = 3
)

// TargetMeta holds minimal target info to enrich the dossier.
type TargetMeta struct {
	Name        string
	Description string
}

// CompactForDocsMini builds a documentation-oriented dossier from the full pipeline output.
// Focus: minimal endpoint contracts + selected runtime evidence (auth, errors).
func CompactForDocsMini(full *DossierV2Full, meta TargetMeta) DocsMiniDossier {
	d := DocsMiniDossier{
		SchemaVersion: docsMiniSchemaVersion,
		RunID:         full.RunID,
		RunMode:       full.RunMode,
	}

	d.Service = buildService(full, meta)
	d.Endpoints = buildEndpoints(full)
	d.Auth = buildAuth(full)
	d.CommonErrors = buildCommonErrors(full.Runtime.EvidenceSamples)

	confirmed := countConfirmed(full)
	domains := countDomains(full)
	d.Gaps = buildGaps(d.Endpoints, d.Auth, len(full.AST.EndpointCatalog.Endpoints), confirmed)
	d.Stats = DocsMiniStats{
		EndpointsTotal:     len(full.AST.EndpointCatalog.Endpoints),
		EndpointsConfirmed: confirmed,
		DomainsCount:       domains,
	}

	return d
}

func buildService(full *DossierV2Full, meta TargetMeta) DocsMiniService {
	svc := DocsMiniService{
		Framework: full.AST.EndpointCatalog.Framework,
		BaseURL:   full.TargetProfile.BaseURL,
	}
	if meta.Name != "" {
		svc.Name = meta.Name
	} else if full.TargetProfile.BaseURL != "" {
		parts := strings.Split(full.TargetProfile.BaseURL, "/")
		if len(parts) >= 3 {
			svc.Name = parts[2]
		}
	}
	svc.Description = meta.Description
	return svc
}

// buildEndpoints creates one entry per endpoint with domain, request fields and response keys.
func buildEndpoints(full *DossierV2Full) []DocsMiniEndpoint {
	statusByEP := buildPrimaryStatuses(full.Runtime.EvidenceSamples)

	// Build response shape index from runtime evidence
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

	// Build inferred contract index by endpoint
	contractByEP := map[string]InferredContract{}
	for _, c := range full.Runtime.InferredContracts {
		if c.EndpointID == "" {
			continue
		}
		existing, ok := contractByEP[c.EndpointID]
		if !ok || c.Confidence > existing.Confidence {
			contractByEP[c.EndpointID] = c
		}
	}

	endpoints := make([]DocsMiniEndpoint, 0, len(full.AST.EndpointCatalog.Endpoints))
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		e := DocsMiniEndpoint{
			Method: ep.Method,
			Path:   ep.Path,
		}
		if ep.HandlerRef != nil {
			e.Domain = simplifyDomain(ep.HandlerRef.Location.Package)
			e.Handler = ep.HandlerRef.Label
		}
		e.OperationHint = inferOperationHint(e.Method, e.Path)

		if c, ok := contractByEP[ep.EndpointID]; ok {
			if c.RequestSchema != nil {
				for _, f := range c.RequestSchema.Fields {
					if f.Name != "" {
						e.RequestFields = append(e.RequestFields, f.Name)
					}
				}
			}
			if len(e.RequestFields) > 1 {
				sort.Strings(e.RequestFields)
			}
			if len(e.RequestFields) > maxContractFields {
				e.RequestFields = e.RequestFields[:maxContractFields]
			}
		}
		if sample, ok := bestByEP[ep.EndpointID]; ok {
			keys := extractTopLevelKeys(sample.Response.BodySnippet)
			if len(keys) > 0 {
				sort.Strings(keys)
				if len(keys) > maxResponseKeys {
					keys = keys[:maxResponseKeys]
				}
				e.ResponseKeys = keys
			}
		}
		if statuses, ok := statusByEP[ep.EndpointID]; ok {
			e.PrimaryStatus = statuses
		}

		endpoints = append(endpoints, e)
	}

	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Domain != endpoints[j].Domain {
			return endpoints[i].Domain < endpoints[j].Domain
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	return endpoints
}

func buildPrimaryStatuses(samples []EvidenceSample) map[string][]int {
	countsByEP := map[string]map[int]int{}
	for _, s := range samples {
		if s.EndpointID == "" || s.Response == nil {
			continue
		}
		if countsByEP[s.EndpointID] == nil {
			countsByEP[s.EndpointID] = map[int]int{}
		}
		countsByEP[s.EndpointID][s.Response.Status]++
	}

	out := map[string][]int{}
	for epID, statusCounts := range countsByEP {
		type pair struct {
			status int
			count  int
		}
		top := make([]pair, 0, len(statusCounts))
		for st, c := range statusCounts {
			top = append(top, pair{status: st, count: c})
		}
		sort.Slice(top, func(i, j int) bool {
			if top[i].count != top[j].count {
				return top[i].count > top[j].count
			}
			return top[i].status < top[j].status
		})
		limit := maxPrimaryStatuses
		if len(top) < limit {
			limit = len(top)
		}
		statuses := make([]int, 0, limit)
		for _, p := range top[:limit] {
			statuses = append(statuses, p.status)
		}
		out[epID] = statuses
	}
	return out
}

func inferOperationHint(method, path string) string {
	m := strings.ToUpper(method)
	p := strings.ToLower(path)
	last := p
	if i := strings.LastIndex(last, "/"); i >= 0 && i+1 < len(last) {
		last = last[i+1:]
	}
	if strings.HasPrefix(last, ":") {
		last = ""
	}
	switch {
	case m == "GET" && strings.Contains(p, ":"):
		return "get_by_id"
	case m == "GET":
		return "list"
	case m == "POST" && (strings.Contains(p, "/close") || strings.Contains(p, "/approve") || strings.Contains(p, "/reject") || strings.Contains(p, "/rollback") || strings.Contains(p, "/apply") || strings.Contains(p, "/create") || strings.Contains(p, "/replay") || strings.Contains(p, "/simulate")):
		return "action"
	case m == "POST":
		return "create"
	case m == "PUT" || m == "PATCH":
		return "update"
	case m == "DELETE":
		return "delete"
	default:
		return "other"
	}
}

// simplifyDomain converts a full package path to a short domain name.
// "nexus-core/internal/actions/handler/dto" → "actions"
// "nexus-core/internal/ops/actionengine" → "ops/actionengine"
func simplifyDomain(pkg string) string {
	if pkg == "" {
		return "other"
	}
	// Strip common prefixes
	for _, prefix := range []string{"/internal/", "/cmd/"} {
		if idx := strings.Index(pkg, prefix); idx >= 0 {
			pkg = pkg[idx+len(prefix):]
			break
		}
	}
	// Strip /handler/dto, /handler, /usecases/domain suffixes
	for _, suffix := range []string{"/handler/dto", "/handler", "/usecases/domain", "/usecases"} {
		pkg = strings.TrimSuffix(pkg, suffix)
	}
	return pkg
}

func buildAuth(full *DossierV2Full) DocsMiniAuth {
	auth := DocsMiniAuth{}

	// Classify endpoints
	authClass := classifyAuth(full)
	for _, c := range authClass {
		switch c {
		case AuthProvenRequired:
			auth.ProvenRequired++
		case AuthProvenNotRequired:
			auth.ProvenNotRequired++
		default:
			auth.Unknown++
		}
	}

	// Observed headers from successful requests
	headerCounts := map[string]int{}
	for _, s := range full.Runtime.EvidenceSamples {
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
	for name, count := range headerCounts {
		auth.HeadersSeen = append(auth.HeadersSeen, DocsMiniAuthHeader{Name: name, Count: count})
	}
	sort.Slice(auth.HeadersSeen, func(i, j int) bool {
		return auth.HeadersSeen[i].Count > auth.HeadersSeen[j].Count
	})

	// Error fingerprints (401/403)
	type fpKey struct {
		status int
		norm   string
	}
	type fpEntry struct {
		body  string
		count int
	}
	fpCounts := map[fpKey]*fpEntry{}
	for _, s := range full.Runtime.EvidenceSamples {
		if s.Response == nil {
			continue
		}
		if s.Response.Status != 401 && s.Response.Status != 403 {
			continue
		}
		body := s.Response.BodySnippet
		if len(body) > 200 {
			body = body[:200]
		}
		code, msg := extractErrorPattern(body)
		norm := fmt.Sprintf("%d|%s|%s", s.Response.Status, code, msg)
		key := fpKey{s.Response.Status, norm}
		entry, ok := fpCounts[key]
		if !ok {
			entry = &fpEntry{body: body}
			fpCounts[key] = entry
		}
		entry.count++
	}
	for fp, entry := range fpCounts {
		auth.ErrorFingerprints = append(auth.ErrorFingerprints, DocsMiniAuthErrorFingerprint{
			Status: fp.status,
			Body:   entry.body,
			Count:  entry.count,
		})
	}
	sort.Slice(auth.ErrorFingerprints, func(i, j int) bool {
		return auth.ErrorFingerprints[i].Count > auth.ErrorFingerprints[j].Count
	})

	// Discrepancies
	allDisc := full.Runtime.Discrepancies.ASTvsRuntime
	auth.DiscrepancyCount = len(allDisc)
	limit := 5
	if len(allDisc) < limit {
		limit = len(allDisc)
	}
	for _, disc := range allDisc[:limit] {
		auth.Discrepancies = append(auth.Discrepancies, DocsMiniDiscrepancy{
			Endpoint:    disc.Method + " " + disc.Path,
			Description: disc.Description,
		})
	}

	return auth
}

func countConfirmed(full *DossierV2Full) int {
	samplesByEP := map[string]bool{}
	for _, s := range full.Runtime.EvidenceSamples {
		if s.EndpointID != "" {
			samplesByEP[s.EndpointID] = true
		}
	}
	count := 0
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		if samplesByEP[ep.EndpointID] {
			count++
		}
	}
	return count
}

func countDomains(full *DossierV2Full) int {
	domains := map[string]bool{}
	for _, ep := range full.AST.EndpointCatalog.Endpoints {
		if ep.HandlerRef != nil {
			domains[simplifyDomain(ep.HandlerRef.Location.Package)] = true
		}
	}
	return len(domains)
}

func buildGaps(endpoints []DocsMiniEndpoint, auth DocsMiniAuth, total, confirmed int) DocsMiniGaps {
	noShape := 0
	for _, ep := range endpoints {
		if len(ep.ResponseKeys) == 0 {
			noShape++
		}
	}
	return DocsMiniGaps{
		UnconfirmedEndpoints: total - confirmed,
		EndpointsNoShape:     noShape,
		EndpointsAuthUnknown: auth.Unknown,
	}
}

// --- Shared helpers (unchanged) ---

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

// buildCommonErrors groups error responses by pattern.
func buildCommonErrors(samples []EvidenceSample) []DocsMiniCommonError {
	type errKey struct {
		status int
		code   string
		msg    string
	}
	counts := map[errKey]int{}
	for _, s := range samples {
		if s.Response == nil || s.Response.Status < 300 {
			continue
		}
		code, msg := extractErrorPattern(s.Response.BodySnippet)
		if msg == "" {
			if len(s.Response.BodySnippet) > 100 {
				msg = s.Response.BodySnippet[:100]
			} else if s.Response.BodySnippet != "" {
				msg = s.Response.BodySnippet
			} else {
				msg = fmt.Sprintf("HTTP %d", s.Response.Status)
			}
		}
		counts[errKey{s.Response.Status, code, msg}]++
	}

	var errors []DocsMiniCommonError
	for key, count := range counts {
		if count < 2 {
			continue
		}
		errors = append(errors, DocsMiniCommonError{
			Status:    key.status,
			ErrorCode: key.code,
			Message:   key.msg,
			Count:     count,
		})
	}
	sort.Slice(errors, func(i, j int) bool { return errors[i].Count > errors[j].Count })
	if len(errors) > 10 {
		errors = errors[:10]
	}
	return errors
}

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
