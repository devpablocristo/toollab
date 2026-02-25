package evidence

import (
	"runtime"
	"sort"

	"toolab-core/pkg/utils"
)

func BuildBundle(input CollectInput) (*Bundle, error) {
	outcomes := toOutcomes(input.Outcomes)
	sort.Slice(outcomes, func(i, j int) bool { return outcomes[i].Seq < outcomes[j].Seq })
	stats := BuildStats(outcomes)

	maxSamples := input.Redaction.MaxSamples
	if maxSamples <= 0 {
		maxSamples = 50
	}
	sampleIdx := SelectSampleIndexes(input.Outcomes, maxSamples, input.RunSeed)
	samples := make([]Sample, 0, len(sampleIdx))
	for _, idx := range sampleIdx {
		if idx < 0 || idx >= len(input.Outcomes) {
			continue
		}
		r := input.Outcomes[idx]
		samples = append(samples, buildSample(r, input.Redaction))
	}

	runID := ComputeRunID(input.ScenarioSHA256, input.RunSeed, input.ChaosSeed, input.DecisionEngineVersion)

	bundle := &Bundle{
		SchemaVersion: 1,
		Metadata: Metadata{
			ToolabVersion:   input.ToolabVersion,
			Mode:            input.Mode,
			RunID:           runID,
			RunSeed:         input.RunSeed,
			ChaosSeed:       input.ChaosSeed,
			DBSeedReference: input.DBSeedReference,
			StartedUTC:      input.StartedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			FinishedUTC:     input.FinishedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		},
		ScenarioFingerprint: ScenarioFingerprint{
			ScenarioPath:          input.ScenarioPath,
			ScenarioSHA256:        input.ScenarioSHA256,
			ScenarioSchemaVersion: input.ScenarioSchemaVersion,
		},
		Execution: Execution{
			ScheduleMode:          input.ScheduleMode,
			TickMS:                input.TickMS,
			Concurrency:           input.Concurrency,
			DurationS:             input.DurationS,
			PlannedRequests:       input.PlannedRequests,
			CompletedRequests:     input.CompletedRequests,
			DecisionEngineVersion: input.DecisionEngineVersion,
			DecisionTapeHash:      input.DecisionTapeHash,
		},
		Stats:            stats,
		Outcomes:         outcomes,
		Samples:          samples,
		Observability:    input.Observability,
		Assertions:       input.Assertions,
		Unknowns:         append([]string(nil), input.Unknowns...),
		RedactionSummary: input.Redaction,
		Repro: Repro{
			Command:    input.ReproCommand,
			ScriptPath: input.ReproScriptPath,
		},
		Environment: &Environment{
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
			HTTPClient: map[string]any{
				"redirects": "disabled",
			},
		},
	}

	fp, err := ComputeDeterministicFingerprint(bundle)
	if err != nil {
		return nil, err
	}
	bundle.DeterministicFingerprint = fp
	bundle.Repro.ExpectedDeterministicFingerprint = fp

	return bundle, nil
}

func toOutcomes(in []OutcomeInput) []Outcome {
	out := make([]Outcome, 0, len(in))
	for _, o := range in {
		out = append(out, Outcome{
			Seq:          o.Seq,
			RequestID:    o.RequestID,
			Method:       o.Method,
			Path:         o.Path,
			StatusCode:   o.StatusCode,
			ErrorKind:    o.ErrorKind,
			LatencyMS:    o.LatencyMS,
			ResponseHash: o.ResponseHash,
			ChaosApplied: ChaosApplied{
				LatencyInjectedMS:   o.ChaosApplied.LatencyInjectedMS,
				ErrorInjected:       o.ChaosApplied.ErrorInjected,
				ErrorMode:           o.ChaosApplied.ErrorMode,
				PayloadDriftApplied: o.ChaosApplied.PayloadDriftApplied,
				PayloadMutations:    cloneStrings(o.ChaosApplied.PayloadMutations),
			},
		})
	}
	return out
}

func buildSample(o OutcomeInput, redaction RedactionSummary) Sample {
	requestHeaders := RedactHeaders(o.RequestHeaders, redaction.HeadersRedacted, redaction.Mask)
	responseHeaders := RedactHeaders(o.ResponseHeaders, redaction.HeadersRedacted, redaction.Mask)
	requestPreview := RedactBodyPreview(o.RequestBody, redaction.JSONPathsRedacted, redaction.Mask, redaction.MaxBodyPreviewBytes)
	responsePreview := RedactBodyPreview(o.ResponseBody, redaction.JSONPathsRedacted, redaction.Mask, redaction.MaxBodyPreviewBytes)

	return Sample{
		Seq: o.Seq,
		Request: SampleRequest{
			Method:      o.Method,
			URL:         o.RequestURL,
			Headers:     requestHeaders,
			BodyPreview: requestPreview,
			BodySHA256:  utils.SHA256Hex(o.RequestBody),
			Redacted:    true,
		},
		Response: SampleResponse{
			StatusCode:  o.StatusCode,
			ErrorKind:   o.ErrorKind,
			Headers:     responseHeaders,
			BodyPreview: responsePreview,
			BodySHA256:  utils.SHA256Hex(o.ResponseBody),
			Redacted:    true,
		},
	}
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
