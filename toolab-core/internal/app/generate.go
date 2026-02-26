package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"toolab-core/internal/discovery"
	"toolab-core/internal/generate"
	"toolab-core/internal/meta"
	"toolab-core/internal/scenario"
	scenariowrite "toolab-core/internal/scenario/write"
)

type GenerateConfig struct {
	From                 string
	OpenAPIURL           string
	OpenAPIFile          string
	OpenAPIAuthFlag      string
	TargetBaseURL        string
	ToolabURL            string
	ToolabAuthFlag       string
	OutPath              string
	Seed                 string
	Mode                 string
	BaseURLOverride      string
	Prefer               string
	FlowSource           string
	RequiredCapabilities []string
	Print                bool
	DryRun               bool
	ToolabVersion        string
}

type GenerateResult struct {
	ScenarioYAML []byte
	ScenarioSHA  string
	MetaJSON     []byte
	MetaFP       string
	Warnings     []string
	Unknowns     []string
	OutPath      string
	MetaPath     string
}

func GenerateScenario(ctx context.Context, cfg GenerateConfig) (*GenerateResult, error) {
	if cfg.Mode == "" {
		cfg.Mode = "smoke"
	}
	if cfg.ToolabVersion == "" {
		cfg.ToolabVersion = defaultToolabVersion
	}
	if !cfg.Print && cfg.OutPath == "" {
		return nil, fmt.Errorf("--out is required unless --print is used")
	}
	if cfg.From != "openapi" && cfg.From != "toolab" {
		return nil, fmt.Errorf("--from must be openapi or toolab")
	}

	openapiAuth, err := discovery.ParseAuthFlag(cfg.OpenAPIAuthFlag)
	if err != nil {
		return nil, err
	}
	toolabAuth, err := discovery.ParseAuthFlag(cfg.ToolabAuthFlag)
	if err != nil {
		return nil, err
	}

	openapiFetcher := discovery.NewOpenAPIFetcher(discovery.HTTPConfig{})
	toolabBase := cfg.ToolabURL
	if toolabBase == "" && cfg.TargetBaseURL != "" {
		toolabBase = strings.TrimRight(cfg.TargetBaseURL, "/") + "/_toolab"
	}
	toolabFetcher := discovery.NewToolabFetcher(toolabBase, discovery.HTTPConfig{})

	var (
		scn            = (*scenario.Scenario)(nil)
		warnings       []string
		unknowns       []string
		openapiHash    string
		manifestHash   string
		profileHash    string
		declaredCaps   []string
		usedCaps       []string
		effectiveSeed  string
		seedDerivation string
		inputSeed      string
		inputs         = map[string]string{}
	)

	switch cfg.From {
	case "openapi":
		input := cfg.OpenAPIFile
		if input == "" {
			input = cfg.OpenAPIURL
		}
		if input == "" {
			return nil, fmt.Errorf("openapi source required: --openapi-file or --openapi-url")
		}
		doc, hash, info, warn, fErr := openapiFetcher.Fetch(ctx, input, openapiAuth)
		if fErr != nil {
			return nil, fErr
		}
		openapiHash = hash
		warnings = append(warnings, warn...)
		inputs["openapi_source"] = info.Source + ":" + info.URL + ":" + info.File + ":" + info.Hash

		inputSeed, effectiveSeed, seedDerivation, err = resolveSeed(cfg.Seed, inputs, cfg)
		if err != nil {
			return nil, err
		}
		scn, warn, err = generate.BuildFromOpenAPIDoc(doc, generate.OpenAPIOptions{
			Mode:          cfg.Mode,
			BaseURL:       cfg.BaseURLOverride,
			EffectiveSeed: effectiveSeed,
		})
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, warn...)
	case "toolab":
		if cfg.TargetBaseURL == "" {
			return nil, fmt.Errorf("--target-base-url is required for --from toolab")
		}

		manifest, mHash, warn, mErr := toolabFetcher.Manifest(ctx, toolabAuth)
		if mErr != nil {
			return nil, mErr
		}
		manifestHash = mHash
		warnings = append(warnings, warn...)
		declaredCaps = append([]string(nil), manifest.Capabilities...)
		sort.Strings(declaredCaps)
		inputs["manifest_sha256"] = manifestHash

		if containsCap(manifest.Capabilities, "profile") && cfg.Prefer != "endpoints" {
			_, pHash, pWarn, pErr := toolabFetcher.Profile(ctx, toolabAuth)
			if pErr == nil {
				profileHash = pHash
				inputs["profile_sha256"] = profileHash
				warnings = append(warnings, pWarn...)
			} else {
				warnings = append(warnings, "profile not available during seed derivation")
			}
		}

		inputSeed, effectiveSeed, seedDerivation, err = resolveSeed(cfg.Seed, inputs, cfg)
		if err != nil {
			return nil, err
		}
		build, bErr := generate.BuildFromToolab(ctx, toolabFetcher, openapiFetcher, toolabAuth, generate.ToolabOptions{
			TargetBaseURL:        cfg.TargetBaseURL,
			ToolabURL:            toolabBase,
			Prefer:               cfg.Prefer,
			FlowSource:           cfg.FlowSource,
			Mode:                 cfg.Mode,
			EffectiveSeed:        effectiveSeed,
			RequiredCapabilities: cfg.RequiredCapabilities,
		})
		if bErr != nil {
			return nil, bErr
		}
		scn = build.Scenario
		manifestHash = build.ManifestHash
		profileHash = build.ProfileHash
		openapiHash = build.OpenAPIHash
		warnings = append(warnings, build.Warnings...)
		unknowns = append(unknowns, build.Unknowns...)
		usedCaps = append(usedCaps, build.UsedCaps...)
		if len(declaredCaps) == 0 {
			declaredCaps = append(declaredCaps, build.DeclaredCaps...)
		}
	}

	scenarioYAML, scenarioSHA, err := scenariowrite.WriteCanonicalScenario(scn)
	if err != nil {
		return nil, err
	}

	metaDoc := meta.Document{
		Operation:     "generate",
		ToolabVersion: cfg.ToolabVersion,
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
		Hashes: meta.HashesInfo{
			OpenAPISHA256:     openapiHash,
			ManifestSHA256:    manifestHash,
			ProfileSHA256:     profileHash,
			OutputScenarioSHA: scenarioSHA,
		},
		Options: map[string]any{
			"mode":          cfg.Mode,
			"prefer":        cfg.Prefer,
			"flow_source":   cfg.FlowSource,
			"print":         cfg.Print,
			"dry_run":       cfg.DryRun,
			"base_override": cfg.BaseURLOverride,
			"from":          cfg.From,
		},
		Capabilities: meta.CapabilityInfo{
			Declared:        meta.SortStrings(declaredCaps),
			Used:            meta.SortStrings(usedCaps),
			MissingRequired: missingCapabilities(declaredCaps, cfg.RequiredCapabilities),
		},
		Warnings: meta.SortStrings(warnings),
		Unknowns: meta.SortStrings(unknowns),
	}
	metaJSON, metaFP, err := meta.WriteCanonical(metaDoc)
	if err != nil {
		return nil, err
	}

	result := &GenerateResult{
		ScenarioYAML: scenarioYAML,
		ScenarioSHA:  scenarioSHA,
		MetaJSON:     metaJSON,
		MetaFP:       metaFP,
		Warnings:     meta.SortStrings(warnings),
		Unknowns:     meta.SortStrings(unknowns),
	}

	if cfg.DryRun {
		return result, nil
	}
	if !cfg.Print {
		if err := os.MkdirAll(filepath.Dir(cfg.OutPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(cfg.OutPath, scenarioYAML, 0o644); err != nil {
			return nil, err
		}
		metaPath := filepath.Join(filepath.Dir(cfg.OutPath), "generate.meta.json")
		if err := os.WriteFile(metaPath, metaJSON, 0o644); err != nil {
			return nil, err
		}
		result.OutPath = filepath.Clean(cfg.OutPath)
		result.MetaPath = filepath.Clean(metaPath)
	}
	return result, nil
}

func resolveSeed(userSeed string, inputs map[string]string, cfg GenerateConfig) (string, string, string, error) {
	if userSeed != "" {
		return userSeed, userSeed, "provided", nil
	}
	options := map[string]any{
		"from":                  cfg.From,
		"mode":                  cfg.Mode,
		"prefer":                cfg.Prefer,
		"flow_source":           cfg.FlowSource,
		"merge_strategy":        "",
		"base_url_override":     cfg.BaseURLOverride,
		"target_base_url":       cfg.TargetBaseURL,
		"toolab_url":            cfg.ToolabURL,
		"required_capabilities": cfg.RequiredCapabilities,
	}
	seed, _, err := meta.DeriveSeed(meta.SeedInput{
		Inputs:  inputs,
		Options: options,
	})
	if err != nil {
		return "", "", "", err
	}
	return "", seed, "sha256(inputs_canonical)", nil
}

func sortedInputList(inputs map[string]string) []string {
	keys := make([]string, 0, len(inputs))
	for k := range inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+inputs[key])
	}
	return out
}

func containsCap(values []string, cap string) bool {
	for _, value := range values {
		if value == cap {
			return true
		}
	}
	return false
}

func missingCapabilities(declared, required []string) []string {
	if len(required) == 0 {
		return []string{}
	}
	decl := map[string]struct{}{}
	for _, item := range declared {
		decl[item] = struct{}{}
	}
	missing := []string{}
	for _, item := range required {
		if _, ok := decl[item]; !ok {
			missing = append(missing, item)
		}
	}
	sort.Strings(missing)
	return missing
}

func ReadJSONFile(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
