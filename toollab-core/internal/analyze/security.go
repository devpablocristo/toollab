package analyze

import (
	"fmt"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func runFullSecurityAudit(pack *evidenceDomain.EvidencePack) domain.SecurityReport {
	var findings []domain.SecurityFinding

	// Base security checks
	findings = append(findings, checkSecurityHeaders(pack)...)
	findings = append(findings, checkAuthBypass(pack)...)
	findings = append(findings, checkSensitiveData(pack)...)
	findings = append(findings, checkErrorLeak(pack)...)
	findings = append(findings, checkCORS(pack)...)
	findings = append(findings, checkRateLimiting(pack)...)

	// Injection vulnerability checks (from probe results)
	findings = append(findings, checkInjectionVulnerabilities(pack)...)

	// Robustness checks (from probe results)
	findings = append(findings, checkInputValidation(pack)...)

	// Hidden endpoint and method checks (from probe results)
	findings = append(findings, checkHiddenEndpoints(pack)...)

	summary := summarizeFindings(findings)
	score := calcSecurityScore(summary)
	return domain.SecurityReport{
		Score:    score,
		Grade:    scoreToGrade(score),
		Findings: findings,
		Summary:  summary,
	}
}

func checkSecurityHeaders(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	required := map[string]string{
		"Strict-Transport-Security": "Missing HSTS. Risk of HTTP downgrade attacks.",
		"X-Content-Type-Options":    "Missing X-Content-Type-Options. Risk of MIME sniffing.",
		"X-Frame-Options":           "Missing X-Frame-Options. Risk of clickjacking.",
		"Content-Security-Policy":   "Missing CSP. Risk of XSS and data injection.",
	}

	headerMissing := map[string][]string{}
	checked := map[string]bool{}
	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status < 200 || item.Response.Status >= 300 {
			continue
		}
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		for header := range required {
			key := header + "|" + ep
			if checked[key] {
				continue
			}
			checked[key] = true
			if !hasHeaderCI(item.Response.Headers, header) {
				headerMissing[header] = append(headerMissing[header], ep)
			}
		}
	}

	var findings []domain.SecurityFinding
	for header, endpoints := range headerMissing {
		desc := required[header]
		sample := endpoints[0]
		extra := ""
		if len(endpoints) > 1 {
			extra = fmt.Sprintf(" (and %d other endpoints)", len(endpoints)-1)
		}
		findings = append(findings, domain.SecurityFinding{
			ID:          fmt.Sprintf("SEC-HDR-%s", strings.ToUpper(strings.ReplaceAll(header, "-", ""))),
			Category:    "security_headers",
			Severity:    "medium",
			Title:       fmt.Sprintf("Missing security header: %s", header),
			Description: fmt.Sprintf("%s Affected: %s%s", desc, sample, extra),
			Endpoint:    sample,
			Remediation: fmt.Sprintf("Add '%s' header to all responses.", header),
		})
	}
	return findings
}

func checkAuthBypass(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	publicPaths := map[string]bool{
		"/healthz": true, "/readyz": true, "/health": true,
		"/openapi.yaml": true, "/openapi.json": true, "/swagger.json": true,
		"/docs": true, "/": true,
	}

	authEndpoints := map[string]bool{}
	noAuthSuccess := map[string]bool{}

	for _, item := range pack.Items {
		path := extractPath(item.Request.URL)
		if publicPaths[path] {
			continue
		}
		ep := item.Request.Method + " " + path
		hasAuth := hasAnyAuthHeader(item.Request.Headers)
		if hasAuth {
			authEndpoints[ep] = true
		}
		if !hasAuth && item.Response != nil && item.Response.Status >= 200 && item.Response.Status < 300 {
			noAuthSuccess[ep] = true
		}
	}

	var findings []domain.SecurityFinding
	for ep := range authEndpoints {
		if noAuthSuccess[ep] {
			findings = append(findings, domain.SecurityFinding{
				ID:          "SEC-AUTH-BYPASS",
				Category:    "authentication",
				Severity:    "critical",
				Title:       "Possible authentication bypass",
				Description: "Endpoint responds successfully both with and without authentication.",
				Endpoint:    ep,
				Remediation: "Verify all protected endpoints reject unauthenticated requests.",
			})
		}
	}
	return findings
}

