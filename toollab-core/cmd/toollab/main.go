package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"toollab-core/internal/app"
	"toollab-core/internal/contract"
	"toollab-core/internal/coverage"
	"toollab-core/internal/enrich"
	"toollab-core/internal/evidence"
	"toollab-core/internal/llm"
	"toollab-core/internal/scenario"
	"toollab-core/internal/security"
	"toollab-core/internal/understanding/comprehension"
	mapmodel "toollab-core/internal/understanding/map"
)

func main() {
	loadEnvFiles()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		handleRun(os.Args[2:])
	case "generate":
		if err := handleGenerate(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "gen":
		// Backward compatibility: gen behaves as generate --from openapi.
		genArgs := append([]string{"--from", "openapi"}, os.Args[2:]...)
		if len(os.Args) >= 3 && !strings.HasPrefix(os.Args[2], "-") {
			// Positional shorthand: toollab gen <openapi-spec>
			genArgs = []string{"--from", "openapi", "--openapi-file", os.Args[2]}
			genArgs = append(genArgs, os.Args[3:]...)
		}
		if err := handleGenerate(genArgs); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "enrich":
		if err := handleEnrich(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "map":
		if err := handleMap(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "explain":
		if err := handleExplain(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "diff":
		if err := handleDiff(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "interpret":
		if err := handleInterpret(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "audit":
		if err := handleAudit(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "coverage":
		if err := handleCoverage(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "comprehend":
		if err := handleComprehend(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func handleRun(args []string) {
	scenarioPath, outDir, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		fmt.Fprintln(os.Stderr, "usage: toollab run <scenario.yaml> [--out DIR]")
		os.Exit(2)
	}

	result, err := app.RunScenario(context.Background(), scenarioPath, outDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Printf("run_dir=%s\n", result.RunDir)
	fmt.Printf("decision_tape_hash=%s\n", result.Bundle.Execution.DecisionTapeHash)
	fmt.Printf("deterministic_fingerprint=%s\n", result.Bundle.DeterministicFingerprint)
}

func parseRunArgs(args []string) (string, string, error) {
	outDir := "golden_runs"
	scenarioPath := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("--out requires a value")
			}
			outDir = args[i]
		case strings.HasPrefix(arg, "--out="):
			outDir = strings.TrimPrefix(arg, "--out=")
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown flag %s", arg)
		default:
			if scenarioPath != "" {
				return "", "", fmt.Errorf("multiple scenario paths provided")
			}
			scenarioPath = arg
		}
	}
	if scenarioPath == "" {
		return "", "", fmt.Errorf("missing scenario path")
	}
	return scenarioPath, outDir, nil
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func usage() {
	fmt.Println("toollab usage:")
	fmt.Println("  toollab run <scenario.yaml> [--out DIR]")
	fmt.Println("  toollab generate --from openapi|toollab [flags]")
	fmt.Println("  toollab enrich <scenario.yaml> --from openapi|toollab [flags]")
	fmt.Println("  toollab map --from openapi|toollab [flags]")
	fmt.Println("  toollab explain <run_dir> [flags]")
	fmt.Println("  toollab interpret <run_dir>              (LLM narrative via Ollama)")
	fmt.Println("  toollab audit <run_dir>                  (security audit)")
	fmt.Println("  toollab coverage <run_dir>               (endpoint coverage report)")
	fmt.Println("  toollab comprehend <run_dir>             (full comprehension report)")
	fmt.Println("  toollab diff <runA_dir> <runB_dir> [flags]")
	fmt.Println("  toollab gen <openapi-spec> [flags] (alias)")
}

func handleGenerate(args []string) error {
	cmd := flag.NewFlagSet("generate", flag.ContinueOnError)
	from := cmd.String("from", "", "source type: openapi|toollab")
	openapiURL := cmd.String("openapi-url", "", "OpenAPI URL")
	openapiFile := cmd.String("openapi-file", "", "OpenAPI file path")
	openapiAuth := cmd.String("openapi-auth", "", "auth for openapi discovery (bearer:ENV or api_key:ENV:header|query:NAME)")
	targetBaseURL := cmd.String("target-base-url", "", "target base URL")
	toollabURL := cmd.String("toollab-url", "", "toollab adapter base URL")
	toollabAuth := cmd.String("toollab-auth", "", "auth for toollab discovery (bearer:ENV or api_key:ENV:header|query:NAME)")
	outPath := cmd.String("out", "scenarios/scenario.yaml", "scenario output path")
	seed := cmd.String("seed", "", "deterministic seed (decimal string)")
	mode := cmd.String("mode", "smoke", "generation mode: smoke|load|chaos")
	baseURLOverride := cmd.String("base-url", "", "override target base URL")
	prefer := cmd.String("prefer", "profile", "toollab discovery preference: profile|endpoints")
	flowSource := cmd.String("flow-source", "suggested_flows", "flow source: suggested_flows|openapi_fallback|manual")
	var requireCaps stringSlice
	cmd.Var(&requireCaps, "require-capability", "required toollab capability (repeatable)")
	printOut := cmd.Bool("print", false, "print scenario to stdout")
	dryRun := cmd.Bool("dry-run", false, "run generation without writing files")
	if err := cmd.Parse(args); err != nil {
		return err
	}
	res, err := app.GenerateScenario(context.Background(), app.GenerateConfig{
		From:                 *from,
		OpenAPIURL:           *openapiURL,
		OpenAPIFile:          *openapiFile,
		OpenAPIAuthFlag:      *openapiAuth,
		TargetBaseURL:        *targetBaseURL,
		ToollabURL:           *toollabURL,
		ToollabAuthFlag:      *toollabAuth,
		OutPath:              *outPath,
		Seed:                 *seed,
		Mode:                 *mode,
		BaseURLOverride:      *baseURLOverride,
		Prefer:               *prefer,
		FlowSource:           *flowSource,
		RequiredCapabilities: []string(requireCaps),
		Print:                *printOut,
		DryRun:               *dryRun,
	})
	if err != nil {
		return err
	}
	if *printOut {
		_, _ = os.Stdout.Write(res.ScenarioYAML)
		if len(res.MetaJSON) > 0 {
			_, _ = os.Stderr.Write(res.MetaJSON)
		}
		return nil
	}
	if *dryRun {
		_, _ = os.Stdout.Write(res.MetaJSON)
		return nil
	}
	fmt.Printf("scenario_path=%s\n", res.OutPath)
	fmt.Printf("meta_path=%s\n", res.MetaPath)
	fmt.Printf("scenario_sha256=%s\n", res.ScenarioSHA)
	fmt.Printf("meta_fingerprint=%s\n", res.MetaFP)
	return nil
}

func handleEnrich(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab enrich <scenario.yml> [flags]")
	}
	baseScenarioPath := ""
	flagArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		baseScenarioPath = args[0]
		flagArgs = args[1:]
	}

	cmd := flag.NewFlagSet("enrich", flag.ContinueOnError)
	var fromValues stringSlice
	cmd.Var(&fromValues, "from", "enrichment source: openapi|toollab (repeatable)")

	openapiURL := cmd.String("openapi-url", "", "OpenAPI URL")
	openapiFile := cmd.String("openapi-file", "", "OpenAPI file")
	openapiAuth := cmd.String("openapi-auth", "", "auth for openapi discovery")
	targetBaseURL := cmd.String("target-base-url", "", "target base URL")
	toollabURL := cmd.String("toollab-url", "", "toollab adapter URL")
	toollabAuth := cmd.String("toollab-auth", "", "auth for toollab discovery")
	outPath := cmd.String("out", "scenarios/enriched.yaml", "output enriched scenario path")
	seed := cmd.String("seed", "", "deterministic seed")
	mergeStrategy := cmd.String("merge-strategy", "conservative", "merge strategy: conservative|aggressive")
	printOut := cmd.Bool("print", false, "print scenario")
	dryRun := cmd.Bool("dry-run", false, "run without writing files")
	if err := cmd.Parse(flagArgs); err != nil {
		return err
	}
	if baseScenarioPath == "" {
		rest := cmd.Args()
		if len(rest) != 1 {
			return fmt.Errorf("usage: toollab enrich <scenario.yml> [flags]")
		}
		baseScenarioPath = rest[0]
	}

	useOpenAPI := false
	useToollab := false
	for _, from := range fromValues {
		switch from {
		case "openapi":
			useOpenAPI = true
		case "toollab":
			useToollab = true
		default:
			return fmt.Errorf("invalid --from value %q (expected openapi or toollab)", from)
		}
	}
	if !useOpenAPI && !useToollab {
		return fmt.Errorf("at least one enrichment source is required: --from openapi and/or --from toollab")
	}

	res, err := app.EnrichScenario(context.Background(), app.EnrichConfig{
		BaseScenarioPath: baseScenarioPath,
		UseOpenAPI:       useOpenAPI,
		OpenAPIURL:       *openapiURL,
		OpenAPIFile:      *openapiFile,
		OpenAPIAuthFlag:  *openapiAuth,
		UseToollab:       useToollab,
		TargetBaseURL:    *targetBaseURL,
		ToollabURL:       *toollabURL,
		ToollabAuthFlag:  *toollabAuth,
		Seed:             *seed,
		OutPath:          *outPath,
		MergeStrategy:    enrich.Strategy(*mergeStrategy),
		Print:            *printOut,
		DryRun:           *dryRun,
	})
	if err != nil {
		return err
	}
	if *printOut {
		_, _ = os.Stdout.Write(res.ScenarioYAML)
		_, _ = os.Stderr.Write(res.MetaJSON)
		return nil
	}
	if *dryRun {
		_, _ = os.Stdout.Write(res.MetaJSON)
		return nil
	}
	fmt.Printf("scenario_path=%s\n", res.OutPath)
	fmt.Printf("meta_path=%s\n", res.MetaPath)
	fmt.Printf("scenario_sha256=%s\n", res.ScenarioSHA)
	fmt.Printf("meta_fingerprint=%s\n", res.MetaFP)
	return nil
}

func handleMap(args []string) error {
	cmd := flag.NewFlagSet("map", flag.ContinueOnError)
	from := cmd.String("from", "", "source: openapi|toollab")
	openapiURL := cmd.String("openapi-url", "", "OpenAPI URL")
	openapiFile := cmd.String("openapi-file", "", "OpenAPI file")
	openapiAuth := cmd.String("openapi-auth", "", "auth for openapi discovery")
	targetBaseURL := cmd.String("target-base-url", "", "target base URL")
	toollabURL := cmd.String("toollab-url", "", "toollab adapter URL")
	toollabAuth := cmd.String("toollab-auth", "", "auth for toollab discovery")
	out := cmd.String("out", "", "output directory")
	seed := cmd.String("seed", "", "deterministic seed")
	printOut := cmd.Bool("print", false, "print map json and md")
	dryRun := cmd.Bool("dry-run", false, "compute only")
	if err := cmd.Parse(args); err != nil {
		return err
	}
	input := *openapiFile
	if input == "" {
		input = *openapiURL
	}
	res, err := app.BuildSystemMap(context.Background(), app.MapConfig{
		From:            *from,
		OpenAPIInput:    input,
		OpenAPIAuthFlag: *openapiAuth,
		TargetBaseURL:   *targetBaseURL,
		ToollabURL:      *toollabURL,
		ToollabAuthFlag: *toollabAuth,
		OutPath:         *out,
		Seed:            *seed,
		Print:           *printOut,
		DryRun:          *dryRun,
	})
	if err != nil {
		return err
	}
	if *printOut || *dryRun {
		_, _ = os.Stdout.Write(res.SystemMapJSON)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stdout.Write(res.SystemMapMD)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stderr.Write(res.MetaJSON)
		return nil
	}
	fmt.Printf("system_map_fingerprint=%s\n", res.MapFP)
	fmt.Printf("map_meta_path=%s\n", res.MetaPath)
	return nil
}

func handleExplain(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab explain <run_dir> [flags]")
	}
	runDir := ""
	flagArgs := args
	if !strings.HasPrefix(args[0], "-") {
		runDir = args[0]
		flagArgs = args[1:]
	}

	cmd := flag.NewFlagSet("explain", flag.ContinueOnError)
	withDiscovery := cmd.String("with-discovery", "", "directory containing discovery artifacts")
	out := cmd.String("out", "", "output directory")
	printOut := cmd.Bool("print", false, "print outputs")
	dryRun := cmd.Bool("dry-run", false, "compute only")
	llm := cmd.String("llm", "off", "llm mode: off|on (narrative-only)")
	if err := cmd.Parse(flagArgs); err != nil {
		return err
	}
	if runDir == "" {
		rest := cmd.Args()
		if len(rest) != 1 {
			return fmt.Errorf("usage: toollab explain <run_dir> [flags]")
		}
		runDir = rest[0]
	}
	res, err := app.ExplainRun(context.Background(), app.ExplainConfig{
		RunDir:       runDir,
		DiscoveryDir: *withDiscovery,
		OutDir:       *out,
		Print:        *printOut,
		DryRun:       *dryRun,
		LLMMode:      *llm,
	})
	if err != nil {
		return err
	}
	if *printOut || *dryRun {
		_, _ = os.Stdout.Write(res.UnderstandingJSON)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stdout.Write(res.UnderstandingMD)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stderr.Write(res.MetaJSON)
		return nil
	}
	fmt.Printf("understanding_fingerprint=%s\n", res.Fingerprint)
	fmt.Printf("explain_meta_path=%s\n", res.MetaPath)
	return nil
}

func handleDiff(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: toollab diff <runA_dir> <runB_dir> [flags]")
	}
	runA := ""
	runB := ""
	flagArgs := args
	if !strings.HasPrefix(args[0], "-") && len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		runA = args[0]
		runB = args[1]
		flagArgs = args[2:]
	}

	cmd := flag.NewFlagSet("diff", flag.ContinueOnError)
	out := cmd.String("out", "", "output directory")
	printOut := cmd.Bool("print", false, "print outputs")
	dryRun := cmd.Bool("dry-run", false, "compute only")
	llm := cmd.String("llm", "off", "llm mode: off|on (narrative-only)")
	if err := cmd.Parse(flagArgs); err != nil {
		return err
	}
	if runA == "" || runB == "" {
		rest := cmd.Args()
		if len(rest) != 2 {
			return fmt.Errorf("usage: toollab diff <runA_dir> <runB_dir> [flags]")
		}
		runA = rest[0]
		runB = rest[1]
	}
	res, err := app.DiffRuns(context.Background(), app.DiffConfig{
		RunADir: runA,
		RunBDir: runB,
		OutDir:  *out,
		Print:   *printOut,
		DryRun:  *dryRun,
		LLMMode: *llm,
	})
	if err != nil {
		return err
	}
	if *printOut || *dryRun {
		_, _ = os.Stdout.Write(res.DiffJSON)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stdout.Write(res.DiffMD)
		_, _ = os.Stdout.Write([]byte("\n"))
		_, _ = os.Stderr.Write(res.MetaJSON)
		return nil
	}
	fmt.Printf("diff_fingerprint=%s\n", res.Fingerprint)
	fmt.Printf("diff_meta_path=%s\n", res.MetaPath)
	return nil
}

func handleInterpret(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab interpret <run_dir>")
	}
	runDir := args[0]

	provider := llm.NewProvider()
	if !provider.Available(context.Background()) {
		return fmt.Errorf("llm provider %q not available — check config (LLM_PROVIDER, GEMINI_API_KEY, OLLAMA_URL, etc.)", provider.Name())
	}

	evidencePath := filepath.Join(runDir, "evidence.json")
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return fmt.Errorf("read evidence: %w", err)
	}

	var bundle evidence.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("parse evidence: %w", err)
	}

	fmt.Fprintf(os.Stderr, "interpretando run %s con %s...\n", bundle.Metadata.RunID, provider.Name())

	summary := buildFullInterpretContext(runDir, &bundle)
	narrative, err := provider.Interpret(context.Background(), summary)
	if err != nil {
		return fmt.Errorf("llm generation failed: %w", err)
	}

	fmt.Println(narrative)
	return nil
}

