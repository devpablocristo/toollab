package evidence

import (
	"toollab-core/pkg/utils"
)

func ComputeRunID(scenarioSHA, runSeed, chaosSeed, decisionEngineVersion string) string {
	payload := []byte(scenarioSHA + ":" + runSeed + ":" + chaosSeed + ":" + decisionEngineVersion)
	return utils.SHA256Hex(payload)
}

func ComputeDeterministicFingerprint(bundle *Bundle) (string, error) {
	subset := map[string]any{
		"schema_version": bundle.SchemaVersion,
		"metadata": map[string]any{
			"toollab_version": bundle.Metadata.ToollabVersion,
			"mode":           bundle.Metadata.Mode,
			"run_id":         bundle.Metadata.RunID,
			"run_seed":       bundle.Metadata.RunSeed,
			"chaos_seed":     bundle.Metadata.ChaosSeed,
		},
		"scenario_fingerprint": bundle.ScenarioFingerprint,
		"execution":            bundle.Execution,
		"stats":                bundle.Stats,
		"outcomes":             bundle.Outcomes,
		"samples":              bundle.Samples,
		"assertions":           bundle.Assertions,
		"unknowns":             bundle.Unknowns,
		"redaction_summary":    bundle.RedactionSummary,
		"repro_command":        bundle.Repro.Command,
	}
	canonical, err := utils.CanonicalJSON(subset)
	if err != nil {
		return "", err
	}
	return utils.SHA256Hex(canonical), nil
}
