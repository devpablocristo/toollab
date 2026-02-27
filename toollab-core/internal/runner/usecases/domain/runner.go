package domain

import (
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
)

type Options struct {
	TimeoutMs    int      `json:"timeout_ms"`
	MaxBodyBytes int64    `json:"max_body_bytes"`
	SubsetIDs    []string `json:"subset_case_ids,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type Runner interface {
	Run(baseURL string, plan scenarioDomain.ScenarioPlan, opts Options) evidenceDomain.ExecutionResult
}
