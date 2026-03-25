package target

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"

	"toollab-core/internal/target/usecases/domain"
)

type SQLite struct{ db *sql.DB }

func NewSQLite(db *sql.DB) *SQLite { return &SQLite{db: db} }

func (r *SQLite) Insert(t domain.Target) error {
	hint, _ := json.Marshal(t.RuntimeHint)
	_, err := r.db.Exec(
		`INSERT INTO targets (id,name,source_type,source_value,runtime_hint,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Source.Type, t.Source.Value, string(hint),
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
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

func (r *SQLite) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM artifacts WHERE run_id IN (SELECT id FROM runs WHERE target_id=?)`, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM runs WHERE target_id=?`, id)
	if err != nil {
		return err
	}
	// the target itself
	res, err := tx.Exec(`DELETE FROM targets WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domainerr.NotFound("not found")
	}
	return tx.Commit()
}

func scanTarget(row *sql.Row) (domain.Target, error) {
	var t domain.Target
	var hint, ca, ua string
	err := row.Scan(&t.ID, &t.Name, &t.Source.Type, &t.Source.Value, &hint, &ca, &ua)
	if err == sql.ErrNoRows {
		return t, domainerr.NotFound("not found")
	}
	if err != nil {
		return t, err
	}
	_ = json.Unmarshal([]byte(hint), &t.RuntimeHint)
	t.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
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
	t.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return t, nil
}
