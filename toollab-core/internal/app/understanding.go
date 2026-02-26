package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"toollab-core/internal/discovery"
	"toollab-core/internal/evidence"
	"toollab-core/internal/llm"
	"toollab-core/internal/meta"
	diffmodel "toollab-core/internal/understanding/diff"
	explainmodel "toollab-core/internal/understanding/explain"
	mapmodel "toollab-core/internal/understanding/map"
	"toollab-core/pkg/utils"
)

type MapConfig struct {
	From            string
	OpenAPIInput    string
	OpenAPIAuthFlag string
	TargetBaseURL   string
	ToollabURL       string
	ToollabAuthFlag  string
	OutPath         string
	Seed            string
	Print           bool
	DryRun          bool
	ToollabVersion   string
}

type MapResult struct {
	SystemMapJSON []byte
	SystemMapMD   []byte
	MetaJSON      []byte
	MapFP         string
	OutDir        string
	MetaPath      string
}

func BuildSystemMap(ctx context.Context, cfg MapConfig) (*MapResult, error) {
	if cfg.ToollabVersion == "" {
		cfg.ToollabVersion = defaultToollabVersion
	}
	if !cfg.Print && cfg.OutPath == "" {
		return nil, fmt.Errorf("--out is required unless --print is used")
	}
	if cfg.From != "openapi" && cfg.From != "toollab" {
		return nil, fmt.Errorf("--from must be openapi or toollab")
	}

	openapiAuth, err := discovery.ParseAuthFlag(cfg.OpenAPIAuthFlag)
	if err != nil {
		return nil, err
	}
	toollabAuth, err := discovery.ParseAuthFlag(cfg.ToollabAuthFlag)
	if err != nil {
		return nil, err
	}

	openapiFetcher := discovery.NewOpenAPIFetcher(discovery.HTTPConfig{})
	toollabBase := cfg.ToollabURL
	if toollabBase == "" && cfg.TargetBaseURL != "" {
		toollabBase = strings.TrimRight(cfg.TargetBaseURL, "/") + "/_toollab"
	}
	toollabFetcher := discovery.NewToollabFetcher(toollabBase, discovery.HTTPConfig{})

	inputs := map[string]string{}
	warnings := []string{}
	unknowns := []string{}
	declaredCaps := []string{}
	usedCaps := []string{}
	hashes := meta.HashesInfo{}

	var systemMap *mapmodel.SystemMap
	switch cfg.From {
	case "openapi":
		if cfg.OpenAPIInput == "" {
			return nil, fmt.Errorf("--from openapi requires --openapi-file or --openapi-url")
		}
		doc, openapiHash, info, warn, fetchErr := openapiFetcher.Fetch(ctx, cfg.OpenAPIInput, openapiAuth)
		if fetchErr != nil {
			return nil, fetchErr
		}
		hashes.OpenAPISHA256 = openapiHash
		inputs["openapi_source"] = info.Source + ":" + info.URL + ":" + info.File + ":" + info.Hash
		warnings = append(warnings, warn...)
		systemMap = mapmodel.FromOpenAPI(doc)
	case "toollab":
		if cfg.TargetBaseURL == "" {
			return nil, fmt.Errorf("--from toollab requires --target-base-url")
		}
		manifest, manifestHash, warn, mErr := toollabFetcher.Manifest(ctx, toollabAuth)
		if mErr != nil {
			return nil, mErr
		}
		warnings = append(warnings, warn...)
		hashes.ManifestSHA256 = manifestHash
		inputs["manifest_sha256"] = manifestHash
		declaredCaps = append(declaredCaps, manifest.Capabilities...)

		profile, profileHash, profileWarn, pErr := toollabFetcher.Profile(ctx, toollabAuth)
		warnings = append(warnings, profileWarn...)
		if pErr == nil {
			hashes.ProfileSHA256 = profileHash
			inputs["profile_sha256"] = profileHash
			usedCaps = append(usedCaps, "profile")
			systemMap = mapmodel.FromToollab(manifest, profile)
		} else {
			warnings = append(warnings, "profile unavailable, generated partial system map")
			systemMap = mapmodel.FromToollab(manifest, nil)
		}
		if systemMap != nil {
			unknowns = append(unknowns, systemMap.Unknowns...)
		}
	}

	inputSeed, effectiveSeed, seedDerivation, err := resolveMapSeed(cfg.Seed, inputs, cfg)
	if err != nil {
		return nil, err
	}

	mapJSON, mapFP, err := mapmodel.WriteCanonical(systemMap)
	if err != nil {
		return nil, err
	}
	if err := validateSchemaByName("system_map.v1.schema.json", mapJSON); err != nil {
		return nil, err
	}
	hashes.SystemMapSHA256 = utils.SHA256Hex(mapJSON)
	mapMD := renderMapMD(systemMap)

	metaDoc := meta.Document{
		Operation:     "map",
		ToollabVersion: cfg.ToollabVersion,
		Seed: meta.SeedInfo{
			Provided:   cfg.Seed != "",
			InputSeed:  inputSeed,
			Effective:  effectiveSeed,
			Derivation: seedDerivation,
		},
		Source: meta.SourceInfo{
			Primary:   cfg.From,
			Secondary: []string{},
			Inputs:    sortedInputList(inputs),
		},
		Hashes: hashes,
		Options: map[string]any{
			"from":    cfg.From,
			"print":   cfg.Print,
			"dry_run": cfg.DryRun,
		},
		Capabilities: meta.CapabilityInfo{
			Declared:        uniqueSortedValues(declaredCaps),
			Used:            uniqueSortedValues(usedCaps),
			MissingRequired: []string{},
		},
		Warnings: uniqueSortedValues(warnings),
		Unknowns: uniqueSortedValues(append(unknowns, systemMap.Unknowns...)),
	}
	metaJSON, _, err := meta.WriteCanonical(metaDoc)
	if err != nil {
		return nil, err
	}

	result := &MapResult{SystemMapJSON: mapJSON, SystemMapMD: mapMD, MetaJSON: metaJSON, MapFP: mapFP}
	if cfg.DryRun {
		return result, nil
	}
	if !cfg.Print {
		outDir := normalizeOutputDir(cfg.OutPath)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(outDir, "system_map.json"), mapJSON, 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(outDir, "system_map.md"), mapMD, 0o644); err != nil {
			return nil, err
		}
		metaPath := filepath.Join(outDir, "map.meta.json")
		if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
			return nil, err
		}
		result.OutDir = outDir
		result.MetaPath = metaPath
	}
	return result, nil
}

