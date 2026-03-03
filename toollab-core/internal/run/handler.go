package run

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	artifactUC "toollab-core/internal/artifact"
	"toollab-core/internal/shared"
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
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	data, _, artErr := h.artifactSvc.GetLatest(run.ID, shared.ArtifactRunSummary)
	if artErr != nil {
		shared.WriteJSON(w, http.StatusOK, map[string]any{"run": run, "run_summary": nil})
		return
	}
	var summary json.RawMessage = data
	shared.WriteJSON(w, http.StatusOK, map[string]any{"run": run, "run_summary": summary})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Get(chi.URLParam(r, "run_id"))
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}
	shared.WriteJSON(w, http.StatusOK, run)
}

func (h *Handler) getAudit(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), shared.ArtifactLLMAudit)
}

func (h *Handler) getDocs(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), shared.ArtifactLLMDocs)
}

func (h *Handler) getArtifact(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	artType, err := shared.ParseArtifactType(chi.URLParam(r, "artifact_type"))
	if err != nil {
		shared.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.serveArtifact(w, runID, artType)
}

func (h *Handler) getEndpointIndex(w http.ResponseWriter, r *http.Request) {
	h.serveArtifact(w, chi.URLParam(r, "run_id"), shared.ArtifactEndpointIntelIndex)
}

func (h *Handler) getEndpointDetail(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	endpointID := chi.URLParam(r, "endpoint_id")

	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactEndpointIntelligence)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

	var intel struct {
		Domains []struct {
			DomainName string          `json:"domain_name"`
			Endpoints  json.RawMessage `json:"endpoints"`
		} `json:"domains"`
	}
	if err := json.Unmarshal(data, &intel); err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "parse intelligence: "+err.Error())
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

	shared.WriteError(w, http.StatusNotFound, "endpoint not found")
}

func (h *Handler) getEndpointScripts(w http.ResponseWriter, r *http.Request) {
	runID := chi.URLParam(r, "run_id")
	endpointID := chi.URLParam(r, "endpoint_id")

	data, _, err := h.artifactSvc.GetLatest(runID, shared.ArtifactEndpointQueries)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

	var scripts struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(data, &scripts); err != nil {
		shared.WriteError(w, http.StatusInternalServerError, "parse scripts: "+err.Error())
		return
	}

	epScripts, ok := scripts.Scripts[endpointID]
	if !ok {
		shared.WriteError(w, http.StatusNotFound, "scripts not found for endpoint")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(epScripts)
}

func (h *Handler) serveArtifact(w http.ResponseWriter, runID string, artType shared.ArtifactType) {
	data, _, err := h.artifactSvc.GetLatest(runID, artType)
	if err != nil {
		shared.WriteError(w, shared.ErrorStatus(err), err.Error())
		return
	}

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
