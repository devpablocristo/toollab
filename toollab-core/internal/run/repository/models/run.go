package models

type RunRow struct {
	ID          string
	TargetID    string
	Status      string
	Seed        string
	Notes       string
	CreatedAt   string
	CompletedAt *string
}
