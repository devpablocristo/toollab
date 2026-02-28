package analyze

import (
	"encoding/json"
	"fmt"
	"strings"

	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	targetDomain "toollab-core/internal/target/usecases/domain"
	"toollab-core/internal/shared"
)

func generateAllProbes(plan scenarioDomain.ScenarioPlan, target targetDomain.Target) scenarioDomain.ScenarioPlan {
	var extra []scenarioDomain.ScenarioCase

	if len(target.RuntimeHint.AuthHeaders) > 0 {
		extra = append(extra, generateNoAuthProbes(plan.Cases)...)
	}
	extra = append(extra, generateInjectionProbes(plan.Cases)...)
	extra = append(extra, generateMalformedProbes(plan.Cases)...)
	extra = append(extra, generateBoundaryProbes(plan.Cases)...)
	extra = append(extra, generateMethodTamperProbes(plan.Cases)...)
	extra = append(extra, generateHiddenEndpointProbes()...)
	extra = append(extra, generateLargePayloadProbes(plan.Cases)...)
	extra = append(extra, generateContentTypeMismatchProbes(plan.Cases)...)

	plan.Cases = append(plan.Cases, extra...)
	return plan
}

// --- No-auth probes: test every endpoint without authentication ---

func generateNoAuthProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	for _, c := range cases {
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID:  shared.NewID(),
			Name:    c.Name + " [no-auth]",
			Enabled: true,
			Tags:    []string{"no-auth", "probe"},
			Request: cloneRequest(c.Request),
			Auth:    &scenarioDomain.CaseAuth{Mode: "none"},
		})
	}
	return probes
}

// --- Injection probes: SQL injection, XSS, path traversal, command injection ---

var sqlInjectionPayloads = []string{
	"' OR '1'='1",
	"'; DROP TABLE users; --",
	"1 UNION SELECT NULL--",
	"admin'--",
	"1; WAITFOR DELAY '0:0:5'--",
}

var xssPayloads = []string{
	"<script>alert(1)</script>",
	"<img src=x onerror=alert(1)>",
	"javascript:alert(1)",
	"'\"><svg onload=alert(1)>",
}

var pathTraversalPayloads = []string{
	"../../../etc/passwd",
	"..\\..\\..\\windows\\system32\\config\\sam",
	"....//....//....//etc/passwd",
	"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
	"/etc/passwd%00",
}

var commandInjectionPayloads = []string{
	"; cat /etc/passwd",
	"| ls -la",
	"`id`",
	"$(whoami)",
	"; sleep 5",
}

func generateInjectionProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	seen := map[string]bool{}

	for _, c := range cases {
		key := c.Request.Method + " " + c.Request.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		if len(c.Request.PathParams) > 0 {
			for paramName := range c.Request.PathParams {
				for i, payload := range sqlInjectionPayloads {
					params := cloneMap(c.Request.PathParams)
					params[paramName] = payload
					probes = append(probes, scenarioDomain.ScenarioCase{
						CaseID:  shared.NewID(),
						Name:    fmt.Sprintf("%s [sqli-path-%s-%d]", key, paramName, i),
						Enabled: true,
						Tags:    []string{"sqli", "injection", "probe"},
						Request: scenarioDomain.CaseRequest{
							Method: c.Request.Method, Path: c.Request.Path,
							PathParams: params, Query: c.Request.Query,
							Headers: c.Request.Headers,
						},
					})
				}
				for i, payload := range pathTraversalPayloads {
					params := cloneMap(c.Request.PathParams)
					params[paramName] = payload
					probes = append(probes, scenarioDomain.ScenarioCase{
						CaseID:  shared.NewID(),
						Name:    fmt.Sprintf("%s [traversal-%s-%d]", key, paramName, i),
						Enabled: true,
						Tags:    []string{"path-traversal", "injection", "probe"},
						Request: scenarioDomain.CaseRequest{
							Method: c.Request.Method, Path: c.Request.Path,
							PathParams: params, Query: c.Request.Query,
							Headers: c.Request.Headers,
						},
					})
				}
			}
		}

		if len(c.Request.Query) > 0 {
			for qName := range c.Request.Query {
				allPayloads := append(append(sqlInjectionPayloads, xssPayloads...), commandInjectionPayloads...)
				for i, payload := range allPayloads {
					query := cloneMap(c.Request.Query)
					query[qName] = payload
					tag := "sqli"
					if i >= len(sqlInjectionPayloads) && i < len(sqlInjectionPayloads)+len(xssPayloads) {
						tag = "xss"
					} else if i >= len(sqlInjectionPayloads)+len(xssPayloads) {
						tag = "cmdi"
					}
					probes = append(probes, scenarioDomain.ScenarioCase{
						CaseID:  shared.NewID(),
						Name:    fmt.Sprintf("%s [%s-query-%s-%d]", key, tag, qName, i),
						Enabled: true,
						Tags:    []string{tag, "injection", "probe"},
						Request: scenarioDomain.CaseRequest{
							Method: c.Request.Method, Path: c.Request.Path,
							PathParams: c.Request.PathParams, Query: query,
							Headers: c.Request.Headers,
						},
					})
				}
			}
		}

		if len(c.Request.BodyJSON) > 0 && (c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH") {
			var body map[string]any
			if json.Unmarshal(c.Request.BodyJSON, &body) == nil {
				for fieldName, originalVal := range body {
					if _, isStr := originalVal.(string); !isStr {
						continue
					}
					allPayloads := append(append(sqlInjectionPayloads, xssPayloads...), commandInjectionPayloads...)
					for i, payload := range allPayloads {
						modified := cloneAnyMap(body)
						modified[fieldName] = payload
						tag := "sqli"
						if i >= len(sqlInjectionPayloads) && i < len(sqlInjectionPayloads)+len(xssPayloads) {
							tag = "xss"
						} else if i >= len(sqlInjectionPayloads)+len(xssPayloads) {
							tag = "cmdi"
						}
						bodyJSON, _ := json.Marshal(modified)
						probes = append(probes, scenarioDomain.ScenarioCase{
							CaseID:  shared.NewID(),
							Name:    fmt.Sprintf("%s [%s-body-%s-%d]", key, tag, fieldName, i),
							Enabled: true,
							Tags:    []string{tag, "injection", "probe"},
							Request: scenarioDomain.CaseRequest{
								Method: c.Request.Method, Path: c.Request.Path,
								PathParams: c.Request.PathParams, Query: c.Request.Query,
								Headers: c.Request.Headers, BodyJSON: bodyJSON,
							},
						})
					}
				}
			}
		}
	}
	return probes
}

// --- Malformed input probes ---

func generateMalformedProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	seen := map[string]bool{}

	for _, c := range cases {
		if c.Request.Method != "POST" && c.Request.Method != "PUT" && c.Request.Method != "PATCH" {
			continue
		}
		key := c.Request.Method + " " + c.Request.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		// Broken JSON
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-broken-json]", key),
			Enabled: true, Tags: []string{"malformed", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers, BodyJSON: json.RawMessage(`{invalid json`),
			},
		})

		// Empty body when body expected
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-empty-body]", key),
			Enabled: true, Tags: []string{"malformed", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers,
			},
		})

		// Array instead of object
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-array-body]", key),
			Enabled: true, Tags: []string{"malformed", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers, BodyJSON: json.RawMessage(`[]`),
			},
		})

		// Null body
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-null-body]", key),
			Enabled: true, Tags: []string{"malformed", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers, BodyJSON: json.RawMessage(`null`),
			},
		})

		// Wrong types in body fields
		if len(c.Request.BodyJSON) > 0 {
			var body map[string]any
			if json.Unmarshal(c.Request.BodyJSON, &body) == nil {
				wrongTypes := buildWrongTypeBody(body)
				wtJSON, _ := json.Marshal(wrongTypes)
				probes = append(probes, scenarioDomain.ScenarioCase{
					CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-wrong-types]", key),
					Enabled: true, Tags: []string{"malformed", "probe"},
					Request: scenarioDomain.CaseRequest{
						Method: c.Request.Method, Path: c.Request.Path,
						PathParams: c.Request.PathParams, Query: c.Request.Query,
						Headers: c.Request.Headers, BodyJSON: wtJSON,
					},
				})

				// Missing each required-looking field
				for fieldName := range body {
					partial := cloneAnyMap(body)
					delete(partial, fieldName)
					pJSON, _ := json.Marshal(partial)
					probes = append(probes, scenarioDomain.ScenarioCase{
						CaseID: shared.NewID(), Name: fmt.Sprintf("%s [malformed-missing-%s]", key, fieldName),
						Enabled: true, Tags: []string{"malformed", "missing-field", "probe"},
						Request: scenarioDomain.CaseRequest{
							Method: c.Request.Method, Path: c.Request.Path,
							PathParams: c.Request.PathParams, Query: c.Request.Query,
							Headers: c.Request.Headers, BodyJSON: pJSON,
						},
					})
				}
			}
		}
	}
	return probes
}

