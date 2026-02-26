package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"toollab-core/internal/adapter"
	"toollab-core/internal/assertions"
	"toollab-core/internal/chaos"
	"toollab-core/internal/determinism"
	"toollab-core/internal/evidence"
	"toollab-core/internal/report"
	"toollab-core/internal/runner"
	"toollab-core/internal/scenario"
	explainmodel "toollab-core/internal/understanding/explain"
	mapmodel "toollab-core/internal/understanding/map"
)

const defaultToollabVersion = "0.1.0"

type RunResult struct {
	RunDir    string
	Bundle    *evidence.Bundle
	Artifacts *report.ArtifactIndex
}

func RunScenario(ctx context.Context, scenarioPath, outBase string) (*RunResult, error) {
	if scenarioPath == "" {
		return nil, fmt.Errorf("scenario path is required")
	}
	if outBase == "" {
		outBase = "golden_runs"
	}

	scn, fp, err := scenario.Load(scenarioPath)
	if err != nil {
		return nil, err
	}

	// --- Adapter auto-discovery ---
	adapterInfo := adapter.Discover(ctx, scn.Target.BaseURL)
	var adapterClient *adapter.Client
	if adapterInfo != nil {
		adapterClient = adapter.NewClient(adapterInfo.BaseURL)

		// Propagate deterministic seed to the target if supported.
		if adapterInfo.HasCapability("seed") {
			_ = adapterClient.SeedApply(ctx, scn.Seeds.RunSeed, []string{"uuid", "timestamp", "jitter"})
			defer func() { _ = adapterClient.SeedClear(ctx) }()
		}
	}

	tape := determinism.NewTapeRecorder()
	runDecider, err := determinism.NewEngine(scn.Seeds.RunSeed, "run_seed", tape)
	if err != nil {
		return nil, err
	}
	chaosDecider, err := determinism.NewEngine(scn.Seeds.ChaosSeed, "chaos_seed", tape)
	if err != nil {
		return nil, err
	}

	plan, err := runner.BuildPlan(scn, runDecider)
	if err != nil {
		return nil, err
	}

	obs, unknowns := collectObservabilityStart(ctx, scn.Observability)

	chaosEngine := chaos.NewEngine(scn.Chaos, chaosDecider)
	executor := runner.NewBaseExecutor(scn, plan, chaosEngine)

	started := time.Now().UTC()
	runnerOutcomes, err := executor.Execute(ctx)
	finished := time.Now().UTC()
	if err != nil {
		return nil, err
	}

	obs, unknowns = collectObservabilityEnd(ctx, scn.Observability, obs, unknowns)

	// --- Adapter-sourced observability ---
	obs, unknowns = collectAdapterObservability(ctx, adapterInfo, adapterClient, obs, unknowns)

	tapeHash, err := tape.Hash()
	if err != nil {
		return nil, err
	}
	decisionTapeJSONL, err := tape.JSONLines()
	if err != nil {
		return nil, err
	}

	outcomes := make([]evidence.OutcomeInput, 0, len(runnerOutcomes))
	for _, o := range runnerOutcomes {
		outcomes = append(outcomes, evidence.OutcomeInput{
			Seq:          o.Seq,
			RequestID:    o.RequestID,
			Method:       o.Method,
			Path:         o.Path,
			StatusCode:   o.StatusCode,
			ErrorKind:    o.ErrorKind,
			LatencyMS:    o.LatencyMS,
			ResponseHash: o.ResponseHash,
			ChaosApplied: evidence.ChaosApplied{
				LatencyInjectedMS:   o.ChaosApplied.LatencyInjectedMS,
				ErrorInjected:       o.ChaosApplied.ErrorInjected,
				ErrorMode:           o.ChaosApplied.ErrorMode,
				PayloadDriftApplied: o.ChaosApplied.PayloadDrift,
				PayloadMutations:    append([]string(nil), o.ChaosApplied.PayloadMutations...),
			},
			RequestURL:      o.RequestURL,
			RequestHeaders:  o.RequestHeaders,
			RequestBody:     o.RequestBody,
			ResponseHeaders: o.ResponseHeaders,
			ResponseBody:    o.ResponseBody,
		})
	}

	redactionSummary := evidence.RedactionSummary{
		HeadersRedacted:     cloneStrings(scn.Redaction.Headers),
		JSONPathsRedacted:   cloneStrings(scn.Redaction.JSONPaths),
		Mask:                scn.Redaction.Mask,
		MaxBodyPreviewBytes: scn.Redaction.MaxBodyPreviewBytes,
		MaxSamples:          scn.Redaction.MaxSamples,
	}

	bundle, err := evidence.BuildBundle(evidence.CollectInput{
		ScenarioPath:          scenarioPath,
		ScenarioSHA256:        fp.ScenarioSHA,
		ScenarioSchemaVersion: 1,
		ToollabVersion:         defaultToollabVersion,
		Mode:                  scn.Mode,
		RunSeed:               scn.Seeds.RunSeed,
		ChaosSeed:             scn.Seeds.ChaosSeed,
		DBSeedReference:       scn.Seeds.DBSeedReference,
		ScheduleMode:          scn.Workload.ScheduleMode,
		TickMS:                scn.Workload.TickMS,
		Concurrency:           scn.Workload.Concurrency,
		DurationS:             scn.Workload.DurationS,
		PlannedRequests:       len(plan.PlannedRequests),
		CompletedRequests:     len(outcomes),
		DecisionEngineVersion: determinism.EngineVersion,
		DecisionTapeHash:      tapeHash,
		Outcomes:              outcomes,
		Unknowns:              unknowns,
		Assertions:            evidence.Assertions{Overall: "PASS", Rules: []evidence.RuleResult{}, ViolatedRules: []string{}},
		StartedAt:             started,
		FinishedAt:            finished,
		ReproCommand:          fmt.Sprintf("toollab run %s --out %s", scenarioPath, outBase),
		ReproScriptPath:       "",
		Redaction:             redactionSummary,
		Observability:         obs,
	})
	if err != nil {
		return nil, err
	}

	bundle.Assertions = assertions.Evaluate(scn.Expectations, bundle)
	bundle.DeterministicFingerprint, err = evidence.ComputeDeterministicFingerprint(bundle)
	if err != nil {
		return nil, err
	}
	bundle.Repro.ExpectedDeterministicFingerprint = bundle.DeterministicFingerprint

	runDir := filepath.Join(outBase, bundle.Metadata.RunID)
	artifacts, err := report.Generate(runDir, bundle, decisionTapeJSONL)
	if err != nil {
		return nil, err
	}

	// Generate partial system map and evidence-driven understanding by default.
	if err := emitUnderstandingArtifacts(runDir, bundle); err != nil {
		return nil, err
	}

	if err := validateEvidenceSchema(bundle); err != nil {
		return nil, err
	}

	return &RunResult{RunDir: runDir, Bundle: bundle, Artifacts: artifacts}, nil
}

