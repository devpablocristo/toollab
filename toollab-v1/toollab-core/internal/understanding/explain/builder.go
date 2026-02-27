package explain

import (
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/evidence"
	mapmodel "toollab-core/internal/understanding/map"
)

func Build(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap, evidencePath string) *Understanding {
	out := &Understanding{
		SchemaVersion: 1,
		RunRef: RunRef{
			RunID:        safeRunID(bundle),
			EvidencePath: evidencePath,
		},
		Claims:   []Claim{},
		Unknowns: []string{},
		Anchors:  []Anchor{},
		Determinism: Determinism{
			CanonicalWriterVersion: "understanding-json-v1",
		},
	}

	out.Sections.WhatIs = sectionWhatIs(bundle, systemMap)
	out.Sections.HowToUse = sectionHowToUse(systemMap)
	out.Sections.WhatWasTested = sectionWhatWasTested(bundle)
	out.Sections.WhatHappened = sectionWhatHappened(bundle)
	out.Sections.WhatFailed = sectionWhatFailed(bundle)
	out.Sections.WhatIsProven = sectionWhatProven(bundle)
	out.Sections.WhatIsUnknown = sectionUnknown(bundle, systemMap, &out.Unknowns)
	out.Sections.HowToReproduce = sectionRepro(bundle)

	out.Claims = append(out.Claims, claimStats(bundle)...)
	out.Claims = append(out.Claims, claimAssertions(bundle)...)
	out.Claims = append(out.Claims, claimUnknowns(bundle, systemMap)...)
	out.Claims = normalizeClaims(out.Claims)
	out.Anchors = mergeClaimAnchors(out.Claims)
	out.Unknowns = uniqueSorted(out.Unknowns)
	return out
}

func sectionWhatIs(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap) Section {
	name := "unknown-service"
	version := "unknown"
	if systemMap != nil && systemMap.ServiceIdentity.Name != "" {
		name = systemMap.ServiceIdentity.Name
		version = systemMap.ServiceIdentity.Version
	}
	return Section{
		Summary: fmt.Sprintf("Service identified as %s (version %s).", name, version),
		Anchors: []Anchor{{Type: "json_pointer", Value: "/service_identity"}},
	}
}

func sectionHowToUse(systemMap *mapmodel.SystemMap) Section {
	if systemMap == nil || len(systemMap.Flows) == 0 {
		return Section{
			Summary: "unknown: no flow data available.",
			Anchors: []Anchor{{Type: "json_pointer", Value: "/flows"}},
		}
	}
	flowIDs := make([]string, 0, len(systemMap.Flows))
	for _, flow := range systemMap.Flows {
		flowIDs = append(flowIDs, flow.ID)
	}
	sort.Strings(flowIDs)
	return Section{
		Summary: "Suggested flows: " + strings.Join(flowIDs, ", "),
		Anchors: []Anchor{{Type: "json_pointer", Value: "/flows"}},
	}
}

func sectionWhatWasTested(bundle *evidence.Bundle) Section {
	if bundle == nil {
		return Section{Summary: "unknown: evidence missing.", Anchors: []Anchor{}}
	}
	return Section{
		Summary: fmt.Sprintf("Scenario %s executed with %d planned requests and %d completed.", bundle.ScenarioFingerprint.ScenarioSHA256, bundle.Execution.PlannedRequests, bundle.Execution.CompletedRequests),
		Anchors: []Anchor{
			{Type: "json_pointer", Value: "/scenario_fingerprint/scenario_sha256"},
			{Type: "json_pointer", Value: "/execution/planned_requests"},
			{Type: "json_pointer", Value: "/execution/completed_requests"},
		},
	}
}

func sectionWhatHappened(bundle *evidence.Bundle) Section {
	if bundle == nil {
		return Section{Summary: "unknown: evidence missing.", Anchors: []Anchor{}}
	}
	return Section{
		Summary: fmt.Sprintf("Observed error_rate=%.4f, p50=%dms, p95=%dms, p99=%dms.", bundle.Stats.ErrorRate, bundle.Stats.P50MS, bundle.Stats.P95MS, bundle.Stats.P99MS),
		Anchors: []Anchor{
			{Type: "json_pointer", Value: "/stats/error_rate"},
			{Type: "json_pointer", Value: "/stats/p50_ms"},
			{Type: "json_pointer", Value: "/stats/p95_ms"},
			{Type: "json_pointer", Value: "/stats/p99_ms"},
		},
	}
}

func sectionWhatFailed(bundle *evidence.Bundle) Section {
	if bundle == nil {
		return Section{Summary: "unknown: evidence missing.", Anchors: []Anchor{}}
	}
	if len(bundle.Assertions.ViolatedRules) == 0 {
		return Section{
			Summary: "No violated rules detected.",
			Anchors: []Anchor{{Type: "json_pointer", Value: "/assertions/violated_rules"}},
		}
	}
	return Section{
		Summary: "Violated rules: " + strings.Join(bundle.Assertions.ViolatedRules, ", "),
		Anchors: []Anchor{{Type: "json_pointer", Value: "/assertions/violated_rules"}},
	}
}