func checkSensitiveData(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	patterns := []struct {
		pattern string
		label   string
	}{
		{"password", "passwords"},
		{"secret", "secrets"},
		{"private_key", "private keys"},
		{"credit_card", "credit cards"},
		{"ssn", "social security numbers"},
	}

	var findings []domain.SecurityFinding
	checked := map[string]bool{}
	for _, item := range pack.Items {
		if item.Response == nil {
			continue
		}
		body := strings.ToLower(item.Response.BodyInlineTruncated)
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		for _, sp := range patterns {
			key := sp.pattern + "|" + ep
			if checked[key] {
				continue
			}
			checked[key] = true
			if strings.Contains(body, fmt.Sprintf(`"%s"`, sp.pattern)) {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-DATA-EXPOSURE",
					Category:    "data_exposure",
					Severity:    "high",
					Title:       fmt.Sprintf("Possible sensitive data exposure: %s", sp.label),
					Description: fmt.Sprintf("Response contains field '%s' which may be sensitive information.", sp.pattern),
					Endpoint:    ep,
					Remediation: "Review response payloads. Redact or exclude sensitive fields.",
				})
			}
		}
	}
	return findings
}

func checkErrorLeak(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	leakPatterns := []string{
		"panic:", "goroutine ", "runtime error", "nil pointer",
		"stack trace", "stacktrace", "traceback",
		"sqlstate", "sql:", "dial tcp",
		"/home/", "c:\\",
	}

	var findings []domain.SecurityFinding
	checked := map[string]bool{}
	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status < 400 {
			continue
		}
		body := strings.ToLower(item.Response.BodyInlineTruncated)
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if checked[ep] {
			continue
		}
		for _, pattern := range leakPatterns {
			if strings.Contains(body, pattern) {
				checked[ep] = true
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-INFO-LEAK",
					Category:    "information_leak",
					Severity:    "high",
					Title:       "Error response leaks internal details",
					Description: fmt.Sprintf("Error response contains internal info pattern: '%s'.", pattern),
					Endpoint:    ep,
					Remediation: "Sanitize error responses. Use generic messages and log details internally.",
				})
				break
			}
		}
	}
	return findings
}

func checkCORS(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	checked := map[string]bool{}
	for _, item := range pack.Items {
		if item.Response == nil {
			continue
		}
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if checked[ep] {
			continue
		}
		checked[ep] = true
		for h, v := range item.Response.Headers {
			if strings.EqualFold(h, "Access-Control-Allow-Origin") && v == "*" {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-CORS-WILDCARD",
					Category:    "cors",
					Severity:    "medium",
					Title:       "CORS configured with wildcard (*)",
					Description: "Access-Control-Allow-Origin: * allows requests from any origin.",
					Endpoint:    ep,
					Remediation: "Restrict CORS to specific allowed domains.",
				})
			}
		}
	}
	return findings
}

func checkRateLimiting(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	hasRateLimit := false
	for _, item := range pack.Items {
		if item.Response == nil {
			continue
		}
		for h := range item.Response.Headers {
			hl := strings.ToLower(h)
			if strings.Contains(hl, "ratelimit") || strings.Contains(hl, "rate-limit") || hl == "retry-after" {
				hasRateLimit = true
				break
			}
		}
		if item.Response.Status == 429 {
			hasRateLimit = true
		}
		if hasRateLimit {
			break
		}
	}

	if !hasRateLimit && len(pack.Items) > 0 {
		return []domain.SecurityFinding{{
			ID:          "SEC-NO-RATELIMIT",
			Category:    "rate_limiting",
			Severity:    "medium",
			Title:       "No evidence of rate limiting",
			Description: "No rate limit headers or 429 responses detected on any endpoint.",
			Remediation: "Implement rate limiting to prevent abuse and brute-force attacks.",
		}}
	}
	return nil
}

func summarizeFindings(findings []domain.SecurityFinding) domain.SeveritySummary {
	s := domain.SeveritySummary{Total: len(findings)}
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			s.Critical++
		case "high":
			s.High++
		case "medium":
			s.Medium++
		case "low":
			s.Low++
		}
	}
	return s
}

func calcSecurityScore(s domain.SeveritySummary) int {
	score := 100
	score -= s.Critical * 25
	score -= s.High * 15
	score -= s.Medium * 5
	score -= s.Low * 2
	if score < 0 {
		score = 0
	}
	return score
}

func scoreToGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

func hasHeaderCI(headers map[string]string, name string) bool {
	for h := range headers {
		if strings.EqualFold(h, name) {
			return true
		}
	}
	return false
}

func hasAnyAuthHeader(headers map[string]string) bool {
	for h := range headers {
		hl := strings.ToLower(h)
		if hl == "authorization" || strings.Contains(hl, "api-key") || strings.Contains(hl, "api_key") ||
			strings.Contains(hl, "token") || strings.Contains(hl, "core-key") || strings.Contains(hl, "gateway-key") {
			return true
		}
	}
	return false
}

func extractPath(rawURL string) string {
	if idx := strings.Index(rawURL, "://"); idx != -1 {
		rest := rawURL[idx+3:]
		if slash := strings.Index(rest, "/"); slash != -1 {
			return rest[slash:]
		}
	}
	return rawURL
}
