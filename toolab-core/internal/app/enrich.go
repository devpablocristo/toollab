package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"toolab-core/internal/discovery"
	"toolab-core/internal/enrich"
	"toolab-core/internal/gen"
	"toolab-core/internal/generate"
	"toolab-core/internal/meta"
	"toolab-core/internal/scenario"
	scenariowrite "toolab-core/internal/scenario/write"
)

type EnrichConfig struct {
	BaseScenarioPath string
	UseOpenAPI       bool
	OpenAPIURL       string
	OpenAPIFile      string
	OpenAPIAuthFlag  string
	UseToolab        bool
	TargetBaseURL    string
	ToolabURL        string
	ToolabAuthFlag   string
	Seed             string
	OutPath          string
	MergeStrategy    enrich.Strategy
	Print            bool
	DryRun           bool
	ToolabVersion    string
}

type EnrichResult struct {
	ScenarioYAML []byte
	ScenarioSHA  string
	MetaJSON     []byte
	MetaFP       string
	OutPath      string
	MetaPath     string
}

func EnrichScenario(ctx context.Context, cfg EnrichConfig) (*EnrichResult, error) {
	if cfg.ToolabVersion == "" {
		cfg.ToolabVersion = defaultToolabVersion
	}
	if !cfg.Print && cfg.OutPath == "" {
		return nil, fmt.Errorf("--out is required unless --print is used")
	}
	base, baseFP, err := scenario.Load(cfg.BaseScenarioPath)
	if err != nil {
		return nil, err
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

	inputs := map[string]string{
		"base_scenario_sha256": baseFP.ScenarioSHA,
	}
	warnings := []string{}
	unknowns := []string{}
	hashes := meta.HashesInfo{
		BaseScenarioSHA256: baseFP.ScenarioSHA,
	}

	var fromOpenAPI *scenario.Scenario
	var fromToolab *scenario.Scenario
	var openAPIDoc *gen.OpenAPIDoc

	if cfg.UseOpenAPI {
		input := cfg.OpenAPIFile
		if input == "" {
			input = cfg.OpenAPIURL
		}
		if input == "" {
			return nil, fmt.Errorf("--from openapi requires --openapi-file or --openapi-url")
		}
		doc, hash, info, warn, fetchErr := openapiFetcher.Fetch(ctx, input, openapiAuth)
		if fetchErr != nil {
			return nil, fetchErr
		}
		hashes.OpenAPISHA256 = hash
		inputs["openapi_source"] = info.Source + ":" + info.URL + ":" + info.File + ":" + info.Hash
		warnings = append(warnings, warn...)
		openAPIDoc = doc
	}

	if cfg.UseToolab {
		if cfg.TargetBaseURL == "" {
			return nil, fmt.Errorf("--from toolab requires --target-base-url")
		}
		manifest, manifestHash, manifestWarn, mErr := toolabFetcher.Manifest(ctx, toolabAuth)
		if mErr != nil {
			return nil, mErr
		}
		warnings = append(warnings, manifestWarn...)
		hashes.ManifestSHA256 = manifestHash
		inputs["manifest_sha256"] = manifestHash
		if containsCap(manifest.Capabilities, "profile") {
			_, profileHash, profileWarn, pErr := toolabFetcher.Profile(ctx, toolabAuth)
			warnings = append(warnings, profileWarn...)
			if pErr == nil {
				hashes.ProfileSHA256 = profileHash
				inputs["profile_sha256"] = profileHash
			} else {
				warnings = append(warnings, "profile not available during seed derivation")
			}
		}
	}

	seedInput, effectiveSeed, derivation, err := resolveEnrichSeed(cfg.Seed, inputs, cfg)
	if err != nil {
		return nil, err
	}

	if cfg.UseOpenAPI {
		if openAPIDoc == nil {
			return nil, fmt.Errorf("openapi document unavailable")
		}
		var warningsFromOpenAPI []string
		var buildErr error
		fromOpenAPI, warningsFromOpenAPI, buildErr = generate.BuildFromOpenAPIDoc(openAPIDoc, generate.OpenAPIOptions{
			Mode:          "smoke",
			BaseURL:       base.Target.BaseURL,
			EffectiveSeed: effectiveSeed,
		})
		if buildErr != nil {
			return nil, buildErr
		}
		warnings = append(warnings, warningsFromOpenAPI...)
	}

	if cfg.UseToolab {
		build, bErr := generate.BuildFromToolab(ctx, toolabFetcher, openapiFetcher, toolabAuth, generate.ToolabOptions{
			TargetBaseURL: cfg.TargetBaseURL,
			ToolabURL:     toolabBase,
			Prefer:        "profile",
			FlowSource:    "suggested_flows",
			Mode:          "smoke",
			EffectiveSeed: effectiveSeed,
		})
		if bErr != nil {
			return nil, bErr
		}
		fromToolab = build.Scenario
		warnings = append(warnings, build.Warnings...)
		unknowns = append(unknowns, build.Unknowns...)
		hashes.ManifestSHA256 = build.ManifestHash
		hashes.ProfileSHA256 = build.ProfileHash
	}

	mergeResult, err := enrich.Enrich(enrich.Inputs{
		Base:          base,
		FromToolab:    fromToolab,
		FromOpenAPI:   fromOpenAPI,
		Strategy:      cfg.MergeStrategy,
		EffectiveSeed: effectiveSeed,
	})
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, mergeResult.Warnings...)

	scenarioYAML, scenarioSHA, err := scenariowrite.WriteCanonicalScenario(mergeResult.Scenario)
	if err != nil {
		return nil, err
	}
	hashes.OutputScenarioSHA = scenarioSHA

	metaChanges := make([]meta.Change, 0, len(mergeResult.Changes))
	for _, c := range mergeResult.Changes {
		metaChanges = append(metaChanges, meta.Change{
			Op:         c.Op,
			Path:       c.Path,
			Reason:     c.Reason,
			Source:     c.Source,
			BeforeHash: c.BeforeHash,
			AfterHash:  c.AfterHash,
		})
	}

	metaDoc := meta.Document{
		Operation:     "enrich",
		ToolabVersion: cfg.ToolabVersion,
		Seed: meta.SeedInfo{
			Provided:   cfg.Seed != "",
			InputSeed:  seedInput,
			Effective:  effectiveSeed,
			Derivation: derivation,
		},
		Source: meta.SourceInfo{
			Primary:   "enrich",
			Secondary: []string{},
			Inputs:    sortedInputList(inputs),
		},
		Hashes: hashes,
		Options: map[string]any{
			"merge_strategy": cfg.MergeStrategy,
			"print":          cfg.Print,
			"dry_run":        cfg.DryRun,
			"use_openapi":    cfg.UseOpenAPI,
			"use_toolab":     cfg.UseToolab,
		},
		Capabilities: meta.CapabilityInfo{
			Declared:        []string{},
			Used:            []string{},
			MissingRequired: []string{},
		},
		Warnings: meta.SortStrings(warnings),
		Unknowns: meta.SortStrings(unknowns),
		Changes:  metaChanges,
	}
	metaJSON, metaFP, err := meta.WriteCanonical(metaDoc)
	if err != nil {
		return nil, err
	}

	result := &EnrichResult{
		ScenarioYAML: scenarioYAML,
		ScenarioSHA:  scenarioSHA,
		MetaJSON:     metaJSON,
		MetaFP:       metaFP,
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

func resolveEnrichSeed(userSeed string, inputs map[string]string, cfg EnrichConfig) (string, string, string, error) {
	if userSeed != "" {
		return userSeed, userSeed, "provided", nil
	}
	options := map[string]any{
		"merge_strategy":  cfg.MergeStrategy,
		"use_openapi":     cfg.UseOpenAPI,
		"use_toolab":      cfg.UseToolab,
		"openapi_url":     cfg.OpenAPIURL,
		"openapi_file":    cfg.OpenAPIFile,
		"target_base_url": cfg.TargetBaseURL,
		"toolab_url":      cfg.ToolabURL,
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

func uniqueSortedValues(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
