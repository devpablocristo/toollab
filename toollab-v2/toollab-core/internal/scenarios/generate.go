package scenarios

import (
	"toollab-v2/internal/common"
	"toollab-v2/internal/model"
)

func Build(service model.ServiceModel, report model.AuditReport) []model.Scenario {
	out := make([]model.Scenario, 0, len(service.Endpoints))
	riskByEndpoint := map[string]string{}
	for _, f := range report.Findings {
		if f.EndpointID == "" {
			continue
		}
		if cur, ok := riskByEndpoint[f.EndpointID]; !ok || rank(f.Severity) > rank(cur) {
			riskByEndpoint[f.EndpointID] = f.Severity
		}
	}

	for _, ep := range service.Endpoints {
		risk := riskByEndpoint[ep.ID]
		if risk == "" {
			risk = "low"
		}
		out = append(out, model.Scenario{
			ID:             common.SHA256String("scenario:" + ep.ID),
			EndpointID:     ep.ID,
			Method:         ep.Method,
			Path:           ep.Path,
			ExpectedStatus: 200,
			RiskCategory:   risk,
			Notes:          "Escenario generado automáticamente desde ServiceModel v2.",
		})
	}
	return out
}

func rank(sev string) int {
	switch sev {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}