func buildFullInterpretContext(runDir string, b *evidence.Bundle) string {
	var sb strings.Builder

	sb.WriteString("# Datos de la auditoría\n\n")
	sb.WriteString(fmt.Sprintf("Escenario: %s\n", b.ScenarioFingerprint.ScenarioPath))
	sb.WriteString(fmt.Sprintf("Modo: %s | Concurrencia: %d | Duración: %ds\n", b.Metadata.Mode, b.Execution.Concurrency, b.Execution.DurationS))
	sb.WriteString(fmt.Sprintf("Total requests: %d | Completados: %d\n", b.Stats.TotalRequests, b.Execution.CompletedRequests))
	sb.WriteString(fmt.Sprintf("Veredicto: %s\n", b.Assertions.Overall))
	sb.WriteString(fmt.Sprintf("Tasa de éxito: %.2f%% | Tasa de error: %.2f%%\n", b.Stats.SuccessRate*100, b.Stats.ErrorRate*100))
	sb.WriteString(fmt.Sprintf("Latencia P50: %dms | P95: %dms | P99: %dms\n\n", b.Stats.P50MS, b.Stats.P95MS, b.Stats.P99MS))

	sb.WriteString("## Reglas evaluadas\n")
	for _, rule := range b.Assertions.Rules {
		status := "OK"
		if !rule.Passed {
			status = "FALLA"
		}
		sb.WriteString(fmt.Sprintf("  [%s] %s — %s (observado: %v, umbral: %v)\n", status, rule.ID, rule.Message, rule.Observed, rule.Expected))
	}

	type flowStats struct {
		method, path                string
		total, errors, totalLatency int
		byStatus                    map[int]int
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

	sb.WriteString(fmt.Sprintf("\n## Endpoints probados (%d)\n\n", len(flows)))
	sb.WriteString("| Método | Path | Requests | Latencia prom. | Error%% | Status codes |\n")
	sb.WriteString("|--------|------|----------|----------------|--------|--------------|\n")
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
		sb.WriteString(fmt.Sprintf("| %s | %s | %d | %dms | %.0f%% | %s |\n",
			fs.method, fs.path, fs.total, avgLat, errPct, strings.Join(statuses, ", ")))
	}

	if len(b.Samples) > 0 {
		sb.WriteString("\n## Muestras de request/response\n\n")
		seen := map[string]bool{}
		count := 0
		for _, s := range b.Samples {
			if count >= 20 {
				break
			}
			key := s.Request.Method + " " + s.Request.URL
			if seen[key] {
				continue
			}
			seen[key] = true
			count++
			sc := "N/A"
			if s.Response.StatusCode != nil {
				sc = fmt.Sprintf("%d", *s.Response.StatusCode)
			}
			sb.WriteString(fmt.Sprintf("### %s %s → %s\n", s.Request.Method, s.Request.URL, sc))
			if s.Request.BodyPreview != "" {
				body := s.Request.BodyPreview
				if len(body) > 300 {
					body = body[:300] + "…"
				}
				sb.WriteString(fmt.Sprintf("Request body: ```%s```\n", body))
			}
			if s.Response.BodyPreview != "" {
				body := s.Response.BodyPreview
				if len(body) > 300 {
					body = body[:300] + "…"
				}
				sb.WriteString(fmt.Sprintf("Response body: ```%s```\n", body))
			}
			sb.WriteString("\n")
		}
	}

	appendArtifact(&sb, runDir, "service_description.json", "Descripción del servicio")
	appendArtifact(&sb, runDir, "security_audit.json", "Auditoría de seguridad")
	appendArtifact(&sb, runDir, "coverage_report.json", "Cobertura de endpoints")
	appendArtifact(&sb, runDir, "contract_validation.json", "Validación de contratos")

	return sb.String()
}

