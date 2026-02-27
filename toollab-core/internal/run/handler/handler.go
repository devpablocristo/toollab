package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	artifactDomain "toollab-core/internal/artifact/usecases/domain"
	discoveryUC "toollab-core/internal/discovery/usecases"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	"toollab-core/internal/run/handler/dto"
	"toollab-core/internal/run/usecases"
	runDomain "toollab-core/internal/run/usecases/domain"
	"toollab-core/internal/shared"

	artifactUC "toollab-core/internal/artifact/usecases"
)

type Handler struct {
	svc          *usecases.Service
	executor     *usecases.Executor
	artifactSvc  *artifactUC.Service
	storage      artifactDomain.ContentStorage
	discoverySvc *discoveryUC.Service
}

func New(svc *usecases.Service, executor *usecases.Executor, artifactSvc *artifactUC.Service, storage artifactDomain.ContentStorage, discoverySvc *discoveryUC.Service) *Handler {
	return &Handler{svc: svc, executor: executor, artifactSvc: artifactSvc, storage: storage, discoverySvc: discoverySvc}
}

func (h *Handler) TargetRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.listByTarget)
	return r
}

func (h *Handler) RunRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{run_id}", h.get)
	r.Put("/{run_id}/scenario-plan", h.putScenarioPlan)
	r.Get("/{run_id}/scenario-plan", h.getScenarioPlan)
	r.Post("/{run_id}/execute", h.execute)
	r.Get("/{run_id}/evidence", h.getEvidence)
	r.Get("/{run_id}/evidence/items/{evidence_id}", h.getEvidenceItem)
	r.Post("/{run_id}/discover", h.discover)
	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "target_id")
	var req dto.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shared.WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	run, err := h.svc.Create(targetID, req.Seed, req.Notes)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusCreated, run)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Get(chi.URLParam(r, "run_id"))
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, run)
}

func (h *Handler) listByTarget(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListByTarget(chi.URLParam(r, "target_id"))
	if err != nil {
		shared.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []runDomain.Run{}
	}
	shared.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) putScenarioPlan(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	body, err := io.ReadAll(io.LimitReader(r.Body, 11*1024*1024))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, "error reading body")
		return
	}
	result, err := h.artifactSvc.Put(runID, shared.ArtifactScenarioPlan, body)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) getScenarioPlan(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactScenarioPlan)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	var opts usecases.ExecuteOptions
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			shared.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	resp, err := h.executor.ExecuteRun(runID, opts)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getEvidence(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactEvidencePack)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) getEvidenceItem(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	evidenceID := chi.URLParam(r, "evidence_id")

	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactEvidencePack)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

	var pack evidenceDomain.EvidencePack
	if err := json.Unmarshal(data, &pack); err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "parsing evidence pack")
		return
	}

	var found *evidenceDomain.EvidenceItem
	for i := range pack.Items {
		if pack.Items[i].EvidenceID == evidenceID {
			found = &pack.Items[i]
			break
		}
	}
	if found == nil {
		shared.WriteError(w, http.StatusNotFound, "evidence item not found")
		return
	}

	type itemDetail struct {
		evidenceDomain.EvidenceItem
		RequestBodyFull  string `json:"request_body_full,omitempty"`
		ResponseBodyFull string `json:"response_body_full,omitempty"`
	}
	detail := itemDetail{EvidenceItem: *found}

	if found.Request.BodyRef != "" {
		if raw, err := h.storage.Read(found.Request.BodyRef); err == nil {
			detail.RequestBodyFull = string(raw)
		}
	}
	if found.Response != nil && found.Response.BodyRef != "" {
		if raw, err := h.storage.Read(found.Response.BodyRef); err == nil {
			detail.ResponseBodyFull = string(raw)
		}
	}

	shared.WriteJSON(w, http.StatusOK, detail)
}

func (h *Handler) discover(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")

	run, err := h.svc.Get(runID)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

	var opts discoveryUC.DiscoverOptions
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			shared.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}

	result, err := h.discoverySvc.Discover(runID, run.TargetID, opts)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, result)
}