func emitUnderstandingArtifacts(runDir string, bundle *evidence.Bundle) error {
	systemMap := mapmodel.FromEvidence(bundle)
	mapJSON, _, err := mapmodel.WriteCanonical(systemMap)
	if err != nil {
		return err
	}
	mapMD := renderMapMD(systemMap)
	if err := os.WriteFile(filepath.Join(runDir, "system_map.json"), mapJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "system_map.md"), mapMD, 0o644); err != nil {
		return err
	}

	explainDoc := explainmodel.Build(bundle, systemMap, filepath.Join(runDir, "evidence.json"))
	understandingJSON, _, err := explainmodel.WriteCanonical(explainDoc)
	if err != nil {
		return err
	}
	understandingMD := explainmodel.RenderMD(explainDoc)
	if err := os.WriteFile(filepath.Join(runDir, "understanding.json"), understandingJSON, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, "understanding.md"), understandingMD, 0o644); err != nil {
		return err
	}
	return nil
}

func validateEvidenceSchema(bundle *evidence.Bundle) error {
	schemaPath, err := findSchemaPath("evidence.v1.schema.json")
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return fmt.Errorf("compile evidence schema: %w", err)
	}
	raw, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("decode evidence doc: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("evidence schema validation: %w", err)
	}
	return nil
}