func appendArtifact(sb *strings.Builder, runDir, filename, title string) {
	raw, err := os.ReadFile(filepath.Join(runDir, filename))
	if err != nil {
		return
	}
	content := string(raw)
	if len(content) > 3000 {
		content = content[:3000] + "\n… (truncado)"
	}
	sb.WriteString(fmt.Sprintf("\n## %s\n```json\n%s\n```\n", title, content))
}

func handleAudit(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab audit <run_dir>")
	}
	runDir := args[0]

	evidencePath := filepath.Join(runDir, "evidence.json")
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return fmt.Errorf("read evidence: %w", err)
	}

	var bundle evidence.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("parse evidence: %w", err)
	}

	report := security.Audit(&bundle)

	fmt.Printf("Security Audit — Grade: %s (Score: %d/100)\n\n", report.Grade, report.Score)
	fmt.Printf("Findings: %d total (%d critical, %d high, %d medium, %d low)\n\n",
		report.Summary.Total, report.Summary.Critical, report.Summary.High, report.Summary.Medium, report.Summary.Low)

	for _, f := range report.Findings {
		icon := "•"
		switch f.Severity {
		case "critical":
			icon = "🔴"
		case "high":
			icon = "🟠"
		case "medium":
			icon = "🟡"
		case "low":
			icon = "🔵"
		}
		fmt.Printf("%s [%s] %s\n", icon, f.Severity, f.Title)
		fmt.Printf("   %s\n", f.Description)
		if f.Endpoint != "" {
			fmt.Printf("   Endpoint: %s\n", f.Endpoint)
		}
		fmt.Printf("   Remediación: %s\n\n", f.Remediation)
	}

	if len(report.Findings) == 0 {
		fmt.Println("No se encontraron hallazgos de seguridad.")
	}

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	outPath := filepath.Join(runDir, "security_audit.json")
	if err := os.WriteFile(outPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write audit report: %w", err)
	}
	fmt.Printf("Reporte guardado en: %s\n", outPath)
	return nil
}

