package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"toollab-dashboard/internal/executor"
	"toollab-dashboard/internal/interpretations"
	"toollab-dashboard/internal/runs"
	"toollab-dashboard/internal/targets"
)

type Router struct {
	mux             *http.ServeMux
	targets         *targets.Store
	runs            *runs.Store
	interpretations *interpretations.Store
	exec            *executor.Executor
}

func NewRouter(db *sql.DB, exec *executor.Executor) *Router {
	r := &Router{
		mux:             http.NewServeMux(),
		targets:         targets.NewStore(db),
		runs:            runs.NewStore(db),
		interpretations: interpretations.NewStore(db),
		exec:            exec,
	}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) routes() {
	r.mux.HandleFunc("GET /healthz", r.healthz)

	r.mux.HandleFunc("GET /api/v1/targets", r.listTargets)
	r.mux.HandleFunc("POST /api/v1/targets", r.createTarget)
	r.mux.HandleFunc("GET /api/v1/targets/{id}", r.getTarget)
	r.mux.HandleFunc("DELETE /api/v1/targets/{id}", r.deleteTarget)

	r.mux.HandleFunc("GET /api/v1/runs", r.listRuns)
	r.mux.HandleFunc("GET /api/v1/runs/{id}", r.getRun)
	r.mux.HandleFunc("DELETE /api/v1/runs/{id}", r.deleteRun)
	r.mux.HandleFunc("POST /api/v1/runs/ingest", r.ingestRun)

	r.mux.HandleFunc("GET /api/v1/runs/{id}/interpretation", r.getInterpretation)

	r.mux.HandleFunc("GET /api/v1/stats", r.getStats)
	r.mux.HandleFunc("GET /api/v1/scenarios", r.listScenarios)

	r.mux.HandleFunc("POST /api/v1/exec/generate", r.execGenerate)
	r.mux.HandleFunc("POST /api/v1/exec/run", r.execRun)
	r.mux.HandleFunc("POST /api/v1/exec/interpret", r.execInterpret)
	r.mux.HandleFunc("POST /api/v1/exec/enrich", r.execEnrich)
	r.mux.HandleFunc("POST /api/v1/exec/audit", r.execAudit)
	r.mux.HandleFunc("POST /api/v1/exec/coverage", r.execCoverage)
	r.mux.HandleFunc("POST /api/v1/exec/full-audit", r.execFullAudit)
	r.mux.HandleFunc("GET /api/v1/runs/{id}/audit", r.getRunAudit)
	r.mux.HandleFunc("GET /api/v1/runs/{id}/coverage", r.getRunCoverage)
	r.mux.HandleFunc("GET /api/v1/runs/{id}/contract", r.getRunContract)
	r.mux.HandleFunc("GET /api/v1/runs/{id}/comprehension", r.getRunComprehension)
	r.mux.HandleFunc("GET /api/v1/runs/{id}/endpoints", r.getRunEndpoints)
}

// ── Data endpoints ──────────────────────────────────

