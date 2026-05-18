package projectaudit

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) listTaskSpecs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	if _, err := h.store.GetProject(projectID); err != nil {
		writeError(w, err)
		return
	}
	items, err := h.store.ListTaskSpecs(projectID, r.URL.Query().Get("module"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createTaskSpec(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskSpecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, validationError("invalid JSON body"))
		return
	}
	spec, err := h.store.CreateTaskSpec(chi.URLParam(r, "project_id"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, spec)
}

func (h *Handler) getTaskSpec(w http.ResponseWriter, r *http.Request) {
	spec, err := h.store.GetTaskSpec(chi.URLParam(r, "spec_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (h *Handler) updateTaskSpec(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskSpecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, validationError("invalid JSON body"))
		return
	}
	spec, err := h.store.UpdateTaskSpec(chi.URLParam(r, "spec_id"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (h *Handler) listPRReviews(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	if _, err := h.store.GetProject(projectID); err != nil {
		writeError(w, err)
		return
	}
	items, err := h.store.ListPRReviews(projectID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createPRReview(w http.ResponseWriter, r *http.Request) {
	var req CreatePRReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, validationError("invalid JSON body"))
		return
	}
	result, err := h.engine.ReviewPR(r.Context(), chi.URLParam(r, "project_id"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getPRReview(w http.ResponseWriter, r *http.Request) {
	result, err := h.store.GetPRReview(chi.URLParam(r, "review_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listPRReviewFindings(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListPRReviewFindings(chi.URLParam(r, "review_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) listPRReviewFiles(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListPRReviewFiles(chi.URLParam(r, "review_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
