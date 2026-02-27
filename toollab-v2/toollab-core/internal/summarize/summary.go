package summarize

import (
	"sort"

	"toollab-v2/internal/model"
)

func Build(service model.ServiceModel) model.Summary {
	out := model.Summary{
		ServiceName:     service.ServiceName,
		EndpointCount:   len(service.Endpoints),
		DependencyCount: len(service.Dependencies),
		TypeCount:       len(service.Types),
	}

	for _, ep := range service.Endpoints {
		if ep.Method == "ANY" {
			out.ComplexEndpoints = append(out.ComplexEndpoints, struct {
				EndpointID string `json:"endpoint_id"`
				Reason     string `json:"reason"`
			}{
				EndpointID: ep.ID,
				Reason:     "usa método ANY (menos determinista)",
			})
		}
	}

	freq := map[string]int{}
	for _, d := range service.Dependencies {
		freq[d.Name]++
	}
	type pair struct {
		Name string
		N    int
	}
	arr := make([]pair, 0, len(freq))
	for k, v := range freq {
		arr = append(arr, pair{Name: k, N: v})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].N > arr[j].N })
	for i := 0; i < len(arr) && i < 8; i++ {
		out.TopDependencies = append(out.TopDependencies, arr[i].Name)
	}
	return out
}
