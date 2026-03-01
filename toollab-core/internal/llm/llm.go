package llm

import (
	"context"
	"encoding/json"
	"log"
	"time"

	artifactUC "toollab-core/internal/artifact/usecases"
	"toollab-core/internal/shared"
)

// Runner generates LLM docs and audit reports asynchronously.
type Runner struct {
	vertex      *VertexProvider
	artifactSvc *artifactUC.Service
}

func NewRunner(vertex *VertexProvider, artifactSvc *artifactUC.Service) *Runner {
	return &Runner{vertex: vertex, artifactSvc: artifactSvc}
}

// RunAsync generates both documentation and audit reports in parallel.
func (r *Runner) RunAsync(ctx context.Context, runID string, dossierLLMJSON []byte) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var dossierMeta struct {
		RunMode string `json:"run_mode"`
	}
	_ = json.Unmarshal(dossierLLMJSON, &dossierMeta)
	runMode := dossierMeta.RunMode

	type result struct {
		kind string
		data []byte
		err  error
	}

	ch := make(chan result, 2)

	for _, kind := range []string{"documentation", "audit"} {
		go func(k string) {
			prompt := buildPrompt(k, string(dossierLLMJSON), runMode)
			data, err := r.vertex.RawPrompt(ctx, prompt)
			ch <- result{kind: k, data: data, err: err}
		}(kind)
	}

	for i := 0; i < 2; i++ {
		res := <-ch
		artType := shared.ArtifactLLMDocs
		if res.kind == "audit" {
			artType = shared.ArtifactLLMAudit
		}

		if res.err != nil {
			log.Printf("LLM %s failed (run %s): %v", res.kind, runID, res.err)
			errJSON, _ := json.Marshal(map[string]string{"status": "failed", "error": res.err.Error()})
			r.artifactSvc.Put(runID, artType, errJSON)
		} else {
			log.Printf("LLM %s completed (run %s)", res.kind, runID)
			r.artifactSvc.Put(runID, artType, res.data)
		}
	}
}

func buildPrompt(kind, dossierJSON, runMode string) string {
	prefix := runModePrefix(kind, runMode)
	if kind == "documentation" {
		return prefix + docsPrompt + "\n\nDOSSIER:\n" + dossierJSON
	}
	return prefix + auditPrompt + "\n\nDOSSIER:\n" + dossierJSON
}

func runModePrefix(kind, runMode string) string {
	switch runMode {
	case "offline":
		if kind == "documentation" {
			return offlineDocsPrefix
		}
		return offlineAuditPrefix
	case "online_partial":
		if kind == "documentation" {
			return partialDocsPrefix
		}
		return partialAuditPrefix
	default:
		return ""
	}
}
