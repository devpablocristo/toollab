package analyze

import (
	"fmt"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func checkInjectionVulnerabilities(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	findings = append(findings, checkSQLInjection(pack)...)
	findings = append(findings, checkXSS(pack)...)
	findings = append(findings, checkPathTraversal(pack)...)
	findings = append(findings, checkCommandInjection(pack)...)
	return findings
}

func checkSQLInjection(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	affectedEndpoints := map[string][]string{}

	for _, item := range pack.Items {
		if !hasTag(item, "sqli") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		body := strings.ToLower(item.Response.BodyInlineTruncated)

		if item.Response.Status == 500 {
			sqlPatterns := []string{"sql", "syntax error", "mysql", "postgresql", "sqlite", "oracle", "mssql", "sqlstate", "operand"}
			for _, pat := range sqlPatterns {
				if strings.Contains(body, pat) {
					affectedEndpoints[ep] = append(affectedEndpoints[ep], "500 with SQL error: "+pat)
					break
				}
			}
			if len(affectedEndpoints[ep]) == 0 {
				affectedEndpoints[ep] = append(affectedEndpoints[ep], "500 Internal Server Error on injection payload")
			}
		}

		if item.Response.Status >= 200 && item.Response.Status < 300 {
			affectedEndpoints[ep] = append(affectedEndpoints[ep], fmt.Sprintf("injection payload accepted with %d", item.Response.Status))
		}
	}

	for ep, evidence := range affectedEndpoints {
		hasSQLError := false
		for _, e := range evidence {
			if strings.Contains(e, "SQL error") {
				hasSQLError = true
				break
			}
		}

		severity := "high"
		title := "Possible SQL injection vulnerability"
		desc := "Endpoint returns 500 when receiving SQL injection payloads, suggesting unhandled database errors."
		if hasSQLError {
			severity = "critical"
			title = "SQL injection vulnerability detected"
			desc = "Endpoint leaks SQL error messages when receiving injection payloads. This strongly indicates SQL injection."
		}

		findings = append(findings, domain.SecurityFinding{
			ID:          "SEC-SQLI",
			Category:    "injection",
			Severity:    severity,
			Title:       title,
			Description: fmt.Sprintf("%s Evidence: %s", desc, strings.Join(uniqueStrings(evidence), "; ")),
			Endpoint:    ep,
			Remediation: "Use parameterized queries/prepared statements. Never concatenate user input into SQL.",
		})
	}
	return findings
}

func checkXSS(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	affected := map[string]bool{}

	for _, item := range pack.Items {
		if !hasTag(item, "xss") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if affected[ep] {
			continue
		}

		body := item.Response.BodyInlineTruncated
		if strings.Contains(body, "<script>") || strings.Contains(body, "onerror=") || strings.Contains(body, "onload=") {
			affected[ep] = true
			findings = append(findings, domain.SecurityFinding{
				ID:          "SEC-XSS-REFLECTED",
				Category:    "injection",
				Severity:    "high",
				Title:       "Reflected XSS vulnerability",
				Description: "Input containing script tags is reflected in the response without sanitization.",
				Endpoint:    ep,
				Remediation: "Sanitize/escape all user input in responses. Use Content-Type: application/json for API responses.",
			})
		}
	}
	return findings
}

func checkPathTraversal(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	affected := map[string]bool{}

	fileSignatures := []string{"root:", "daemon:", "[boot loader]", "<?php", "#!/", "# /etc/"}

	for _, item := range pack.Items {
		if !hasTag(item, "path-traversal") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if affected[ep] {
			continue
		}

		if item.Response.Status >= 200 && item.Response.Status < 300 {
			body := item.Response.BodyInlineTruncated
			for _, sig := range fileSignatures {
				if strings.Contains(body, sig) {
					affected[ep] = true
					findings = append(findings, domain.SecurityFinding{
						ID:          "SEC-PATH-TRAVERSAL",
						Category:    "injection",
						Severity:    "critical",
						Title:       "Path traversal vulnerability",
						Description: fmt.Sprintf("Endpoint returns system file contents when given traversal payload. Detected signature: '%s'.", sig),
						Endpoint:    ep,
						Remediation: "Validate and sanitize file paths. Use allowlists. Never use user input directly in file operations.",
					})
					break
				}
			}
		}
	}
	return findings
}

func checkCommandInjection(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	affected := map[string]bool{}

	cmdSignatures := []string{"uid=", "gid=", "groups=", "total ", "drwx", "-rw-"}

	for _, item := range pack.Items {
		if !hasTag(item, "cmdi") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if affected[ep] {
			continue
		}

		body := item.Response.BodyInlineTruncated
		for _, sig := range cmdSignatures {
			if strings.Contains(body, sig) {
				affected[ep] = true
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-CMDI",
					Category:    "injection",
					Severity:    "critical",
					Title:       "Command injection vulnerability",
					Description: fmt.Sprintf("Endpoint appears to execute shell commands from user input. Detected output signature: '%s'.", sig),
					Endpoint:    ep,
					Remediation: "Never pass user input to shell commands. Use safe APIs instead of shell execution.",
				})
				break
			}
		}
	}
	return findings
}

func hasTag(item evidenceDomain.EvidenceItem, tag string) bool {
	for _, t := range item.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
