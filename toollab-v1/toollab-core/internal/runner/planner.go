package runner

import (
	"fmt"

	"toollab-core/internal/determinism"
	"toollab-core/internal/scenario"
)

const closedLoopTickMS = 1000

func BuildPlan(s *scenario.Scenario, runDecider *determinism.Engine) (*Plan, error) {
	if len(s.Workload.Requests) == 0 {
		return nil, fmt.Errorf("workload.requests cannot be empty")
	}
	if runDecider == nil {
		eng, err := determinism.NewEngine(s.Seeds.RunSeed, "run_seed", nil)
		if err != nil {
			return nil, fmt.Errorf("init run decider: %w", err)
		}
		runDecider = eng
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
		idx := pickRequestIndexWeighted(s.Workload.Requests, runDecider, int64(seq))
		plan.PlannedRequests = append(plan.PlannedRequests, PlannedRequest{
			Seq:        int64(seq),
			RequestIdx: idx,
		})
	}
	return plan, nil
}

func pickRequestIndexWeighted(requests []scenario.RequestSpec, runDecider *determinism.Engine, seq int64) int {
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

	r := runDecider.Uint64("workload_pick", seq, "request_pick", 0) % uint64(totalWeight)
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
