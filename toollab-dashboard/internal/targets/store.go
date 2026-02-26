package targets

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) List() ([]Target, error) {
	rows, err := s.db.Query("SELECT id, name, base_url, description, created_at, updated_at FROM targets ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Target
	for rows.Next() {
		var t Target
		if err := rows.Scan(&t.ID, &t.Name, &t.BaseURL, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) GetByID(id string) (*Target, error) {
	var t Target
	err := s.db.QueryRow("SELECT id, name, base_url, description, created_at, updated_at FROM targets WHERE id = ?", id).
		Scan(&t.ID, &t.Name, &t.BaseURL, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) Create(req CreateRequest) (*Target, error) {
	t := Target{
		ID:          uuid.NewString(),
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		Description: req.Description,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	_, err := s.db.Exec(
		"INSERT INTO targets (id, name, base_url, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		t.ID, t.Name, t.BaseURL, t.Description, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM targets WHERE id = ?", id)
	return err
}
