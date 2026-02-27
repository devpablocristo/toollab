package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"toollab-core/internal/discovery"
	"toollab-core/internal/gen"
	"toollab-core/internal/scenario"
)

type ToollabOptions struct {
	TargetBaseURL        string
	ToollabURL            string
	Prefer               string
	FlowSource           string
	Mode                 string
	EffectiveSeed        string
	RequiredCapabilities []string
}

type ToollabBuildResult struct {
	Scenario           *scenario.Scenario
	ServiceDescription *discovery.ServiceDescription
	ManifestHash       string
	ProfileHash        string
	OpenAPIHash        string
	Warnings           []string
	Unknowns           []string
	DeclaredCaps       []string
	UsedCaps           []string
}

var invariantRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func BuildFromToollab(ctx context.Context, fetcher *discovery.ToollabFetcher, openapiFetcher *discovery.OpenAPIFetcher, auth *discovery.AuthConfig, opts ToollabOptions) (*ToollabBuildResult, error) {
	manifest, manifestHash, warnings, err := fetcher.Manifest(ctx, auth)
	if err != nil {
		return nil, err
	}
	declared := append([]string(nil), manifest.Capabilities...)
	sort.Strings(declared)

	if opts.ToollabURL == "" {
		opts.ToollabURL = strings.TrimRight(opts.TargetBaseURL, "/") + "/_toollab"
	}
	if opts.Prefer == "" {
		opts.Prefer = "profile"
	}
	if opts.FlowSource == "" {
		opts.FlowSource = "suggested_flows"
	}
	if opts.Mode == "" {
		opts.Mode = "smoke"
	}
	if opts.EffectiveSeed == "" {
		return nil, fmt.Errorf("effective seed required")
	}

	for _, required := range opts.RequiredCapabilities {
		if !contains(manifest.Capabilities, required) {
			return nil, fmt.Errorf("required capability not available: %s", required)
		}
	}

	var (
		profile     *discovery.Profile
		profileHash string
		usedCaps    []string
		unknowns    []string
	)
	if opts.Prefer == "profile" && contains(manifest.Capabilities, "profile") {
		p, hash, warn, pErr := fetcher.Profile(ctx, auth)
		warnings = append(warnings, warn...)
		if pErr == nil {
			profile = p
			profileHash = hash
			usedCaps = append(usedCaps, "profile")
		} else {
			warnings = append(warnings, "profile unavailable, using endpoint discovery fallback")
		}
	}

	scn := defaultScenarioSkeleton(opts)
	applyObservabilityFromCapabilities(scn, manifest.Capabilities, opts.ToollabURL, &usedCaps)

	requests := []scenario.RequestSpec{}
	defaultHeaders := map[string]string{}
	invariants := []scenario.InvariantConfig{}
	var limits map[string]any

	if profile != nil {
		flowReqs, hdrs := requestsFromProfile(profile)
		requests = append(requests, flowReqs...)
		mergeHeaders(defaultHeaders, hdrs)
		invariants = append(invariants, invariantsFromProfile(profile)...)
		limits = limitsFromProfile(profile)
	} else {
		flowReqs, hdrs, flowWarn := requestsFromEndpoints(ctx, fetcher, auth, manifest.Capabilities)
		warnings = append(warnings, flowWarn...)
		requests = append(requests, flowReqs...)
		mergeHeaders(defaultHeaders, hdrs)
		invariants = append(invariants, invariantsFromEndpoints(ctx, fetcher, auth, manifest.Capabilities, &warnings)...)
		limits = limitsFromEndpoints(ctx, fetcher, auth, manifest.Capabilities, &warnings)
	}

	if len(requests) == 0 && opts.FlowSource == "openapi_fallback" && contains(manifest.Capabilities, "openapi") {
		raw, openapiHash, warn, oErr := fetcher.OpenAPI(ctx, auth)
		warnings = append(warnings, warn...)
		if oErr != nil {
			unknowns = append(unknowns, "openapi_fallback requested but openapi fetch failed")
		} else {
			tmpPath, tErr := writeTempOpenAPIRaw(raw)
			if tErr != nil {
				unknowns = append(unknowns, "openapi_fallback temporary file error")
			} else {
				defer os.Remove(tmpPath)
				doc, _, _, owarn, parseErr := openapiFetcher.Fetch(ctx, tmpPath, nil)
				if parseErr != nil {
					unknowns = append(unknowns, "openapi_fallback parse failed")
				} else {
					genScn, gWarn, gErr := BuildFromOpenAPIDoc(doc, OpenAPIOptions{
						Mode:          opts.Mode,
						BaseURL:       opts.TargetBaseURL,
						EffectiveSeed: opts.EffectiveSeed,
					})
					warnings = append(warnings, owarn...)
					warnings = append(warnings, gWarn...)
					if gErr == nil {
						requests = append(requests, genScn.Workload.Requests...)
					}
				}
			}
			usedCaps = append(usedCaps, "openapi")
			_ = openapiHash
		}
	}

	if len(requests) == 0 {
		unknowns = append(unknowns, "suggested_flows unavailable and no openapi fallback requests generated")
	}

	// Detect auth templates in default_headers (e.g. "X-KEY": "{{ENV_VAR}}")
	// and convert to proper api_key auth config.
	authHeader, authEnv := extractAuthFromHeaders(defaultHeaders)
	if authHeader != "" && authEnv != "" {
		scn.Target.Auth = scenario.Auth{
			Type:      "api_key",
			APIKeyEnv: authEnv,
			In:        "header",
			Name:      authHeader,
		}
		delete(defaultHeaders, authHeader)
	}

	// Enrich placeholder request bodies from OpenAPI spec when available.
	if contains(manifest.Capabilities, "openapi") && hasPlaceholderBodies(requests) {
		raw, openapiHash, oWarn, oErr := fetcher.OpenAPI(ctx, auth)
		warnings = append(warnings, oWarn...)
		if oErr == nil {
			doc, parseErr := gen.ParseSpec(raw)
			if parseErr == nil {
				enrichRequestBodies(requests, doc)
				usedCaps = append(usedCaps, "openapi")
				_ = openapiHash
			} else {
				unknowns = append(unknowns, "openapi parse failed during body enrichment")
			}
		} else {
			unknowns = append(unknowns, "openapi fetch failed during body enrichment")
		}
	}

	requests = dedupeRequests(requests)
	if len(requests) > 0 {
		scn.Workload.Requests = requests
	}
	if len(defaultHeaders) > 0 {
		mergeHeaders(scn.Target.Headers, defaultHeaders)
	}
	if len(invariants) > 0 {
		scn.Expectations.Invariants = normalizeInvariants(invariants)
	}
	applyLimitsDefaults(scn, limits)

	// Fetch service description if available.
	var serviceDesc *discovery.ServiceDescription
	if contains(manifest.Capabilities, "description") {
		desc, _, dWarn, dErr := fetcher.Description(ctx, auth)
		warnings = append(warnings, dWarn...)
		if dErr != nil {
			unknowns = append(unknowns, "description fetch failed")
		} else {
			serviceDesc = desc
			usedCaps = append(usedCaps, "description")
		}
	}

	out := &ToollabBuildResult{
		Scenario:           scn,
		ServiceDescription: serviceDesc,
		ManifestHash:       manifestHash,
		ProfileHash:        profileHash,
		Warnings:           uniqueSortedStrings(warnings),
		Unknowns:           uniqueSortedStrings(unknowns),
		DeclaredCaps:       declared,
		UsedCaps:           uniqueSortedStrings(usedCaps),
	}
	return out, nil
}

