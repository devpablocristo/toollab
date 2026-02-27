package diff

import (
	"fmt"
	"sort"

	"toollab-core/internal/evidence"
)

func Compare(a *evidence.Bundle, b *evidence.Bundle, aPath, bPath string) *Diff {
	out := &Diff{
		SchemaVersion: 1,
		RunA: RunRef{
			RunID:        runID(a),
			EvidencePath: aPath,
		},
		RunB: RunRef{
			RunID:        runID(b),
			EvidencePath: bPath,
		},
		ScenarioDelta: ScenarioDelta{
			Changed:      scenarioSHA(a) != scenarioSHA(b),
			ScenarioSHAA: scenarioSHA(a),
			ScenarioSHAB: scenarioSHA(b),
		},
		StatsDelta: StatsDelta{
			P50MS:     stat(a, "p50") - stat(b, "p50"),
			P95MS:     stat(a, "p95") - stat(b, "p95"),
			P99MS:     stat(a, "p99") - stat(b, "p99"),
			ErrorRate: errorRate(a) - errorRate(b),
		},
		EndpointDelta:  endpointDelta(a, b),
		InvariantDelta: invariantDelta(a, b),
		DiscoveryDelta: DiscoveryDelta{},
		Unknowns:       []string{},
		Anchors: []Anchor{
			{Type: "json_pointer", Value: "/stats"},
			{Type: "json_pointer", Value: "/assertions/violated_rules"},
		},
		Determinism: Determinism{
			CanonicalWriterVersion: "diff-json-v1",
		},
	}
	if a == nil || b == nil {
		out.Unknowns = append(out.Unknowns, "one or both evidence bundles missing")
	}
	return out
}

func endpointDelta(a *evidence.Bundle, b *evidence.Bundle) []EndpointDelta {
	type agg struct {
		count   int
		errors  int
		latency int
	}
	aggA := map[string]agg{}
	aggB := map[string]agg{}

	collect := func(bundle *evidence.Bundle, out map[string]agg) {
		if bundle == nil {
			return
		}
		for _, o := range bundle.Outcomes {
			key := fmt.Sprintf("%s %s", o.Method, o.Path)
			cur := out[key]
			cur.count++
			cur.latency += o.LatencyMS
			if o.ErrorKind != "none" {
				cur.errors++
			}
			out[key] = cur
		}
	}
	collect(a, aggA)
	collect(b, aggB)

	keys := map[string]struct{}{}
	for k := range aggA {
		keys[k] = struct{}{}
	}
	for k := range aggB {
		keys[k] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for k := range keys {
		ordered = append(ordered, k)
	}
	sort.Strings(ordered)

	out := make([]EndpointDelta, 0, len(ordered))
	for _, key := range ordered {
		aAgg := aggA[key]
		bAgg := aggB[key]
		aLatency := avgLatency(aAgg)
		bLatency := avgLatency(bAgg)
		aErr := avgError(aAgg)
		bErr := avgError(bAgg)
		out = append(out, EndpointDelta{
			Key:            key,
			LatencyDeltaMS: aLatency - bLatency,
			ErrorRateDelta: aErr - bErr,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LatencyDeltaMS != out[j].LatencyDeltaMS {
			return out[i].LatencyDeltaMS > out[j].LatencyDeltaMS
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func invariantDelta(a *evidence.Bundle, b *evidence.Bundle) InvariantDelta {
	if a == nil || b == nil {
		return InvariantDelta{
			Changed:            true,
			NewViolations:      []string{},
			ResolvedViolations: []string{},
		}
	}
	aSet := toSet(a.Assertions.ViolatedRules)
	bSet := toSet(b.Assertions.ViolatedRules)
	newViolations := []string{}
	resolved := []string{}
	for v := range aSet {
		if _, ok := bSet[v]; !ok {
			newViolations = append(newViolations, v)
		}
	}
	for v := range bSet {
		if _, ok := aSet[v]; !ok {
			resolved = append(resolved, v)
		}
	}
	sort.Strings(newViolations)
	sort.Strings(resolved)
	return InvariantDelta{
		Changed:            len(newViolations) > 0 || len(resolved) > 0 || a.Assertions.Overall != b.Assertions.Overall,
		NewViolations:      newViolations,
		ResolvedViolations: resolved,
	}
}

func toSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func avgLatency(a struct {
	count   int
	errors  int
	latency int
}) int {
	if a.count == 0 {
		return 0
	}
	return a.latency / a.count
}

func avgError(a struct {
	count   int
	errors  int
	latency int
}) float64 {
	if a.count == 0 {
		return 0
	}
	return float64(a.errors) / float64(a.count)
}

func stat(b *evidence.Bundle, key string) int {
	if b == nil {
		return 0
	}
	switch key {
	case "p50":
		return b.Stats.P50MS
	case "p95":
		return b.Stats.P95MS
	case "p99":
		return b.Stats.P99MS
	default:
		return 0
	}
}

func errorRate(b *evidence.Bundle) float64 {
	if b == nil {
		return 0
	}
	return b.Stats.ErrorRate
}

func scenarioSHA(b *evidence.Bundle) string {
	if b == nil {
		return ""
	}
	return b.ScenarioFingerprint.ScenarioSHA256
}

func runID(b *evidence.Bundle) string {
	if b == nil {
		return "unknown-run"
	}
	return b.Metadata.RunID
}
