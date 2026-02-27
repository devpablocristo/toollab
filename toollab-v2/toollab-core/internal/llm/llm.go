// Package llm construye la interpretación de alto nivel sobre artefactos deterministas.
package llm

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"toollab-v2/internal/model"
)

func Interpret(service model.ServiceModel, report model.AuditReport) (model.LLMInterpretation, error) {
	provider := os.Getenv("TOOLLAB_V2_LLM_PROVIDER")
	modelName := os.Getenv("TOOLLAB_V2_LLM_MODEL")
	if provider == "" {
		provider = "deterministic-local"
		if modelName == "" {
			modelName = "rules-v1"
		}
	}

	domainGroups := make([]string, 0, len(service.DomainGroups))
	for _, g := range service.DomainGroups {
		domainGroups = append(domainGroups, fmt.Sprintf("%s (%d endpoints)", g.Name, len(g.EndpointIDs)))
	}
	sort.Strings(domainGroups)

	riskHypotheses := make([]string, 0, 8)
	seenRisk := map[string]bool{}
	for _, f := range report.Findings {
		line := fmt.Sprintf("[%s] %s", strings.ToUpper(f.Severity), f.Title)
		if !seenRisk[line] {
			seenRisk[line] = true
			riskHypotheses = append(riskHypotheses, line)
		}
		if len(riskHypotheses) >= 8 {
			break
		}
	}

	suggested := make([]string, 0, 8)
	for _, ep := range service.Endpoints {
		if ep.Method == "POST" || ep.Method == "PUT" || ep.Method == "PATCH" || ep.Method == "DELETE" {
			suggested = append(suggested, fmt.Sprintf("Prueba de escritura y autorización en %s %s", ep.Method, ep.Path))
		}
		if len(suggested) >= 8 {
			break
		}
	}
	if len(suggested) == 0 {
		for i, ep := range service.Endpoints {
			suggested = append(suggested, fmt.Sprintf("Prueba funcional de contrato en %s %s", ep.Method, ep.Path))
			if i >= 5 {
				break
			}
		}
	}

	severityCount := map[string]int{"high": 0, "medium": 0, "low": 0}
	for _, f := range report.Findings {
		severityCount[f.Severity]++
	}

	topDeps := topDependencies(service.Dependencies, 10)
	flowLines := topFlowLines(service, 10)
	criticalEndpoints := topCriticalEndpoints(service.Endpoints, report.Findings, 10)

	functionalSummary := fmt.Sprintf(
		"Servicio %s detectado como %s/%s. Se identificaron %d endpoints, %d tipos y %d dependencias. Hallazgos de auditoría: %d.",
		service.ServiceName,
		service.LanguageDetected,
		service.FrameworkDetected,
		len(service.Endpoints),
		len(service.Types),
		len(service.Dependencies),
		len(report.Findings),
	)

	raw := strings.Join([]string{
		"# Análisis de los datos",
		"",
		"## Objetivo funcional inferido",
		"- " + functionalSummary,
		"",
		"## Arquitectura detectada",
		fmt.Sprintf("- Lenguaje/framework: `%s/%s`", service.LanguageDetected, service.FrameworkDetected),
		fmt.Sprintf("- Huella estructural: `%d` endpoints, `%d` tipos, `%d` dependencias, `%d` flujos", len(service.Endpoints), len(service.Types), len(service.Dependencies), len(service.Flows)),
		"",
		"## Dominios del servicio",
		mdList(domainGroups),
		"",
		"## Endpoints críticos (prioridad de auditoría)",
		mdList(criticalEndpoints),
		"",
		"## Flujos principales detectados",
		mdList(flowLines),
		"",
		"## Riesgos priorizados",
		fmt.Sprintf("- High: `%d` | Medium: `%d` | Low: `%d`", severityCount["high"], severityCount["medium"], severityCount["low"]),
		mdList(riskHypotheses),
		"",
		"## Dependencias relevantes",
		mdList(topDeps),
		"",
		"## Plan de pruebas recomendado",
		mdList(suggested),
	}, "\n")

	return model.LLMInterpretation{
		ModelFingerprint:       service.Fingerprint,
		Provider:               provider,
		Model:                  modelName,
		FunctionalSummary:      functionalSummary,
		DomainGroups:           domainGroups,
		RiskHypotheses:         riskHypotheses,
		SuggestedTestScenarios: suggested,
		Raw:                    raw,
	}, nil
}