func defaultScenarioSkeleton(opts ToollabOptions) *scenario.Scenario {
	chaos := scenario.Chaos{
		Latency:     scenario.LatencyConfig{Mode: "none"},
		ErrorRate:   0,
		ErrorStatus: []int{503},
		ErrorMode:   "abort",
	}
	if opts.Mode == "chaos" {
		chaos.Latency = scenario.LatencyConfig{Mode: "uniform", MinMS: 10, MaxMS: 120}
		chaos.ErrorRate = 0.02
	}
	invariants := []scenario.InvariantConfig{{Type: "no_5xx_allowed"}}
	if opts.Mode == "chaos" {
		invariants = append(invariants, scenario.InvariantConfig{Type: "max_4xx_rate", Max: 0.3})
	} else {
		invariants = append(invariants, scenario.InvariantConfig{Type: "max_4xx_rate", Max: 0.1})
	}
	return &scenario.Scenario{
		Version: 1,
		Mode:    "black",
		Target: scenario.Target{
			BaseURL: strings.TrimRight(opts.TargetBaseURL, "/"),
			Headers: map[string]string{},
			Auth:    scenario.Auth{Type: "none"},
		},
		Workload: scenario.Workload{
			Requests:     []scenario.RequestSpec{},
			Concurrency:  2,
			DurationS:    30,
			ScheduleMode: "closed_loop",
		},
		Chaos: chaos,
		Expectations: scenario.Expectations{
			MaxErrorRate: 0.1,
			MaxP95MS:     1000,
			Invariants:   invariants,
		},
		Seeds: scenario.Seeds{
			RunSeed:   opts.EffectiveSeed,
			ChaosSeed: deriveSeed(opts.EffectiveSeed, "chaos"),
		},
		Redaction: scenario.RedactionConfig{
			Headers:             []string{"authorization", "cookie", "set-cookie", "x-api-key"},
			JSONPaths:           []string{},
			Mask:                "***REDACTED***",
			MaxBodyPreviewBytes: 4096,
			MaxSamples:          50,
		},
	}
}

