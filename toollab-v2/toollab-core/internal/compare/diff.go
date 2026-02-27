package compare

import "toollab-v2/internal/model"

type Diff struct {
	AddedEndpoints     []model.Endpoint `json:"added_endpoints"`
	RemovedEndpoints   []model.Endpoint `json:"removed_endpoints"`
	ChangedFingerprint bool             `json:"changed_fingerprint"`
}

func ServiceModels(oldM, newM model.ServiceModel) Diff {
	oldMap := map[string]model.Endpoint{}
	newMap := map[string]model.Endpoint{}

	for _, ep := range oldM.Endpoints {
		oldMap[ep.ID] = ep
	}
	for _, ep := range newM.Endpoints {
		newMap[ep.ID] = ep
	}

	out := Diff{ChangedFingerprint: oldM.Fingerprint != newM.Fingerprint}
	for id, ep := range newMap {
		if _, ok := oldMap[id]; !ok {
			out.AddedEndpoints = append(out.AddedEndpoints, ep)
		}
	}
	for id, ep := range oldMap {
		if _, ok := newMap[id]; !ok {
			out.RemovedEndpoints = append(out.RemovedEndpoints, ep)
		}
	}
	return out
}
