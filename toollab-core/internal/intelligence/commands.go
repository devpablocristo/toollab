package intelligence

import (
	"fmt"
	"strings"

	d "toollab-core/internal/pipeline/usecases/domain"
)

type commandContext struct {
	baseURL       string
	ep            d.EndpointEntry
	authInfo      IntelAuthInfo
	evidence      []d.EvidenceSample
	contract      *d.InferredContract
	annotations   *d.SemanticAnnotation
}

type IntelAuthInfo struct {
	Required  string
	Mechanism string
	HeaderKey string
}

// buildReadyCommands generates curl commands following evidence > schema > AST-only hierarchy.
func buildReadyCommands(ctx commandContext) ([]d.IntelReadyCommand, []d.IntelPlaceholder) {
	var commands []d.IntelReadyCommand
	var allPlaceholders []d.IntelPlaceholder

	successEvidence := findSuccessEvidence(ctx.evidence)

	if successEvidence != nil {
		cmd, phs := buildFromEvidence(ctx, successEvidence)
		commands = append(commands, cmd)
		allPlaceholders = append(allPlaceholders, phs...)
	} else if ctx.contract != nil && ctx.contract.RequestSchema != nil {
		cmd, phs := buildFromSchema(ctx)
		commands = append(commands, cmd)
		allPlaceholders = append(allPlaceholders, phs...)
	} else {
		cmd, phs := buildFromASTOnly(ctx)
		commands = append(commands, cmd)
		allPlaceholders = append(allPlaceholders, phs...)
	}

	if ctx.authInfo.Required != "no" && ctx.authInfo.Mechanism != "none" {
		noAuthCmd := buildNoAuthCommand(ctx)
		commands = append(commands, noAuthCmd)
	}

	return commands, allPlaceholders
}

func buildFromEvidence(ctx commandContext, sample *d.EvidenceSample) (d.IntelReadyCommand, []d.IntelPlaceholder) {
	var parts []string
	parts = append(parts, "curl -s")

	if sample.Request.Method != "GET" {
		parts = append(parts, "-X "+sample.Request.Method)
	}

	var placeholders []d.IntelPlaceholder

	for k, v := range sample.Request.Headers {
		lk := strings.ToLower(k)
		if lk == "host" || lk == "user-agent" || lk == "accept-encoding" || lk == "content-length" {
			continue
		}
		if isAuthHeader(k) {
			parts = append(parts, fmt.Sprintf("-H '%s: $TOKEN'", k))
			placeholders = append(placeholders, d.IntelPlaceholder{Name: "TOKEN", Example: "<set auth token>"})
		} else {
			parts = append(parts, fmt.Sprintf("-H '%s: %s'", k, v))
		}
	}

	if sample.Request.Body != "" {
		body := sample.Request.Body
		if len(body) > 500 {
			body = body[:500]
		}
		body = strings.ReplaceAll(body, "'", "'\\''")
		parts = append(parts, fmt.Sprintf("-d '%s'", body))
	}

	url := rebuildURL(ctx.baseURL, sample.Request.Path, sample.Request.Query)
	parts = append(parts, fmt.Sprintf("'%s'", url))

	return d.IntelReadyCommand{
		Name:         "curl_happy_path",
		Kind:         "curl",
		Command:      strings.Join(parts, " \\\n  "),
		Placeholders: placeholders,
		BasedOn:      "evidence",
		EvidenceRefs: []string{sample.EvidenceID},
		Notes:        "Reconstructed from captured evidence sample",
	}, placeholders
}

func buildFromSchema(ctx commandContext) (d.IntelReadyCommand, []d.IntelPlaceholder) {
	var parts []string
	parts = append(parts, "curl -s")

	if ctx.ep.Method != "GET" {
		parts = append(parts, "-X "+ctx.ep.Method)
	}

	var placeholders []d.IntelPlaceholder

	if ctx.contract.RequestSchema.ContentType != "" {
		parts = append(parts, fmt.Sprintf("-H 'Content-Type: %s'", ctx.contract.RequestSchema.ContentType))
	}

	if ctx.authInfo.Required == "yes" || ctx.authInfo.Required == "unknown" {
		if ctx.authInfo.Mechanism == "jwt" || ctx.authInfo.Mechanism == "unknown" {
			parts = append(parts, "-H 'Authorization: Bearer $TOKEN'")
			placeholders = append(placeholders, d.IntelPlaceholder{Name: "TOKEN", Example: "<set auth token>"})
		} else if ctx.authInfo.Mechanism == "api_key" && ctx.authInfo.HeaderKey != "" {
			parts = append(parts, fmt.Sprintf("-H '%s: $API_KEY'", ctx.authInfo.HeaderKey))
			placeholders = append(placeholders, d.IntelPlaceholder{Name: "API_KEY", Example: "<set API key>"})
		}
	}

	if len(ctx.contract.RequestSchema.Fields) > 0 && isBodyMethod(ctx.ep.Method) {
		body := buildPlaceholderBody(ctx.contract.RequestSchema.Fields, ctx.annotations)
		parts = append(parts, fmt.Sprintf("-d '%s'", body))
		for _, f := range ctx.contract.RequestSchema.Fields {
			if !f.Optional {
				placeholders = append(placeholders, d.IntelPlaceholder{
					Name:    strings.ToUpper(f.Name),
					Example: fmt.Sprintf("<%s value>", f.Name),
				})
			}
		}
	}

	url := buildTemplateURL(ctx.baseURL, ctx.ep.Path)
	pathPlaceholders := extractPathPlaceholders(ctx.ep.Path)
	placeholders = append(placeholders, pathPlaceholders...)
	parts = append(parts, fmt.Sprintf("'%s'", url))

	return d.IntelReadyCommand{
		Name:         "curl_happy_path",
		Kind:         "curl",
		Command:      strings.Join(parts, " \\\n  "),
		Placeholders: placeholders,
		BasedOn:      "schema_inferred",
		Notes:        "Built from inferred schema; replace placeholders before executing",
	}, placeholders
}

