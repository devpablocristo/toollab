package evidence

import "testing"

func TestNearestRankPercentilesDeterministic(t *testing.T) {
	outcomes := []Outcome{
		{LatencyMS: 10, ErrorKind: "none"},
		{LatencyMS: 20, ErrorKind: "none"},
		{LatencyMS: 30, ErrorKind: "none"},
		{LatencyMS: 40, ErrorKind: "none"},
		{LatencyMS: 50, ErrorKind: "none"},
	}
	stats := BuildStats(outcomes)
	if stats.P50MS != 30 {
		t.Fatalf("expected p50=30 got %d", stats.P50MS)
	}
	if stats.P95MS != 50 {
		t.Fatalf("expected p95=50 got %d", stats.P95MS)
	}
	if stats.P99MS != 50 {
		t.Fatalf("expected p99=50 got %d", stats.P99MS)
	}
}
