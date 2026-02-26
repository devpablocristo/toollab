package runs

import (
	"database/sql"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) List(targetID string, limit int) ([]Run, error) {
	query := "SELECT id, scenario_id, target_id, run_seed, chaos_seed, verdict, total_requests, completed_requests, success_rate, error_rate, p50_ms, p95_ms, p99_ms, duration_s, concurrency, status_histogram, deterministic_fingerprint, golden_run_dir, started_at, finished_at, created_at FROM runs"
	var args []any

	if targetID != "" {
		query += " WHERE target_id = ?"
		args = append(args, targetID)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Run
	for rows.Next() {
		var r Run
		var scenarioID sql.NullString
		if err := rows.Scan(&r.ID, &scenarioID, &r.TargetID, &r.RunSeed, &r.ChaosSeed, &r.Verdict,
			&r.TotalRequests, &r.CompletedRequests, &r.SuccessRate, &r.ErrorRate,
			&r.P50MS, &r.P95MS, &r.P99MS, &r.DurationS, &r.Concurrency,
			&r.StatusHistogram, &r.DeterministicFingerprint, &r.GoldenRunDir,
			&r.StartedAt, &r.FinishedAt, &r.CreatedAt); err != nil {
			return nil, err
		}
		if scenarioID.Valid {
			r.ScenarioID = &scenarioID.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) GetByID(id string) (*Run, error) {
	var r Run
	var scenarioID sql.NullString
	err := s.db.QueryRow(
		"SELECT id, scenario_id, target_id, run_seed, chaos_seed, verdict, total_requests, completed_requests, success_rate, error_rate, p50_ms, p95_ms, p99_ms, duration_s, concurrency, status_histogram, deterministic_fingerprint, golden_run_dir, started_at, finished_at, created_at FROM runs WHERE id = ?", id,
	).Scan(&r.ID, &scenarioID, &r.TargetID, &r.RunSeed, &r.ChaosSeed, &r.Verdict,
		&r.TotalRequests, &r.CompletedRequests, &r.SuccessRate, &r.ErrorRate,
		&r.P50MS, &r.P95MS, &r.P99MS, &r.DurationS, &r.Concurrency,
		&r.StatusHistogram, &r.DeterministicFingerprint, &r.GoldenRunDir,
		&r.StartedAt, &r.FinishedAt, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	if scenarioID.Valid {
		r.ScenarioID = &scenarioID.String
	}
	return &r, nil
}

func (s *Store) Insert(r *Run) error {
	_, err := s.db.Exec(
		`INSERT INTO runs (id, scenario_id, target_id, run_seed, chaos_seed, verdict,
			total_requests, completed_requests, success_rate, error_rate,
			p50_ms, p95_ms, p99_ms, duration_s, concurrency,
			status_histogram, deterministic_fingerprint, golden_run_dir,
			started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ScenarioID, r.TargetID, r.RunSeed, r.ChaosSeed, r.Verdict,
		r.TotalRequests, r.CompletedRequests, r.SuccessRate, r.ErrorRate,
		r.P50MS, r.P95MS, r.P99MS, r.DurationS, r.Concurrency,
		r.StatusHistogram, r.DeterministicFingerprint, r.GoldenRunDir,
		r.StartedAt, r.FinishedAt, r.CreatedAt,
	)
	return err
}

func (s *Store) InsertAssertion(a *AssertionResult) error {
	_, err := s.db.Exec(
		"INSERT INTO assertion_results (run_id, rule_id, rule_type, passed, observed, expected, message) VALUES (?, ?, ?, ?, ?, ?, ?)",
		a.RunID, a.RuleID, a.RuleType, a.Passed, a.Observed, a.Expected, a.Message,
	)
	return err
}

func (s *Store) GetAssertions(runID string) ([]AssertionResult, error) {
	rows, err := s.db.Query("SELECT id, run_id, rule_id, rule_type, passed, observed, expected, message FROM assertion_results WHERE run_id = ?", runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AssertionResult
	for rows.Next() {
		var a AssertionResult
		if err := rows.Scan(&a.ID, &a.RunID, &a.RuleID, &a.RuleType, &a.Passed, &a.Observed, &a.Expected, &a.Message); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *Store) Delete(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM assertion_results WHERE run_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM interpretations WHERE run_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM runs WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
