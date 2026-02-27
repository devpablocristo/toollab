package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"toollab-core/internal/artifact/usecases"
	"toollab-core/internal/artifact/usecases/domain"
	"toollab-core/internal/shared"
)

type Handler struct{ svc *usecases.Service }

func New(svc *usecases.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.listByRun)
	r.Route("/{type}", func(r chi.Router) {
		r.Put("/", h.put)
		r.Get("/", h.getLatest)
		r.Get("/meta", h.getLatestMeta)
		r.Get("/revisions", h.listRevisions)
		r.Get("/v/{revision}", h.getByRevision)
		r.Get("/v/{revision}/meta", h.getRevisionMeta)
	})
	return r
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 11*1024*1024))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "error reading body")
		return
	}
	result, err := h.svc.Put(runID, artType, body)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) getLatest(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, _, err := h.svc.GetLatest(runID, artType)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) getLatestMeta(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	idx, err := h.svc.GetLatestMeta(runID, artType)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, idx)
}

func (h *Handler) listRevisions(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.svc.ListRevisions(runID, artType)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	if items == nil {
		items = []domain.Index{}
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getByRevision(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	rev, err := strconv.Atoi(chi.URLParam(r, "revision"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "revision must be an integer")
		return
	}
	data, _, err := h.svc.GetByRevision(runID, artType, rev)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) getRevisionMeta(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	rev, err := strconv.Atoi(chi.URLParam(r, "revision"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "revision must be an integer")
		return
	}
	idx, err := h.svc.GetRevisionMeta(runID, artType, rev)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, idx)
}

func (h *Handler) listByRun(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	items, err := h.svc.ListByRun(runID)
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []domain.Index{}
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}
