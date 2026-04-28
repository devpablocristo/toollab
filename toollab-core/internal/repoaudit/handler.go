package repoaudit

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/httpjson"
)

type Handler struct {
	store  *Store
	engine *Engine
}

func NewHandler(store *Store, engine *Engine) *Handler {
	return &Handler{store: store, engine: engine}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/repos", h.listRepos)
	r.Post("/repos", h.createRepo)
	r.Get("/repos/{repo_id}/audits", h.listRepoAudits)
	r.Post("/repos/{repo_id}/audits", h.createAudit)
	r.Get("/audits/{audit_id}", h.getAudit)
	r.Get("/audits/{audit_id}/findings", h.listFindings)
	r.Get("/audits/{audit_id}/docs", h.listDocs)
	r.Get("/audits/{audit_id}/tests", h.listTests)
	r.Get("/audits/{audit_id}/evidence", h.listEvidence)
	r.Get("/audits/{audit_id}/score", h.listScoreItems)
	return r
}

func (h *Handler) createRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		SourceType string `json:"source_type"`
		SourcePath string `json:"source_path"`
		DocPolicy  string `json:"doc_policy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	repo, err := h.store.CreateRepo(req.Name, req.SourceType, req.SourcePath, req.DocPolicy)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, repo)
}

func (h *Handler) listRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListRepos()
	if err != nil {
		writeError(w, err)
		return
	}
	if repos == nil {
		repos = []Repo{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": repos})
}

func (h *Handler) createAudit(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repo_id")
	repo, err := h.store.GetRepo(repoID)
	if err != nil {
		writeError(w, err)
		return
	}
	cfg := DefaultAuditConfig()
	if r.Body != nil {
		var req auditConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
			return
		}
		cfg = req.apply(cfg)
	}
	result, err := h.engine.Run(r.Context(), repo, cfg)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, result)
}

type auditConfigRequest struct {
	GenerateTests          *bool `json:"generate_tests"`
	RunExistingTests       *bool `json:"run_existing_tests"`
	AllowDocsRead          *bool `json:"allow_docs_read"`
	AllowDependencyInstall *bool `json:"allow_dependency_install"`
}

func (r auditConfigRequest) apply(cfg AuditConfig) AuditConfig {
	if r.GenerateTests != nil {
		cfg.GenerateTests = *r.GenerateTests
	}
	if r.RunExistingTests != nil {
		cfg.RunExistingTests = *r.RunExistingTests
	}
	if r.AllowDocsRead != nil {
		cfg.AllowDocsRead = *r.AllowDocsRead
	}
	if r.AllowDependencyInstall != nil {
		cfg.AllowDependencyInstall = *r.AllowDependencyInstall
	}
	return cfg
}

func (h *Handler) listRepoAudits(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repo_id")
	runs, err := h.store.ListAudits(repoID)
	if err != nil {
		writeError(w, err)
		return
	}
	if runs == nil {
		runs = []AuditRun{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (h *Handler) getAudit(w http.ResponseWriter, r *http.Request) {
	run, err := h.store.GetAudit(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, run)
}

func (h *Handler) listFindings(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListFindings(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []Finding{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listDocs(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListDocs(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []GeneratedDoc{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listTests(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListTests(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []TestResult{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listEvidence(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListEvidence(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []Evidence{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listScoreItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListScoreItems(chi.URLParam(r, "audit_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if items == nil {
		items = []ScoreItem{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func writeError(w http.ResponseWriter, err error) {
	if domainerr.IsValidation(err) {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if domainerr.IsNotFound(err) {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if errors.Is(err, http.ErrAbortHandler) {
		httpjson.WriteError(w, http.StatusRequestTimeout, "CANCELLED", err.Error())
		return
	}
	httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
}
