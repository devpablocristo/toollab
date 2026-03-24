package llm

import (
	"context"
	"encoding/json"
	"log"
	"time"

	artifactUC "toollab-core/internal/artifact"
	artDomain "toollab-core/internal/artifact/usecases/domain"
)

// Runner generates LLM docs and audit reports asynchronously.
type Runner struct {
	vertex      *VertexProvider
	artifactSvc *artifactUC.Service
}

func NewRunner(vertex *VertexProvider, artifactSvc *artifactUC.Service) *Runner {
	return &Runner{vertex: vertex, artifactSvc: artifactSvc}
}

// RunAsync generates LLM reports. Currently only docs is active; audit is skipped to save tokens.
// Set auditEnabled = true below to re-enable audit generation.
func (r *Runner) RunAsync(ctx context.Context, runID string, docsMiniJSON, auditLLMJSON []byte, lang string) {
	const auditEnabled = false

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	var docsMeta struct {
		RunMode string `json:"run_mode"`
	}
	_ = json.Unmarshal(docsMiniJSON, &docsMeta)
	docsRunMode := docsMeta.RunMode

	expected := 1
	if auditEnabled {
		expected = 2
	}

	type result struct {
		kind string
		data []byte
		err  error
	}
	ch := make(chan result, expected)

	log.Printf("LLM starting (run %s): docs_mini=%dKB audit=%dKB audit_enabled=%v lang=%s",
		runID, len(docsMiniJSON)/1024, len(auditLLMJSON)/1024, auditEnabled, lang)

	go func() {
		t0 := time.Now()
		prompt := buildDocsPrompt(string(docsMiniJSON), docsRunMode, lang)
		md, err := r.vertex.TextPrompt(ctx, prompt)
		if err == nil {
			var runID2 string
			var meta struct {
				RunID string `json:"run_id"`
			}
			if json.Unmarshal(docsMiniJSON, &meta) == nil {
				runID2 = meta.RunID
			}
			wrapped, _ := json.Marshal(map[string]string{
				"schema_version": "docs-mini-v5",
				"run_id":         runID2,
				"format":         "markdown",
				"content":        md,
			})
			log.Printf("LLM docs finished (run %s): %v, md=%dKB", runID, time.Since(t0).Round(time.Second), len(md)/1024)
			ch <- result{kind: "documentation", data: wrapped, err: nil}
		} else {
			log.Printf("LLM docs failed (run %s): %v, err=%v", runID, time.Since(t0).Round(time.Second), err)
			ch <- result{kind: "documentation", data: nil, err: err}
		}
	}()

	if auditEnabled {
		go func() {
			var auditMeta struct {
				RunMode string `json:"run_mode"`
			}
			_ = json.Unmarshal(auditLLMJSON, &auditMeta)

			t0 := time.Now()
			prompt := buildAuditPrompt(string(auditLLMJSON), auditMeta.RunMode, lang)
			data, err := r.vertex.RawPrompt(ctx, prompt)
			log.Printf("LLM audit finished (run %s): %v, output=%dKB, err=%v", runID, time.Since(t0).Round(time.Second), len(data)/1024, err)
			ch <- result{kind: "audit", data: data, err: err}
		}()
	}

	for i := 0; i < expected; i++ {
		res := <-ch
		artType := artDomain.ArtifactLLMDocs
		if res.kind == "audit" {
			artType = artDomain.ArtifactLLMAudit
		}

		if res.err != nil {
			log.Printf("LLM %s failed (run %s): %v", res.kind, runID, res.err)
			r.saveFailure(runID, artType, res.err.Error())
		} else {
			log.Printf("LLM %s completed (run %s)", res.kind, runID)
			if _, err := r.artifactSvc.Put(runID, artType, res.data); err != nil {
				log.Printf("LLM %s save failed (run %s): %v", res.kind, runID, err)
				r.saveFailure(runID, artType, "invalid LLM output: "+err.Error())
			}
		}
	}
}

func (r *Runner) saveFailure(runID string, artType artDomain.ArtifactType, msg string) {
	errJSON, _ := json.Marshal(map[string]string{"status": "failed", "error": msg})
	if _, err := r.artifactSvc.Put(runID, artType, errJSON); err != nil {
		log.Printf("LLM failure artifact save failed (run %s, type %s): %v", runID, artType, err)
	}
}

func buildDocsPrompt(docsMiniJSON, runMode, lang string) string {
	prefix := ""
	switch runMode {
	case "offline":
		prefix = offlineDocsPrefix
	case "online_partial":
		prefix = partialDocsPrefix
	}
	suffix := ""
	if lang == "es" {
		suffix = langSuffixES
	}
	return prefix + docsPrompt + suffix + "\n\nDOSSIER:\n" + docsMiniJSON
}

func buildAuditPrompt(auditJSON, runMode, lang string) string {
	prefix := ""
	switch runMode {
	case "offline":
		prefix = offlineAuditPrefix
	case "online_partial":
		prefix = partialAuditPrefix
	}
	suffix := ""
	if lang == "es" {
		suffix = auditLangSuffixES
	}
	return prefix + auditPrompt + suffix + "\n\nDOSSIER:\n" + auditJSON
}
