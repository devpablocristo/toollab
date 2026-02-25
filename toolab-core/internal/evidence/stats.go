package evidence

import (
	"math"
	"sort"
	"strconv"
)

func BuildStats(outcomes []Outcome) Stats {
	total := len(outcomes)
	statusHist := map[string]int{}
	lat := make([]int, 0, total)
	success := 0
	for _, out := range outcomes {
		lat = append(lat, out.LatencyMS)
		if out.StatusCode != nil {
			statusHist[strconv.Itoa(*out.StatusCode)]++
		}
		if out.ErrorKind == "none" {
			success++
		}
	}
	sort.Ints(lat)

	successRate := 0.0
	if total > 0 {
		successRate = float64(success) / float64(total)
	}
	errorRate := 1 - successRate
	if total == 0 {
		errorRate = 0
	}

	return Stats{
		TotalRequests:   total,
		SuccessRate:     successRate,
		ErrorRate:       errorRate,
		P50MS:           nearestRank(lat, 50),
		P95MS:           nearestRank(lat, 95),
		P99MS:           nearestRank(lat, 99),
		StatusHistogram: statusHist,
	}
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
