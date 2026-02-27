package usecases

import (
	"encoding/json"
	"fmt"
	"strings"

	"toollab-core/internal/discovery/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
)

func GenerateScenarioPlan(runID string, model domain.ServiceModel) scenarioDomain.ScenarioPlan {
	plan := scenarioDomain.ScenarioPlan{
		PlanID:        shared.NewID(),
		RunID:         runID,
		SchemaVersion: "v1",
		Cases:         make([]scenarioDomain.ScenarioCase, 0, len(model.Endpoints)),
	}

	for i, ep := range model.Endpoints {
		c := buildCase(i, ep)
		plan.Cases = append(plan.Cases, c)
	}

	return plan
}

func buildCase(idx int, ep domain.Endpoint) scenarioDomain.ScenarioCase {
	name := fmt.Sprintf("%s %s", ep.Method, ep.Path)
	if ep.HandlerName != "" && ep.HandlerName != "(anonymous)" && ep.HandlerName != "(unknown)" {
		name = fmt.Sprintf("%s %s (%s)", ep.Method, ep.Path, ep.HandlerName)
	}

	tags := []string{"auto-generated"}
	tags = append(tags, strings.ToLower(ep.Method))

	c := scenarioDomain.ScenarioCase{
		CaseID:  shared.NewID(),
		Name:    name,
		Enabled: true,
		Tags:    tags,
		Request: scenarioDomain.CaseRequest{
			Method: ep.Method,
			Path:   ep.Path,
		},
	}

	if hasPathParams(ep.Path) {
		c.Request.PathParams = defaultPathParams(ep.Path)
	}

	switch ep.Method {
	case "POST", "PUT", "PATCH":
		c.Request.BodyJSON = json.RawMessage(`{}`)
		c.Request.Headers = map[string]string{"Content-Type": "application/json"}
	}

	return c
}

func hasPathParams(path string) bool {
	return strings.Contains(path, "{")
}

func defaultPathParams(path string) map[string]string {
	params := make(map[string]string)
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			name := p[1 : len(p)-1]
			params[name] = "1"
		}
	}
	return params
}
