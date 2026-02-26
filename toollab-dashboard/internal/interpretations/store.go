package interpretations

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Interpretation struct {
	ID         string    `json:"id"`
	RunID      string    `json:"run_id"`
	Model      string    `json:"model"`
	Narrative  string    `json:"narrative"`
	InputSHA256 string   `json:"input_sha256,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) GetByRunID(runID string) (*Interpretation, error) {
	var i Interpretation
	err := s.db.QueryRow(
		"SELECT id, run_id, model, narrative, input_sha256, created_at FROM interpretations WHERE run_id = ? ORDER BY created_at DESC LIMIT 1", runID,
	).Scan(&i.ID, &i.RunID, &i.Model, &i.Narrative, &i.InputSHA256, &i.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *Store) Insert(runID, model, narrative, inputSHA string) (*Interpretation, error) {
	i := Interpretation{
		ID:          uuid.NewString(),
		RunID:       runID,
		Model:       model,
		Narrative:   narrative,
		InputSHA256: inputSHA,
		CreatedAt:   time.Now().UTC(),
	}
	_, err := s.db.Exec(
		"INSERT INTO interpretations (id, run_id, model, narrative, input_sha256, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		i.ID, i.RunID, i.Model, i.Narrative, i.InputSHA256, i.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}
