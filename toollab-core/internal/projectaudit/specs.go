package projectaudit

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	specStatusProvided = "provided"
	specStatusInferred = "inferred"
	specStatusDraft    = "draft"
)

type TaskSpec struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	Module          string `json:"module"`
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	TaskDescription string `json:"task_description"`
	SpecMD          string `json:"spec_md"`
	SpecStatus      string `json:"spec_status"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type CreateTaskSpecRequest struct {
	Module          string `json:"module"`
	Title           string `json:"title"`
	TaskDescription string `json:"task_description"`
	SpecMD          string `json:"spec_md"`
	SpecStatus      string `json:"spec_status"`
}

func (s *Store) CreateTaskSpec(projectID string, req CreateTaskSpecRequest) (TaskSpec, error) {
	if _, err := s.GetProject(projectID); err != nil {
		return TaskSpec{}, err
	}
	spec, err := normalizeTaskSpec(projectID, req)
	if err != nil {
		return TaskSpec{}, err
	}
	now := fmtTime(time.Now().UTC())
	spec.ID = newID()
	spec.CreatedAt = now
	spec.UpdatedAt = now
	_, err = s.db.Exec(
		`INSERT INTO task_specs (id,project_id,module,title,slug,task_description,spec_md,spec_status,created_at,updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		spec.ID, spec.ProjectID, spec.Module, spec.Title, spec.Slug, spec.TaskDescription, spec.SpecMD, spec.SpecStatus, spec.CreatedAt, spec.UpdatedAt,
	)
	return spec, err
}

func (s *Store) ListTaskSpecs(projectID string, module string) ([]TaskSpec, error) {
	if strings.TrimSpace(module) != "" {
		rows, err := s.db.Query(
			`SELECT id,project_id,module,title,slug,task_description,spec_md,spec_status,created_at,updated_at
			 FROM task_specs WHERE project_id=? AND module=? ORDER BY created_at DESC`,
			projectID, strings.TrimSpace(module),
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanTaskSpecRows(rows)
	}
	rows, err := s.db.Query(
		`SELECT id,project_id,module,title,slug,task_description,spec_md,spec_status,created_at,updated_at
		 FROM task_specs WHERE project_id=? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTaskSpecRows(rows)
}

func (s *Store) GetTaskSpec(id string) (TaskSpec, error) {
	row := s.db.QueryRow(
		`SELECT id,project_id,module,title,slug,task_description,spec_md,spec_status,created_at,updated_at
		 FROM task_specs WHERE id=?`,
		id,
	)
	var spec TaskSpec
	err := row.Scan(&spec.ID, &spec.ProjectID, &spec.Module, &spec.Title, &spec.Slug, &spec.TaskDescription, &spec.SpecMD, &spec.SpecStatus, &spec.CreatedAt, &spec.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskSpec{}, notFoundError("task spec not found")
	}
	if err != nil {
		return TaskSpec{}, err
	}
	return spec, nil
}

func (s *Store) UpdateTaskSpec(id string, req CreateTaskSpecRequest) (TaskSpec, error) {
	current, err := s.GetTaskSpec(id)
	if err != nil {
		return TaskSpec{}, err
	}
	next, err := normalizeTaskSpec(current.ProjectID, req)
	if err != nil {
		return TaskSpec{}, err
	}
	next.ID = current.ID
	next.CreatedAt = current.CreatedAt
	next.UpdatedAt = fmtTime(time.Now().UTC())
	_, err = s.db.Exec(
		`UPDATE task_specs
		 SET module=?, title=?, slug=?, task_description=?, spec_md=?, spec_status=?, updated_at=?
		 WHERE id=?`,
		next.Module, next.Title, next.Slug, next.TaskDescription, next.SpecMD, next.SpecStatus, next.UpdatedAt, id,
	)
	if err != nil {
		return TaskSpec{}, err
	}
	return s.GetTaskSpec(id)
}

func normalizeTaskSpec(projectID string, req CreateTaskSpecRequest) (TaskSpec, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return TaskSpec{}, validationError("title is required")
	}
	specMD := strings.TrimSpace(req.SpecMD)
	if specMD == "" {
		return TaskSpec{}, validationError("spec_md is required")
	}
	status, err := normalizeSpecStatus(req.SpecStatus)
	if err != nil {
		return TaskSpec{}, err
	}
	module := strings.TrimSpace(req.Module)
	return TaskSpec{
		ProjectID:       projectID,
		Module:          module,
		Title:           title,
		Slug:            slugify(strings.TrimSpace(module + " " + title)),
		TaskDescription: strings.TrimSpace(req.TaskDescription),
		SpecMD:          specMD,
		SpecStatus:      status,
	}, nil
}

func normalizeSpecStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return specStatusProvided, nil
	}
	switch status {
	case specStatusProvided, specStatusInferred, specStatusDraft:
		return status, nil
	default:
		return "", validationError("spec_status must be provided, inferred, or draft")
	}
}

func scanTaskSpecRows(rows *sql.Rows) ([]TaskSpec, error) {
	var out []TaskSpec
	for rows.Next() {
		var spec TaskSpec
		if err := rows.Scan(&spec.ID, &spec.ProjectID, &spec.Module, &spec.Title, &spec.Slug, &spec.TaskDescription, &spec.SpecMD, &spec.SpecStatus, &spec.CreatedAt, &spec.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	if out == nil {
		out = []TaskSpec{}
	}
	return out, rows.Err()
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	value = re.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "spec"
	}
	return value
}
