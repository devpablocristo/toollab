package contract

import (
	"encoding/json"
	"fmt"
	"strings"

	"toollab-core/internal/evidence"
)

type Violation struct {
	Endpoint    string `json:"endpoint"`
	StatusCode  int    `json:"status_code"`
	Field       string `json:"field"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Description string `json:"description"`
}

type EndpointCompliance struct {
	Endpoint   string  `json:"endpoint"`
	TotalReqs  int     `json:"total_requests"`
	Compliant  int     `json:"compliant"`
	Violations int     `json:"violations"`
	Rate       float64 `json:"compliance_rate"`
}

type ContractReport struct {
	Compliant            bool                 `json:"compliant"`
	TotalChecks          int                  `json:"total_checks"`
	TotalViolations      int                  `json:"total_violations"`
	ComplianceRate       float64              `json:"compliance_rate"`
	Violations           []Violation          `json:"violations"`
	EndpointCompliance   []EndpointCompliance `json:"endpoint_compliance"`
	ContentTypeViolation []Violation          `json:"content_type_violations"`
	StatusCodeViolation  []Violation          `json:"status_code_violations"`
}

func Validate(bundle *evidence.Bundle) *ContractReport {
	report := &ContractReport{
		Violations:           []Violation{},
		EndpointCompliance:   []EndpointCompliance{},
		ContentTypeViolation: []Violation{},
		StatusCodeViolation:  []Violation{},
	}

	report.ContentTypeViolation = checkContentType(bundle)
	report.StatusCodeViolation = checkStatusCodes(bundle)
	report.Violations = append(report.Violations, checkResponseStructure(bundle)...)
	report.Violations = append(report.Violations, report.ContentTypeViolation...)
	report.Violations = append(report.Violations, report.StatusCodeViolation...)

	report.EndpointCompliance = buildEndpointCompliance(bundle, report.Violations)

	report.TotalChecks = len(bundle.Samples)
	report.TotalViolations = len(report.Violations)
	if report.TotalChecks > 0 {
		report.ComplianceRate = float64(report.TotalChecks-report.TotalViolations) / float64(report.TotalChecks)
		if report.ComplianceRate < 0 {
			report.ComplianceRate = 0
		}
	}
	report.Compliant = report.TotalViolations == 0
	return report
}

func checkContentType(bundle *evidence.Bundle) []Violation {
	var violations []Violation
	checked := map[string]bool{}

	for _, sample := range bundle.Samples {
		if sample.Response.StatusCode == nil {
			continue
		}
		sc := *sample.Response.StatusCode
		if sc < 200 || sc >= 300 {
			continue
		}
		endpoint := sample.Request.Method + " " + sample.Request.URL
		if checked[endpoint] {
			continue
		}
		checked[endpoint] = true

		ct := ""
		for h, v := range sample.Response.Headers {
			if strings.EqualFold(h, "content-type") {
				ct = v
				break
			}
		}

		if sample.Response.BodyPreview != "" && ct == "" {
			violations = append(violations, Violation{
				Endpoint:    endpoint,
				StatusCode:  sc,
				Field:       "Content-Type",
				Expected:    "application/json (o tipo adecuado)",
				Actual:      "(vacío)",
				Description: "Respuesta con body pero sin Content-Type header.",
			})
			continue
		}

		if sample.Response.BodyPreview != "" && isJSONBody(sample.Response.BodyPreview) && !strings.Contains(ct, "json") {
			violations = append(violations, Violation{
				Endpoint:    endpoint,
				StatusCode:  sc,
				Field:       "Content-Type",
				Expected:    "application/json",
				Actual:      ct,
				Description: "Body parece JSON pero Content-Type no lo indica.",
			})
		}
	}
	return violations
}

func checkStatusCodes(bundle *evidence.Bundle) []Violation {
	var violations []Violation

	for _, sample := range bundle.Samples {
		if sample.Response.StatusCode == nil {
			continue
		}
		sc := *sample.Response.StatusCode
		method := strings.ToUpper(sample.Request.Method)
		endpoint := method + " " + sample.Request.URL

		if method == "DELETE" && sc == 200 && sample.Response.BodyPreview == "" {
			violations = append(violations, Violation{
				Endpoint:    endpoint,
				StatusCode:  sc,
				Field:       "status_code",
				Expected:    "204 No Content",
				Actual:      "200 OK (sin body)",
				Description: "DELETE exitoso con 200 pero sin body. Se recomienda 204.",
			})
		}

		if method == "POST" && sc == 200 {
			if isJSONBody(sample.Response.BodyPreview) {
				var obj map[string]any
				if json.Unmarshal([]byte(sample.Response.BodyPreview), &obj) == nil {
					if _, hasID := obj["id"]; hasID {
						violations = append(violations, Violation{
							Endpoint:    endpoint,
							StatusCode:  sc,
							Field:       "status_code",
							Expected:    "201 Created",
							Actual:      "200 OK",
							Description: "POST que crea recurso (respuesta tiene 'id') debería devolver 201.",
						})
					}
				}
			}
		}
	}
	return violations
}

func checkResponseStructure(bundle *evidence.Bundle) []Violation {
	var violations []Violation
	checked := map[string]bool{}

	for _, sample := range bundle.Samples {
		if sample.Response.StatusCode == nil {
			continue
		}
		sc := *sample.Response.StatusCode
		if sc < 400 || sc >= 500 {
			continue
		}
		endpoint := sample.Request.Method + " " + sample.Request.URL
		key := fmt.Sprintf("%s|%d", endpoint, sc)
		if checked[key] {
			continue
		}
		checked[key] = true

		if sample.Response.BodyPreview == "" {
			violations = append(violations, Violation{
				Endpoint:    endpoint,
				StatusCode:  sc,
				Field:       "error_body",
				Expected:    `{"error": "...", "message": "..."}`,
				Actual:      "(vacío)",
				Description: "Respuesta de error sin body. Los errores deben tener un body explicativo.",
			})
			continue
		}

		if isJSONBody(sample.Response.BodyPreview) {
			var obj map[string]any
			if json.Unmarshal([]byte(sample.Response.BodyPreview), &obj) == nil {
				hasError := false
				for k := range obj {
					kl := strings.ToLower(k)
					if kl == "error" || kl == "message" || kl == "detail" || kl == "details" || kl == "msg" {
						hasError = true
						break
					}
				}
				if !hasError {
					violations = append(violations, Violation{
						Endpoint:    endpoint,
						StatusCode:  sc,
						Field:       "error_structure",
						Expected:    "Campo 'error' o 'message' en el body",
						Actual:      fmt.Sprintf("Campos: %s", strings.Join(objectKeys(obj), ", ")),
						Description: "Respuesta de error sin campo descriptivo estándar (error/message).",
					})
				}
			}
		}
	}
	return violations
}

func buildEndpointCompliance(bundle *evidence.Bundle, violations []Violation) []EndpointCompliance {
	violationCount := map[string]int{}
	for _, v := range violations {
		violationCount[v.Endpoint]++
	}

	endpointReqs := map[string]int{}
	for _, o := range bundle.Outcomes {
		key := o.Method + " " + o.Path
		endpointReqs[key]++
	}

	var result []EndpointCompliance
	for ep, total := range endpointReqs {
		vc := violationCount[ep]
		compliant := total - vc
		if compliant < 0 {
			compliant = 0
		}
		rate := 1.0
		if total > 0 {
			rate = float64(compliant) / float64(total)
		}
		result = append(result, EndpointCompliance{
			Endpoint:   ep,
			TotalReqs:  total,
			Compliant:  compliant,
			Violations: vc,
			Rate:       rate,
		})
	}
	return result
}

func isJSONBody(body string) bool {
	body = strings.TrimSpace(body)
	return (strings.HasPrefix(body, "{") || strings.HasPrefix(body, "["))
}

func objectKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys
}
