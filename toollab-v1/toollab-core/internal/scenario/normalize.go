package scenario

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	defaultTimeoutMS        = 5000
	defaultWeight           = 1
	defaultRedactionMask    = "***REDACTED***"
	defaultPreviewBytes     = 4096
	defaultMaxSamples       = 50
	defaultMetricsTimeoutMS = 2000
	defaultTracesTimeoutMS  = 2000
	defaultLogsFormat       = "jsonl"
	defaultLogsMaxLines     = 500
)

var defaultSensitiveHeaders = []string{"authorization", "cookie", "set-cookie", "x-api-key"}

func normalizeScenario(raw rawScenario) (*Scenario, error) {
	runSeed, err := normalizeSeed(raw.Seeds.RunSeed)
	if err != nil {
		return nil, fmt.Errorf("normalize run_seed: %w", err)
	}
	chaosSeed, err := normalizeSeed(raw.Seeds.ChaosSeed)
	if err != nil {
		return nil, fmt.Errorf("normalize chaos_seed: %w", err)
	}

	s := &Scenario{
		Version:      raw.Version,
		Mode:         raw.Mode,
		Target:       raw.Target,
		Workload:     raw.Workload,
		Chaos:        raw.Chaos,
		Expectations: raw.Expectations,
		Seeds: Seeds{
			RunSeed:         runSeed,
			ChaosSeed:       chaosSeed,
			DBSeedReference: raw.Seeds.DBSeedReference,
		},
		Observability: raw.Observability,
		Redaction:     raw.Redaction,
	}

	if s.Target.Headers == nil {
		s.Target.Headers = map[string]string{}
	}
	for i := range s.Workload.Requests {
		r := &s.Workload.Requests[i]
		if r.Query == nil {
			r.Query = map[string]string{}
		}
		if r.Headers == nil {
			r.Headers = map[string]string{}
		}
		if r.TimeoutMS == 0 {
			r.TimeoutMS = defaultTimeoutMS
		}
		if r.Weight == 0 {
			r.Weight = defaultWeight
		}
	}

	if len(s.Redaction.Headers) == 0 {
		s.Redaction.Headers = append([]string(nil), defaultSensitiveHeaders...)
	}
	if s.Redaction.JSONPaths == nil {
		s.Redaction.JSONPaths = []string{}
	}
	if s.Redaction.Mask == "" {
		s.Redaction.Mask = defaultRedactionMask
	}
	if s.Redaction.MaxBodyPreviewBytes == 0 {
		s.Redaction.MaxBodyPreviewBytes = defaultPreviewBytes
	}
	if s.Redaction.MaxSamples == 0 {
		s.Redaction.MaxSamples = defaultMaxSamples
	}

	if s.Observability != nil {
		if s.Observability.Metrics != nil && s.Observability.Metrics.Timeout == 0 {
			s.Observability.Metrics.Timeout = defaultMetricsTimeoutMS
		}
		if s.Observability.Traces != nil && s.Observability.Traces.Timeout == 0 {
			s.Observability.Traces.Timeout = defaultTracesTimeoutMS
		}
		if s.Observability.Logs != nil {
			if s.Observability.Logs.Format == "" {
				s.Observability.Logs.Format = defaultLogsFormat
			}
			if s.Observability.Logs.MaxLines == 0 {
				s.Observability.Logs.MaxLines = defaultLogsMaxLines
			}
		}
	}

	return s, nil
}

func normalizeSeed(seed any) (string, error) {
	switch v := seed.(type) {
	case int:
		if v < 0 {
			return "", fmt.Errorf("seed must be >= 0")
		}
		return strconv.Itoa(v), nil
	case int64:
		if v < 0 {
			return "", fmt.Errorf("seed must be >= 0")
		}
		return strconv.FormatInt(v, 10), nil
	case float64:
		if v < 0 {
			return "", fmt.Errorf("seed must be >= 0")
		}
		return strconv.FormatInt(int64(v), 10), nil
	case string:
		if v == "" {
			return "", fmt.Errorf("seed cannot be empty")
		}
		if _, err := strconv.ParseUint(v, 10, 64); err != nil {
			return "", fmt.Errorf("seed must be decimal string: %w", err)
		}
		return v, nil
	case json.Number:
		if _, err := v.Int64(); err != nil {
			return "", fmt.Errorf("invalid numeric seed: %w", err)
		}
		return v.String(), nil
	default:
		return "", fmt.Errorf("unsupported seed type %T", seed)
	}
}
