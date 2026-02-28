package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	artifactUC "toollab-core/internal/artifact/usecases"
	"toollab-core/internal/run/usecases"
	"toollab-core/internal/shared"
)

type Handler struct {
	svc         *usecases.Service
	artifactSvc *artifactUC.Service
}

func New(svc *usecases.Service, artifactSvc *artifactUC.Service) *Handler {
	return &Handler{svc: svc, artifactSvc: artifactSvc}
}

func (h *Handler) RunRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{run_id}", h.get)
	r.Get("/{run_id}/interpretation", h.getInterpretation)
	return r
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Get(chi.URLParam(r, "run_id"))
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, run)
}

func (h *Handler) getInterpretation(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactLLMInterpretation)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

	// Check if the artifact is an error marker
	var probe struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.Status == "failed" {
		shared.WriteError(w, http.StatusServiceUnavailable, probe.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