func (r *Router) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) listTargets(w http.ResponseWriter, _ *http.Request) {
	list, err := r.targets.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []targets.Target{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (r *Router) createTarget(w http.ResponseWriter, req *http.Request) {
	var body targets.CreateRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" || body.BaseURL == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name and base_url required"))
		return
	}
	t, err := r.targets.Create(body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (r *Router) getTarget(w http.ResponseWriter, req *http.Request) {
	t, err := r.targets.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (r *Router) deleteTarget(w http.ResponseWriter, req *http.Request) {
	if err := r.targets.Delete(req.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) listRuns(w http.ResponseWriter, req *http.Request) {
	targetID := req.URL.Query().Get("target_id")
	limit := 50
	if l := req.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	list, err := r.runs.List(targetID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if list == nil {
		list = []runs.Run{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (r *Router) getRun(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	assertions, _ := r.runs.GetAssertions(run.ID)
	if assertions == nil {
		assertions = []runs.AssertionResult{}
	}

	var narrative *string
	if interp, err := r.interpretations.GetByRunID(run.ID); err == nil {
		narrative = &interp.Narrative
	}

	writeJSON(w, http.StatusOK, runs.RunDetail{
		Run:            *run,
		Assertions:     assertions,
		Interpretation: narrative,
	})
}

func (r *Router) deleteRun(w http.ResponseWriter, req *http.Request) {
	if err := r.runs.Delete(req.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) ingestRun(w http.ResponseWriter, req *http.Request) {
	var body struct {
		RunDir   string `json:"run_dir"`
		TargetID string `json:"target_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.RunDir == "" || body.TargetID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("run_dir and target_id required"))
		return
	}
	run, err := runs.IngestFromDir(r.runs, body.RunDir, body.TargetID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (r *Router) getInterpretation(w http.ResponseWriter, req *http.Request) {
	runID := req.PathValue("id")
	interp, err := r.interpretations.GetByRunID(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("no interpretation for run %s", runID))
		return
	}
	writeJSON(w, http.StatusOK, interp)
}

func (r *Router) getStats(w http.ResponseWriter, _ *http.Request) {
	tgts, _ := r.targets.List()
	allRuns, _ := r.runs.List("", 0)

	pass, fail := 0, 0
	for _, run := range allRuns {
		if run.Verdict == "pass" {
			pass++
		} else {
			fail++
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total_targets": len(tgts),
		"total_runs":    len(allRuns),
		"passed":        pass,
		"failed":        fail,
	})
}

func (r *Router) listScenarios(w http.ResponseWriter, _ *http.Request) {
	scenariosDir := filepath.Join(r.exec.CoreDir(), "scenarios")
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	type scenarioFile struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	var out []scenarioFile
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		out = append(out, scenarioFile{
			Name: e.Name(),
			Path: filepath.Join(scenariosDir, e.Name()),
			Size: size,
		})
	}
	if out == nil {
		out = []scenarioFile{}
	}
	writeJSON(w, http.StatusOK, out)
}

// ── Execution endpoints ─────────────────────────────

func (r *Router) execGenerate(w http.ResponseWriter, req *http.Request) {
	var body struct {
		From          string `json:"from"`
		TargetBaseURL string `json:"target_base_url"`
		OpenAPIFile   string `json:"openapi_file,omitempty"`
		OpenAPIURL    string `json:"openapi_url,omitempty"`
		Mode          string `json:"mode,omitempty"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.From == "" {
		body.From = "toollab"
	}
	if body.Mode == "" {
		body.Mode = "smoke"
	}

	args := []string{"generate", "--from", body.From, "--mode", body.Mode}
	if body.TargetBaseURL != "" {
		args = append(args, "--target-base-url", body.TargetBaseURL)
	}
	if body.OpenAPIFile != "" {
		args = append(args, "--openapi-file", body.OpenAPIFile)
	}
	if body.OpenAPIURL != "" {
		args = append(args, "--openapi-url", body.OpenAPIURL)
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	result := r.exec.Run(ctx, args...)
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) execRun(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ScenarioPath string `json:"scenario_path"`
		TargetID     string `json:"target_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.ScenarioPath == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("scenario_path required"))
		return
	}

	ctx, cancel := context.WithTimeout(req.Context(), 2*time.Minute)
	defer cancel()
	result := r.exec.Run(ctx, "run", body.ScenarioPath)

	response := map[string]any{"exec": result}

	if result.Success && body.TargetID != "" {
		runDir := extractRunDir(result.Output)
		if runDir != "" {
			absDir := runDir
			if !filepath.IsAbs(runDir) {
				absDir = filepath.Join(r.exec.CoreDir(), runDir)
			}
			ingested, err := runs.IngestFromDir(r.runs, absDir, body.TargetID)
			if err == nil {
				response["run"] = ingested
				response["run_id"] = ingested.ID
			} else if strings.Contains(err.Error(), "UNIQUE constraint") {
				runID := filepath.Base(absDir)
				response["run_id"] = runID
				response["note"] = "run already ingested"
			} else {
				result.Error = "ingest failed: " + err.Error()
				response["exec"] = result
			}
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *Router) execEnrich(w http.ResponseWriter, req *http.Request) {
	var body struct {
		ScenarioPath  string `json:"scenario_path"`
		From          string `json:"from"`
		TargetBaseURL string `json:"target_base_url"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.ScenarioPath == "" || body.From == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("scenario_path and from required"))
		return
	}

	args := []string{"enrich", body.ScenarioPath, "--from", body.From}
	if body.TargetBaseURL != "" {
		args = append(args, "--target-base-url", body.TargetBaseURL)
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	result := r.exec.Run(ctx, args...)
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) execInterpret(w http.ResponseWriter, req *http.Request) {
	var body struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	run, err := r.runs.GetByID(body.RunID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("run not found: %s", body.RunID))
		return
	}

	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}

	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Minute)
	defer cancel()
	result := r.exec.Run(ctx, "interpret", runDir)

	response := map[string]any{"exec": result}

	if result.Success {
		interp, err := r.interpretations.Insert(run.ID, "ollama", result.Output, "")
		if err == nil {
			response["interpretation"] = interp
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func (r *Router) execAudit(w http.ResponseWriter, req *http.Request) {
	var body struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := r.runs.GetByID(body.RunID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("run not found: %s", body.RunID))
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	result := r.exec.Run(ctx, "audit", runDir)
	writeJSON(w, http.StatusOK, map[string]any{"exec": result})
}

func (r *Router) execCoverage(w http.ResponseWriter, req *http.Request) {
	var body struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	run, err := r.runs.GetByID(body.RunID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("run not found: %s", body.RunID))
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second)
	defer cancel()
	result := r.exec.Run(ctx, "coverage", runDir)
	writeJSON(w, http.StatusOK, map[string]any{"exec": result})
}

func (r *Router) execFullAudit(w http.ResponseWriter, req *http.Request) {
	var body struct {
		BaseURL    string `json:"base_url"`
		TargetName string `json:"target_name"`
		TargetID   string `json:"target_id"`
		Mode       string `json:"mode"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.TargetID == "" || body.BaseURL == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("target_id and base_url required"))
		return
	}
	if body.Mode == "" {
		body.Mode = "smoke"
	}

	type stepResult struct {
		Step   string `json:"step"`
		Status string `json:"status"`
		Output string `json:"output,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	steps := make([]stepResult, 0, 6)
	appendStep := func(name string, result executor.Result) bool {
		step := stepResult{
			Step:   name,
			Status: "ok",
			Output: result.Output,
			Error:  result.Error,
		}
		if !result.Success {
			step.Status = "error"
		}
		steps = append(steps, step)
		return result.Success
	}

	ctxGen, cancelGen := context.WithTimeout(req.Context(), 45*time.Second)
	generateRes := r.exec.Run(ctxGen, "generate", "--from", "toollab", "--mode", body.Mode, "--target-base-url", body.BaseURL)
	cancelGen()
	if !appendStep("generate", generateRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID})
		return
	}

	scenarioPath := extractScenarioPath(generateRes.Output)
	if scenarioPath == "" {
		latestPath, err := latestScenarioPath(r.exec.CoreDir())
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"steps":     append(steps, stepResult{Step: "resolve-scenario", Status: "error", Error: err.Error()}),
				"target_id": body.TargetID,
			})
			return
		}
		scenarioPath = latestPath
	}
	if !filepath.IsAbs(scenarioPath) {
		scenarioPath = filepath.Join(r.exec.CoreDir(), scenarioPath)
	}

	ctxEnrich, cancelEnrich := context.WithTimeout(req.Context(), 60*time.Second)
	enrichRes := r.exec.Run(ctxEnrich, "enrich", scenarioPath, "--from", "toollab", "--target-base-url", body.BaseURL)
	cancelEnrich()
	if !appendStep("enrich", enrichRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID})
		return
	}

	ctxRun, cancelRun := context.WithTimeout(req.Context(), 3*time.Minute)
	runRes := r.exec.Run(ctxRun, "run", scenarioPath)
	cancelRun()
	if !appendStep("run", runRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID})
		return
	}

	runDir := extractRunDir(runRes.Output)
	if runDir == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"steps": append(steps, stepResult{
				Step:   "extract-run-dir",
				Status: "error",
				Error:  "run completed but run_dir was not found in command output",
			}),
			"target_id": body.TargetID,
		})
		return
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}

	runID := ""
	ingested, err := runs.IngestFromDir(r.runs, runDir, body.TargetID)
	if err == nil {
		runID = ingested.ID
		steps = append(steps, stepResult{Step: "ingest", Status: "ok", Output: "run_id=" + runID})
	} else if strings.Contains(err.Error(), "UNIQUE constraint") {
		runID = filepath.Base(runDir)
		steps = append(steps, stepResult{Step: "ingest", Status: "ok", Output: "run already ingested, run_id=" + runID})
	} else {
		steps = append(steps, stepResult{Step: "ingest", Status: "error", Error: err.Error()})
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID})
		return
	}

	ctxAudit, cancelAudit := context.WithTimeout(req.Context(), 45*time.Second)
	auditRes := r.exec.Run(ctxAudit, "audit", runDir)
	cancelAudit()
	if !appendStep("audit", auditRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID, "run_id": runID})
		return
	}

	ctxCoverage, cancelCoverage := context.WithTimeout(req.Context(), 45*time.Second)
	coverageRes := r.exec.Run(ctxCoverage, "coverage", runDir)
	cancelCoverage()
	if !appendStep("coverage", coverageRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID, "run_id": runID})
		return
	}

	ctxComprehend, cancelComprehend := context.WithTimeout(req.Context(), 45*time.Second)
	comprehendRes := r.exec.Run(ctxComprehend, "comprehend", runDir)
	cancelComprehend()
	if !appendStep("comprehend", comprehendRes) {
		writeJSON(w, http.StatusOK, map[string]any{"steps": steps, "target_id": body.TargetID, "run_id": runID})
		return
	}

	steps = append(steps, stepResult{
		Step:   "interpret",
		Status: "queued",
		Output: "Interpretación LLM en segundo plano (async)",
	})
	go r.runInterpretationAsync(runID, runDir)

	writeJSON(w, http.StatusOK, map[string]any{
		"steps":     steps,
		"target_id": body.TargetID,
		"run_id":    runID,
	})
}

