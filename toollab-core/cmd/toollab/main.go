package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"toollab-core/internal/app"
	"toollab-core/internal/enrich"
)

func main() {
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
			// Legacy style: toollab gen <openapi-spec>
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
	outPath := cmd.String("out", "", "scenario output path")
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
		ToollabURL:            *toollabURL,
		ToollabAuthFlag:       *toollabAuth,
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
	// backward-compatible booleans
	useOpenAPILegacy := cmd.Bool("from-openapi", false, "enable openapi enrichment")
	useToollabLegacy := cmd.Bool("from-toollab", false, "enable toollab enrichment")

	openapiURL := cmd.String("openapi-url", "", "OpenAPI URL")
	openapiFile := cmd.String("openapi-file", "", "OpenAPI file")
	openapiAuth := cmd.String("openapi-auth", "", "auth for openapi discovery")
	targetBaseURL := cmd.String("target-base-url", "", "target base URL")
	toollabURL := cmd.String("toollab-url", "", "toollab adapter URL")
	toollabAuth := cmd.String("toollab-auth", "", "auth for toollab discovery")
	outPath := cmd.String("out", "", "output enriched scenario path")
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

	useOpenAPI := *useOpenAPILegacy
	useToollab := *useToollabLegacy
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
		UseToollab:        useToollab,
		TargetBaseURL:    *targetBaseURL,
		ToollabURL:        *toollabURL,
		ToollabAuthFlag:   *toollabAuth,
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
		ToollabURL:       *toollabURL,
		ToollabAuthFlag:  *toollabAuth,
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
