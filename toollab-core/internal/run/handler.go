package run

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/devpablocristo/core/backend/go/domainerr"
	"github.com/devpablocristo/core/backend/go/httpjson"

	artifactUC "toollab-core/internal/artifact"
	artDomain "toollab-core/internal/artifact/usecases/domain"
)

type Handler struct {
	svc         *Service
	artifactSvc *artifactUC.Service
}

func New(svc *Service, artifactSvc *artifactUC.Service) *Handler {
	return &Handler{svc: svc, artifactSvc: artifactSvc}
}

func (h *Handler) RunRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{run_id}", h.get)
	r.Get("/{run_id}/audit", h.getAudit)
	r.Get("/{run_id}/docs", h.getDocs)
	r.Get("/{run_id}/endpoints", h.getEndpointIndex)
	r.Get("/{run_id}/endpoints/{endpoint_id}", h.getEndpointDetail)
	r.Get("/{run_id}/endpoints/{endpoint_id}/scripts", h.getEndpointScripts)
	r.Get("/{run_id}/artifact/{artifact_type}", h.getArtifact)
	return r
}

func (h *Handler) LatestRunForTarget(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "target_id")
	run, err := h.svc.LatestCompleted(targetID)
	if err != nil {
		writeError(w, err)
		return
	}
	data, _, artErr := h.artifactSvc.GetLatest(run.ID, artDomain.ArtifactRunSummary)
	if artErr != nil {
		httpjson.WriteJSON(w, http.StatusOK, map[string]any{"run": run, "run_summary": nil})
		return
	}
	var summary json.RawMessage = data
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"run": run, "run_summary": summary})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Get(chi.URLParam(r, "run_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, run)
}

func (h *Handler) getAudit(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), artDomain.ArtifactLLMAudit)
}

func (h *Handler) getDocs(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), artDomain.ArtifactLLMDocs)
}

func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := artDomain.ParseArtifactType(chi.URLParam(r, "artifact_type"))
	if err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	h.serveArtifact(w, runID, artType)
}

func (h *Handler) getEndpointIndex(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), artDomain.ArtifactEndpointIntelIndex)
}

func (h *Handler) getEndpointDetail(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	endpointID := chi.URLParam(r, "endpoint_id")

	data, _, err := h.artifactSvc.GetLatest(runID, artDomain.ArtifactEndpointIntelligence)
	if err != nil {
		writeError(w, err)
		return
	}

	var intel struct {
		Domains []struct {
			DomainName string          `json:"domain_name"`
			Endpoints  json.RawMessage `json:"endpoints"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(data, &intel); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	for _, dom := range intel.Domains {
		var endpoints []json.RawMessage
		if err := json.Unmarshal(dom.Endpoints, &endpoints); err != nil {
			continue
		}
		for _, epRaw := range endpoints {
			var ep struct {
				EndpointID string `json:"endpoint_id"`
			}
			if err := json.Unmarshal(epRaw, &ep); err == nil && ep.EndpointID == endpointID {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				resp, _ := json.Marshal(map[string]interface{}{
					"domain":   dom.DomainName,
					"endpoint": json.RawMessage(epRaw),
				})
				w.Write(resp)
				return
			}
		}
	}

	httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "endpoint not found")
}

func (h *Handler) getEndpointScripts(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	endpointID := chi.URLParam(r, "endpoint_id")

	data, _, err := h.artifactSvc.GetLatest(runID, artDomain.ArtifactEndpointQueries)
	if err != nil {
		writeError(w, err)
		return
	}

	var scripts struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(data, &scripts); err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}

	epScripts, ok := scripts.Scripts[endpointID]
	if !ok {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "scripts not found for endpoint")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(epScripts)
}

func (h *Handler) serveArtifact(w http.ResponseWriter, runID string, artType artDomain.ArtifactType) {
	data, _, err := h.artifactSvc.GetLatest(runID, artType)
	if err != nil {
		writeError(w, err)
		return
	}

	var probe struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.Status == "failed" {
		httpjson.WriteError(w, http.StatusServiceUnavailable, "UNAVAILABLE", probe.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeError(w http.ResponseWriter, err error) {
	if domainerr.IsNotFound(err) {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	if domainerr.IsValidation(err) {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
}