func handleCoverage(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab coverage <run_dir>")
	}
	runDir := args[0]

	evidencePath := filepath.Join(runDir, "evidence.json")
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return fmt.Errorf("read evidence: %w", err)
	}

	var bundle evidence.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("parse evidence: %w", err)
	}

	scenarioPath := bundle.ScenarioFingerprint.ScenarioPath
	scn, _, err := scenario.Load(scenarioPath)
	if err != nil {
		return fmt.Errorf("load scenario for coverage analysis: %w (path: %s)", err, scenarioPath)
	}

	report := coverage.Analyze(scn, &bundle)

	fmt.Println(coverage.RenderMarkdown(report))

	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	outPath := filepath.Join(runDir, "coverage_report.json")
	if err := os.WriteFile(outPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write coverage report: %w", err)
	}
	fmt.Printf("Reporte guardado en: %s\n", outPath)

	contractReport := contract.Validate(&bundle)
	contractJSON, _ := json.MarshalIndent(contractReport, "", "  ")
	contractPath := filepath.Join(runDir, "contract_validation.json")
	if err := os.WriteFile(contractPath, contractJSON, 0o644); err != nil {
		return fmt.Errorf("write contract report: %w", err)
	}

	fmt.Printf("\nValidación de Contratos: %s\n", func() string {
		if contractReport.Compliant {
			return "CONFORME"
		}
		return "NO CONFORME"
	}())
	fmt.Printf("Compliance Rate: %.1f%% (%d violaciones de %d checks)\n",
		contractReport.ComplianceRate*100, contractReport.TotalViolations, contractReport.TotalChecks)

	if len(contractReport.Violations) > 0 {
		fmt.Println("\nViolaciones detectadas:")
		for _, v := range contractReport.Violations {
			fmt.Printf("  • [%d] %s — %s\n", v.StatusCode, v.Endpoint, v.Description)
			fmt.Printf("    Esperado: %s | Actual: %s\n", v.Expected, v.Actual)
		}
	}

	fmt.Printf("\nReporte de contratos guardado en: %s\n", contractPath)
	return nil
}

