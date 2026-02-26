package coverage

import (
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/evidence"
	"toollab-core/internal/scenario"
)

type EndpointStatus struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Tested  bool   `json:"tested"`
	Hits    int    `json:"hits"`
	Success int    `json:"success"`
	Errors  int    `json:"errors"`
}

type StatusCodeCoverage struct {
	StatusCode int  `json:"status_code"`
	Observed   bool `json:"observed"`
}

type MethodCoverage struct {
	Method string  `json:"method"`
	Total  int     `json:"total"`
	Tested int     `json:"tested"`
	Rate   float64 `json:"rate"`
}

type CoverageReport struct {
	TotalEndpoints  int                  `json:"total_endpoints"`
	TestedEndpoints int                  `json:"tested_endpoints"`
	CoverageRate    float64              `json:"coverage_rate"`
	Endpoints       []EndpointStatus     `json:"endpoints"`
	Untested        []EndpointStatus     `json:"untested"`
	ByMethod        []MethodCoverage     `json:"by_method"`
	StatusCodes     []StatusCodeCoverage `json:"status_codes"`
}

func Analyze(scn *scenario.Scenario, bundle *evidence.Bundle) *CoverageReport {
	report := &CoverageReport{
		Endpoints:  []EndpointStatus{},
		Untested:   []EndpointStatus{},
		ByMethod:   []MethodCoverage{},
		StatusCodes: []StatusCodeCoverage{},
	}

	declared := map[string]*EndpointStatus{}
	var order []string
	for _, req := range scn.Workload.Requests {
		key := req.Method + " " + req.Path
		if _, exists := declared[key]; !exists {
			declared[key] = &EndpointStatus{Method: req.Method, Path: req.Path}
			order = append(order, key)
		}
	}

	for _, o := range bundle.Outcomes {
		key := o.Method + " " + o.Path
		ep, exists := declared[key]
		if !exists {
			ep = &EndpointStatus{Method: o.Method, Path: o.Path}
			declared[key] = ep
			order = append(order, key)
		}
		ep.Tested = true
		ep.Hits++
		if o.StatusCode != nil && *o.StatusCode >= 200 && *o.StatusCode < 400 {
			ep.Success++
		} else {
			ep.Errors++
		}
	}

	sort.Strings(order)
	for _, key := range order {
		ep := declared[key]
		report.Endpoints = append(report.Endpoints, *ep)
		if !ep.Tested {
			report.Untested = append(report.Untested, *ep)
		}
	}

	report.TotalEndpoints = len(declared)
	tested := 0
	for _, ep := range report.Endpoints {
		if ep.Tested {
			tested++
		}
	}
	report.TestedEndpoints = tested
	if report.TotalEndpoints > 0 {
		report.CoverageRate = float64(tested) / float64(report.TotalEndpoints)
	}

	report.ByMethod = buildMethodCoverage(report.Endpoints)
	report.StatusCodes = buildStatusCodeCoverage(bundle)
	return report
}

func buildMethodCoverage(endpoints []EndpointStatus) []MethodCoverage {
	methods := map[string]*MethodCoverage{}
	var methodOrder []string
	for _, ep := range endpoints {
		m := strings.ToUpper(ep.Method)
		mc, exists := methods[m]
		if !exists {
			mc = &MethodCoverage{Method: m}
			methods[m] = mc
			methodOrder = append(methodOrder, m)
		}
		mc.Total++
		if ep.Tested {
			mc.Tested++
		}
	}
	sort.Strings(methodOrder)
	result := make([]MethodCoverage, 0, len(methods))
	for _, m := range methodOrder {
		mc := methods[m]
		if mc.Total > 0 {
			mc.Rate = float64(mc.Tested) / float64(mc.Total)
		}
		result = append(result, *mc)
	}
	return result
}

func buildStatusCodeCoverage(bundle *evidence.Bundle) []StatusCodeCoverage {
	observed := map[int]bool{}
	for _, o := range bundle.Outcomes {
		if o.StatusCode != nil {
			observed[*o.StatusCode] = true
		}
	}

	commonCodes := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 405, 409, 422, 429, 500, 502, 503}
	result := make([]StatusCodeCoverage, 0, len(commonCodes))
	for _, code := range commonCodes {
		result = append(result, StatusCodeCoverage{
			StatusCode: code,
			Observed:   observed[code],
		})
	}
	return result
}

func RenderMarkdown(report *CoverageReport) string {
	var sb strings.Builder

	sb.WriteString("# Reporte de Cobertura\n\n")
	sb.WriteString(fmt.Sprintf("**Cobertura total:** %.1f%% (%d/%d endpoints)\n\n",
		report.CoverageRate*100, report.TestedEndpoints, report.TotalEndpoints))

	sb.WriteString("## Endpoints Testeados\n\n")
	sb.WriteString("| Método | Path | Hits | Éxitos | Errores |\n")
	sb.WriteString("|--------|------|------|--------|---------|\n")
	for _, ep := range report.Endpoints {
		if ep.Tested {
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d |\n",
				ep.Method, ep.Path, ep.Hits, ep.Success, ep.Errors))
		}
	}

	if len(report.Untested) > 0 {
		sb.WriteString("\n## Endpoints Sin Testear\n\n")
		sb.WriteString("| Método | Path |\n")
		sb.WriteString("|--------|------|\n")
		for _, ep := range report.Untested {
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", ep.Method, ep.Path))
		}
	}

	sb.WriteString("\n## Cobertura por Método HTTP\n\n")
	sb.WriteString("| Método | Total | Testeados | Cobertura |\n")
	sb.WriteString("|--------|-------|-----------|-----------|\n")
	for _, mc := range report.ByMethod {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f%% |\n",
			mc.Method, mc.Total, mc.Tested, mc.Rate*100))
	}

	sb.WriteString("\n## Status Codes Observados\n\n")
	for _, sc := range report.StatusCodes {
		mark := "[ ]"
		if sc.Observed {
			mark = "[x]"
		}
		sb.WriteString(fmt.Sprintf("- %s %d\n", mark, sc.StatusCode))
	}

	return sb.String()
}
