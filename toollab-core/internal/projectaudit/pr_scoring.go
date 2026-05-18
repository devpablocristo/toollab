package projectaudit

import "strings"

func scorePRReview(findings []PRReviewFinding, files []PRReviewFile, specStatus string, diffText string, testOutput string) (int, string, string) {
	score := 100
	hasCritical := false
	hasHigh := false
	hasSecret := false
	criticalHighWithoutTests := 0

	for _, finding := range findings {
		switch finding.Severity {
		case prSeverityCritical:
			score -= 35
			hasCritical = true
		case prSeverityHigh:
			score -= 18
			hasHigh = true
		case prSeverityMedium:
			score -= 8
		case prSeverityLow:
			score -= 3
		}
		if finding.Code == "pr.secret_pattern" {
			hasSecret = true
		}
		if finding.Severity == prSeverityHigh && isAuthDBAPIHighFinding(finding.Code) {
			criticalHighWithoutTests++
		}
	}

	score = applyCap(score, 100)
	if hasCritical {
		score = applyCap(score, 39)
	}
	if hasSecret {
		score = applyCap(score, 35)
	}
	if hasFinding(findings, "pr.critical_zone_without_tests") {
		score = applyCap(score, 65)
	}
	if specStatus == prSpecMissing {
		score = applyCap(score, 85)
	}
	if len(files) == 0 || strings.TrimSpace(diffText) == "" {
		score = applyCap(score, 50)
	}
	if testOutputFails(testOutput) {
		score = applyCap(score, 60)
	}
	if score < 0 {
		score = 0
	}

	decision := prDecisionReviewRequired
	switch {
	case hasCritical:
		decision = prDecisionBlockMerge
	case criticalHighWithoutTests >= 2 && hasFinding(findings, "pr.critical_zone_without_tests"):
		decision = prDecisionBlockMerge
	case score >= 85 && !hasHigh && !hasCritical:
		decision = prDecisionApprove
	default:
		decision = prDecisionReviewRequired
	}

	confidence := prConfidenceLow
	diffValid := len(files) > 0 && strings.TrimSpace(diffText) != ""
	testProvided := strings.TrimSpace(testOutput) != ""
	switch {
	case specStatus == prSpecProvided && diffValid && testProvided && !testOutputFails(testOutput):
		confidence = prConfidenceHigh
	case specStatus == prSpecProvided || diffValid:
		confidence = prConfidenceMedium
	}

	return score, decision, confidence
}

func applyCap(score int, cap int) int {
	if score > cap {
		return cap
	}
	return score
}

func hasFinding(findings []PRReviewFinding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}

func isAuthDBAPIHighFinding(code string) bool {
	switch code {
	case "pr.api_contract_touched", "pr.db_migration_touched", "pr.auth_or_security_touched":
		return true
	default:
		return false
	}
}
