package analyze

import (
	"encoding/json"
	"fmt"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func runContractValidation(pack *evidenceDomain.EvidencePack) domain.ContractReport {
	var violations []domain.ContractViolation
	violations = append(violations, checkContentType(pack)...)
	violations = append(violations, checkStatusCodes(pack)...)
	violations = append(violations, checkErrorStructure(pack)...)

	totalChecks := len(pack.Items)
	complianceRate := 1.0
	if totalChecks > 0 {
		complianceRate = float64(totalChecks-len(violations)) / float64(totalChecks)
		if complianceRate < 0 {
			complianceRate = 0
		}
	}

	return domain.ContractReport{
		Compliant:       len(violations) == 0,
		ComplianceRate:  complianceRate,
		TotalChecks:     totalChecks,
		TotalViolations: len(violations),
		Violations:      violations,
	}
}

func checkContentType(pack *evidenceDomain.EvidencePack) []domain.ContractViolation {
	var violations []domain.ContractViolation
	checked := map[string]bool{}

	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status < 200 || item.Response.Status >= 300 {
			continue
		}
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		if checked[ep] {
			continue
		}
		checked[ep] = true

		body := item.Response.BodyInlineTruncated
		ct := headerValCI(item.Response.Headers, "Content-Type")

		if body != "" && ct == "" {
			violations = append(violations, domain.ContractViolation{
				Endpoint:    ep,
				StatusCode:  item.Response.Status,
				Field:       "Content-Type",
				Expected:    "application/json (or appropriate type)",
				Actual:      "(empty)",
				Description: "Response has body but no Content-Type header.",
			})
			continue
		}

		if body != "" && isJSONBody(body) && !strings.Contains(ct, "json") {
			violations = append(violations, domain.ContractViolation{
				Endpoint:    ep,
				StatusCode:  item.Response.Status,
				Field:       "Content-Type",
				Expected:    "application/json",
				Actual:      ct,
				Description: "Body looks like JSON but Content-Type doesn't indicate it.",
			})
		}
	}
	return violations
}

func checkStatusCodes(pack *evidenceDomain.EvidencePack) []domain.ContractViolation {
	var violations []domain.ContractViolation

	for _, item := range pack.Items {
		if item.Response == nil {
			continue
		}
		method := strings.ToUpper(item.Request.Method)
		ep := method + " " + extractPath(item.Request.URL)
		sc := item.Response.Status
		body := item.Response.BodyInlineTruncated

		if method == "DELETE" && sc == 200 && body == "" {
			violations = append(violations, domain.ContractViolation{
				Endpoint:    ep,
				StatusCode:  sc,
				Field:       "status_code",
				Expected:    "204 No Content",
				Actual:      "200 OK (no body)",
				Description: "Successful DELETE with 200 but no body. Should use 204.",
			})
		}

		if method == "POST" && sc == 200 && isJSONBody(body) {
			var obj map[string]any
			if json.Unmarshal([]byte(body), &obj) == nil {
				if _, hasID := obj["id"]; hasID {
					violations = append(violations, domain.ContractViolation{
						Endpoint:    ep,
						StatusCode:  sc,
						Field:       "status_code",
						Expected:    "201 Created",
						Actual:      "200 OK",
						Description: "POST creating a resource (response has 'id') should return 201.",
					})
				}
			}
		}
	}
	return violations
}

func checkErrorStructure(pack *evidenceDomain.EvidencePack) []domain.ContractViolation {
	var violations []domain.ContractViolation
	checked := map[string]bool{}

	for _, item := range pack.Items {
		if item.Response == nil || item.Response.Status < 400 || item.Response.Status >= 500 {
			continue
		}
		ep := item.Request.Method + " " + extractPath(item.Request.URL)
		key := fmt.Sprintf("%s|%d", ep, item.Response.Status)
		if checked[key] {
			continue
		}
		checked[key] = true

		body := item.Response.BodyInlineTruncated
		if body == "" {
			violations = append(violations, domain.ContractViolation{
				Endpoint:    ep,
				StatusCode:  item.Response.Status,
				Field:       "error_body",
				Expected:    `{"error": "...", "message": "..."}`,
				Actual:      "(empty)",
				Description: "Error response with no body. Errors should have an explanatory body.",
			})
			continue
		}

		if isJSONBody(body) {
			var obj map[string]any
			if json.Unmarshal([]byte(body), &obj) == nil {
				hasErrorField := false
				for k := range obj {
					kl := strings.ToLower(k)
					if kl == "error" || kl == "message" || kl == "detail" || kl == "details" || kl == "msg" {
						hasErrorField = true
						break
					}
				}
				if !hasErrorField {
					violations = append(violations, domain.ContractViolation{
						Endpoint:    ep,
						StatusCode:  item.Response.Status,
						Field:       "error_structure",
						Expected:    "Field 'error' or 'message' in body",
						Actual:      fmt.Sprintf("Fields: %s", strings.Join(objectKeys(obj), ", ")),
						Description: "Error response missing standard descriptive field (error/message).",
					})
				}
			}
		}
	}
	return violations
}

func headerValCI(h map[string]string, key string) string {
	lower := strings.ToLower(key)
	for k, v := range h {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return ""
}

func isJSONBody(body string) bool {
	body = strings.TrimSpace(body)
	return strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[")
}

func objectKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}