func applyObservabilityFromCapabilities(s *scenario.Scenario, caps []string, base string, usedCaps *[]string) {
	obs := &scenario.Observability{}
	hasAny := false
	if contains(caps, "metrics") {
		hasAny = true
		obs.Metrics = &scenario.MetricsConfig{
			Endpoint: strings.TrimRight(base, "/") + "/metrics",
			ScrapeAt: "both",
			Timeout:  2000,
		}
		*usedCaps = append(*usedCaps, "metrics")
	}
	if contains(caps, "traces") {
		hasAny = true
		obs.Traces = &scenario.TracesConfig{
			Enabled:  true,
			Endpoint: strings.TrimRight(base, "/") + "/traces",
			Timeout:  2000,
		}
		*usedCaps = append(*usedCaps, "traces")
	}
	if contains(caps, "logs") {
		hasAny = true
		obs.Logs = &scenario.LogsConfig{
			Enabled:  true,
			Source:   "http",
			Endpoint: strings.TrimRight(base, "/") + "/logs",
			Format:   "jsonl",
			MaxLines: 500,
		}
		*usedCaps = append(*usedCaps, "logs")
	}
	if hasAny {
		s.Observability = obs
	}
}

func requestsFromProfile(profile *discovery.Profile) ([]scenario.RequestSpec, map[string]string) {
	if profile == nil || len(profile.SuggestedFlows) == 0 {
		return nil, nil
	}
	return parseSuggestedFlows(profile.SuggestedFlows)
}

func requestsFromEndpoints(ctx context.Context, fetcher *discovery.ToollabFetcher, auth *discovery.AuthConfig, caps []string) ([]scenario.RequestSpec, map[string]string, []string) {
	if !contains(caps, "suggested_flows") {
		return nil, nil, []string{"suggested_flows capability unavailable"}
	}
	raw, _, warn, err := fetcher.RawJSON(ctx, "/suggested_flows", auth)
	if err != nil {
		return nil, nil, []string{"suggested_flows fetch failed"}
	}
	requests, headers := parseSuggestedFlows(raw)
	return requests, headers, warn
}

func parseSuggestedFlows(raw json.RawMessage) ([]scenario.RequestSpec, map[string]string) {
	type flowReq struct {
		Method         string            `json:"method"`
		Path           string            `json:"path"`
		Query          map[string]string `json:"query"`
		Headers        map[string]string `json:"headers"`
		Body           *string           `json:"body"`
		JSONBody       json.RawMessage   `json:"json_body"`
		TimeoutMS      int               `json:"timeout_ms"`
		Weight         int               `json:"weight"`
		IdempotencyKey string            `json:"idempotency_key"`
	}
	type flow struct {
		ID       string    `json:"id"`
		Weight   int       `json:"weight"`
		Requests []flowReq `json:"requests"`
	}
	type payload struct {
		Flows          []flow            `json:"flows"`
		DefaultHeaders map[string]string `json:"default_headers"`
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, nil
	}
	requests := []scenario.RequestSpec{}
	for _, fl := range p.Flows {
		for idx, req := range fl.Requests {
			id := sanitizeID(fmt.Sprintf("%s_%d_%s_%s", fl.ID, idx, strings.ToLower(req.Method), strings.Trim(strings.ReplaceAll(req.Path, "/", "_"), "_")))
			spec := scenario.RequestSpec{
				ID:             id,
				Method:         strings.ToUpper(req.Method),
				Path:           req.Path,
				Query:          req.Query,
				Headers:        req.Headers,
				Body:           req.Body,
				JSONBody:       req.JSONBody,
				TimeoutMS:      req.TimeoutMS,
				Weight:         req.Weight,
				IdempotencyKey: req.IdempotencyKey,
			}
			if spec.Weight == 0 {
				if fl.Weight > 0 {
					spec.Weight = fl.Weight
				} else {
					spec.Weight = 1
				}
			}
			if spec.TimeoutMS == 0 {
				spec.TimeoutMS = 5000
			}
			normalizeRequestContentType(&spec)
			requests = append(requests, spec)
		}
	}
	return dedupeRequests(requests), p.DefaultHeaders
}