type ExplainConfig struct {
	RunDir        string
	DiscoveryDir  string
	OutDir        string
	Print         bool
	DryRun        bool
	LLMMode       string
	ToollabVersion string
}

type ExplainResult struct {
	UnderstandingJSON []byte
	UnderstandingMD   []byte
	MetaJSON          []byte
	Fingerprint       string
	MetaPath          string
}

func ExplainRun(ctx context.Context, cfg ExplainConfig) (*ExplainResult, error) {
	_ = ctx
	if cfg.ToollabVersion == "" {
		cfg.ToollabVersion = defaultToollabVersion
	}
	if !cfg.Print && cfg.OutDir == "" {
		return nil, fmt.Errorf("--out is required unless --print is used")
	}
	if cfg.LLMMode == "" {
		cfg.LLMMode = "off"
	}
	if cfg.LLMMode != "off" && cfg.LLMMode != "on" {
		return nil, fmt.Errorf("--llm must be off or on")
	}

	evidencePath := filepath.Join(cfg.RunDir, "evidence.json")
	evidenceRaw, err := os.ReadFile(evidencePath)
	if err != nil {
		return nil, err
	}
	bundle, err := loadEvidence(evidencePath)
	if err != nil {
		return nil, err
	}

	mapPath := filepath.Join(cfg.RunDir, "system_map.json")
	if cfg.DiscoveryDir != "" {
		mapPath = filepath.Join(cfg.DiscoveryDir, "system_map.json")
	}
	var (
		systemMap *mapmodel.SystemMap
		mapRaw    []byte
		mapSHA256 string
		warnings  []string
	)
	if raw, readErr := os.ReadFile(mapPath); readErr == nil {
		var mapDoc mapmodel.SystemMap
		if json.Unmarshal(raw, &mapDoc) == nil {
			systemMap = &mapDoc
			mapRaw = raw
			mapSHA256 = utils.SHA256Hex(raw)
		}
	}
	if systemMap == nil {
		systemMap = mapmodel.FromEvidence(bundle)
		warnings = append(warnings, "discovery map missing; derived partial map from evidence")
	}

	understanding := explainmodel.Build(bundle, systemMap, evidencePath)
	var llmNarrative string
	if cfg.LLMMode == "on" {
		llmInput, cErr := utils.CanonicalJSON(understanding)
		if cErr == nil {
			understanding.Determinism.NarrativeOnly = true
			understanding.Determinism.LLMInputSHA256 = utils.SHA256Hex(llmInput)
		}

		ollamaClient := llm.NewClient()
		if ollamaClient.Available(ctx) {
			summary := buildEvidenceSummary(bundle)
			narrative, llmErr := ollamaClient.ExplainEvidence(ctx, summary)
			if llmErr != nil {
				warnings = append(warnings, "llm narrative failed: "+llmErr.Error())
			} else {
				llmNarrative = narrative
			}
		} else {
			warnings = append(warnings, "llm requested but ollama not available at "+os.Getenv("OLLAMA_URL"))
		}
	}
	understandingJSON, fp, err := explainmodel.WriteCanonical(understanding)
	if err != nil {
		return nil, err
	}
	if err := validateSchemaByName("understanding.v1.schema.json", understandingJSON); err != nil {
		return nil, err
	}
	understandingMD := explainmodel.RenderMD(understanding)
	if llmNarrative != "" {
		understandingMD = append(understandingMD, []byte("\n\n---\n\n## LLM Narrative (Ollama)\n\n"+llmNarrative+"\n")...)
	}

	hashes := meta.HashesInfo{
		EvidenceSHA256:      utils.SHA256Hex(evidenceRaw),
		SystemMapSHA256:     mapSHA256,
		UnderstandingSHA256: utils.SHA256Hex(understandingJSON),
	}
	if len(mapRaw) == 0 {
		hashes.SystemMapSHA256 = ""
	}

	metaDoc := meta.Document{
		Operation:     "explain",
		ToollabVersion: cfg.ToollabVersion,
		Seed: meta.SeedInfo{
			Provided:   false,
			Effective:  "0",
			Derivation: "not_applicable",
		},
		Source: meta.SourceInfo{
			Primary:   "evidence",
			Secondary: []string{"system_map"},
			Inputs: []string{
				"run_dir=" + cfg.RunDir,
				"evidence_path=" + evidencePath,
			},
		},
		Hashes: hashes,
		Options: map[string]any{
			"llm":     cfg.LLMMode,
			"print":   cfg.Print,
			"dry_run": cfg.DryRun,
		},
		Capabilities: meta.CapabilityInfo{
			Declared:        []string{},
			Used:            []string{},
			MissingRequired: []string{},
		},
		Warnings: uniqueSortedValues(warnings),
		Unknowns: uniqueSortedValues(understanding.Unknowns),
	}
	metaJSON, _, err := meta.WriteCanonical(metaDoc)
	if err != nil {
		return nil, err
	}

	result := &ExplainResult{
		UnderstandingJSON: understandingJSON,
		UnderstandingMD:   understandingMD,
		MetaJSON:          metaJSON,
		Fingerprint:       fp,
	}
	if cfg.DryRun {
		return result, nil
	}
	if !cfg.Print {
		if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(cfg.OutDir, "understanding.json"), understandingJSON, 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(cfg.OutDir, "understanding.md"), understandingMD, 0o644); err != nil {
			return nil, err
		}
		metaPath := filepath.Join(cfg.OutDir, "explain.meta.json")
		if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
			return nil, err
		}
		result.MetaPath = metaPath
	}
	return result, nil
}