func (r *Router) getRunAudit(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "security_audit.json"))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("security audit not available"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func (r *Router) getRunCoverage(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "coverage_report.json"))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("coverage report not available"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func (r *Router) getRunContract(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "contract_validation.json"))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("contract validation not available"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

func (r *Router) getRunComprehension(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "comprehension.md"))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("comprehension report not available"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"markdown": string(raw)})
}

func (r *Router) getRunEndpoints(w http.ResponseWriter, req *http.Request) {
	run, err := r.runs.GetByID(req.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	runDir := run.GoldenRunDir
	if runDir == "" {
		runDir = "golden_runs/" + run.ID
	}
	if !filepath.IsAbs(runDir) {
		runDir = filepath.Join(r.exec.CoreDir(), runDir)
	}
	raw, err := os.ReadFile(filepath.Join(runDir, "endpoints_catalog.json"))
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("endpoints catalog not available"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(raw)
}

// ── Helpers ─────────────────────────────────────────

func extractRunDir(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "run_dir=") {
			return strings.TrimPrefix(line, "run_dir=")
		}
	}
	return ""
}

func extractScenarioPath(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "scenario_path="):
			return strings.TrimPrefix(line, "scenario_path=")
		case strings.HasPrefix(line, "scenario="):
			return strings.TrimPrefix(line, "scenario=")
		case strings.HasPrefix(line, "written:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "written:"))
		}
	}
	return ""
}

func latestScenarioPath(coreDir string) (string, error) {
	scenariosDir := filepath.Join(coreDir, "scenarios")
	entries, err := os.ReadDir(scenariosDir)
	if err != nil {
		return "", err
	}
	latestTime := time.Time{}
	latestPath := ""
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestPath = filepath.Join(scenariosDir, e.Name())
		}
	}
	if latestPath == "" {
		return "", fmt.Errorf("no scenario file found in %s", scenariosDir)
	}
	return latestPath, nil
}

func (r *Router) runInterpretationAsync(runID, runDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	result := r.exec.Run(ctx, "interpret", runDir)
	if !result.Success {
		fmt.Printf("full-audit interpret async failed for run %s: %s\n", runID, result.Error)
		return
	}
	if _, err := r.interpretations.Insert(runID, "llm", result.Output, ""); err != nil && !strings.Contains(err.Error(), "UNIQUE constraint") {
		fmt.Printf("full-audit interpret async persist failed for run %s: %v\n", runID, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
