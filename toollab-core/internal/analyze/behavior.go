package analyze

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func buildBehaviorAnalysis(pack *evidenceDomain.EvidencePack) domain.BehaviorAnalysis {
	return domain.BehaviorAnalysis{
		InvalidInput:     analyzeInvalidInput(pack),
		MissingAuth:      analyzeMissingAuth(pack),
		NotFound:         analyzeNotFound(pack),
		ErrorConsistency: analyzeErrorConsistency(pack),
		InferredModels:   inferModels(pack),
		EndpointBehavior: analyzeEndpointBehavior(pack),
	}
}

func analyzeInvalidInput(pack *evidenceDomain.EvidencePack) domain.BehaviorObservation {
	total := 0
	returns400 := 0
	returns500 := 0
	returns2xx := 0

	for _, item := range pack.Items {
		if !hasTag(item, "malformed") && !hasTag(item, "boundary") {
			continue
		}
		if item.Response == nil {
			continue
		}
		total++
		switch {
		case item.Response.Status >= 400 && item.Response.Status < 500:
			returns400++
		case item.Response.Status >= 500:
			returns500++
		case item.Response.Status >= 200 && item.Response.Status < 300:
			returns2xx++
		}
	}

	quality := "unknown"
	if total > 0 {
		r400 := float64(returns400) / float64(total)
		r500 := float64(returns500) / float64(total)
		if r400 > 0.8 && r500 < 0.05 {
			quality = "good"
		} else if r500 > 0.3 {
			quality = "poor"
		} else {
			quality = "mixed"
		}
	}

	return domain.BehaviorObservation{
		Quality: quality,
		Summary: fmt.Sprintf("Of %d invalid input tests: %d returned 4xx (correct), %d returned 5xx (crash), %d returned 2xx (accepted bad input)", total, returns400, returns500, returns2xx),
		Tested:  total,
	}
}

func analyzeMissingAuth(pack *evidenceDomain.EvidencePack) domain.BehaviorObservation {
	total := 0
	returns401 := 0
	returns403 := 0
	returns2xx := 0

	for _, item := range pack.Items {
		if !hasTag(item, "no-auth") {
			continue
		}
		if item.Response == nil {
			continue
		}
		total++
		switch item.Response.Status {
		case 401:
			returns401++
		case 403:
			returns403++
		default:
			if item.Response.Status >= 200 && item.Response.Status < 300 {
				returns2xx++
			}
		}
	}

	quality := "unknown"
	if total > 0 {
		rejected := float64(returns401+returns403) / float64(total)
		if rejected > 0.9 {
			quality = "good"
		} else if returns2xx > 0 {
			quality = "poor"
		} else {
			quality = "mixed"
		}
	}

	return domain.BehaviorObservation{
		Quality: quality,
		Summary: fmt.Sprintf("Of %d no-auth tests: %d returned 401, %d returned 403, %d returned 2xx (bypass)", total, returns401, returns403, returns2xx),
		Tested:  total,
	}
}

func analyzeNotFound(pack *evidenceDomain.EvidencePack) domain.BehaviorObservation {
	total404 := 0
	hasBody := 0
	hasErrorField := 0

	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status != 404 {
			continue
		}
		total404++
		body := item.Response.BodyInlineTruncated
		if body != "" {
			hasBody++
			if isJSONBody(body) {
				var obj map[string]any
				if json.Unmarshal([]byte(body), &obj) == nil {
					for k := range obj {
						kl := strings.ToLower(k)
						if kl == "error" || kl == "message" || kl == "detail" {
							hasErrorField++
							break
						}
					}
				}
			}
		}
	}

	quality := "unknown"
	if total404 > 0 {
		if float64(hasErrorField)/float64(total404) > 0.8 {
			quality = "good"
		} else if hasBody == 0 {
			quality = "poor"
		} else {
			quality = "mixed"
		}
	}

	return domain.BehaviorObservation{
		Quality: quality,
		Summary: fmt.Sprintf("Of %d 404 responses: %d have body, %d have structured error field", total404, hasBody, hasErrorField),
		Tested:  total404,
	}
}

func analyzeErrorConsistency(pack *evidenceDomain.EvidencePack) domain.BehaviorObservation {
	errorFormats := map[string]int{}

	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status < 400 {
			continue
		}
		body := strings.TrimSpace(item.Response.BodyInlineTruncated)
		if body == "" {
			errorFormats["empty"]++
			continue
		}
		if !isJSONBody(body) {
			errorFormats["non-json"]++
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(body), &obj) != nil {
			errorFormats["invalid-json"]++
			continue
		}

		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, strings.ToLower(k))
		}
		sort.Strings(keys)
		format := strings.Join(keys, ",")
		errorFormats[format]++
	}

	quality := "unknown"
	totalErrors := 0
	for _, c := range errorFormats {
		totalErrors += c
	}

	if totalErrors > 0 {
		if len(errorFormats) <= 2 {
			quality = "good"
		} else if len(errorFormats) <= 4 {
			quality = "mixed"
		} else {
			quality = "poor"
		}
	}

	formatDesc := make([]string, 0, len(errorFormats))
	for format, count := range errorFormats {
		formatDesc = append(formatDesc, fmt.Sprintf("%s (%d)", format, count))
	}
	sort.Strings(formatDesc)

	return domain.BehaviorObservation{
		Quality: quality,
		Summary: fmt.Sprintf("%d unique error formats across %d error responses: %s", len(errorFormats), totalErrors, strings.Join(formatDesc, ", ")),
		Tested:  totalErrors,
	}
}