func topDependencies(deps []model.Dependency, limit int) []string {
	if len(deps) == 0 {
		return []string{"Sin dependencias detectadas"}
	}
	freq := map[string]int{}
	types := map[string]string{}
	for _, d := range deps {
		freq[d.Name]++
		if _, ok := types[d.Name]; !ok {
			types[d.Name] = d.Type
		}
	}
	type item struct {
		name string
		n    int
	}
	arr := make([]item, 0, len(freq))
	for name, n := range freq {
		arr = append(arr, item{name: name, n: n})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].n == arr[j].n {
			return arr[i].name < arr[j].name
		}
		return arr[i].n > arr[j].n
	})
	out := make([]string, 0, min(limit, len(arr)))
	for i := 0; i < len(arr) && i < limit; i++ {
		out = append(out, fmt.Sprintf("`%s` (%s, %d usos)", arr[i].name, types[arr[i].name], arr[i].n))
	}
	return out
}

func topFlowLines(service model.ServiceModel, limit int) []string {
	if len(service.Flows) == 0 || len(service.Endpoints) == 0 {
		return []string{"Sin flujos detectados"}
	}
	epByID := map[string]model.Endpoint{}
	for _, ep := range service.Endpoints {
		epByID[ep.ID] = ep
	}
	out := make([]string, 0, limit)
	for _, flow := range service.Flows {
		ep, ok := epByID[flow.EndpointID]
		if !ok {
			continue
		}
		chain := make([]string, 0, len(flow.Steps))
		for _, s := range flow.Steps {
			chain = append(chain, fmt.Sprintf("%s->%s", s.From, s.To))
		}
		out = append(out, fmt.Sprintf("`%s %s` | %s", ep.Method, ep.Path, strings.Join(chain, " · ")))
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return []string{"Sin flujos detectados"}
	}
	return out
}

func topCriticalEndpoints(endpoints []model.Endpoint, findings []model.Finding, limit int) []string {
	if len(endpoints) == 0 {
		return []string{"Sin endpoints detectados"}
	}
	epByID := map[string]model.Endpoint{}
	for _, ep := range endpoints {
		epByID[ep.ID] = ep
	}
	score := map[string]int{}
	for _, f := range findings {
		if f.EndpointID == "" {
			continue
		}
		switch f.Severity {
		case "high":
			score[f.EndpointID] += 3
		case "medium":
			score[f.EndpointID] += 2
		default:
			score[f.EndpointID] += 1
		}
	}
	type item struct {
		id    string
		score int
	}
	arr := make([]item, 0, len(score))
	for id, s := range score {
		arr = append(arr, item{id: id, score: s})
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].score == arr[j].score {
			return arr[i].id < arr[j].id
		}
		return arr[i].score > arr[j].score
	})
	out := make([]string, 0, limit)
	for i := 0; i < len(arr) && i < limit; i++ {
		if ep, ok := epByID[arr[i].id]; ok {
			out = append(out, fmt.Sprintf("`%s %s` (score riesgo %d)", ep.Method, ep.Path, arr[i].score))
		}
	}
	if len(out) == 0 {
		for i, ep := range endpoints {
			out = append(out, fmt.Sprintf("`%s %s`", ep.Method, ep.Path))
			if i+1 >= limit {
				break
			}
		}
	}
	return out
}

func mdList(items []string) string {
	if len(items) == 0 {
		return "- Sin datos"
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		lines = append(lines, "- "+it)
	}
	return strings.Join(lines, "\n")
}