type DiffConfig struct {
	RunADir       string
	RunBDir       string
	OutDir        string
	Print         bool
	DryRun        bool
	LLMMode       string
	ToollabVersion string
}

type DiffResult struct {
	DiffJSON    []byte
	DiffMD      []byte
	MetaJSON    []byte
	Fingerprint string
	MetaPath    string
}

func DiffRuns(ctx context.Context, cfg DiffConfig) (*DiffResult, error) {
	_ = ctx
	if cfg.ToollabVersion == "" {
		cfg.ToollabVersion = defaultToollabVersion
	}
	if !cfg.Print && cfg.OutDir == "" {
		return nil, fmt.Errorf("--out is required unless --print is used")
	}
	if cfg.LLMMode == "" {
		cfg.LLMMode = "off"
	}
	if cfg.LLMMode != "off" && cfg.LLMMode != "on" {
		return nil, fmt.Errorf("--llm must be off or on")
	}

	evidenceAPath := filepath.Join(cfg.RunADir, "evidence.json")
	evidenceBPath := filepath.Join(cfg.RunBDir, "evidence.json")
	rawA, err := os.ReadFile(evidenceAPath)
	if err != nil {
		return nil, err
	}
	rawB, err := os.ReadFile(evidenceBPath)
	if err != nil {
		return nil, err
	}
	bundleA, err := loadEvidence(evidenceAPath)
	if err != nil {
		return nil, err
	}
	bundleB, err := loadEvidence(evidenceBPath)
	if err != nil {
		return nil, err
	}

	diffDoc := diffmodel.Compare(bundleA, bundleB, evidenceAPath, evidenceBPath)
	if cfg.LLMMode == "on" {
		diffDoc.Unknowns = append(diffDoc.Unknowns, "llm narrative mode enabled; pass/fail remains evidence-driven")
	}
	diffDoc.Unknowns = uniqueSortedValues(diffDoc.Unknowns)
	diffJSON, fp, err := diffmodel.WriteCanonical(diffDoc)
	if err != nil {
		return nil, err
	}
	if err := validateSchemaByName("diff.v1.schema.json", diffJSON); err != nil {
		return nil, err
	}
	diffMD := diffmodel.RenderMD(diffDoc)

	metaDoc := meta.Document{
		Operation:     "diff",
		ToollabVersion: cfg.ToollabVersion,
		Seed: meta.SeedInfo{
			Provided:   false,
			Effective:  "0",
			Derivation: "not_applicable",
		},
		Source: meta.SourceInfo{
			Primary:   "evidence",
			Secondary: []string{},
			Inputs: []string{
				"run_a=" + cfg.RunADir,
				"run_b=" + cfg.RunBDir,
			},
		},
		Hashes: meta.HashesInfo{
			EvidenceSHA256: utils.SHA256Hex([]byte(utils.SHA256Hex(rawA) + ":" + utils.SHA256Hex(rawB))),
			DiffSHA256:     utils.SHA256Hex(diffJSON),
		},
		Options: map[string]any{
			"llm":     cfg.LLMMode,
			"print":   cfg.Print,
			"dry_run": cfg.DryRun,
		},
		Capabilities: meta.CapabilityInfo{
			Declared:        []string{},
			Used:            []string{},
			MissingRequired: []string{},
		},
		Warnings: []string{},
		Unknowns: uniqueSortedValues(diffDoc.Unknowns),
	}
	metaJSON, _, err := meta.WriteCanonical(metaDoc)
	if err != nil {
		return nil, err
	}

	result := &DiffResult{DiffJSON: diffJSON, DiffMD: diffMD, MetaJSON: metaJSON, Fingerprint: fp}
	if cfg.DryRun {
		return result, nil
	}
	if !cfg.Print {
		if err := os.MkdirAll(cfg.OutDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(cfg.OutDir, "diff.json"), diffJSON, 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(cfg.OutDir, "diff.md"), diffMD, 0o644); err != nil {
			return nil, err
		}
		metaPath := filepath.Join(cfg.OutDir, "diff.meta.json")
		if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
			return nil, err
		}
		result.MetaPath = metaPath
	}
	return result, nil
}