func analyzeEndpointBehavior(pack *evidenceDomain.EvidencePack) []domain.EndpointBehaviorSummary {
	type epStats struct {
		method     string
		path       string
		statuses   map[int]int
		totalMs    int64
		count      int
		errors     int
		hasAuth    bool
		noAuthOK   bool
	}

	endpoints := map[string]*epStats{}
	for _, item := range pack.Items {
		if hasTag(item, "probe") {
			continue
		}
		if item.Response == nil {
			continue
		}
		path := extractPath(item.Request.URL)
		key := item.Request.Method + " " + path
		ep, ok := endpoints[key]
		if !ok {
			ep = &epStats{method: item.Request.Method, path: path, statuses: map[int]int{}}
			endpoints[key] = ep
		}
		ep.statuses[item.Response.Status]++
		ep.totalMs += item.TimingMs
		ep.count++
		if item.Response.Status >= 500 {
			ep.errors++
		}
		if hasAnyAuthHeader(item.Request.Headers) {
			ep.hasAuth = true
		}
		if !hasAnyAuthHeader(item.Request.Headers) && item.Response.Status >= 200 && item.Response.Status < 300 {
			ep.noAuthOK = true
		}
	}

	var result []domain.EndpointBehaviorSummary
	for key, ep := range endpoints {
		avgMs := int64(0)
		if ep.count > 0 {
			avgMs = ep.totalMs / int64(ep.count)
		}
		result = append(result, domain.EndpointBehaviorSummary{
			Endpoint:      key,
			Method:        ep.method,
			Path:          ep.path,
			StatusCodes:   ep.statuses,
			RequestCount:  ep.count,
			AvgLatencyMs:  avgMs,
			ErrorCount:    ep.errors,
			RequiresAuth:  ep.hasAuth && !ep.noAuthOK,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Endpoint < result[j].Endpoint
	})
	return result
}

func inferModels(pack *evidenceDomain.EvidencePack) []domain.InferredModel {
	models := map[string]*domain.InferredModel{}

	for _, item := range pack.Items {
		if hasTag(item, "probe") {
			continue
		}
		if item.Response == nil || item.Response.Status >= 400 {
			continue
		}
		body := strings.TrimSpace(item.Response.BodyInlineTruncated)
		if body == "" || !isJSONBody(body) {
			continue
		}

		path := extractPath(item.Request.URL)
		resource := inferResourceName(path)
		if resource == "" {
			continue
		}

		var obj any
		if json.Unmarshal([]byte(body), &obj) != nil {
			continue
		}

		switch v := obj.(type) {
		case map[string]any:
			mergeFields(models, resource, path, v)
			for _, val := range v {
				if arr, ok := val.([]any); ok && len(arr) > 0 {
					if inner, ok := arr[0].(map[string]any); ok {
						itemResource := resource + "_item"
						mergeFields(models, itemResource, path, inner)
					}
				}
			}
		case []any:
			if len(v) > 0 {
				if inner, ok := v[0].(map[string]any); ok {
					mergeFields(models, resource, path, inner)
				}
			}
		}
	}

	var result []domain.InferredModel
	for _, m := range models {
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func mergeFields(models map[string]*domain.InferredModel, name, sourcePath string, obj map[string]any) {
	m, ok := models[name]
	if !ok {
		m = &domain.InferredModel{
			Name:     name,
			Fields:   []domain.InferredField{},
			SeenFrom: []string{},
		}
		models[name] = m
	}

	existingFields := map[string]bool{}
	for _, f := range m.Fields {
		existingFields[f.Name] = true
	}

	for k, v := range obj {
		if existingFields[k] {
			continue
		}
		existingFields[k] = true
		m.Fields = append(m.Fields, domain.InferredField{
			Name:     k,
			JSONType: jsonTypeName(v),
			Example:  truncateVal(v),
		})
	}

	found := false
	for _, s := range m.SeenFrom {
		if s == sourcePath {
			found = true
			break
		}
	}
	if !found {
		m.SeenFrom = append(m.SeenFrom, sourcePath)
	}
}

func inferResourceName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p == "" || strings.HasPrefix(p, "{") || strings.HasPrefix(p, ":") {
			continue
		}
		if isLikelyID(p) {
			continue
		}
		return strings.ReplaceAll(p, "-", "_")
	}
	return ""
}

func isLikelyID(s string) bool {
	if len(s) > 20 {
		return true
	}
	digits := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	return digits > len(s)/2
}

func jsonTypeName(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func truncateVal(v any) string {
	switch val := v.(type) {
	case string:
		if len(val) > 50 {
			return val[:50] + "..."
		}
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		return fmt.Sprintf("%v", val)
	case nil:
		return "null"
	default:
		b, _ := json.Marshal(v)
		s := string(b)
		if len(s) > 80 {
			return s[:80] + "..."
		}
		return s
	}
}