func buildFromASTOnly(ctx commandContext) (d.IntelReadyCommand, []d.IntelPlaceholder) {
	var parts []string
	parts = append(parts, "curl -s")

	if ctx.ep.Method != "GET" {
		parts = append(parts, "-X "+ctx.ep.Method)
	}

	var placeholders []d.IntelPlaceholder

	if ctx.authInfo.Required == "yes" || ctx.authInfo.Required == "unknown" {
		if ctx.authInfo.HeaderKey != "" {
			parts = append(parts, fmt.Sprintf("-H '%s: $TOKEN'", ctx.authInfo.HeaderKey))
		} else {
			parts = append(parts, "-H 'Authorization: Bearer $TOKEN'")
		}
		placeholders = append(placeholders, d.IntelPlaceholder{Name: "TOKEN", Example: "<set auth token>"})
	}

	if isBodyMethod(ctx.ep.Method) {
		parts = append(parts, "-H 'Content-Type: application/json'")
		parts = append(parts, "-d '{}'")
	}

	url := buildTemplateURL(ctx.baseURL, ctx.ep.Path)
	pathPlaceholders := extractPathPlaceholders(ctx.ep.Path)
	placeholders = append(placeholders, pathPlaceholders...)
	parts = append(parts, fmt.Sprintf("'%s'", url))

	return d.IntelReadyCommand{
		Name:         "curl_happy_path",
		Kind:         "curl",
		Command:      strings.Join(parts, " \\\n  "),
		Placeholders: placeholders,
		BasedOn:      "ast_only",
		Notes:        "Minimal command from AST; no runtime evidence available",
	}, placeholders
}

func buildNoAuthCommand(ctx commandContext) d.IntelReadyCommand {
	var parts []string
	parts = append(parts, "curl -s")

	if ctx.ep.Method != "GET" {
		parts = append(parts, "-X "+ctx.ep.Method)
	}

	if isBodyMethod(ctx.ep.Method) {
		parts = append(parts, "-H 'Content-Type: application/json'")
		parts = append(parts, "-d '{}'")
	}

	url := buildTemplateURL(ctx.baseURL, ctx.ep.Path)
	parts = append(parts, fmt.Sprintf("'%s'", url))

	return d.IntelReadyCommand{
		Name:    "curl_no_auth",
		Kind:    "curl",
		Command: strings.Join(parts, " \\\n  "),
		BasedOn: "ast_only",
		Notes:   "Test without authentication to verify auth enforcement",
	}
}