func findSchemaPath(fileName string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "schemas", fileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return "", fmt.Errorf("schema %s not found", fileName)
}

func collectObservabilityStart(ctx context.Context, cfg *scenario.Observability) (*evidence.Observability, []string) {
	obs := &evidence.Observability{}
	unknowns := []string{}
	if cfg == nil {
		return nil, []string{
			"metrics_snapshot unavailable: not configured",
			"trace_refs unavailable: not configured",
			"logs_excerpt unavailable: not configured",
		}
	}

	if cfg.Metrics != nil && (cfg.Metrics.ScrapeAt == "start" || cfg.Metrics.ScrapeAt == "both") {
		body, err := fetchURL(ctx, cfg.Metrics.Endpoint, cfg.Metrics.Timeout)
		if err != nil {
			unknowns = append(unknowns, "metrics start scrape failed: "+err.Error())
		} else {
			if obs.MetricsSnapshot == nil {
				obs.MetricsSnapshot = map[string]any{}
			}
			obs.MetricsSnapshot["start"] = body
		}
	}
	return obs, unknowns
}

func collectObservabilityEnd(ctx context.Context, cfg *scenario.Observability, obs *evidence.Observability, unknowns []string) (*evidence.Observability, []string) {
	if cfg == nil {
		return obs, unknowns
	}
	if obs == nil {
		obs = &evidence.Observability{}
	}

	if cfg.Metrics != nil && (cfg.Metrics.ScrapeAt == "end" || cfg.Metrics.ScrapeAt == "both") {
		body, err := fetchURL(ctx, cfg.Metrics.Endpoint, cfg.Metrics.Timeout)
		if err != nil {
			unknowns = append(unknowns, "metrics end scrape failed: "+err.Error())
		} else {
			if obs.MetricsSnapshot == nil {
				obs.MetricsSnapshot = map[string]any{}
			}
			obs.MetricsSnapshot["end"] = body
		}
	}

	if cfg.Traces != nil && cfg.Traces.Enabled {
		if cfg.Traces.Endpoint == "" {
			unknowns = append(unknowns, "traces enabled but endpoint missing")
		} else {
			endpoint := cfg.Traces.Endpoint
			if cfg.Traces.Query != "" {
				sep := "?"
				if strings.Contains(endpoint, "?") {
					sep = "&"
				}
				endpoint = endpoint + sep + cfg.Traces.Query
			}
			body, err := fetchURL(ctx, endpoint, cfg.Traces.Timeout)
			if err != nil {
				unknowns = append(unknowns, "traces query failed: "+err.Error())
			} else {
				obs.TraceRefs = []string{body}
			}
		}
	} else {
		unknowns = append(unknowns, "trace_refs unavailable: not configured")
	}

	if cfg.Logs != nil && cfg.Logs.Enabled {
		logs, err := collectLogs(ctx, cfg.Logs)
		if err != nil {
			unknowns = append(unknowns, "logs collection failed: "+err.Error())
		} else {
			obs.LogsExcerpt = logs
		}
	} else {
		unknowns = append(unknowns, "logs_excerpt unavailable: not configured")
	}

	if cfg.Metrics == nil {
		unknowns = append(unknowns, "metrics_snapshot unavailable: not configured")
	}
	return obs, unknowns
}

