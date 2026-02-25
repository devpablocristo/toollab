package gen

import "fmt"

type latencyYAML struct {
	Mode  string `yaml:"mode"`
	MinMS int    `yaml:"min_ms,omitempty"`
	MaxMS int    `yaml:"max_ms,omitempty"`
	MS    int    `yaml:"ms,omitempty"`
}

type flappingYAML struct {
	Enabled        bool    `yaml:"enabled"`
	PeriodRequests int     `yaml:"period_requests"`
	DownRatio      float64 `yaml:"down_ratio"`
	Behavior       string  `yaml:"behavior"`
}

type payloadDriftYAML struct {
	Enabled          bool     `yaml:"enabled"`
	Rate             float64  `yaml:"rate"`
	AllowedMutations []string `yaml:"allowed_mutations"`
}

type chaosYAML struct {
	Latency       latencyYAML       `yaml:"latency"`
	ErrorRate     float64           `yaml:"error_rate"`
	ErrorStatuses []int             `yaml:"error_statuses"`
	ErrorMode     string            `yaml:"error_mode"`
	Flapping      *flappingYAML     `yaml:"flapping,omitempty"`
	PayloadDrift  *payloadDriftYAML `yaml:"payload_drift,omitempty"`
}

type expectationsYAML struct {
	MaxErrorRate float64         `yaml:"max_error_rate"`
	MaxP95MS     int             `yaml:"max_p95_ms"`
	Invariants   []invariantYAML `yaml:"invariants"`
}

type invariantYAML struct {
	Type string  `yaml:"type"`
	Max  float64 `yaml:"max,omitempty"`
}

func ChaosProfile(name string) (chaosYAML, expectationsYAML, error) {
	base := chaosYAML{
		ErrorStatuses: []int{503},
		ErrorMode:     "abort",
	}
	switch name {
	case "none":
		base.Latency = latencyYAML{Mode: "none"}
		base.ErrorRate = 0
		return base, expectationsYAML{
			MaxErrorRate: 0.05,
			MaxP95MS:     200,
			Invariants:   defaultInvariants(),
		}, nil

	case "light":
		base.Latency = latencyYAML{Mode: "uniform", MinMS: 5, MaxMS: 30}
		base.ErrorRate = 0.02
		return base, expectationsYAML{
			MaxErrorRate: 0.10,
			MaxP95MS:     500,
			Invariants:   defaultInvariants(),
		}, nil

	case "moderate":
		base.Latency = latencyYAML{Mode: "uniform", MinMS: 10, MaxMS: 100}
		base.ErrorRate = 0.05
		base.Flapping = &flappingYAML{
			Enabled:        true,
			PeriodRequests: 20,
			DownRatio:      0.2,
			Behavior:       "abort",
		}
		return base, expectationsYAML{
			MaxErrorRate: 0.20,
			MaxP95MS:     800,
			Invariants:   defaultInvariants(),
		}, nil

	case "aggressive":
		base.Latency = latencyYAML{Mode: "uniform", MinMS: 50, MaxMS: 300}
		base.ErrorRate = 0.15
		base.Flapping = &flappingYAML{
			Enabled:        true,
			PeriodRequests: 20,
			DownRatio:      0.3,
			Behavior:       "abort",
		}
		base.PayloadDrift = &payloadDriftYAML{
			Enabled:          true,
			Rate:             0.1,
			AllowedMutations: []string{"json_set", "json_remove", "json_swap"},
		}
		return base, expectationsYAML{
			MaxErrorRate: 0.30,
			MaxP95MS:     1500,
			Invariants:   defaultInvariants(),
		}, nil

	default:
		return chaosYAML{}, expectationsYAML{}, fmt.Errorf("unknown chaos profile %q (valid: none, light, moderate, aggressive)", name)
	}
}

func defaultInvariants() []invariantYAML {
	return []invariantYAML{
		{Type: "no_5xx_allowed"},
		{Type: "max_4xx_rate", Max: 0.1},
	}
}