func invariantsFromProfile(profile *discovery.Profile) []scenario.InvariantConfig {
	if profile == nil || len(profile.Invariants) == 0 {
		return nil
	}
	return parseInvariants(profile.Invariants)
}

func invariantsFromEndpoints(ctx context.Context, fetcher *discovery.ToollabFetcher, auth *discovery.AuthConfig, caps []string, warnings *[]string) []scenario.InvariantConfig {
	if !contains(caps, "invariants") {
		return nil
	}
	raw, _, warn, err := fetcher.RawJSON(ctx, "/invariants", auth)
	*warnings = append(*warnings, warn...)
	if err != nil {
		*warnings = append(*warnings, "invariants fetch failed")
		return nil
	}
	return parseInvariants(raw)
}

func parseInvariants(raw json.RawMessage) []scenario.InvariantConfig {
	type inv struct {
		Type      string  `json:"type"`
		Max       float64 `json:"max"`
		Status    int     `json:"status"`
		RequestID string  `json:"request_id"`
	}
	type payload struct {
		Invariants []inv `json:"invariants"`
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	out := []scenario.InvariantConfig{}
	for _, in := range p.Invariants {
		switch in.Type {
		case "no_5xx_allowed":
			out = append(out, scenario.InvariantConfig{Type: in.Type})
		case "max_4xx_rate":
			if in.Max < 0 || in.Max > 1 {
				continue
			}
			out = append(out, scenario.InvariantConfig{
				Type: in.Type,
				Max:  in.Max,
			})
		case "status_code_rate":
			if in.Status < 100 || in.Status > 599 {
				continue
			}
			if in.Max < 0 || in.Max > 1 {
				continue
			}
			out = append(out, scenario.InvariantConfig{
				Type:   in.Type,
				Max:    in.Max,
				Status: in.Status,
			})
		case "idempotent_key_identical_response":
			if strings.TrimSpace(in.RequestID) == "" {
				continue
			}
			if !invariantRequestIDPattern.MatchString(in.RequestID) {
				continue
			}
			out = append(out, scenario.InvariantConfig{
				Type:      in.Type,
				RequestID: in.RequestID,
			})
		default:
			continue
		}
	}
	return normalizeInvariants(out)
}

func normalizeInvariants(values []scenario.InvariantConfig) []scenario.InvariantConfig {
	seen := map[string]scenario.InvariantConfig{}
	keys := []string{}
	for _, inv := range values {
		key := fmt.Sprintf("%s|%d|%.6f|%s", inv.Type, inv.Status, inv.Max, inv.RequestID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = inv
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]scenario.InvariantConfig, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func limitsFromProfile(profile *discovery.Profile) map[string]any {
	if profile == nil || len(profile.Limits) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(profile.Limits, &out); err != nil {
		return nil
	}
	return out
}

func limitsFromEndpoints(ctx context.Context, fetcher *discovery.ToollabFetcher, auth *discovery.AuthConfig, caps []string, warnings *[]string) map[string]any {
	if !contains(caps, "limits") {
		return nil
	}
	raw, _, warn, err := fetcher.RawJSON(ctx, "/limits", auth)
	*warnings = append(*warnings, warn...)
	if err != nil {
		*warnings = append(*warnings, "limits fetch failed")
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		*warnings = append(*warnings, "limits parse failed")
		return nil
	}
	return out
}

func applyLimitsDefaults(scn *scenario.Scenario, limits map[string]any) {
	if limits == nil {
		return
	}
	if rate, ok := limits["rate"].(map[string]any); ok {
		if rps, ok := rate["requests_per_second"].(float64); ok && rps > 0 {
			if scn.Workload.Concurrency <= 2 {
				c := int(rps / 10)
				if c < 1 {
					c = 1
				}
				if c > 64 {
					c = 64
				}
				scn.Workload.Concurrency = c
			}
		}
	}
	if timeouts, ok := limits["timeouts"].(map[string]any); ok {
		if reqDefault, ok := timeouts["request_default_ms"].(float64); ok {
			for i := range scn.Workload.Requests {
				if scn.Workload.Requests[i].TimeoutMS == 5000 || scn.Workload.Requests[i].TimeoutMS == 0 {
					scn.Workload.Requests[i].TimeoutMS = int(reqDefault)
				}
			}
		}
	}
}

func mergeHeaders(dst map[string]string, src map[string]string) {
	if dst == nil || src == nil {
		return
	}
	keys := make([]string, 0, len(src))
	for k := range src {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, ok := dst[k]; !ok {
			dst[k] = src[k]
		}
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, " ", "_")
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "{", "")
	id = strings.ReplaceAll(id, "}", "")
	id = strings.ReplaceAll(id, "-", "_")
	for strings.Contains(id, "__") {
		id = strings.ReplaceAll(id, "__", "_")
	}
	return strings.Trim(id, "_")
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

var templatePattern = regexp.MustCompile(`^\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}$`)

func extractAuthFromHeaders(headers map[string]string) (string, string) {
	for name, value := range headers {
		m := templatePattern.FindStringSubmatch(value)
		if len(m) == 2 {
			return name, m[1]
		}
	}
	return "", ""
}

func hasPlaceholderBodies(requests []scenario.RequestSpec) bool {
	for _, req := range requests {
		if isPlaceholderBody(req.JSONBody) {
			return true
		}
	}
	return false
}

func isPlaceholderBody(body json.RawMessage) bool {
	if len(body) == 0 {
		return false
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return false
	}
	if v, ok := obj["placeholder"]; ok {
		if s, ok := v.(string); ok && strings.Contains(s, "{{") {
			return true
		}
	}
	return false
}

func enrichRequestBodies(requests []scenario.RequestSpec, doc *gen.OpenAPIDoc) {
	opMap := buildOperationMap(doc)
	for i := range requests {
		if !isPlaceholderBody(requests[i].JSONBody) {
			continue
		}
		key := strings.ToUpper(requests[i].Method) + " " + requests[i].Path
		op, ok := opMap[key]
		if !ok {
			continue
		}
		body := extractJSONBodyFromOp(op, doc)
		if body != nil {
			raw, err := json.Marshal(body)
			if err != nil {
				continue
			}
			requests[i].JSONBody = raw
		} else {
			// OpenAPI says no body needed — clear placeholder.
			requests[i].JSONBody = nil
			empty := ""
			requests[i].Body = &empty
		}
	}
}

func buildOperationMap(doc *gen.OpenAPIDoc) map[string]*gen.Operation {
	out := map[string]*gen.Operation{}
	for path, pi := range doc.Paths {
		if pi.Get != nil {
			out["GET "+path] = pi.Get
		}
		if pi.Post != nil {
			out["POST "+path] = pi.Post
		}
		if pi.Put != nil {
			out["PUT "+path] = pi.Put
		}
		if pi.Patch != nil {
			out["PATCH "+path] = pi.Patch
		}
		if pi.Delete != nil {
			out["DELETE "+path] = pi.Delete
		}
	}
	return out
}

func extractJSONBodyFromOp(op *gen.Operation, doc *gen.OpenAPIDoc) any {
	if op.RequestBody == nil || op.RequestBody.Content == nil {
		return nil
	}
	mt, ok := op.RequestBody.Content["application/json"]
	if !ok {
		for ct, m := range op.RequestBody.Content {
			if strings.Contains(ct, "json") {
				mt = m
				ok = true
				break
			}
		}
	}
	if !ok || mt.Schema == nil {
		return nil
	}
	body, err := gen.GenerateBodyAll(mt.Schema, doc)
	if err != nil || len(body) == 0 {
		return nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return body
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return body
	}
	return result
}

func writeTempOpenAPIRaw(raw []byte) (string, error) {
	file, err := os.CreateTemp("", "toollab-openapi-fallback-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	return file.Name(), nil
}
