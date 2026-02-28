package analyze

import (
	"fmt"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func checkInputValidation(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	findings = append(findings, checkMalformedHandling(pack)...)
	findings = append(findings, checkBoundaryHandling(pack)...)
	findings = append(findings, checkContentTypeMismatch(pack)...)
	findings = append(findings, checkLargePayloadHandling(pack)...)
	return findings
}

func checkMalformedHandling(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	crashEndpoints := map[string][]string{}

	for _, item := range pack.Items {
		if !hasTag(item, "malformed") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)

		if item.Response.Status >= 500 {
			detail := item.CaseID
			for _, t := range item.Tags {
				if strings.HasPrefix(t, "missing-field") || t == "malformed" {
					detail = t
					break
				}
			}
			crashEndpoints[ep] = append(crashEndpoints[ep], fmt.Sprintf("500 on %s input", detail))
		}
	}

	var findings []domain.SecurityFinding
	for ep, evidence := range crashEndpoints {
		findings = append(findings, domain.SecurityFinding{
			ID:          "ROB-MALFORMED-CRASH",
			Category:    "robustness",
			Severity:    "high",
			Title:       "Server crash on malformed input",
			Description: fmt.Sprintf("Endpoint returns 500 when receiving malformed data instead of 400. Evidence: %s", strings.Join(uniqueStrings(evidence), "; ")),
			Endpoint:    ep,
			Remediation: "Validate all input before processing. Return 400 Bad Request with clear error messages for invalid data.",
		})
	}
	return findings
}

func checkBoundaryHandling(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	crashEndpoints := map[string][]string{}

	for _, item := range pack.Items {
		if !hasTag(item, "boundary") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)

		if item.Response.Status >= 500 {
			crashEndpoints[ep] = append(crashEndpoints[ep], fmt.Sprintf("500 on boundary value (%s)", item.CaseID[:8]))
		}
	}

	var findings []domain.SecurityFinding
	for ep, evidence := range crashEndpoints {
		findings = append(findings, domain.SecurityFinding{
			ID:          "ROB-BOUNDARY-CRASH",
			Category:    "robustness",
			Severity:    "medium",
			Title:       "Server error on boundary values",
			Description: fmt.Sprintf("Endpoint returns 500 when receiving extreme/boundary values. Evidence: %s", strings.Join(uniqueStrings(evidence), "; ")),
			Endpoint:    ep,
			Remediation: "Handle edge cases: empty strings, null values, huge numbers, very long strings, special characters.",
		})
	}
	return findings
}

func checkContentTypeMismatch(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	acceptsWrongCT := map[string][]string{}

	for _, item := range pack.Items {
		if !hasTag(item, "content-type-mismatch") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)

		if item.Response.Status >= 200 && item.Response.Status < 300 {
			ct := ""
			for k, v := range item.Request.Headers {
				if strings.EqualFold(k, "Content-Type") {
					ct = v
					break
				}
			}
			acceptsWrongCT[ep] = append(acceptsWrongCT[ep], ct)
		}

		if item.Response.Status >= 500 {
			ct := ""
			for k, v := range item.Request.Headers {
				if strings.EqualFold(k, "Content-Type") {
					ct = v
					break
				}
			}
			acceptsWrongCT[ep+"_crash"] = append(acceptsWrongCT[ep+"_crash"], fmt.Sprintf("500 with Content-Type: %s", ct))
		}
	}

	var findings []domain.SecurityFinding
	for ep, types := range acceptsWrongCT {
		if strings.HasSuffix(ep, "_crash") {
			realEp := strings.TrimSuffix(ep, "_crash")
			findings = append(findings, domain.SecurityFinding{
				ID:          "ROB-CT-CRASH",
				Category:    "robustness",
				Severity:    "medium",
				Title:       "Server crash on wrong Content-Type",
				Description: fmt.Sprintf("Endpoint returns 500 when receiving wrong Content-Type: %s", strings.Join(types, ", ")),
				Endpoint:    realEp,
				Remediation: "Validate Content-Type header. Return 415 Unsupported Media Type for unacceptable content types.",
			})
		} else {
			findings = append(findings, domain.SecurityFinding{
				ID:          "ROB-CT-ACCEPTS-WRONG",
				Category:    "robustness",
				Severity:    "low",
				Title:       "Endpoint accepts wrong Content-Type",
				Description: fmt.Sprintf("Endpoint processes requests with unexpected Content-Type headers without rejection: %s", strings.Join(uniqueStrings(types), ", ")),
				Endpoint:    ep,
				Remediation: "Strictly validate Content-Type header. Return 415 for unsupported media types.",
			})
		}
	}
	return findings
}

func checkLargePayloadHandling(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	checked := map[string]bool{}

	for _, item := range pack.Items {
		if !hasTag(item, "large-payload") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if checked[ep] {
			continue
		}
		checked[ep] = true

		if item.Response.Status >= 200 && item.Response.Status < 300 {
			findings = append(findings, domain.SecurityFinding{
				ID:          "ROB-NO-SIZE-LIMIT",
				Category:    "robustness",
				Severity:    "medium",
				Title:       "No request size limit",
				Description: "Endpoint accepts very large payloads (1MB+) without rejection. Risk of DoS via resource exhaustion.",
				Endpoint:    ep,
				Remediation: "Implement request body size limits. Return 413 Payload Too Large for oversized requests.",
			})
		}

		if item.Response.Status >= 500 {
			findings = append(findings, domain.SecurityFinding{
				ID:          "ROB-LARGE-PAYLOAD-CRASH",
				Category:    "robustness",
				Severity:    "high",
				Title:       "Server crash on large payload",
				Description: "Endpoint returns 500 when receiving large payloads instead of rejecting with 413.",
				Endpoint:    ep,
				Remediation: "Implement request body size limits at the middleware level. Return 413 before attempting to parse.",
			})
		}
	}
	return findings
}
