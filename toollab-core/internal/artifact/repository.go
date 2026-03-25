package artifact

import (
	"database/sql"
	"time"

	"github.com/devpablocristo/core/errors/go/domainerr"

	"toollab-core/internal/artifact/usecases/domain"
)

type SQLite struct{ db *sql.DB }

func NewSQLite(db *sql.DB) *SQLite { return &SQLite{db: db} }

func (r *SQLite) Insert(idx domain.Index) error {
	_, err := r.db.Exec(
		`INSERT INTO artifacts (id,run_id,type,schema_version,revision,content_hash,size_bytes,storage_path,created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		idx.ID, idx.RunID, idx.Type, idx.SchemaVersion, idx.Revision,
		idx.ContentHash, idx.SizeBytes, idx.StoragePath,
		idx.CreatedAt.Format(time.RFC3339),
	)
	return err
}

func (r *SQLite) GetLatest(runID string, artType domain.ArtifactType) (domain.Index, error) {
	row := r.db.QueryRow(
		`SELECT id,run_id,type,schema_version,revision,content_hash,size_bytes,storage_path,created_at
		 FROM artifacts WHERE run_id=? AND type=? ORDER BY revision DESC LIMIT 1`, runID, artType)
	return scanIndex(row)
}

func (r *SQLite) GetByRevision(runID string, artType domain.ArtifactType, revision int) (domain.Index, error) {
	row := r.db.QueryRow(
		`SELECT id,run_id,type,schema_version,revision,content_hash,size_bytes,storage_path,created_at
		 FROM artifacts WHERE run_id=? AND type=? AND revision=?`, runID, artType, revision)
	return scanIndex(row)
}

func (r *SQLite) ListRevisions(runID string, artType domain.ArtifactType) ([]domain.Index, error) {
	rows, err := r.db.Query(
		`SELECT id,run_id,type,schema_version,revision,content_hash,size_bytes,storage_path,created_at
		 FROM artifacts WHERE run_id=? AND type=? ORDER BY revision DESC`, runID, artType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectIndices(rows)
}

func (r *SQLite) ListByRun(runID string) ([]domain.Index, error) {
	rows, err := r.db.Query(
		`SELECT id,run_id,type,schema_version,revision,content_hash,size_bytes,storage_path,created_at
		 FROM artifacts WHERE run_id=? ORDER BY type, revision DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectIndices(rows)
}

func (r *SQLite) NextRevision(runID string, artType domain.ArtifactType) (int, error) {
	var maxRev sql.NullInt64
	err := r.db.QueryRow(
		`SELECT MAX(revision) FROM artifacts WHERE run_id=? AND type=?`, runID, artType,
	).Scan(&maxRev)
	if err != nil {
		return 0, err
	}
	if !maxRev.Valid {
		return 1, nil
	}
	return int(maxRev.Int64) + 1, nil
}

func scanIndex(row *sql.Row) (domain.Index, error) {
	var idx domain.Index
	var ca string
	err := row.Scan(&idx.ID, &idx.RunID, &idx.Type, &idx.SchemaVersion,
		&idx.Revision, &idx.ContentHash, &idx.SizeBytes, &idx.StoragePath, &ca)
	if err == sql.ErrNoRows {
		return idx, domainerr.NotFound("not found")
	}
	if err != nil {
		return idx, err
	}
	idx.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	return idx, nil
}

func collectIndices(rows *sql.Rows) ([]domain.Index, error) {
	var out []domain.Index
	for rows.Next() {
		var idx domain.Index
		var ca string
		err := rows.Scan(&idx.ID, &idx.RunID, &idx.Type, &idx.SchemaVersion,
			&idx.Revision, &idx.ContentHash, &idx.SizeBytes, &idx.StoragePath, &ca)
		if err != nil {
			return nil, err
		}
		idx.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		out = append(out, idx)
	}
	return out, rows.Err()
}
