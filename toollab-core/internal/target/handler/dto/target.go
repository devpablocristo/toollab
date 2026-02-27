package dto

import "toollab-core/internal/target/usecases/domain"

type CreateRequest struct {
	Name        string             `json:"name"`
	Source      domain.Source      `json:"source"`
	RuntimeHint domain.RuntimeHint `json:"runtime_hint"`
}
