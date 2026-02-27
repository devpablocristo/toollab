package repository

import (
	"database/sql"
	"encoding/json"

	"toollab-core/internal/shared"
	"toollab-core/internal/target/usecases/domain"
)

type SQLite struct{ db *sql.DB }

func NewSQLite(db *sql.DB) *SQLite { return &SQLite{db: db} }

func (r *SQLite) Insert(t domain.Target) error {
	hint, _ := json.Marshal(t.RuntimeHint)
	_, err := r.db.Exec(
		`INSERT INTO targets (id,name,source_type,source_value,runtime_hint,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Source.Type, t.Source.Value, string(hint),
		t.CreatedAt.Format(shared.TimeFormat), t.UpdatedAt.Format(shared.TimeFormat),
	)
	return err
}

func (r *SQLite) GetByID(id string) (domain.Target, error) {
	row := r.db.QueryRow(
		`SELECT id,name,source_type,source_value,runtime_hint,created_at,updated_at FROM targets WHERE id=?`, id)
	return scanTarget(row)
}

func (r *SQLite) List() ([]domain.Target, error) {
	rows, err := r.db.Query(
		`SELECT id,name,source_type,source_value,runtime_hint,created_at,updated_at FROM targets ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Target
	for rows.Next() {
		t, err := scanTargetRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanTarget(row *sql.Row) (domain.Target, error) {
	var t domain.Target
	var hint, ca, ua string
	err := row.Scan(&t.ID, &t.Name, &t.Source.Type, &t.Source.Value, &hint, &ca, &ua)
	if err == sql.ErrNoRows {
		return t, shared.ErrNotFound
	}
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(hint), &t.RuntimeHint)
	t.CreatedAt, _ = shared.ParseTime(ca)
	t.UpdatedAt, _ = shared.ParseTime(ua)
	return t, nil
}

func scanTargetRow(rows *sql.Rows) (domain.Target, error) {
	var t domain.Target
	var hint, ca, ua string
	err := rows.Scan(&t.ID, &t.Name, &t.Source.Type, &t.Source.Value, &hint, &ca, &ua)
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(hint), &t.RuntimeHint)
	t.CreatedAt, _ = shared.ParseTime(ca)
	t.UpdatedAt, _ = shared.ParseTime(ua)
	return t, nil
}