// --- Boundary value probes ---

func generateBoundaryProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	seen := map[string]bool{}

	for _, c := range cases {
		if c.Request.Method != "POST" && c.Request.Method != "PUT" && c.Request.Method != "PATCH" {
			continue
		}
		if len(c.Request.BodyJSON) == 0 {
			continue
		}
		key := c.Request.Method + " " + c.Request.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		var body map[string]any
		if json.Unmarshal(c.Request.BodyJSON, &body) != nil {
			continue
		}

		for fieldName, val := range body {
			switch val.(type) {
			case string:
				// Huge string
				huge := cloneAnyMap(body)
				huge[fieldName] = strings.Repeat("A", 100000)
				hJSON, _ := json.Marshal(huge)
				probes = append(probes, makeProbe(key, "boundary-huge-string-"+fieldName, []string{"boundary", "probe"}, c, hJSON))

				// Empty string
				empty := cloneAnyMap(body)
				empty[fieldName] = ""
				eJSON, _ := json.Marshal(empty)
				probes = append(probes, makeProbe(key, "boundary-empty-string-"+fieldName, []string{"boundary", "probe"}, c, eJSON))

				// Unicode/special chars
				uni := cloneAnyMap(body)
				uni[fieldName] = "é\x00\xff\n\t<>&\"'\u202E\u0000"
				uJSON, _ := json.Marshal(uni)
				probes = append(probes, makeProbe(key, "boundary-unicode-"+fieldName, []string{"boundary", "probe"}, c, uJSON))

				// Null value
				nul := cloneAnyMap(body)
				nul[fieldName] = nil
				nJSON, _ := json.Marshal(nul)
				probes = append(probes, makeProbe(key, "boundary-null-"+fieldName, []string{"boundary", "probe"}, c, nJSON))

			case float64:
				// Negative number
				neg := cloneAnyMap(body)
				neg[fieldName] = -99999
				nJSON, _ := json.Marshal(neg)
				probes = append(probes, makeProbe(key, "boundary-negative-"+fieldName, []string{"boundary", "probe"}, c, nJSON))

				// Zero
				zero := cloneAnyMap(body)
				zero[fieldName] = 0
				zJSON, _ := json.Marshal(zero)
				probes = append(probes, makeProbe(key, "boundary-zero-"+fieldName, []string{"boundary", "probe"}, c, zJSON))

				// Max int
				max := cloneAnyMap(body)
				max[fieldName] = 9999999999999
				mJSON, _ := json.Marshal(max)
				probes = append(probes, makeProbe(key, "boundary-maxint-"+fieldName, []string{"boundary", "probe"}, c, mJSON))
			}
		}
	}
	return probes
}

// --- Method tampering probes ---

func generateMethodTamperProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	knownMethods := map[string][]string{}
	for _, c := range cases {
		knownMethods[c.Request.Path] = append(knownMethods[c.Request.Path], c.Request.Method)
	}

	allMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}
	seen := map[string]bool{}

	for path, methods := range knownMethods {
		methodSet := map[string]bool{}
		for _, m := range methods {
			methodSet[m] = true
		}
		for _, m := range allMethods {
			if methodSet[m] {
				continue
			}
			key := m + " " + path
			if seen[key] {
				continue
			}
			seen[key] = true
			probes = append(probes, scenarioDomain.ScenarioCase{
				CaseID: shared.NewID(), Name: fmt.Sprintf("%s %s [method-tamper]", m, path),
				Enabled: true, Tags: []string{"method-tamper", "probe"},
				Request: scenarioDomain.CaseRequest{Method: m, Path: path},
			})
		}
	}
	return probes
}

// --- Hidden endpoint probes ---

