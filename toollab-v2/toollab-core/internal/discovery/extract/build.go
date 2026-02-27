package extract

import (
	"sort"
	"strings"

	astx "toollab-v2/internal/discovery/ast"
	"toollab-v2/internal/shared/common"
	"toollab-v2/internal/shared/model"
)

func BuildServiceModel(snapshot model.RepoSnapshot) (model.ServiceModel, error) {
	endpoints, types, deps, err := astx.ExtractGoModel(snapshot)
	if err != nil {
		return model.ServiceModel{}, err
	}

	flows := make([]model.Flow, 0, len(endpoints))
	for _, ep := range endpoints {
		flows = append(flows, model.Flow{
			ID:         common.SHA256String("flow:" + ep.ID),
			EndpointID: ep.ID,
			Steps: []model.FlowStep{
				{Order: 1, From: "client", To: ep.Path, Kind: "http_request"},
				{Order: 2, From: ep.Path, To: ep.HandlerName, Kind: "handler"},
			},
			Evidence: ep.Evidence,
		})
	}

	groupMap := map[string][]string{}
	for _, ep := range endpoints {
		group := domainFromPath(ep.Path)
		groupMap[group] = append(groupMap[group], ep.ID)
	}
	var domains []model.DomainGroup
	for name, ids := range groupMap {
		sort.Strings(ids)
		domains = append(domains, model.DomainGroup{Name: name, EndpointIDs: ids})
	}
	sort.Slice(domains, func(i, j int) bool { return domains[i].Name < domains[j].Name })

	service := model.ServiceModel{
		ModelVersion:      "v2",
		SnapshotID:        snapshot.SnapshotID,
		HashTree:          snapshot.HashTree,
		ServiceName:       snapshot.RepoName,
		LanguageDetected:  snapshot.LanguageDetected,
		FrameworkDetected: snapshot.FrameworkDetected,
		Endpoints:         endpoints,
		Types:             types,
		Dependencies:      deps,
		Flows:             flows,
		DomainGroups:      domains,
	}

	service.Fingerprint = common.SHA256String(string(common.MustStableJSON(service)))
	return service, nil
}

func domainFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "root"
	}
	if parts[0] == "v1" || parts[0] == "v2" {
		if len(parts) > 1 {
			return parts[1]
		}
		return parts[0]
	}
	return parts[0]
}
