package projectaudit

import (
	"fmt"
	"strings"
)

func buildReviewPrompt(req CreatePRReviewRequest, sddContext string, files []PRReviewFile, findings []PRReviewFinding, score int, decision string, confidence string, specStatus string) string {
	var b strings.Builder
	b.WriteString("# ToolLab PR Guard Review\n\n")
	b.WriteString("Actuá como ToolLab, un auditor técnico especializado en revisar PRs y cambios de código creados o modificados con IA.\n\n")
	b.WriteString("Tu objetivo es decidir si este cambio puede mergearse con confianza, necesita cambios o debe bloquearse.\n\n")
	b.WriteString("Usá la mini-spec SDD como fuente principal de intención.\n\n")
	b.WriteString("No inventes reglas de negocio.\n\n")
	b.WriteString("No reportes problemas sin evidencia.\n\n")
	b.WriteString("Si falta contexto, marcá incertidumbre.\n\n")
	b.WriteString("Cada hallazgo debe tener evidencia, impacto, corrección sugerida y prompt para que una IA lo corrija.\n\n")

	b.WriteString("## Task / PR\n\n")
	b.WriteString(strings.TrimSpace(req.Title))
	if strings.TrimSpace(req.Description) != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(req.Description))
	}
	b.WriteString("\n\n")

	b.WriteString("## SDD Context\n\n")
	b.WriteString(blankFallback(sddContext, "MISSING_CONTEXT"))
	b.WriteString("\n\n")

	b.WriteString("## Project Rules\n\n")
	b.WriteString(blankFallback(req.ProjectRules, "MISSING_CONTEXT"))
	b.WriteString("\n\n")

	b.WriteString("## Changed Files Summary\n\n")
	b.WriteString("| File | Change | Risk area | Risk level | Additions | Deletions |\n")
	b.WriteString("|---|---|---|---|---:|---:|\n")
	for _, file := range files {
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %d | %d |\n", file.Path, file.ChangeType, file.RiskArea, file.RiskLevel, file.Additions, file.Deletions))
	}
	if len(files) == 0 {
		b.WriteString("| MISSING_CONTEXT | unknown | unknown | LOW | 0 | 0 |\n")
	}
	b.WriteString("\n")

	b.WriteString("## Detected Findings\n\n")
	if len(findings) == 0 {
		b.WriteString("- No deterministic findings detected by ToolLab PR Guard.\n")
	}
	for i, finding := range findings {
		b.WriteString(fmt.Sprintf("- TL-%02d [%s/%s] %s: %s\n", i+1, finding.Severity, finding.Status, finding.Title, finding.Evidence))
	}
	b.WriteString("\n")

	b.WriteString("## Test Output\n\n")
	b.WriteString(blankFallback(req.TestOutput, "NOT_PROVIDED"))
	b.WriteString("\n\n")

	b.WriteString("## Diff\n\n")
	b.WriteString("```diff\n")
	b.WriteString(strings.TrimSpace(req.DiffText))
	b.WriteString("\n```\n\n")

	b.WriteString("## Output Format\n\n")
	b.WriteString("Devolvé:\n\n")
	b.WriteString("# ToolLab PR Review\n\n")
	b.WriteString("## 1. Decision\n\n")
	b.WriteString(fmt.Sprintf("Score: %d/100\n\nDecision: %s\n\nConfidence: %s\n\n", score, decision, confidence))
	b.WriteString("## 2. SDD Context Used\n\n")
	b.WriteString(specStatus)
	b.WriteString("\n\n")
	b.WriteString("## 3. PR Change Summary\n\n")
	b.WriteString("| Area | Files | Change | Risk |\n\n|---|---|---|---|\n\n")
	b.WriteString("## 4. Findings by Criticality\n\n")
	b.WriteString("Para cada finding:\n\n")
	b.WriteString("### TL-[number] — [severity] — [title]\n\n")
	b.WriteString("Status:\n\nCONFIRMED | LIKELY | POSSIBLE | MISSING_CONTEXT\n\n")
	b.WriteString("File(s):\n\n...\n\n")
	b.WriteString("Spec rule affected:\n\n...\n\n")
	b.WriteString("Problem:\n\n...\n\n")
	b.WriteString("Evidence:\n\n...\n\n")
	b.WriteString("Impact:\n\n...\n\n")
	b.WriteString("Suggested fix:\n\n...\n\n")
	b.WriteString("AI correction prompt:\n\n\"\"\"\n...\n\"\"\"\n\n")
	b.WriteString("## 5. Tests Assessment\n\n")
	b.WriteString("Existing tests:\n\nMissing tests:\n\nRecommended tests:\n\nTest output: PASS | FAIL | NOT_PROVIDED | PARTIAL\n\n")
	b.WriteString("## 6. Security Assessment\n\nNo inventes. Indicá solo riesgos con evidencia.\n\n")
	b.WriteString("## 7. Suggested Fix Order\n\n")
	b.WriteString("## 8. Specs to Store or Update\n\nNo consolides spec global.\n\n")
	b.WriteString("## 9. Final Recommendation\n\n")
	b.WriteString("## 10. Machine-readable JSON\n\n")
	b.WriteString("Devolvé JSON válido con:\n\nscore, decision, confidence, spec_status, findings, tests, security, spec_storage.\n")
	return b.String()
}

func buildAICorrectionPrompt(req CreatePRReviewRequest, sddContext string, finding PRReviewFinding) string {
	var b strings.Builder
	b.WriteString("Actuá como un ingeniero senior corrigiendo un cambio de código con mínima superficie.\n\n")
	b.WriteString("No reescribas el sistema. No cambies contratos públicos salvo que el hallazgo lo exija.\n\n")
	b.WriteString("## Task\n\n")
	b.WriteString(strings.TrimSpace(req.Title))
	b.WriteString("\n\n")
	b.WriteString("## SDD Context\n\n")
	b.WriteString(blankFallback(sddContext, "MISSING_CONTEXT"))
	b.WriteString("\n\n")
	b.WriteString("## Finding\n\n")
	b.WriteString(fmt.Sprintf("Code: %s\nSeverity: %s\nStatus: %s\nTitle: %s\n", finding.Code, finding.Severity, finding.Status, finding.Title))
	b.WriteString("\nFiles:\n")
	for _, file := range finding.Files {
		b.WriteString("- " + file + "\n")
	}
	b.WriteString("\nProblem:\n")
	b.WriteString(finding.Problem)
	b.WriteString("\n\nEvidence:\n")
	b.WriteString(finding.Evidence)
	b.WriteString("\n\nSuggested fix:\n")
	b.WriteString(finding.SuggestedFix)
	b.WriteString("\n\nReturn a focused patch and the validation commands to run.\n")
	return b.String()
}

func blankFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