// buildQueryVariants generates query variants only if backed by evidence or strong inference.
func buildQueryVariants(ctx commandContext) []d.IntelQueryVariant {
	var variants []d.IntelQueryVariant

	observedParams := collectObservedQueryParams(ctx.evidence)

	paginationParams := []string{"page", "per_page", "limit", "offset", "cursor", "page_size"}
	for _, param := range paginationParams {
		if vals, ok := observedParams[param]; ok {
			url := buildTemplateURL(ctx.baseURL, ctx.ep.Path) + "?" + param + "=" + safeFirst(vals, "1")
			variants = append(variants, d.IntelQueryVariant{
				VariantName: "pagination_" + param,
				Description: fmt.Sprintf("Pagination via %s (observed in evidence)", param),
				Command:     fmt.Sprintf("curl -s '%s'", url),
				Source:      "runtime_observed",
				Confidence:  0.9,
				Notes:       fmt.Sprintf("Observed values: %s", strings.Join(vals, ", ")),
			})
		}
	}

	sortParams := []string{"sort", "order", "order_by", "sort_by"}
	for _, param := range sortParams {
		if vals, ok := observedParams[param]; ok {
			url := buildTemplateURL(ctx.baseURL, ctx.ep.Path) + "?" + param + "=" + safeFirst(vals, "asc")
			variants = append(variants, d.IntelQueryVariant{
				VariantName: "sort_" + param,
				Description: fmt.Sprintf("Sorting via %s (observed in evidence)", param),
				Command:     fmt.Sprintf("curl -s '%s'", url),
				Source:      "runtime_observed",
				Confidence:  0.9,
				Notes:       fmt.Sprintf("Observed values: %s", strings.Join(vals, ", ")),
			})
		}
	}

	searchParams := []string{"q", "search", "query", "filter", "status"}
	for _, param := range searchParams {
		if vals, ok := observedParams[param]; ok {
			url := buildTemplateURL(ctx.baseURL, ctx.ep.Path) + "?" + param + "=$SEARCH_TERM"
			variants = append(variants, d.IntelQueryVariant{
				VariantName: "search_" + param,
				Description: fmt.Sprintf("Search/filter via %s (observed in evidence)", param),
				Command:     fmt.Sprintf("curl -s '%s'", url),
				Source:      "runtime_observed",
				Confidence:  0.9,
				Notes:       fmt.Sprintf("Observed values: %s", strings.Join(vals, ", ")),
			})
		}
	}

	if ctx.ep.Method == "GET" && len(observedParams) == 0 && ctx.annotations != nil {
		for _, fa := range ctx.annotations.Fields {
			if fa.Tag == "id_field" || fa.Tag == "status_field" {
				url := buildTemplateURL(ctx.baseURL, ctx.ep.Path) + "?" + fa.FieldPath + "=$VALUE"
				variants = append(variants, d.IntelQueryVariant{
					VariantName: "filter_by_" + fa.FieldPath,
					Description: fmt.Sprintf("Filter by %s (inferred from semantic annotation)", fa.FieldPath),
					Command:     fmt.Sprintf("curl -s '%s'", url),
					Source:      "inferred_heuristic",
					Confidence:  0.5,
					Notes:       "Inferred from semantic annotations; may not be supported by the API",
				})
			}
		}
	}

	return variants
}

func findSuccessEvidence(samples []d.EvidenceSample) *d.EvidenceSample {
	for i := range samples {
		if samples[i].Response != nil && samples[i].Response.Status >= 200 && samples[i].Response.Status < 400 {
			return &samples[i]
		}
	}
	if len(samples) > 0 {
		return &samples[0]
	}
	return nil
}

func collectObservedQueryParams(samples []d.EvidenceSample) map[string][]string {
	result := make(map[string][]string)
	for _, s := range samples {
		for k, v := range s.Request.Query {
			vals := result[k]
			found := false
			for _, existing := range vals {
				if existing == v {
					found = true
					break
				}
			}
			if !found && len(vals) < 5 {
				result[k] = append(vals, v)
			}
		}
	}
	return result
}

func rebuildURL(baseURL, path string, query map[string]string) string {
	u := strings.TrimRight(baseURL, "/") + path
	if len(query) > 0 {
		var params []string
		for k, v := range query {
			params = append(params, k+"="+v)
		}
		u += "?" + strings.Join(params, "&")
	}
	return u
}

func buildTemplateURL(baseURL, pathTemplate string) string {
	base := strings.TrimRight(baseURL, "/")
	path := pathTemplate
	path = strings.ReplaceAll(path, ":", "$")
	return base + path
}

func extractPathPlaceholders(path string) []d.IntelPlaceholder {
	var phs []d.IntelPlaceholder
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			name := strings.Trim(p, "{}")
			phs = append(phs, d.IntelPlaceholder{
				Name:    strings.ToUpper(name),
				Example: fmt.Sprintf("<%s value>", name),
			})
		} else if strings.HasPrefix(p, ":") {
			name := strings.TrimPrefix(p, ":")
			phs = append(phs, d.IntelPlaceholder{
				Name:    strings.ToUpper(name),
				Example: fmt.Sprintf("<%s value>", name),
			})
		}
	}
	return phs
}

func buildPlaceholderBody(fields []d.SchemaField, annot *d.SemanticAnnotation) string {
	if len(fields) == 0 {
		return "{}"
	}
	var parts []string
	for _, f := range fields {
		if f.Optional {
			continue
		}
		val := fmt.Sprintf("\"$%s\"", strings.ToUpper(f.Name))
		switch f.Type {
		case "int", "integer", "number", "float":
			val = "0"
		case "bool", "boolean":
			val = "false"
		case "array":
			val = "[]"
		case "object":
			val = "{}"
		}
		parts = append(parts, fmt.Sprintf("\"%s\": %s", f.Name, val))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func isAuthHeader(name string) bool {
	ln := strings.ToLower(name)
	return ln == "authorization" || strings.Contains(ln, "key") || strings.Contains(ln, "token") || strings.Contains(ln, "auth")
}

func isBodyMethod(m string) bool {
	return m == "POST" || m == "PUT" || m == "PATCH"
}

func safeFirst(vals []string, fallback string) string {
	if len(vals) > 0 {
		return vals[0]
	}
	return fallback
}
