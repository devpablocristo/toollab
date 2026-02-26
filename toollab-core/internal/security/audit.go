package security

import (
	"fmt"
	"strings"

	"toollab-core/internal/evidence"
)

type Finding struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Endpoint    string `json:"endpoint,omitempty"`
	Remediation string `json:"remediation"`
}

type AuditReport struct {
	Score     int       `json:"score"`
	Grade     string    `json:"grade"`
	Findings  []Finding `json:"findings"`
	Summary   Summary   `json:"summary"`
}

type Summary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

func Audit(bundle *evidence.Bundle) *AuditReport {
	report := &AuditReport{Findings: []Finding{}}

	report.Findings = append(report.Findings, checkSecurityHeaders(bundle)...)
	report.Findings = append(report.Findings, checkAuthBypass(bundle)...)
	report.Findings = append(report.Findings, checkSensitiveDataExposure(bundle)...)
	report.Findings = append(report.Findings, checkErrorInfoLeak(bundle)...)
	report.Findings = append(report.Findings, checkCORS(bundle)...)
	report.Findings = append(report.Findings, checkRateLimiting(bundle)...)

	report.Summary = summarizeFindings(report.Findings)
	report.Score = calculateScore(report.Summary)
	report.Grade = scoreToGrade(report.Score)
	return report
}

func checkSecurityHeaders(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	requiredHeaders := map[string]string{
		"Strict-Transport-Security": "Falta HSTS. Riesgo de downgrade a HTTP.",
		"X-Content-Type-Options":    "Falta X-Content-Type-Options. Riesgo de MIME sniffing.",
		"X-Frame-Options":           "Falta X-Frame-Options. Riesgo de clickjacking.",
		"Content-Security-Policy":   "Falta CSP. Riesgo de XSS y data injection.",
	}

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
		for header, desc := range requiredHeaders {
			key := header + "|" + endpoint
			if checked[key] {
				continue
			}
			checked[key] = true
			found := false
			for h := range sample.Response.Headers {
				if strings.EqualFold(h, header) {
					found = true
					break
				}
			}
			if !found {
				findings = append(findings, Finding{
					ID:          fmt.Sprintf("SEC-HDR-%s", strings.ToUpper(strings.ReplaceAll(header, "-", ""))),
					Category:    "security_headers",
					Severity:    "medium",
					Title:       fmt.Sprintf("Header de seguridad faltante: %s", header),
					Description: desc,
					Endpoint:    endpoint,
					Remediation: fmt.Sprintf("Agregar header '%s' a todas las respuestas.", header),
				})
			}
		}
	}
	return findings
}

func checkAuthBypass(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	authEndpoints := map[string]bool{}
	noAuthSuccess := map[string]bool{}

	for _, sample := range bundle.Samples {
		endpoint := sample.Request.Method + " " + sample.Request.URL
		hasAuth := false
		for h := range sample.Request.Headers {
			hl := strings.ToLower(h)
			if hl == "authorization" || strings.Contains(hl, "api-key") || strings.Contains(hl, "api_key") || strings.Contains(hl, "token") {
				hasAuth = true
				break
			}
		}
		if hasAuth {
			authEndpoints[endpoint] = true
		}
		if !hasAuth && sample.Response.StatusCode != nil && *sample.Response.StatusCode >= 200 && *sample.Response.StatusCode < 300 {
			noAuthSuccess[endpoint] = true
		}
	}

	for ep := range authEndpoints {
		if noAuthSuccess[ep] {
			findings = append(findings, Finding{
				ID:          "SEC-AUTH-BYPASS",
				Category:    "authentication",
				Severity:    "critical",
				Title:       "Posible bypass de autenticación",
				Description: "Endpoint responde con éxito tanto con auth como sin auth.",
				Endpoint:    ep,
				Remediation: "Verificar que todos los endpoints protegidos rechacen requests sin autenticación.",
			})
		}
	}
	return findings
}