func sectionWhatProven(bundle *evidence.Bundle) Section {
	if bundle == nil {
		return Section{Summary: "unknown: evidence missing.", Anchors: []Anchor{}}
	}
	if bundle.Assertions.Overall == "PASS" {
		return Section{
			Summary: "Assertions passed for this run.",
			Anchors: []Anchor{{Type: "json_pointer", Value: "/assertions/overall"}},
		}
	}
	return Section{
		Summary: "Assertions did not fully pass.",
		Anchors: []Anchor{{Type: "json_pointer", Value: "/assertions/overall"}},
	}
}

func sectionUnknown(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap, unknowns *[]string) Section {
	items := []string{}
	if bundle != nil {
		items = append(items, bundle.Unknowns...)
	}
	if systemMap != nil {
		items = append(items, systemMap.Unknowns...)
	}
	items = uniqueSorted(items)
	*unknowns = append(*unknowns, items...)
	if len(items) == 0 {
		return Section{
			Summary: "No unknown gaps were reported.",
			Anchors: []Anchor{},
		}
	}
	return Section{
		Summary: "unknowns: " + strings.Join(items, "; "),
		Anchors: []Anchor{{Type: "json_pointer", Value: "/unknowns"}},
	}
}

func sectionRepro(bundle *evidence.Bundle) Section {
	if bundle == nil {
		return Section{Summary: "unknown: evidence missing.", Anchors: []Anchor{}}
	}
	return Section{
		Summary: fmt.Sprintf("Reproduce with: %s (expected fingerprint %s).", bundle.Repro.Command, bundle.DeterministicFingerprint),
		Anchors: []Anchor{
			{Type: "json_pointer", Value: "/repro/command"},
			{Type: "json_pointer", Value: "/deterministic_fingerprint"},
		},
	}
}

func claimStats(bundle *evidence.Bundle) []Claim {
	if bundle == nil {
		return []Claim{{Statement: "unknown: evidence missing", Status: "unknown", Anchors: nil, MissingEvidence: []string{"evidence.json"}}}
	}
	return []Claim{
		{
			Statement: fmt.Sprintf("Total requests = %d.", bundle.Stats.TotalRequests),
			Status:    "supported",
			Anchors:   []Anchor{{Type: "json_pointer", Value: "/stats/total_requests"}},
		},
		{
			Statement: fmt.Sprintf("P95 latency = %dms.", bundle.Stats.P95MS),
			Status:    "supported",
			Anchors:   []Anchor{{Type: "json_pointer", Value: "/stats/p95_ms"}},
		},
	}
}

func claimAssertions(bundle *evidence.Bundle) []Claim {
	if bundle == nil {
		return nil
	}
	status := "supported"
	missing := []string{}
	if bundle.Assertions.Overall == "" {
		status = "unknown"
		missing = []string{"assertions.overall"}
	}
	return []Claim{
		{
			Statement:       fmt.Sprintf("Assertion overall result is %s.", bundle.Assertions.Overall),
			Status:          status,
			Anchors:         []Anchor{{Type: "json_pointer", Value: "/assertions/overall"}},
			MissingEvidence: missing,
		},
	}
}

func claimUnknowns(bundle *evidence.Bundle, systemMap *mapmodel.SystemMap) []Claim {
	unknown := []string{}
	if bundle != nil {
		unknown = append(unknown, bundle.Unknowns...)
	}
	if systemMap != nil {
		unknown = append(unknown, systemMap.Unknowns...)
	}
	unknown = uniqueSorted(unknown)
	if len(unknown) == 0 {
		return nil
	}
	return []Claim{
		{
			Statement:       "Some claims cannot be supported with available evidence.",
			Status:          "unknown",
			Anchors:         []Anchor{{Type: "json_pointer", Value: "/unknowns"}},
			MissingEvidence: unknown,
		},
	}
}

func normalizeClaims(claims []Claim) []Claim {
	for i := range claims {
		if claims[i].Anchors == nil {
			claims[i].Anchors = []Anchor{}
		}
		if claims[i].MissingEvidence == nil {
			claims[i].MissingEvidence = []string{}
		}
		if claims[i].Status == "" {
			claims[i].Status = "unknown"
			claims[i].MissingEvidence = append(claims[i].MissingEvidence, "status not provided")
		}
		if claims[i].Status == "supported" && len(claims[i].Anchors) == 0 {
			claims[i].Status = "unknown"
			claims[i].MissingEvidence = append(claims[i].MissingEvidence, "missing anchors")
		}
	}
	sort.SliceStable(claims, func(i, j int) bool {
		return claims[i].Statement < claims[j].Statement
	})
	return claims
}

func mergeClaimAnchors(claims []Claim) []Anchor {
	seen := map[string]Anchor{}
	keys := []string{}
	for _, claim := range claims {
		for _, anchor := range claim.Anchors {
			key := anchor.Type + ":" + anchor.Value
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = anchor
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	out := make([]Anchor, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func uniqueSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range in {
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

func safeRunID(bundle *evidence.Bundle) string {
	if bundle == nil {
		return "unknown-run"
	}
	if bundle.Metadata.RunID == "" {
		return "unknown-run"
	}
	return bundle.Metadata.RunID
}
