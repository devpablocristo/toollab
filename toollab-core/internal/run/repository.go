package run

import (
	"database/sql"
	"time"

	"github.com/devpablocristo/core/backend/go/domainerr"

	"toollab-core/internal/run/usecases/domain"
)

type SQLite struct{ db *sql.DB }

func NewSQLite(db *sql.DB) *SQLite { return &SQLite{db: db} }

func (r *SQLite) Insert(run domain.Run) error {
	var ca *string
	if run.CompletedAt != nil {
		s := run.CompletedAt.Format(time.RFC3339)
		ca = &s
	}
	_, err := r.db.Exec(
		`INSERT INTO runs (id,target_id,status,seed,notes,created_at,completed_at) VALUES (?,?,?,?,?,?,?)`,
		run.ID, run.TargetID, run.Status, run.Seed, run.Notes,
		run.CreatedAt.Format(time.RFC3339), ca,
	)
	return err
}

func (r *SQLite) GetByID(id string) (domain.Run, error) {
	row := r.db.QueryRow(
		`SELECT id,target_id,status,seed,notes,created_at,completed_at FROM runs WHERE id=?`, id)
	return scanRun(row)
}

func (r *SQLite) ListByTarget(targetID string) ([]domain.Run, error) {
	rows, err := r.db.Query(
		`SELECT id,target_id,status,seed,notes,created_at,completed_at FROM runs WHERE target_id=? ORDER BY created_at DESC`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Run
	for rows.Next() {
		run, err := scanRunRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *SQLite) UpdateStatus(id string, status domain.Status) error {
	res, err := r.db.Exec(`UPDATE runs SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domainerr.NotFound("not found")
	}
	return nil
}

func (r *SQLite) UpdateStatusCompleted(id string, status domain.Status, completedAt time.Time) error {
	res, err := r.db.Exec(
		`UPDATE runs SET status=?, completed_at=? WHERE id=?`,
		status, completedAt.Format(time.RFC3339), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domainerr.NotFound("not found")
	}
	return nil
}

func scanRun(row *sql.Row) (domain.Run, error) {
	var run domain.Run
	var ca string
	var completedAt *string
	err := row.Scan(&run.ID, &run.TargetID, &run.Status, &run.Seed, &run.Notes, &ca, &completedAt)
	if err == sql.ErrNoRows {
		return run, domainerr.NotFound("not found")
	}
	if err != nil {
		return run, err
	}
	run.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if completedAt != nil {
		t, _ := time.Parse(time.RFC3339, *completedAt)
		run.CompletedAt = &t
	}
	return run, nil
}

func scanRunRow(rows *sql.Rows) (domain.Run, error) {
	var run domain.Run
	var ca string
	var completedAt *string
	err := rows.Scan(&run.ID, &run.TargetID, &run.Status, &run.Seed, &run.Notes, &ca, &completedAt)
	if err != nil {
		return run, err
	}
	run.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if completedAt != nil {
		t, _ := time.Parse(time.RFC3339, *completedAt)
		run.CompletedAt = &t
	}
	return run, nil
}
