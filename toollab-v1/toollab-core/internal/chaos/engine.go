package chaos

import (
	"encoding/json"
	"sort"
	"strconv"

	"toollab-core/internal/determinism"
	"toollab-core/internal/scenario"
	"toollab-core/pkg/utils"
)

type Engine struct {
	cfg     scenario.Chaos
	decider *determinism.Engine
}

type Decision struct {
	Abort          bool
	LatencyMS      int
	ErrorInjected  bool
	DriftApplied   bool
	DriftMutations []string
}

func NewEngine(cfg scenario.Chaos, decider *determinism.Engine) *Engine {
	return &Engine{cfg: cfg, decider: decider}
}

func (e *Engine) Apply(req scenario.RequestSpec, seq int64, body []byte) (Decision, []byte) {
	decision := Decision{DriftMutations: []string{}}

	decision.LatencyMS = e.pickLatency(seq)
	isDown := e.flappingDown(seq)
	injectError := isDown || e.pickRate(seq, "chaos_error", "error_rate", e.cfg.ErrorRate)
	decision.ErrorInjected = injectError
	decision.Abort = injectError

	mutatedBody := body
	if e.cfg.PayloadDrift != nil && e.cfg.PayloadDrift.Enabled && len(req.JSONBody) > 0 {
		shouldDrift := e.pickRate(seq, "chaos_drift", "drift_rate", e.cfg.PayloadDrift.Rate)
		if shouldDrift {
			mutated, mutation, ok := applyDrift(req, seq, body, e.cfg.PayloadDrift.AllowedMutations, e.decider)
			if ok {
				mutatedBody = mutated
				decision.DriftApplied = true
				decision.DriftMutations = append(decision.DriftMutations, mutation)
			}
		}
	}

	return decision, mutatedBody
}

func (e *Engine) pickLatency(seq int64) int {
	if e.cfg.Latency.Mode == "none" {
		return 0
	}
	if e.cfg.Latency.Mode == "fixed" {
		return e.cfg.Latency.MS
	}
	if e.cfg.Latency.Mode == "uniform" {
		minMS := e.cfg.Latency.MinMS
		maxMS := e.cfg.Latency.MaxMS
		if maxMS < minMS {
			maxMS = minMS
		}
		rangeSize := (maxMS - minMS) + 1
		if rangeSize <= 1 || e.decider == nil {
			return minMS
		}
		offset := e.decider.IntN(rangeSize, "chaos", seq, "latency_uniform", 0, strconv.Itoa(minMS), strconv.Itoa(maxMS))
		return minMS + offset
	}
	return 0
}

func (e *Engine) flappingDown(seq int64) bool {
	if e.cfg.Flapping == nil || !e.cfg.Flapping.Enabled {
		return false
	}
	period := e.cfg.Flapping.PeriodRequest
	if period <= 0 {
		return false
	}
	down := int(float64(period) * e.cfg.Flapping.DownRatio)
	if down <= 0 {
		return false
	}
	position := int(seq % int64(period))
	return position < down
}

func (e *Engine) pickRate(seq int64, stream, decisionType string, rate float64) bool {
	if rate <= 0 || e.decider == nil {
		return false
	}
	if rate >= 1 {
		return true
	}
	v := e.decider.Float64(stream, seq, decisionType, 0)
	return v < rate
}

func applyDrift(req scenario.RequestSpec, seq int64, body []byte, allowed []string, decider *determinism.Engine) ([]byte, string, bool) {
	if len(allowed) == 0 {
		return body, "", false
	}
	idx := 0
	if decider != nil {
		idx = decider.IntN(len(allowed), "chaos", seq, "drift_mutation", 0)
	}
	mutation := allowed[idx]

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return body, "", false
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return body, "", false
	}

	switch mutation {
	case "json_set":
		obj["toollab_drift_"+strconv.FormatInt(seq, 10)] = "1"
	case "json_remove":
		if len(obj) == 0 {
			return body, "", false
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		delete(obj, keys[0])
	case "json_swap":
		if len(obj) < 2 {
			return body, "", false
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		obj[keys[0]], obj[keys[1]] = obj[keys[1]], obj[keys[0]]
	default:
		return body, "", false
	}

	canonical, err := utils.CanonicalJSON(obj)
	if err != nil {
		return body, "", false
	}
	return canonical, mutation, true
}
