package analyze

import (
	"fmt"
	"strings"

	"toollab-core/internal/analyze/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func checkHiddenEndpoints(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	findings = append(findings, checkExposedHiddenPaths(pack)...)
	findings = append(findings, checkServerHeaderLeak(pack)...)
	findings = append(findings, checkMethodNotAllowed(pack)...)
	findings = append(findings, checkTraceEnabled(pack)...)
	return findings
}

func checkExposedHiddenPaths(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	checked := map[string]bool{}

	criticalPaths := map[string]bool{
		"/.env": true, "/.git/config": true, "/.git/HEAD": true,
		"/debug/pprof": true, "/debug/vars": true,
		"/actuator/env": true, "/actuator/configprops": true,
		"/config": true, "/config.json": true, "/config.yaml": true,
	}

	adminPaths := map[string]bool{
		"/admin": true, "/admin/": true, "/_admin": true,
		"/console": true, "/shell": true,
		"/phpmyadmin": true, "/wp-admin": true,
	}

	debugPaths := map[string]bool{
		"/debug": true, "/_debug": true, "/trace": true,
		"/internal": true, "/_internal": true,
		"/_stats": true, "/status": true, "/_status": true,
	}

	for _, item := range pack.Items {
		if !hasTag(item, "hidden-endpoint") {
			continue
		}
		if item.Response == nil {
			continue
		}
		path := extractPath(item.Request.URL)
		if checked[path] {
			continue
		}
		checked[path] = true

		if item.Response.Status == 404 || item.Response.Status == 405 {
			continue
		}

		if item.Response.Status >= 200 && item.Response.Status < 400 {
			ep := item.Request.Method + " " + path

			if criticalPaths[path] {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-HIDDEN-CRITICAL",
					Category:    "hidden_endpoints",
					Severity:    "critical",
					Title:       fmt.Sprintf("Critical hidden path exposed: %s", path),
					Description: fmt.Sprintf("Path '%s' returned status %d. This path may expose sensitive configuration, secrets, or debug information.", path, item.Response.Status),
					Endpoint:    ep,
					Remediation: "Block access to this path immediately. It should not be accessible from outside.",
				})
			} else if adminPaths[path] {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-HIDDEN-ADMIN",
					Category:    "hidden_endpoints",
					Severity:    "high",
					Title:       fmt.Sprintf("Admin/management path exposed: %s", path),
					Description: fmt.Sprintf("Admin path '%s' returned status %d. Administrative interfaces should not be publicly accessible.", path, item.Response.Status),
					Endpoint:    ep,
					Remediation: "Restrict access to admin paths via IP allowlist, VPN, or strong authentication.",
				})
			} else if debugPaths[path] {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-HIDDEN-DEBUG",
					Category:    "hidden_endpoints",
					Severity:    "high",
					Title:       fmt.Sprintf("Debug/internal path exposed: %s", path),
					Description: fmt.Sprintf("Debug path '%s' returned status %d. Debug endpoints should be disabled in production.", path, item.Response.Status),
					Endpoint:    ep,
					Remediation: "Disable debug endpoints in production. Use environment-based configuration.",
				})
			} else {
				findings = append(findings, domain.SecurityFinding{
					ID:          "SEC-HIDDEN-EXPOSED",
					Category:    "hidden_endpoints",
					Severity:    "medium",
					Title:       fmt.Sprintf("Undocumented path accessible: %s", path),
					Description: fmt.Sprintf("Path '%s' returned status %d but is not in the API specification.", path, item.Response.Status),
					Endpoint:    ep,
					Remediation: "Review if this path should be publicly accessible. Document it or restrict access.",
				})
			}
		}
	}
	return findings
}

func checkServerHeaderLeak(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	checked := false
	for _, item := range pack.Items {
		if item.Response == nil || checked {
			continue
		}
		for h, v := range item.Response.Headers {
			hl := strings.ToLower(h)
			if hl == "server" {
				if containsVersion(v) {
					checked = true
					return []domain.SecurityFinding{{
						ID:          "SEC-SERVER-VERSION",
						Category:    "information_leak",
						Severity:    "low",
						Title:       "Server header exposes version information",
						Description: fmt.Sprintf("Server header contains version info: '%s'. This helps attackers identify known vulnerabilities.", v),
						Remediation: "Remove or genericize the Server header. Avoid exposing version numbers.",
					}}
				}
			}
			if hl == "x-powered-by" {
				checked = true
				return []domain.SecurityFinding{{
					ID:          "SEC-XPOWEREDBY",
					Category:    "information_leak",
					Severity:    "low",
					Title:       "X-Powered-By header exposes technology stack",
					Description: fmt.Sprintf("X-Powered-By header reveals: '%s'.", v),
					Remediation: "Remove the X-Powered-By header.",
				}}
			}
		}
	}
	return nil
}

func checkMethodNotAllowed(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	var findings []domain.SecurityFinding
	silentAccepts := map[string][]string{}

	for _, item := range pack.Items {
		if !hasTag(item, "method-tamper") {
			continue
		}
		if item.Response == nil {
			continue
		}

		ep := item.Request.Method + " " + extractPath(item.Request.URL)

		if item.Response.Status >= 200 && item.Response.Status < 300 {
			silentAccepts[extractPath(item.Request.URL)] = append(
				silentAccepts[extractPath(item.Request.URL)],
				item.Request.Method,
			)
		}

		if item.Response.Status >= 500 {
			findings = append(findings, domain.SecurityFinding{
				ID:          "ROB-METHOD-CRASH",
				Category:    "robustness",
				Severity:    "medium",
				Title:       "Server crash on unexpected HTTP method",
				Description: fmt.Sprintf("Sending %s to this endpoint causes a 500 error instead of 405 Method Not Allowed.", item.Request.Method),
				Endpoint:    ep,
				Remediation: "Return 405 Method Not Allowed for unsupported HTTP methods.",
			})
		}
	}

	for path, methods := range silentAccepts {
		findings = append(findings, domain.SecurityFinding{
			ID:          "SEC-METHOD-ACCEPT",
			Category:    "method_handling",
			Severity:    "medium",
			Title:       fmt.Sprintf("Endpoint silently accepts unexpected methods: %s", strings.Join(methods, ", ")),
			Description: fmt.Sprintf("Path '%s' accepts HTTP methods that are not documented: %s. This may allow unintended actions.", path, strings.Join(methods, ", ")),
			Endpoint:    methods[0] + " " + path,
			Remediation: "Restrict endpoints to only accept documented HTTP methods. Return 405 for others.",
		})
	}
	return findings
}

func checkTraceEnabled(pack *evidenceDomain.EvidencePack) []domain.SecurityFinding {
	for _, item := range pack.Items {
		if item.Request.Method != "TRACE" {
			continue
		}
		if item.Response != nil && item.Response.Status >= 200 && item.Response.Status < 300 {
			return []domain.SecurityFinding{{
				ID:          "SEC-TRACE-ENABLED",
				Category:    "method_handling",
				Severity:    "medium",
				Title:       "HTTP TRACE method enabled",
				Description: "TRACE method is enabled, which can be used for Cross-Site Tracing (XST) attacks.",
				Endpoint:    "TRACE /",
				Remediation: "Disable the TRACE HTTP method on the server.",
			}}
		}
	}
	return nil
}

func containsVersion(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}