func resolveMapSeed(userSeed string, inputs map[string]string, cfg MapConfig) (string, string, string, error) {
	if userSeed != "" {
		return userSeed, userSeed, "provided", nil
	}
	options := map[string]any{
		"from":            cfg.From,
		"target_base_url": cfg.TargetBaseURL,
		"toollab_url":      cfg.ToollabURL,
	}
	seed, _, err := meta.DeriveSeed(meta.SeedInput{Inputs: inputs, Options: options})
	if err != nil {
		return "", "", "", err
	}
	return "", seed, "sha256(inputs_canonical)", nil
}

func loadEvidence(path string) (*evidence.Bundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out evidence.Bundle
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func validateSchemaByName(fileName string, raw []byte) error {
	schemaPath, err := findSchemaPath(fileName)
	if err != nil {
		return err
	}
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return fmt.Errorf("compile schema %s: %w", fileName, err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	return schema.Validate(doc)
}

func renderMapMD(in *mapmodel.SystemMap) []byte {
	lines := []string{
		"# TOOLLAB System Map",
		"",
		fmt.Sprintf("- service: %s", in.ServiceIdentity.Name),
		fmt.Sprintf("- version: %s", in.ServiceIdentity.Version),
		fmt.Sprintf("- partial: %t", in.Partial),
		"",
		"## Endpoints",
	}
	for _, ep := range in.Endpoints {
		lines = append(lines, fmt.Sprintf("- %s %s", ep.Method, ep.Path))
	}
	lines = append(lines, "", "## Unknowns")
	for _, item := range in.Unknowns {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "")
	return []byte(strings.Join(lines, "\n"))
}

func normalizeOutputDir(path string) string {
	if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".md") {
		return filepath.Dir(path)
	}
	return path
}

func buildEvidenceSummary(b *evidence.Bundle) string {
	var sb strings.Builder

	sb.WriteString("# Test Run Summary\n\n")
	sb.WriteString(fmt.Sprintf("Target API: %s\n", b.ScenarioFingerprint.ScenarioPath))
	sb.WriteString(fmt.Sprintf("Mode: %s | Concurrency: %d | Duration: %ds\n", b.Metadata.Mode, b.Execution.Concurrency, b.Execution.DurationS))
	sb.WriteString(fmt.Sprintf("Total requests: %d (planned: %d, completed: %d)\n", b.Stats.TotalRequests, b.Execution.PlannedRequests, b.Execution.CompletedRequests))
	sb.WriteString(fmt.Sprintf("Overall verdict: %s\n", b.Assertions.Overall))
	sb.WriteString(fmt.Sprintf("Success rate: %.2f%% | Error rate: %.2f%%\n", b.Stats.SuccessRate*100, b.Stats.ErrorRate*100))
	sb.WriteString(fmt.Sprintf("Latency P50: %dms | P95: %dms | P99: %dms\n", b.Stats.P50MS, b.Stats.P95MS, b.Stats.P99MS))

	sb.WriteString("\n## Assertion Rules\n")
	for _, rule := range b.Assertions.Rules {
		status := "PASS"
		if !rule.Passed {
			status = "FAIL"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s — %s (observed: %v, threshold: %v)\n", status, rule.ID, rule.Message, rule.Observed, rule.Expected))
	}

	type flowStats struct {
		method       string
		path         string
		total        int
		byStatus     map[int]int
		errors       int
		totalLatency int
	}
	flows := map[string]*flowStats{}
	var flowOrder []string
	for _, o := range b.Outcomes {
		key := o.Method + " " + o.Path
		fs, ok := flows[key]
		if !ok {
			fs = &flowStats{method: o.Method, path: o.Path, byStatus: map[int]int{}}
			flows[key] = fs
			flowOrder = append(flowOrder, key)
		}
		fs.total++
		fs.totalLatency += o.LatencyMS
		if o.StatusCode != nil {
			fs.byStatus[*o.StatusCode]++
		}
		if o.ErrorKind != "" || (o.StatusCode != nil && *o.StatusCode >= 400) {
			fs.errors++
		}
	}

	sb.WriteString(fmt.Sprintf("\n## Per-Flow Breakdown (%d unique endpoints tested)\n\n", len(flows)))
	sb.WriteString("| Method | Path | Reqs | AvgMs | Err% | Status Codes |\n")
	sb.WriteString("|--------|------|------|-------|------|--------------|\n")
	for _, key := range flowOrder {
		fs := flows[key]
		avgLat := 0
		if fs.total > 0 {
			avgLat = fs.totalLatency / fs.total
		}
		errPct := float64(fs.errors) / float64(fs.total) * 100
		var statuses []string
		for code, count := range fs.byStatus {
			statuses = append(statuses, fmt.Sprintf("%d×%d", count, code))
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %.0f%% | %s |\n",
			fs.method, fs.path, fs.total, avgLat, errPct, strings.Join(statuses, ", ")))
	}
	sb.WriteString("\n")

	if len(b.Unknowns) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Unknowns\n%s\n", strings.Join(b.Unknowns, "; ")))
	}
	return sb.String()
}