func checkSensitiveDataExposure(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	sensitivePatterns := []struct {
		pattern string
		label   string
	}{
		{"password", "passwords"},
		{"secret", "secrets"},
		{"private_key", "claves privadas"},
		{"credit_card", "tarjetas de crédito"},
		{"ssn", "números de seguro social"},
		{"token", "tokens"},
	}

	checked := map[string]bool{}
	for _, sample := range bundle.Samples {
		if sample.Response.BodyPreview == "" {
			continue
		}
		body := strings.ToLower(sample.Response.BodyPreview)
		endpoint := sample.Request.Method + " " + sample.Request.URL
		for _, sp := range sensitivePatterns {
			key := sp.pattern + "|" + endpoint
			if checked[key] {
				continue
			}
			checked[key] = true
			if strings.Contains(body, fmt.Sprintf(`"%s"`, sp.pattern)) {
				findings = append(findings, Finding{
					ID:          "SEC-DATA-EXPOSURE",
					Category:    "data_exposure",
					Severity:    "high",
					Title:       fmt.Sprintf("Posible exposición de datos sensibles: %s", sp.label),
					Description: fmt.Sprintf("La respuesta contiene el campo '%s' que podría ser información sensible.", sp.pattern),
					Endpoint:    endpoint,
					Remediation: "Revisar qué datos se envían en la respuesta. Redactar o excluir campos sensibles.",
				})
			}
		}
	}
	return findings
}

func checkErrorInfoLeak(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	leakPatterns := []string{
		"stack trace", "stacktrace", "traceback",
		"panic:", "goroutine ",
		"at java.", "at org.",
		"sqlalchemy", "psycopg2", "mysql",
		"internal server error",
		"exception in thread",
		"file \"", "line ",
	}

	checked := map[string]bool{}
	for _, sample := range bundle.Samples {
		if sample.Response.StatusCode == nil || *sample.Response.StatusCode < 400 {
			continue
		}
		body := strings.ToLower(sample.Response.BodyPreview)
		endpoint := sample.Request.Method + " " + sample.Request.URL
		for _, pattern := range leakPatterns {
			key := pattern + "|" + endpoint
			if checked[key] {
				continue
			}
			checked[key] = true
			if strings.Contains(body, pattern) {
				findings = append(findings, Finding{
					ID:          "SEC-INFO-LEAK",
					Category:    "information_leak",
					Severity:    "high",
					Title:       "Fuga de información en respuesta de error",
					Description: fmt.Sprintf("La respuesta de error contiene información técnica interna ('%s').", pattern),
					Endpoint:    endpoint,
					Remediation: "No exponer detalles internos en respuestas de error. Usar mensajes genéricos y loguear detalles internamente.",
				})
				break
			}
		}
	}
	return findings
}

func checkCORS(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	checked := map[string]bool{}
	for _, sample := range bundle.Samples {
		endpoint := sample.Request.Method + " " + sample.Request.URL
		if checked[endpoint] {
			continue
		}
		checked[endpoint] = true
		for h, v := range sample.Response.Headers {
			if strings.EqualFold(h, "Access-Control-Allow-Origin") && v == "*" {
				findings = append(findings, Finding{
					ID:          "SEC-CORS-WILDCARD",
					Category:    "cors",
					Severity:    "medium",
					Title:       "CORS configurado con wildcard (*)",
					Description: "Access-Control-Allow-Origin: * permite requests desde cualquier origen.",
					Endpoint:    endpoint,
					Remediation: "Restringir CORS a dominios específicos permitidos.",
				})
			}
		}
	}
	return findings
}

func checkRateLimiting(bundle *evidence.Bundle) []Finding {
	var findings []Finding
	hasRateLimit := false
	for _, sample := range bundle.Samples {
		for h := range sample.Response.Headers {
			hl := strings.ToLower(h)
			if strings.Contains(hl, "ratelimit") || strings.Contains(hl, "rate-limit") || strings.Contains(hl, "x-ratelimit") || hl == "retry-after" {
				hasRateLimit = true
				break
			}
		}
		if hasRateLimit {
			break
		}
		if sample.Response.StatusCode != nil && *sample.Response.StatusCode == 429 {
			hasRateLimit = true
			break
		}
	}

	if !hasRateLimit && len(bundle.Samples) > 0 {
		findings = append(findings, Finding{
			ID:          "SEC-NO-RATELIMIT",
			Category:    "rate_limiting",
			Severity:    "medium",
			Title:       "Sin evidencia de rate limiting",
			Description: "No se detectaron headers de rate limiting ni respuestas 429 en ningún endpoint.",
			Remediation: "Implementar rate limiting para prevenir abuso y ataques de fuerza bruta.",
		})
	}
	return findings
}

func summarizeFindings(findings []Finding) Summary {
	s := Summary{Total: len(findings)}
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
		case "info":
			s.Info++
		}
	}
	return s
}

func calculateScore(s Summary) int {
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
