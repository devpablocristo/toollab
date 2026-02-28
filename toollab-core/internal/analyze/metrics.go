package analyze

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"toollab-core/internal/analyze/domain"
	discoveryDomain "toollab-core/internal/discovery/usecases/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
)

func buildPerformanceMetrics(pack *evidenceDomain.EvidencePack) domain.PerformanceMetrics {
	total := len(pack.Items)
	latencies := make([]int, 0, total)
	statusHist := map[string]int{}
	success := 0

	var timings []domain.EndpointTiming

	for _, item := range pack.Items {
		latencies = append(latencies, int(item.TimingMs))
		if item.Response != nil {
			statusHist[strconv.Itoa(item.Response.Status)]++
			if item.Response.Status >= 200 && item.Response.Status < 400 {
				success++
			}
			timings = append(timings, domain.EndpointTiming{
				Method:   item.Request.Method,
				Path:     extractPath(item.Request.URL),
				TimingMs: item.TimingMs,
				Status:   item.Response.Status,
			})
		}
	}

	sort.Ints(latencies)

	successRate := 0.0
	errorRate := 0.0
	if total > 0 {
		successRate = float64(success) / float64(total)
		errorRate = 1 - successRate
	}

	sort.Slice(timings, func(i, j int) bool {
		return timings[i].TimingMs > timings[j].TimingMs
	})
	slowest := timings
	if len(slowest) > 5 {
		slowest = slowest[:5]
	}

	return domain.PerformanceMetrics{
		TotalRequests:    total,
		SuccessRate:      successRate,
		ErrorRate:        errorRate,
		P50Ms:            nearestRank(latencies, 50),
		P95Ms:            nearestRank(latencies, 95),
		P99Ms:            nearestRank(latencies, 99),
		StatusHistogram:  statusHist,
		SlowestEndpoints: slowest,
	}
}

func buildCoverageReport(model *discoveryDomain.ServiceModel, pack *evidenceDomain.EvidencePack) domain.CoverageReport {
	declared := map[string]*struct {
		method string
		path   string
		tested bool
	}{}
	var order []string

	if model != nil {
		for _, ep := range model.Endpoints {
			key := ep.Method + " " + ep.Path
			if _, exists := declared[key]; !exists {
				declared[key] = &struct {
					method string
					path   string
					tested bool
				}{ep.Method, ep.Path, false}
				order = append(order, key)
			}
		}
	}

	for _, item := range pack.Items {
		path := extractPath(item.Request.URL)
		key := item.Request.Method + " " + path
		ep, exists := declared[key]
		if !exists {
			ep = &struct {
				method string
				path   string
				tested bool
			}{item.Request.Method, path, false}
			declared[key] = ep
			order = append(order, key)
		}
		ep.tested = true
	}

	sort.Strings(order)

	totalEndpoints := len(declared)
	testedCount := 0
	var untested []domain.EndpointRef

	for _, key := range order {
		ep := declared[key]
		if ep.tested {
			testedCount++
		} else {
			untested = append(untested, domain.EndpointRef{Method: ep.method, Path: ep.path})
		}
	}

	coverageRate := 0.0
	if totalEndpoints > 0 {
		coverageRate = float64(testedCount) / float64(totalEndpoints)
	}

	byMethod := buildMethodCoverage(declared)
	statusCodes := buildStatusCodeObs(pack)

	return domain.CoverageReport{
		TotalEndpoints:  totalEndpoints,
		TestedEndpoints: testedCount,
		CoverageRate:    coverageRate,
		ByMethod:        byMethod,
		StatusCodes:     statusCodes,
		Untested:        untested,
	}
}

func buildMethodCoverage(declared map[string]*struct {
	method string
	path   string
	tested bool
}) []domain.MethodCoverage {
	methods := map[string]*domain.MethodCoverage{}
	var methodOrder []string
	for _, ep := range declared {
		m := strings.ToUpper(ep.method)
		mc, exists := methods[m]
		if !exists {
			mc = &domain.MethodCoverage{Method: m}
			methods[m] = mc
			methodOrder = append(methodOrder, m)
		}
		mc.Total++
		if ep.tested {
			mc.Tested++
		}
	}
	sort.Strings(methodOrder)
	result := make([]domain.MethodCoverage, 0, len(methods))
	for _, m := range methodOrder {
		mc := methods[m]
		if mc.Total > 0 {
			mc.Rate = float64(mc.Tested) / float64(mc.Total)
		}
		result = append(result, *mc)
	}
	return result
}

func buildStatusCodeObs(pack *evidenceDomain.EvidencePack) []domain.StatusCodeObs {
	observed := map[int]bool{}
	for _, item := range pack.Items {
		if item.Response != nil {
			observed[item.Response.Status] = true
		}
	}
	commonCodes := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 405, 409, 422, 429, 500, 502, 503}
	result := make([]domain.StatusCodeObs, 0, len(commonCodes))
	for _, code := range commonCodes {
		result = append(result, domain.StatusCodeObs{Code: code, Observed: observed[code]})
	}
	return result
}

func calcOverallScore(security domain.SecurityReport, contract domain.ContractReport, coverage domain.CoverageReport, perf domain.PerformanceMetrics) int {
	secWeight := 0.40
	conWeight := 0.25
	covWeight := 0.20
	perfWeight := 0.15

	secScore := float64(security.Score)

	conScore := contract.ComplianceRate * 100

	covScore := coverage.CoverageRate * 100

	perfScore := 100.0
	if perf.P95Ms > 2000 {
		perfScore -= 30
	} else if perf.P95Ms > 1000 {
		perfScore -= 15
	} else if perf.P95Ms > 500 {
		perfScore -= 5
	}
	if perf.ErrorRate > 0.5 {
		perfScore -= 30
	} else if perf.ErrorRate > 0.2 {
		perfScore -= 15
	}
	if perfScore < 0 {
		perfScore = 0
	}

	overall := secScore*secWeight + conScore*conWeight + covScore*covWeight + perfScore*perfWeight
	return int(math.Round(overall))
}

func nearestRank(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	rank := int(math.Ceil((float64(p) / 100) * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
