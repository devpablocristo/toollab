package runner

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"

	"toolab-core/internal/scenario"
)

const closedLoopTickMS = 1000

func BuildPlan(s *scenario.Scenario) (*Plan, error) {
	if len(s.Workload.Requests) == 0 {
		return nil, fmt.Errorf("workload.requests cannot be empty")
	}

	var total int
	switch s.Workload.ScheduleMode {
	case "open_loop":
		total = (s.Workload.DurationS * 1000) / s.Workload.TickMS
	case "closed_loop":
		turns := (s.Workload.DurationS * 1000) / closedLoopTickMS
		if turns == 0 {
			turns = 1
		}
		total = turns * s.Workload.Concurrency
	default:
		return nil, fmt.Errorf("unsupported schedule_mode %q", s.Workload.ScheduleMode)
	}
	if total < 0 {
		total = 0
	}

	plan := &Plan{
		ScheduleMode: s.Workload.ScheduleMode,
		TickMS:       s.Workload.TickMS,
		Concurrency:  s.Workload.Concurrency,
		DurationS:    s.Workload.DurationS,
	}
	plan.PlannedRequests = make([]PlannedRequest, 0, total)
	for seq := 0; seq < total; seq++ {
		idx := pickRequestIndexWeighted(s.Workload.Requests, s.Seeds.RunSeed, int64(seq))
		plan.PlannedRequests = append(plan.PlannedRequests, PlannedRequest{
			Seq:        int64(seq),
			RequestIdx: idx,
		})
	}
	return plan, nil
}

func pickRequestIndexWeighted(requests []scenario.RequestSpec, runSeed string, seq int64) int {
	totalWeight := 0
	for _, req := range requests {
		weight := req.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}
	if totalWeight <= 1 {
		return 0
	}

	r := deterministicUint64(runSeed, seq, "workload_pick") % uint64(totalWeight)
	acc := 0
	for i, req := range requests {
		weight := req.Weight
		if weight <= 0 {
			weight = 1
		}
		acc += weight
		if int(r) < acc {
			return i
		}
	}
	return len(requests) - 1
}

func deterministicUint64(seed string, seq int64, purpose string) uint64 {
	src := seed + ":" + strconv.FormatInt(seq, 10) + ":" + purpose
	sum := sha256.Sum256([]byte(src))
	return binary.BigEndian.Uint64(sum[:8])
}
