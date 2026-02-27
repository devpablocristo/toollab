package audit

import (
	"strings"
	"time"

	"toollab-v2/internal/common"
	"toollab-v2/internal/model"
)

func Run(service model.ServiceModel) model.AuditReport {
	findings := make([]model.Finding, 0, 16)

	for _, ep := range service.Endpoints {
		if isWriteMethod(ep.Method) && len(ep.MiddlewareChain) == 0 {
			findings = append(findings, finding("high", "auth", "Endpoint mutante sin middleware explícito",
				"El endpoint modifica estado y no tiene cadena de middlewares detectada.",
				"Agregar middleware de autenticación/autorización verificable.", ep.ID, ep.Evidence))
		}
		if ep.Method == "ANY" {
			findings = append(findings, finding("medium", "design", "Handle/ANY reduce precisión contractual",
				"El endpoint no declara método HTTP específico.",
				"Separar por método para fortalecer trazabilidad y pruebas.", ep.ID, ep.Evidence))
		}
		if strings.Contains(ep.Path, "{") || strings.Contains(ep.Path, ":") {
			findings = append(findings, finding("low", "api", "Endpoint con path dinámico",
				"Se detectó parámetro dinámico en el path.",
				"Documentar validaciones de parámetros de ruta en contrato.", ep.ID, ep.Evidence))
		}
	}

	if len(service.Endpoints) == 0 {
		findings = append(findings, finding("high", "discovery", "No se detectaron endpoints HTTP",
			"El extractor no encontró rutas HTTP del servicio.",
			"Validar framework soportado y estructura del proyecto.", "", model.EvidenceRef{}))
	}
	if len(service.Types) == 0 {
		findings = append(findings, finding("medium", "model", "No se detectaron modelos estructurados",
			"No se detectaron structs exportadas en Go.",
			"Exponer DTOs de request/response para auditoría determinista.", "", model.EvidenceRef{}))
	}
	if len(service.Dependencies) > 120 {
		findings = append(findings, finding("low", "maintainability", "Alta cantidad de dependencias",
			"El servicio presenta una huella de dependencias elevada.",
			"Revisar minimización de dependencias y hardening de supply chain.", "", model.EvidenceRef{}))
	}

	return model.AuditReport{
		ModelFingerprint: service.Fingerprint,
		GeneratedAt:      time.Now().UTC(),
		Findings:         findings,
	}
}

func finding(severity, category, title, description, recommendation, endpointID string, ev model.EvidenceRef) model.Finding {
	id := common.SHA256String(severity + ":" + category + ":" + title + ":" + endpointID)
	return model.Finding{
		ID:             id,
		Severity:       severity,
		Category:       category,
		Title:          title,
		Description:    description,
		Recommendation: recommendation,
		EndpointID:     endpointID,
		Evidence:       ev,
	}
}

func isWriteMethod(m string) bool {
	return m == "POST" || m == "PUT" || m == "PATCH" || m == "DELETE"
}
