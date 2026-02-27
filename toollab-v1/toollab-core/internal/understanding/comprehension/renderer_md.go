package comprehension

import (
	"fmt"
	"strings"
)

func RenderMarkdown(r *Report) string {
	var sb strings.Builder

	sb.WriteString("# Reporte de Comprensión del Servicio\n\n")
	sb.WriteString("---\n\n")

	if r.DataSource == "declared+observed" {
		sb.WriteString("> *Fuente de datos:* **Declarada + Observada** — El servicio expone metadata semántica via `/_toollab/description` que enriquece este reporte.\n\n")
	} else {
		sb.WriteString("> *Fuente de datos:* **Observada** — Este reporte fue generado por inferencia heurística del tráfico. Para obtener un reporte más preciso, implementá `ServiceDescriptionProvider` en el adapter del servicio.\n\n")
	}

	// Maturity badge
	sb.WriteString(fmt.Sprintf("> **Madurez del servicio:** %s (%d/100) | ", r.MaturityGrade, r.MaturityScore))
	if r.Verdict.ProductionReady {
		sb.WriteString("**Listo para producción**\n\n")
	} else {
		sb.WriteString("**No listo para producción**\n\n")
	}

	// 1. IDENTITY
	sb.WriteString("## 1. ¿Qué es este servicio?\n\n")
	sb.WriteString(fmt.Sprintf("**Nombre:** %s  \n", r.Identity.Name))
	sb.WriteString(fmt.Sprintf("**Versión:** %s  \n", r.Identity.Version))
	sb.WriteString(fmt.Sprintf("**Tipo de API:** %s  \n", r.Identity.APIType))
	sb.WriteString(fmt.Sprintf("**Dominio:** %s  \n", r.Identity.Domain))
	sb.WriteString(fmt.Sprintf("**Propósito:** %s  \n", r.Identity.Purpose))
	if r.Identity.Consumers != "" {
		sb.WriteString(fmt.Sprintf("**Consumidores:** %s  \n", r.Identity.Consumers))
	}
	sb.WriteString("\n")

	// 2. ARCHITECTURE
	sb.WriteString("## 2. ¿Cómo está construido?\n\n")
	sb.WriteString(fmt.Sprintf("| Aspecto | Detalle |\n"))
	sb.WriteString(fmt.Sprintf("|---------|--------|\n"))
	sb.WriteString(fmt.Sprintf("| Tipo | %s |\n", r.Architecture.Type))
	sb.WriteString(fmt.Sprintf("| Autenticación | %s |\n", r.Architecture.AuthType))
	sb.WriteString(fmt.Sprintf("| Formato de datos | %s |\n", r.Architecture.DataFormat))
	sb.WriteString(fmt.Sprintf("| Versionamiento | %s |\n", boolToSiNo(r.Architecture.HasVersioning)))
	sb.WriteString(fmt.Sprintf("| Documentación OpenAPI | %s |\n", boolToSiNo(r.Architecture.HasOpenAPI)))
	sb.WriteString(fmt.Sprintf("| Total de endpoints | %d |\n", r.Architecture.TotalEndpoints))
	sb.WriteString(fmt.Sprintf("| Recursos identificados | %d |\n", r.Architecture.ResourceCount))
	sb.WriteString("\n")

	// 2b. DEPENDENCIES
	if len(r.Dependencies) > 0 {
		sb.WriteString("### Dependencias externas\n\n")
		sb.WriteString("| Servicio | Tipo | Descripción | Requerido |\n")
		sb.WriteString("|----------|------|-------------|----------|\n")
		for _, d := range r.Dependencies {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", d.Name, d.Type, d.Description, boolToSiNo(d.Required)))
		}
		sb.WriteString("\n")
	}

	// 3. MODELS
	if len(r.Models) > 0 {
		sb.WriteString("## 3. ¿Con qué datos trabaja?\n\n")
		for _, m := range r.Models {
			kindBadge := ""
			if m.Kind == "declared" {
				kindBadge = " *(declarado por el servicio)*"
			} else if m.Kind == "inferred" {
				kindBadge = " *(inferido del tráfico)*"
			}
			sb.WriteString(fmt.Sprintf("### %s%s\n\n", m.Name, kindBadge))
			if m.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", m.Description))
			}
			if len(m.Operations) > 0 {
				sb.WriteString(fmt.Sprintf("**Operaciones:** %s\n\n", strings.Join(m.Operations, ", ")))
			}
			if len(m.Fields) > 0 {
				sb.WriteString("| Campo | Tipo | Requerido | Descripción | Ejemplo |\n")
				sb.WriteString("|-------|------|-----------|-------------|--------|\n")
				for _, f := range m.Fields {
					example := f.Example
					if len(example) > 50 {
						example = example[:50] + "…"
					}
					example = strings.ReplaceAll(example, "|", "\\|")
					desc := f.Description
					if desc == "" {
						desc = "—"
					}
					req := ""
					if f.Required {
						req = "Si"
					}
					sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", f.Name, f.Type, req, desc, example))
				}
				sb.WriteString("\n")
			}
			if len(m.Relations) > 0 {
				sb.WriteString("**Relaciones:**\n\n")
				for _, rel := range m.Relations {
					relDesc := ""
					if rel.Description != "" {
						relDesc = " — " + rel.Description
					}
					sb.WriteString(fmt.Sprintf("- → `%s` (%s)%s\n", rel.Target, rel.Type, relDesc))
				}
				sb.WriteString("\n")
			}
		}
	}

	// 4. ALL FLOWS
	sb.WriteString("## 4. ¿Qué se puede hacer con este servicio?\n\n")
	sb.WriteString(fmt.Sprintf("El servicio expone **%d flujos/endpoints** organizados de la siguiente manera:\n\n", len(r.AllFlows)))

	categories := map[string][]FlowDetail{}
	var catOrder []string
	for _, f := range r.AllFlows {
		if _, exists := categories[f.Category]; !exists {
			catOrder = append(catOrder, f.Category)
		}
		categories[f.Category] = append(categories[f.Category], f)
	}

	for _, cat := range catOrder {
		flows := categories[cat]
		sb.WriteString(fmt.Sprintf("### %s (%d endpoints)\n\n", cat, len(flows)))
		sb.WriteString("| Endpoint | Descripción | Latencia | Error Rate | Status Codes |\n")
		sb.WriteString("|----------|-------------|----------|------------|-------------|\n")
		for _, f := range flows {
			codes := make([]string, len(f.StatusCodes))
			for i, c := range f.StatusCodes {
				codes[i] = fmt.Sprintf("%d", c)
			}
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %dms | %.0f%% | %s |\n",
				f.Name, f.Description, f.AvgLatency, f.ErrorRate*100, strings.Join(codes, ", ")))
		}
		sb.WriteString("\n")
	}

	// 5. PAYLOADS
	sb.WriteString("## 5. ¿Qué se envía y qué se recibe?\n\n")
	hasPayloads := false
	for _, f := range r.AllFlows {
		if f.Payload != nil || f.Response != nil {
			hasPayloads = true
			sb.WriteString(fmt.Sprintf("### `%s`\n\n", f.Name))
			sb.WriteString(fmt.Sprintf("*%s*\n\n", f.Description))

			if f.Payload != nil {
				sb.WriteString("**Request:**\n")
				if len(f.Payload.Fields) > 0 {
					sb.WriteString("| Campo | Tipo |\n|-------|------|\n")
					for name, typ := range f.Payload.Fields {
						sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", name, typ))
					}
					sb.WriteString("\n")
				}
				if f.Payload.Example != "" {
					example := f.Payload.Example
					if len(example) > 300 {
						example = example[:300] + "…"
					}
					sb.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", example))
				}
			}

			if f.Response != nil {
				sb.WriteString("**Response:**\n")
				if len(f.Response.Fields) > 0 {
					sb.WriteString("| Campo | Tipo |\n|-------|------|\n")
					for name, typ := range f.Response.Fields {
						sb.WriteString(fmt.Sprintf("| `%s` | %s |\n", name, typ))
					}
					sb.WriteString("\n")
				}
				if f.Response.Example != "" {
					example := f.Response.Example
					if len(example) > 300 {
						example = example[:300] + "…"
					}
					sb.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", example))
				}
			}
		}
	}
	if !hasPayloads {
		sb.WriteString("No se capturaron payloads de ejemplo.\n\n")
	}

	// 6. BEHAVIOR
	sb.WriteString("## 6. ¿Cómo se comporta?\n\n")
	sb.WriteString("| Aspecto | Observación |\n")
	sb.WriteString("|---------|------------|\n")
	sb.WriteString(fmt.Sprintf("| Datos inválidos | %s |\n", r.Behavior.InvalidInput))
	sb.WriteString(fmt.Sprintf("| Sin autenticación | %s |\n", r.Behavior.MissingAuth))
	sb.WriteString(fmt.Sprintf("| Recurso inexistente | %s |\n", r.Behavior.NotFound))
	sb.WriteString(fmt.Sprintf("| Duplicados | %s |\n", r.Behavior.Duplicates))
	sb.WriteString(fmt.Sprintf("| Consistencia de errores | %s |\n", r.Behavior.ErrorConsistency))
	sb.WriteString(fmt.Sprintf("| Idempotencia | %s |\n", r.Behavior.Idempotency))
	sb.WriteString("\n")

	// 7. PERFORMANCE
	sb.WriteString("## 7. ¿Qué tan rápido es?\n\n")
	sb.WriteString(fmt.Sprintf("| Percentil | Latencia |\n"))
	sb.WriteString(fmt.Sprintf("|-----------|----------|\n"))
	sb.WriteString(fmt.Sprintf("| P50 | %dms |\n", r.Performance.OverallP50))
	sb.WriteString(fmt.Sprintf("| P95 | %dms |\n", r.Performance.OverallP95))
	sb.WriteString(fmt.Sprintf("| P99 | %dms |\n", r.Performance.OverallP99))
	sb.WriteString("\n")

	if len(r.Performance.FastEndpoints) > 0 {
		sb.WriteString("**Endpoints más rápidos:**\n\n")
		for _, ep := range r.Performance.FastEndpoints {
			sb.WriteString(fmt.Sprintf("- `%s` — %dms promedio (%d requests)\n", ep.Endpoint, ep.AvgMs, ep.Requests))
		}
		sb.WriteString("\n")
	}

	if len(r.Performance.SlowEndpoints) > 0 {
		sb.WriteString("**Endpoints más lentos:**\n\n")
		for _, ep := range r.Performance.SlowEndpoints {
			sb.WriteString(fmt.Sprintf("- `%s` — %dms promedio (%d requests)\n", ep.Endpoint, ep.AvgMs, ep.Requests))
		}
		sb.WriteString("\n")
	}

	if len(r.Performance.Bottlenecks) > 0 {
		sb.WriteString("**Cuellos de botella detectados:**\n\n")
		for _, b := range r.Performance.Bottlenecks {
			sb.WriteString(fmt.Sprintf("- %s\n", b))
		}
		sb.WriteString("\n")
	}

	// 8. SECURITY
	sb.WriteString("## 8. ¿Es seguro?\n\n")
	sb.WriteString(fmt.Sprintf("**Grade:** %s | **Score:** %d/100\n\n", r.Security.Grade, r.Security.Score))

	if len(r.Security.Risks) > 0 {
		sb.WriteString("**Riesgos:**\n\n")
		for _, risk := range r.Security.Risks {
			sb.WriteString(fmt.Sprintf("- %s\n", risk))
		}
		sb.WriteString("\n")
	}
	if len(r.Security.Strengths) > 0 {
		sb.WriteString("**Fortalezas:**\n\n")
		for _, s := range r.Security.Strengths {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// 9. CONTRACT
	sb.WriteString("## 9. ¿Está bien hecho?\n\n")
	sb.WriteString(fmt.Sprintf("**Compliance:** %.1f%%\n\n", r.ContractQuality.ComplianceRate*100))
	if len(r.ContractQuality.Issues) > 0 {
		sb.WriteString("**Problemas:**\n\n")
		for _, issue := range r.ContractQuality.Issues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}
	if len(r.ContractQuality.Strengths) > 0 {
		sb.WriteString("**Fortalezas:**\n\n")
		for _, s := range r.ContractQuality.Strengths {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
		sb.WriteString("\n")
	}

	// 10. VERDICT
	sb.WriteString("## 10. Veredicto y Decisiones\n\n")
	if r.Verdict.ProductionReady {
		sb.WriteString("**El servicio está listo para producción** (confianza: " + r.Verdict.Confidence + ")\n\n")
	} else {
		sb.WriteString("**El servicio NO está listo para producción** (confianza: " + r.Verdict.Confidence + ")\n\n")
	}

	if len(r.Verdict.Risks) > 0 {
		sb.WriteString("### Riesgos identificados\n\n")
		for i, risk := range r.Verdict.Risks {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, risk))
		}
		sb.WriteString("\n")
	}

	if len(r.Verdict.Improvements) > 0 {
		sb.WriteString("### Mejoras recomendadas\n\n")
		for i, imp := range r.Verdict.Improvements {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, imp))
		}
		sb.WriteString("\n")
	}

	if len(r.Verdict.MissingFeatures) > 0 {
		sb.WriteString("### Funcionalidad pendiente\n\n")
		for _, mf := range r.Verdict.MissingFeatures {
			sb.WriteString(fmt.Sprintf("- %s\n", mf))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n\n")
	sb.WriteString("*Reporte generado por toollab — Auditoría de software creado por IA*\n")

	return sb.String()
}

func boolToSiNo(b bool) string {
	if b {
		return "Sí"
	}
	return "No"
}