func fetchURL(ctx context.Context, endpoint string, timeoutMS int) (string, error) {
	if timeoutMS <= 0 {
		timeoutMS = 2000
	}
	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	buf := make([]byte, 16384)
	n, _ := resp.Body.Read(buf)
	return string(buf[:n]), nil
}

func collectLogs(ctx context.Context, cfg *scenario.LogsConfig) ([]evidence.LogLine, error) {
	maxLines := cfg.MaxLines
	if maxLines <= 0 {
		maxLines = 500
	}
	if cfg.Source == "file" {
		raw, err := os.ReadFile(cfg.FilePath)
		if err != nil {
			return nil, err
		}
		return parseLogLines(string(raw), maxLines), nil
	}
	body, err := fetchURL(ctx, cfg.Endpoint, 2000)
	if err != nil {
		return nil, err
	}
	return parseLogLines(body, maxLines), nil
}

func parseLogLines(raw string, maxLines int) []evidence.LogLine {
	lines := strings.Split(raw, "\n")
	out := make([]evidence.LogLine, 0, maxLines)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(out) >= maxLines {
			break
		}
		entry := evidence.LogLine{Timestamp: "", Level: "INFO", Message: line}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err == nil {
			if v, ok := obj["timestamp"].(string); ok {
				entry.Timestamp = v
			}
			if v, ok := obj["level"].(string); ok {
				entry.Level = v
			}
			if v, ok := obj["message"].(string); ok {
				entry.Message = v
			}
			entry.Attrs = obj
		}
		out = append(out, entry)
	}
	return out
}

func collectAdapterObservability(ctx context.Context, info *adapter.Info, client *adapter.Client, obs *evidence.Observability, unknowns []string) (*evidence.Observability, []string) {
	if info == nil || client == nil {
		return obs, unknowns
	}
	if obs == nil {
		obs = &evidence.Observability{}
	}
	if obs.MetricsSnapshot == nil {
		obs.MetricsSnapshot = map[string]any{}
	}

	obs.MetricsSnapshot["adapter"] = map[string]any{
		"app_name":        info.AppName,
		"app_version":     info.AppVersion,
		"capabilities":    info.Capabilities,
		"adapter_version": info.AdapterVersion,
	}

	if info.HasCapability("state.fingerprint") {
		fp, err := client.StateFingerprint(ctx)
		if err == nil {
			obs.MetricsSnapshot["adapter_post_fingerprint"] = fp
		} else {
			unknowns = append(unknowns, "adapter state.fingerprint failed: "+err.Error())
		}
	}

	if info.HasCapability("metrics") {
		metrics, err := client.Metrics(ctx)
		if err == nil {
			obs.MetricsSnapshot["adapter_metrics"] = metrics
		} else {
			unknowns = append(unknowns, "adapter metrics collection failed: "+err.Error())
		}
	}

	if info.HasCapability("logs") {
		logs, err := client.Logs(ctx, time.Now().UTC().Add(-10*time.Minute), 500)
		if err == nil {
			for _, l := range logs {
				ts, _ := l["timestamp"].(string)
				level, _ := l["level"].(string)
				msg, _ := l["message"].(string)
				obs.LogsExcerpt = append(obs.LogsExcerpt, evidence.LogLine{
					Timestamp: ts,
					Level:     level,
					Message:   msg,
					Attrs:     l,
				})
			}
		} else {
			unknowns = append(unknowns, "adapter logs collection failed: "+err.Error())
		}
	}

	if info.HasCapability("traces") {
		traces, err := client.Traces(ctx, time.Now().UTC().Add(-10*time.Minute), 100)
		if err == nil {
			for _, tr := range traces {
				if tid, ok := tr["trace_id"].(string); ok {
					obs.TraceRefs = append(obs.TraceRefs, tid)
				}
			}
		} else {
			unknowns = append(unknowns, "adapter traces collection failed: "+err.Error())
		}
	}

	return obs, unknowns
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
