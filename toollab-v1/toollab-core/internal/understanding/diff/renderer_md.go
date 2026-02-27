package diff

import (
	"fmt"
	"strings"
)

func RenderMD(in *Diff) []byte {
	lines := []string{
		"# TOOLLAB Diff",
		"",
		"## Scenario",
		fmt.Sprintf("- Changed: %t", in.ScenarioDelta.Changed),
		fmt.Sprintf("- A: %s", in.ScenarioDelta.ScenarioSHAA),
		fmt.Sprintf("- B: %s", in.ScenarioDelta.ScenarioSHAB),
		"",
		"## Stats Delta (A - B)",
		fmt.Sprintf("- p50: %dms", in.StatsDelta.P50MS),
		fmt.Sprintf("- p95: %dms", in.StatsDelta.P95MS),
		fmt.Sprintf("- p99: %dms", in.StatsDelta.P99MS),
		fmt.Sprintf("- error_rate: %.6f", in.StatsDelta.ErrorRate),
		"",
		"## Endpoint Delta",
	}
	for _, item := range in.EndpointDelta {
		lines = append(lines, fmt.Sprintf("- %s latency_delta=%dms error_rate_delta=%.6f", item.Key, item.LatencyDeltaMS, item.ErrorRateDelta))
	}
	lines = append(lines,
		"",
		"## Invariant Delta",
		fmt.Sprintf("- changed: %t", in.InvariantDelta.Changed),
		fmt.Sprintf("- new_violations: %s", strings.Join(in.InvariantDelta.NewViolations, ", ")),
		fmt.Sprintf("- resolved_violations: %s", strings.Join(in.InvariantDelta.ResolvedViolations, ", ")),
		"",
		"## Unknowns",
	)
	for _, item := range in.Unknowns {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "")
	return []byte(strings.Join(lines, "\n"))
}
