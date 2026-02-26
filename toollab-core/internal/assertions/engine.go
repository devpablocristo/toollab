package assertions

import (
	"fmt"

	"toollab-core/internal/evidence"
	"toollab-core/internal/scenario"
)

func Evaluate(expect scenario.Expectations, bundle *evidence.Bundle) evidence.Assertions {
	result := evidence.Assertions{
		Overall:       "PASS",
		Rules:         []evidence.RuleResult{},
		ViolatedRules: []string{},
	}

	thresholdErrorRule := evidence.RuleResult{
		ID:       "threshold_error_rate",
		Type:     "threshold_error_rate",
		Passed:   bundle.Stats.ErrorRate <= expect.MaxErrorRate,
		Observed: bundle.Stats.ErrorRate,
		Expected: expect.MaxErrorRate,
		Message:  fmt.Sprintf("error_rate %.6f <= %.6f", bundle.Stats.ErrorRate, expect.MaxErrorRate),
	}
	addRule(&result, thresholdErrorRule)

	thresholdP95Rule := evidence.RuleResult{
		ID:       "threshold_p95_ms",
		Type:     "threshold_p95_ms",
		Passed:   bundle.Stats.P95MS <= expect.MaxP95MS,
		Observed: bundle.Stats.P95MS,
		Expected: expect.MaxP95MS,
		Message:  fmt.Sprintf("p95_ms %d <= %d", bundle.Stats.P95MS, expect.MaxP95MS),
	}
	addRule(&result, thresholdP95Rule)

	for i, inv := range expect.Invariants {
		rule := evaluateInvariant(inv, bundle, i)
		addRule(&result, rule)
	}

	if len(result.ViolatedRules) > 0 {
		result.Overall = "FAIL"
	}
	return result
}

func addRule(result *evidence.Assertions, rule evidence.RuleResult) {
	result.Rules = append(result.Rules, rule)
	if !rule.Passed {
		result.ViolatedRules = append(result.ViolatedRules, rule.ID)
	}
}

func evaluateInvariant(inv scenario.InvariantConfig, bundle *evidence.Bundle, idx int) evidence.RuleResult {
	ruleID := fmt.Sprintf("invariant_%02d_%s", idx, inv.Type)
	switch inv.Type {
	case "no_5xx_allowed":
		count5xx := 0
		for _, o := range bundle.Outcomes {
			if o.StatusCode != nil && *o.StatusCode >= 500 {
				count5xx++
			}
		}
		return evidence.RuleResult{
			ID:       ruleID,
			Type:     inv.Type,
			Passed:   count5xx == 0,
			Observed: count5xx,
			Expected: 0,
			Message:  fmt.Sprintf("5xx_count=%d", count5xx),
		}
	case "max_4xx_rate":
		total := len(bundle.Outcomes)
		if total == 0 {
			return evidence.RuleResult{ID: ruleID, Type: inv.Type, Passed: true, Observed: 0.0, Expected: inv.Max, Message: "empty outcomes"}
		}
		count4xx := 0
		for _, o := range bundle.Outcomes {
			if o.StatusCode != nil && *o.StatusCode >= 400 && *o.StatusCode <= 499 {
				count4xx++
			}
		}
		rate := float64(count4xx) / float64(total)
		return evidence.RuleResult{
			ID:       ruleID,
			Type:     inv.Type,
			Passed:   rate <= inv.Max,
			Observed: rate,
			Expected: inv.Max,
			Message:  fmt.Sprintf("4xx_rate=%.6f", rate),
		}
	case "status_code_rate":
		total := len(bundle.Outcomes)
		if total == 0 {
			return evidence.RuleResult{ID: ruleID, Type: inv.Type, Passed: true, Observed: 0.0, Expected: inv.Max, Message: "empty outcomes"}
		}
		count := 0
		for _, o := range bundle.Outcomes {
			if o.StatusCode != nil && *o.StatusCode == inv.Status {
				count++
			}
		}
		rate := float64(count) / float64(total)
		return evidence.RuleResult{
			ID:       ruleID,
			Type:     inv.Type,
			Passed:   rate <= inv.Max,
			Observed: rate,
			Expected: inv.Max,
			Message:  fmt.Sprintf("status_%d_rate=%.6f", inv.Status, rate),
		}
	case "idempotent_key_identical_response":
		seen := map[string]string{}
		violations := 0
		for _, o := range bundle.Outcomes {
			if o.RequestID != inv.RequestID {
				continue
			}
			status := -1
			if o.StatusCode != nil {
				status = *o.StatusCode
			}
			signature := fmt.Sprintf("%d:%s", status, o.ResponseHash)
			if prev, ok := seen[o.RequestID]; ok {
				if prev != signature {
					violations++
				}
			} else {
				seen[o.RequestID] = signature
			}
		}
		return evidence.RuleResult{
			ID:       ruleID,
			Type:     inv.Type,
			Passed:   violations == 0,
			Observed: violations,
			Expected: 0,
			Message:  fmt.Sprintf("idempotency_signature_mismatches=%d", violations),
		}
	default:
		return evidence.RuleResult{
			ID:       ruleID,
			Type:     inv.Type,
			Passed:   false,
			Observed: "unsupported_invariant",
			Expected: "supported_invariant",
			Message:  "unsupported invariant",
		}
	}
}
