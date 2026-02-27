// Package api expone la API HTTP de ToolLab v2.
package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"toollab-v2/internal/audit"
	"toollab-v2/internal/detect"
	"toollab-v2/internal/extract"
	"toollab-v2/internal/ingest"
	"toollab-v2/internal/llm"
	"toollab-v2/internal/model"
	"toollab-v2/internal/scenarios"
	"toollab-v2/internal/store"
	"toollab-v2/internal/summarize"
)

type Server struct {
	store *store.Store
	mux   *http.ServeMux
}

func NewServer(st *store.Store) *Server {
	s := &Server{store: st, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return corsMiddleware(s.mux) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("POST /v1/runs", s.createRun)
	s.mux.HandleFunc("GET /v1/runs", s.listRuns)
	s.mux.HandleFunc("DELETE /v1/runs/{id}", s.deleteRun)
	s.mux.HandleFunc("GET /v1/runs/{id}", s.getRun)
	s.mux.HandleFunc("GET /v1/runs/{id}/model", s.getModel)
	s.mux.HandleFunc("GET /v1/runs/{id}/summary", s.getSummary)
	s.mux.HandleFunc("GET /v1/runs/{id}/audit", s.getAudit)
	s.mux.HandleFunc("GET /v1/runs/{id}/scenarios", s.getScenarios)
	s.mux.HandleFunc("GET /v1/runs/{id}/llm", s.getLLM)
	s.mux.HandleFunc("GET /v1/runs/{id}/artifacts", s.getArtifacts)
	s.mux.HandleFunc("GET /v1/runs/{id}/logs", s.getLogs)
}

type createRunRequest struct {
	SourceType string `json:"source_type"`
	LocalPath  string `json:"local_path,omitempty"`
	GitURL     string `json:"git_url,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	LLMEnabled bool   `json:"llm_enabled"`
}

type createRunResponse struct {
	RunID  string          `json:"run_id"`
	Status model.RunStatus `json:"status"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	var req createRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body inválido")
		return
	}
	if req.SourceType == "" {
		req.SourceType = "local_path"
	}
	if req.SourceType != "local_path" {
		writeError(w, http.StatusBadRequest, "por ahora solo source_type=local_path está soportado en v2")
		return
	}
	if req.LocalPath == "" {
		writeError(w, http.StatusBadRequest, "local_path es requerido")
		return
	}

	now := time.Now().UTC()
	runID := strings.ReplaceAll(now.Format("20060102T150405.000000000"), ".", "")
	run := model.Run{
		ID:         runID,
		Status:     model.RunQueued,
		SourceType: req.SourceType,
		SourceRef:  req.LocalPath,
		CreatedAt:  now,
	}
	s.store.InsertRun(run)
	s.store.AppendLog(runID, "run creado")
	go s.executePipeline(runID, req)

	writeJSON(w, http.StatusAccepted, createRunResponse{
		RunID:  runID,
		Status: model.RunQueued,
	})
}

func (s *Server) executePipeline(runID string, req createRunRequest) {
	_ = s.store.UpdateRun(runID, func(r *model.Run) {
		r.Status = model.RunRunning
		r.StartedAt = time.Now().UTC()
	})
	s.store.AppendLog(runID, "pipeline iniciado")

	fail := func(err error) {
		_ = s.store.UpdateRun(runID, func(r *model.Run) {
			r.Status = model.RunFailed
			r.ErrorMessage = err.Error()
			r.FinishedAt = time.Now().UTC()
		})
		s.store.AppendLog(runID, "pipeline falló: "+err.Error())
	}

	snap, err := ingest.BuildSnapshot(req.SourceType, req.LocalPath, req.LocalPath)
	if err != nil {
		fail(err)
		return
	}
	snap.LanguageDetected, snap.FrameworkDetected = detect.LanguageAndFramework(snap)
	s.store.SaveSnapshot(runID, snap)
	s.store.AppendLog(runID, "snapshot generado")

	service, err := extract.BuildServiceModel(snap)
	if err != nil {
		fail(err)
		return
	}
	s.store.SaveServiceModel(runID, service)
	s.store.AppendLog(runID, "service model generado")

	sum := summarize.Build(service)
	s.store.SaveSummary(runID, sum)
	s.store.AppendLog(runID, "summary generado")

	report := audit.Run(service)
	s.store.SaveAudit(runID, report)
	s.store.AppendLog(runID, "audit report generado")

	sc := scenarios.Build(service, report)
	s.store.SaveScenarios(runID, sc)
	s.store.AppendLog(runID, "scenarios generados")

	if req.LLMEnabled {
		interp, ierr := llm.Interpret(service, report)
		if ierr == nil {
			s.store.SaveInterpretation(runID, interp)
			s.store.AppendLog(runID, "llm interpretation generada")
		} else {
			s.store.AppendLog(runID, "llm falló: "+ierr.Error())
		}
	}

	_ = s.store.UpdateRun(runID, func(r *model.Run) {
		r.Status = model.RunSucceeded
		r.FinishedAt = time.Now().UTC()
	})
	s.store.AppendLog(runID, "pipeline finalizado")
}

func (s *Server) listRuns(w http.ResponseWriter, _ *http.Request) {
	runs := s.store.ListRuns()
	sort.Slice(runs, func(i, j int) bool { return runs[i].CreatedAt.After(runs[j].CreatedAt) })
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.GetRun(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "run no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteRun(id); err != nil {
		writeError(w, http.StatusNotFound, "run no encontrado")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getModel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.store.GetServiceModel(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "model no encontrado para run")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) getSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.store.GetSummary(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "summary no encontrado para run")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) getAudit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.store.GetAudit(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "audit no encontrado para run")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) getScenarios(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.store.GetScenarios(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "scenarios no encontrados para run")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": v})
}

func (s *Server) getLLM(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := s.store.GetInterpretation(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "llm no encontrado para run")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) getArtifacts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snap, serr := s.store.GetSnapshot(id)
	modelV, merr := s.store.GetServiceModel(id)
	auditV, aerr := s.store.GetAudit(id)
	sumV, sumErr := s.store.GetSummary(id)

	if serr != nil && merr != nil && aerr != nil && sumErr != nil {
		writeError(w, http.StatusNotFound, "sin artefactos para run")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"snapshot": snap,
		"model":    modelV,
		"summary":  sumV,
		"audit":    auditV,
	})
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.GetLogs(id)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