var hiddenPaths = []struct {
	path string
	tag  string
	desc string
}{
	{"/admin", "hidden-admin", "admin panel"},
	{"/admin/", "hidden-admin", "admin panel"},
	{"/_admin", "hidden-admin", "admin panel"},
	{"/dashboard", "hidden-admin", "dashboard"},
	{"/debug", "hidden-debug", "debug endpoint"},
	{"/debug/pprof", "hidden-debug", "Go pprof profiler"},
	{"/debug/vars", "hidden-debug", "Go expvar"},
	{"/_debug", "hidden-debug", "debug endpoint"},
	{"/.env", "hidden-config", "environment file"},
	{"/config", "hidden-config", "configuration"},
	{"/config.json", "hidden-config", "configuration file"},
	{"/config.yaml", "hidden-config", "configuration file"},
	{"/settings", "hidden-config", "settings"},
	{"/internal", "hidden-internal", "internal endpoint"},
	{"/_internal", "hidden-internal", "internal endpoint"},
	{"/metrics", "hidden-metrics", "Prometheus metrics"},
	{"/prometheus", "hidden-metrics", "Prometheus endpoint"},
	{"/.git/config", "hidden-git", "git configuration"},
	{"/.git/HEAD", "hidden-git", "git HEAD"},
	{"/.gitignore", "hidden-git", "gitignore file"},
	{"/server-status", "hidden-status", "Apache server-status"},
	{"/server-info", "hidden-status", "Apache server-info"},
	{"/status", "hidden-status", "status page"},
	{"/_status", "hidden-status", "status page"},
	{"/actuator", "hidden-actuator", "Spring Boot actuator"},
	{"/actuator/health", "hidden-actuator", "Spring Boot health"},
	{"/actuator/env", "hidden-actuator", "Spring Boot env"},
	{"/actuator/configprops", "hidden-actuator", "Spring Boot config"},
	{"/graphql", "hidden-graphql", "GraphQL endpoint"},
	{"/graphiql", "hidden-graphql", "GraphiQL IDE"},
	{"/api", "hidden-api", "API root"},
	{"/api/v1", "hidden-api", "API v1 root"},
	{"/api/v2", "hidden-api", "API v2 root"},
	{"/console", "hidden-console", "web console"},
	{"/shell", "hidden-console", "web shell"},
	{"/phpmyadmin", "hidden-db", "phpMyAdmin"},
	{"/wp-admin", "hidden-cms", "WordPress admin"},
	{"/wp-login.php", "hidden-cms", "WordPress login"},
	{"/robots.txt", "hidden-info", "robots.txt"},
	{"/sitemap.xml", "hidden-info", "sitemap"},
	{"/favicon.ico", "hidden-info", "favicon"},
	{"/trace", "hidden-debug", "TRACE endpoint"},
	{"/test", "hidden-test", "test endpoint"},
	{"/.well-known/security.txt", "hidden-info", "security.txt"},
	{"/backup", "hidden-backup", "backup files"},
	{"/dump", "hidden-backup", "data dump"},
	{"/export", "hidden-backup", "data export"},
	{"/login", "hidden-auth", "login page"},
	{"/register", "hidden-auth", "registration"},
	{"/signup", "hidden-auth", "signup page"},
	{"/reset-password", "hidden-auth", "password reset"},
	{"/token", "hidden-auth", "token endpoint"},
	{"/oauth", "hidden-auth", "OAuth endpoint"},
}

func generateHiddenEndpointProbes() []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	for _, hp := range hiddenPaths {
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("Hidden: %s (%s)", hp.path, hp.desc),
			Enabled: true, Tags: []string{hp.tag, "hidden-endpoint", "probe"},
			Request: scenarioDomain.CaseRequest{Method: "GET", Path: hp.path},
			Auth:    &scenarioDomain.CaseAuth{Mode: "none"},
		})
	}

	// TRACE method on root
	probes = append(probes, scenarioDomain.ScenarioCase{
		CaseID: shared.NewID(), Name: "TRACE method on root",
		Enabled: true, Tags: []string{"method-tamper", "hidden-endpoint", "probe"},
		Request: scenarioDomain.CaseRequest{Method: "TRACE", Path: "/"},
		Auth:    &scenarioDomain.CaseAuth{Mode: "none"},
	})

	return probes
}

// --- Large payload probes ---

func generateLargePayloadProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	seen := map[string]bool{}

	for _, c := range cases {
		if c.Request.Method != "POST" && c.Request.Method != "PUT" {
			continue
		}
		key := c.Request.Method + " " + c.Request.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		// 1MB payload
		huge := map[string]string{"data": strings.Repeat("X", 1024*1024)}
		hugeJSON, _ := json.Marshal(huge)
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [large-payload-1mb]", key),
			Enabled: true, Tags: []string{"large-payload", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers, BodyJSON: hugeJSON,
			},
		})

		// Deeply nested JSON
		nested := buildDeeplyNested(50)
		nestedJSON, _ := json.Marshal(nested)
		probes = append(probes, scenarioDomain.ScenarioCase{
			CaseID: shared.NewID(), Name: fmt.Sprintf("%s [deep-nested-json]", key),
			Enabled: true, Tags: []string{"large-payload", "probe"},
			Request: scenarioDomain.CaseRequest{
				Method: c.Request.Method, Path: c.Request.Path,
				PathParams: c.Request.PathParams, Query: c.Request.Query,
				Headers: c.Request.Headers, BodyJSON: nestedJSON,
			},
		})
	}
	return probes
}

// --- Content-Type mismatch probes ---

func generateContentTypeMismatchProbes(cases []scenarioDomain.ScenarioCase) []scenarioDomain.ScenarioCase {
	var probes []scenarioDomain.ScenarioCase
	seen := map[string]bool{}

	for _, c := range cases {
		if c.Request.Method != "POST" && c.Request.Method != "PUT" {
			continue
		}
		key := c.Request.Method + " " + c.Request.Path
		if seen[key] {
			continue
		}
		seen[key] = true

		wrongTypes := []string{
			"text/plain",
			"application/xml",
			"multipart/form-data",
			"text/html",
			"application/x-www-form-urlencoded",
		}

		for _, ct := range wrongTypes {
			headers := cloneMap(c.Request.Headers)
			if headers == nil {
				headers = map[string]string{}
			}
			headers["Content-Type"] = ct
			probes = append(probes, scenarioDomain.ScenarioCase{
				CaseID: shared.NewID(), Name: fmt.Sprintf("%s [wrong-content-type-%s]", key, ct),
				Enabled: true, Tags: []string{"content-type-mismatch", "probe"},
				Request: scenarioDomain.CaseRequest{
					Method: c.Request.Method, Path: c.Request.Path,
					PathParams: c.Request.PathParams, Query: c.Request.Query,
					Headers: headers, BodyJSON: c.Request.BodyJSON,
				},
			})
		}
	}
	return probes
}

// --- Helpers ---

func cloneRequest(r scenarioDomain.CaseRequest) scenarioDomain.CaseRequest {
	return scenarioDomain.CaseRequest{
		Method:     r.Method,
		Path:       r.Path,
		PathParams: cloneMap(r.PathParams),
		Query:      cloneMap(r.Query),
		Headers:    cloneMap(r.Headers),
		BodyJSON:   r.BodyJSON,
	}
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneAnyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func buildWrongTypeBody(original map[string]any) map[string]any {
	out := make(map[string]any, len(original))
	for k, v := range original {
		switch v.(type) {
		case string:
			out[k] = 12345
		case float64:
			out[k] = "not-a-number"
		case bool:
			out[k] = "not-a-bool"
		case []any:
			out[k] = "not-an-array"
		case map[string]any:
			out[k] = "not-an-object"
		default:
			out[k] = v
		}
	}
	return out
}

func buildDeeplyNested(depth int) map[string]any {
	if depth <= 0 {
		return map[string]any{"value": "deep"}
	}
	return map[string]any{"nested": buildDeeplyNested(depth - 1)}
}

func makeProbe(key, suffix string, tags []string, base scenarioDomain.ScenarioCase, bodyJSON json.RawMessage) scenarioDomain.ScenarioCase {
	return scenarioDomain.ScenarioCase{
		CaseID: shared.NewID(), Name: fmt.Sprintf("%s [%s]", key, suffix),
		Enabled: true, Tags: tags,
		Request: scenarioDomain.CaseRequest{
			Method: base.Request.Method, Path: base.Request.Path,
			PathParams: base.Request.PathParams, Query: base.Request.Query,
			Headers: base.Request.Headers, BodyJSON: bodyJSON,
		},
	}
}
