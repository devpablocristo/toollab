package runner

import "time"

type PlannedRequest struct {
	Seq        int64
	RequestIdx int
}

type Plan struct {
	ScheduleMode    string
	TickMS          int
	Concurrency     int
	DurationS       int
	PlannedRequests []PlannedRequest
}

type ChaosApplied struct {
	LatencyInjectedMS int
	ErrorInjected     bool
	ErrorMode         string
	PayloadDrift      bool
	PayloadMutations  []string
}

type Outcome struct {
	Seq             int64
	RequestID       string
	Method          string
	Path            string
	StatusCode      *int
	ErrorKind       string
	LatencyMS       int
	ResponseHash    string
	RequestURL      string
	RequestHeaders  map[string]string
	RequestBody     []byte
	ResponseHeaders map[string]string
	ResponseBody    []byte
	IdempotencyKey  string
	ChaosApplied    ChaosApplied
	OccurredAt      time.Time
}