func handleComprehend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: toollab comprehend <run_dir>")
	}
	runDir := args[0]

	evidencePath := filepath.Join(runDir, "evidence.json")
	raw, err := os.ReadFile(evidencePath)
	if err != nil {
		return fmt.Errorf("read evidence: %w", err)
	}

	var bundle evidence.Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return fmt.Errorf("parse evidence: %w", err)
	}

	scenarioPath := bundle.ScenarioFingerprint.ScenarioPath
	var scn *scenario.Scenario
	if scenarioPath != "" {
		loaded, _, err := scenario.Load(scenarioPath)
		if err == nil {
			scn = loaded
		}
	}

	systemMap := mapmodel.FromEvidence(&bundle)

	report := comprehension.Build(&bundle, scn, systemMap, nil)

	md := comprehension.RenderMarkdown(report)
	fmt.Println(md)

	reportJSON, _, err := comprehension.WriteCanonical(report)
	if err != nil {
		return fmt.Errorf("write comprehension: %w", err)
	}
	outJSON := filepath.Join(runDir, "comprehension.json")
	if err := os.WriteFile(outJSON, reportJSON, 0o644); err != nil {
		return fmt.Errorf("write comprehension json: %w", err)
	}
	outMD := filepath.Join(runDir, "comprehension.md")
	if err := os.WriteFile(outMD, []byte(md), 0o644); err != nil {
		return fmt.Errorf("write comprehension md: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\nReportes guardados en:\n  %s\n  %s\n", outJSON, outMD)
	return nil
}

// loadEnvFiles loads variables from .env files without overwriting
// existing environment variables. It searches: .env in the current
// directory, then walks up to 5 parent directories looking for .env.
func loadEnvFiles() {
	paths := []string{".env"}

	dir, err := os.Getwd()
	if err == nil {
		for i := 0; i < 5; i++ {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
			candidate := filepath.Join(dir, ".env")
			paths = append(paths, candidate)
		}
	}

	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			value = strings.Trim(value, "'\"")
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
}
