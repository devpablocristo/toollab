package enrich

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"toolab-core/internal/scenario"
)

type Strategy string

const (
	Conservative Strategy = "conservative"
	Aggressive   Strategy = "aggressive"
)

type Change struct {
	Op         string `json:"op"`
	Path       string `json:"path"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	BeforeHash string `json:"before_hash,omitempty"`
	AfterHash  string `json:"after_hash,omitempty"`
}

type Inputs struct {
	Base          *scenario.Scenario
	FromToolab    *scenario.Scenario
	FromOpenAPI   *scenario.Scenario
	Strategy      Strategy
	EffectiveSeed string
}

type Result struct {
	Scenario *scenario.Scenario
	Changes  []Change
	Warnings []string
}

func Enrich(input Inputs) (*Result, error) {
	if input.Base == nil {
		return nil, fmt.Errorf("base scenario is required")
	}
	if input.Strategy == "" {
		input.Strategy = Conservative
	}
	if input.Strategy != Conservative && input.Strategy != Aggressive {
		return nil, fmt.Errorf("unsupported merge strategy %q", input.Strategy)
	}

	out := cloneScenario(*input.Base)
	changes := []Change{}
	warnings := []string{}

	if input.FromToolab != nil {
		mergeScenario(&out, input.FromToolab, "toolab", input.Strategy, &changes)
	}
	if input.FromOpenAPI != nil {
		// OpenAPI never overwrites existing manual/toolab values in aggressive mode.
		mergeScenario(&out, input.FromOpenAPI, "openapi", Conservative, &changes)
	}

	out.Workload.Requests = dedupeByFingerprint(out.Workload.Requests)
	out.Expectations.Invariants = normalizeInvariants(out.Expectations.Invariants)
	sortChanges(changes)

	return &Result{
		Scenario: &out,
		Changes:  changes,
		Warnings: uniqueSorted(warnings),
	}, nil
}

func mergeScenario(dst *scenario.Scenario, src *scenario.Scenario, source string, strategy Strategy, changes *[]Change) {
	if dst == nil || src == nil {
		return
	}
	mergeRequests(dst, src, source, strategy, changes)
	mergeInvariants(dst, src, source, changes)
	mergeObservability(dst, src, source, strategy, changes)
	mergeLimitsLikeDefaults(dst, src, source, strategy, changes)
}

func mergeRequests(dst *scenario.Scenario, src *scenario.Scenario, source string, strategy Strategy, changes *[]Change) {
	dstByFP := map[string]int{}
	usedIDs := map[string]struct{}{}
	for i, req := range dst.Workload.Requests {
		dstByFP[fingerprint(req)] = i
		if req.ID != "" {
			usedIDs[req.ID] = struct{}{}
		}
	}

	for _, candidate := range src.Workload.Requests {
		fp := fingerprint(candidate)
		if index, ok := dstByFP[fp]; ok {
			updated, reqChanges := enrichExistingRequest(dst.Workload.Requests[index], candidate, source, strategy, index)
			dst.Workload.Requests[index] = updated
			*changes = append(*changes, reqChanges...)
			continue
		}
		candidate.ID = ensureStableID(candidate, usedIDs)
		dst.Workload.Requests = append(dst.Workload.Requests, candidate)
		usedIDs[candidate.ID] = struct{}{}
		*changes = append(*changes, Change{
			Op:         "add",
			Path:       "/workload/requests/-",
			Reason:     "missing request in base scenario",
			Source:     source,
			AfterHash:  hashAny(candidate),
			BeforeHash: "",
		})
	}
}

func enrichExistingRequest(base scenario.RequestSpec, candidate scenario.RequestSpec, source string, strategy Strategy, idx int) (scenario.RequestSpec, []Change) {
	changes := []Change{}
	out := base
	basePath := fmt.Sprintf("/workload/requests/%d", idx)

	if isGapStringMap(out.Headers) && len(candidate.Headers) > 0 {
		out.Headers = cloneMap(candidate.Headers)
		changes = append(changes, change("replace", basePath+"/headers", "filled missing headers", source, base.Headers, out.Headers))
	}
	if isGapStringMap(out.Query) && len(candidate.Query) > 0 {
		out.Query = cloneMap(candidate.Query)
		changes = append(changes, change("replace", basePath+"/query", "filled missing query", source, base.Query, out.Query))
	}
	if len(out.JSONBody) == 0 && len(candidate.JSONBody) > 0 {
		out.JSONBody = append([]byte(nil), candidate.JSONBody...)
		out.Body = nil
		changes = append(changes, change("replace", basePath+"/json_body", "filled missing json_body", source, base.JSONBody, out.JSONBody))
	}
	if out.Body == nil && candidate.Body != nil && len(candidate.JSONBody) == 0 {
		v := *candidate.Body
		out.Body = &v
		changes = append(changes, change("replace", basePath+"/body", "filled missing body", source, base.Body, out.Body))
	}
	if strategy == Aggressive {
		if isDefaultTimeout(out.TimeoutMS) && candidate.TimeoutMS > 0 && candidate.TimeoutMS != out.TimeoutMS {
			before := out.TimeoutMS
			out.TimeoutMS = candidate.TimeoutMS
			changes = append(changes, change("replace", basePath+"/timeout_ms", "aggressive overwrite default timeout", source, before, out.TimeoutMS))
		}
		if isDefaultWeight(out.Weight) && candidate.Weight > 0 && candidate.Weight != out.Weight {
			before := out.Weight
			out.Weight = candidate.Weight
			changes = append(changes, change("replace", basePath+"/weight", "aggressive overwrite default weight", source, before, out.Weight))
		}
	}
	return out, changes
}

func mergeInvariants(dst *scenario.Scenario, src *scenario.Scenario, source string, changes *[]Change) {
	seen := map[string]struct{}{}
	for _, inv := range dst.Expectations.Invariants {
		seen[invariantKey(inv)] = struct{}{}
	}
	for _, inv := range src.Expectations.Invariants {
		key := invariantKey(inv)
		if _, ok := seen[key]; ok {
			continue
		}
		dst.Expectations.Invariants = append(dst.Expectations.Invariants, inv)
		seen[key] = struct{}{}
		*changes = append(*changes, Change{
			Op:        "add",
			Path:      "/expectations/invariants/-",
			Reason:    "added missing invariant",
			Source:    source,
			AfterHash: hashAny(inv),
		})
	}
}

func mergeObservability(dst *scenario.Scenario, src *scenario.Scenario, source string, strategy Strategy, changes *[]Change) {
	if src.Observability == nil {
		return
	}
	if dst.Observability == nil {
		dst.Observability = cloneObservability(src.Observability)
		*changes = append(*changes, Change{
			Op:        "add",
			Path:      "/observability",
			Reason:    "filled missing observability",
			Source:    source,
			AfterHash: hashAny(dst.Observability),
		})
		return
	}
	if strategy == Conservative {
		return
	}
	if dst.Observability.Metrics == nil && src.Observability.Metrics != nil {
		dst.Observability.Metrics = src.Observability.Metrics
		*changes = append(*changes, Change{
			Op:        "add",
			Path:      "/observability/metrics",
			Reason:    "aggressive filled missing metrics observability",
			Source:    source,
			AfterHash: hashAny(dst.Observability.Metrics),
		})
	}
	if dst.Observability.Traces == nil && src.Observability.Traces != nil {
		dst.Observability.Traces = src.Observability.Traces
		*changes = append(*changes, Change{
			Op:        "add",
			Path:      "/observability/traces",
			Reason:    "aggressive filled missing traces observability",
			Source:    source,
			AfterHash: hashAny(dst.Observability.Traces),
		})
	}
	if dst.Observability.Logs == nil && src.Observability.Logs != nil {
		dst.Observability.Logs = src.Observability.Logs
		*changes = append(*changes, Change{
			Op:        "add",
			Path:      "/observability/logs",
			Reason:    "aggressive filled missing logs observability",
			Source:    source,
			AfterHash: hashAny(dst.Observability.Logs),
		})
	}
}

func mergeLimitsLikeDefaults(dst *scenario.Scenario, src *scenario.Scenario, source string, strategy Strategy, changes *[]Change) {
	if strategy != Aggressive {
		return
	}
	if isDefaultConcurrency(dst.Workload.Concurrency) && src.Workload.Concurrency > 0 && src.Workload.Concurrency != dst.Workload.Concurrency {
		before := dst.Workload.Concurrency
		dst.Workload.Concurrency = src.Workload.Concurrency
		*changes = append(*changes, change("replace", "/workload/concurrency", "aggressive overwrite default concurrency", source, before, dst.Workload.Concurrency))
	}
	if isDefaultTickMS(dst.Workload.TickMS) && src.Workload.TickMS > 0 && src.Workload.TickMS != dst.Workload.TickMS {
		before := dst.Workload.TickMS
		dst.Workload.TickMS = src.Workload.TickMS
		*changes = append(*changes, change("replace", "/workload/tick_ms", "aggressive overwrite default tick_ms", source, before, dst.Workload.TickMS))
	}
}

func dedupeByFingerprint(in []scenario.RequestSpec) []scenario.RequestSpec {
	seen := map[string]scenario.RequestSpec{}
	keys := []string{}
	for _, req := range in {
		fp := fingerprint(req)
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = req
		keys = append(keys, fp)
	}
	sort.Strings(keys)
	out := make([]scenario.RequestSpec, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	sort.SliceStable(out, func(i, j int) bool {
		leftFP := fingerprint(out[i])
		rightFP := fingerprint(out[j])
		if leftFP != rightFP {
			return leftFP < rightFP
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func stableRequestBaseID(req scenario.RequestSpec) string {
	base := req.ID
	if base == "" {
		base = strings.ToLower(req.Method) + "_" + strings.Trim(strings.ReplaceAll(req.Path, "/", "_"), "_")
	}
	base = strings.ReplaceAll(base, "{", "")
	base = strings.ReplaceAll(base, "}", "")
	base = strings.ReplaceAll(base, "-", "_")
	for strings.Contains(base, "__") {
		base = strings.ReplaceAll(base, "__", "_")
	}
	base = strings.Trim(base, "_")
	if base == "" {
		base = "request"
	}
	return base
}

func ensureStableID(req scenario.RequestSpec, used map[string]struct{}) string {
	base := stableRequestBaseID(req)
	if _, exists := used[base]; !exists {
		return base
	}
	withHash := fmt.Sprintf("%s_%s", base, shortHash(fingerprint(req)))
	if _, exists := used[withHash]; !exists {
		return withHash
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", withHash, i)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func fingerprint(req scenario.RequestSpec) string {
	queryKeys := make([]string, 0, len(req.Query))
	for k := range req.Query {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	queryPairs := make([]string, 0, len(queryKeys))
	for _, key := range queryKeys {
		queryPairs = append(queryPairs, key+"="+req.Query[key])
	}
	contentType := ""
	if req.Headers != nil {
		contentType = req.Headers["Content-Type"]
	}
	bodyShape := "empty"
	if len(req.JSONBody) > 0 {
		bodyShape = shortHash(string(req.JSONBody))
	} else if req.Body != nil {
		bodyShape = shortHash(*req.Body)
	}
	return strings.Join([]string{
		strings.ToUpper(req.Method),
		req.Path,
		strings.Join(queryPairs, "&"),
		contentType,
		bodyShape,
	}, "|")
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:3])
}

func normalizeInvariants(in []scenario.InvariantConfig) []scenario.InvariantConfig {
	seen := map[string]scenario.InvariantConfig{}
	keys := []string{}
	for _, inv := range in {
		key := invariantKey(inv)
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

func invariantKey(inv scenario.InvariantConfig) string {
	return fmt.Sprintf("%s|%d|%.6f|%s", inv.Type, inv.Status, inv.Max, inv.RequestID)
}

func cloneScenario(in scenario.Scenario) scenario.Scenario {
	raw, _ := json.Marshal(in)
	var out scenario.Scenario
	_ = json.Unmarshal(raw, &out)
	return out
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneObservability(in *scenario.Observability) *scenario.Observability {
	if in == nil {
		return nil
	}
	raw, _ := json.Marshal(in)
	var out scenario.Observability
	_ = json.Unmarshal(raw, &out)
	return &out
}

func isGapStringMap(in map[string]string) bool {
	return len(in) == 0
}

func isDefaultTimeout(v int) bool {
	return v == 0 || v == 5000
}

func isDefaultWeight(v int) bool {
	return v == 0 || v == 1
}

func isDefaultConcurrency(v int) bool {
	return v <= 2
}

func isDefaultTickMS(v int) bool {
	return v == 0 || v == 100
}

func change(op, path, reason, source string, before, after any) Change {
	return Change{
		Op:         op,
		Path:       path,
		Reason:     reason,
		Source:     source,
		BeforeHash: hashAny(before),
		AfterHash:  hashAny(after),
	}
}

func hashAny(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:])
}

func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		if changes[i].Op != changes[j].Op {
			return changes[i].Op < changes[j].Op
		}
		return changes[i].Source < changes[j].Source
	})
}

func uniqueSorted(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
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
