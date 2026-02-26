package mapmodel

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"toollab-core/internal/discovery"
	"toollab-core/internal/evidence"
	"toollab-core/internal/gen"
)

func FromOpenAPI(doc *gen.OpenAPIDoc) *SystemMap {
	out := baseMap()
	if doc == nil {
		out.Partial = true
		out.Unknowns = append(out.Unknowns, "openapi document unavailable")
		return out
	}
	for path, item := range doc.Paths {
		appendOpenAPIOperation(&out.Endpoints, path, "GET", item.Get)
		appendOpenAPIOperation(&out.Endpoints, path, "POST", item.Post)
		appendOpenAPIOperation(&out.Endpoints, path, "PUT", item.Put)
		appendOpenAPIOperation(&out.Endpoints, path, "PATCH", item.Patch)
		appendOpenAPIOperation(&out.Endpoints, path, "DELETE", item.Delete)
		appendOpenAPIOperation(&out.Endpoints, path, "HEAD", item.Head)
		appendOpenAPIOperation(&out.Endpoints, path, "OPTIONS", item.Options)
	}
	sortEndpoints(out.Endpoints)
	out.Flows = inferFlowsFromEndpoints(out.Endpoints)
	return out
}

func FromToollab(manifest *discovery.Manifest, profile *discovery.Profile) *SystemMap {
	out := baseMap()
	if manifest != nil {
		out.ServiceIdentity.Name = manifest.AppName
		out.ServiceIdentity.Version = manifest.AppVersion
		out.Resources = append(out.Resources, Resource{Name: "toollab-ready", Kind: "adapter", Tags: manifest.Capabilities})
	}
	if profile == nil {
		out.Partial = true
		out.Unknowns = append(out.Unknowns, "profile unavailable")
		return out
	}
	if profile.Manifest != nil && out.ServiceIdentity.Name == "" {
		out.ServiceIdentity.Name = profile.Manifest.AppName
		out.ServiceIdentity.Version = profile.Manifest.AppVersion
	}
	if len(profile.SuggestedFlows) > 0 {
		parseSuggestedFlows(profile.SuggestedFlows, &out.Flows, &out.Endpoints)
	} else {
		out.Unknowns = append(out.Unknowns, "suggested_flows unavailable")
	}
	if len(profile.Invariants) > 0 {
		_ = json.Unmarshal(profile.Invariants, &out.Invariants)
	} else {
		out.Unknowns = append(out.Unknowns, "invariants unavailable")
	}
	if len(profile.Limits) > 0 {
		_ = json.Unmarshal(profile.Limits, &out.Limits)
	} else {
		out.Unknowns = append(out.Unknowns, "limits unavailable")
	}
	if len(profile.Environment) > 0 {
		var env map[string]any
		_ = json.Unmarshal(profile.Environment, &env)
		if mode, ok := env["mode"].(string); ok {
			out.ServiceIdentity.Environment = mode
		}
	}
	sortEndpoints(out.Endpoints)
	sortFlows(out.Flows)
	return out
}

func FromEvidence(bundle *evidence.Bundle) *SystemMap {
	out := baseMap()
	out.Partial = true
	if bundle == nil {
		out.Unknowns = append(out.Unknowns, "evidence missing")
		return out
	}
	out.ServiceIdentity.Name = "unknown-service"
	out.ServiceIdentity.Version = bundle.Metadata.ToollabVersion
	out.ServiceIdentity.Environment = "unknown"
	endpointSet := map[string]struct{}{}
	for _, outcome := range bundle.Outcomes {
		key := outcome.Method + " " + outcome.Path
		if _, ok := endpointSet[key]; ok {
			continue
		}
		endpointSet[key] = struct{}{}
		out.Endpoints = append(out.Endpoints, Endpoint{
			Method: outcome.Method,
			Path:   outcome.Path,
		})
	}
	sortEndpoints(out.Endpoints)
	out.Flows = inferFlowsFromEndpoints(out.Endpoints)
	out.Unknowns = append(out.Unknowns, "discovery unavailable: map derived from observed outcomes only")
	return out
}

func baseMap() *SystemMap {
	return &SystemMap{
		SchemaVersion: 1,
		ServiceIdentity: ServiceIdentity{
			Name:    "unknown-service",
			Version: "unknown",
		},
		Resources:  []Resource{},
		Endpoints:  []Endpoint{},
		Flows:      []Flow{},
		Invariants: []map[string]any{},
		Limits:     map[string]any{},
		Unknowns:   []string{},
		Anchors:    []Anchor{},
		Partial:    false,
		Determinism: Determinism{
			CanonicalWriterVersion: "system-map-json-v1",
		},
	}
}

func appendOpenAPIOperation(endpoints *[]Endpoint, path, method string, op *gen.Operation) {
	if op == nil || strings.HasPrefix(path, "/_toollab/") {
		return
	}
	*endpoints = append(*endpoints, Endpoint{
		Method: method,
		Path:   path,
		Tags:   append([]string(nil), op.Tags...),
	})
}

func parseSuggestedFlows(raw json.RawMessage, flows *[]Flow, endpoints *[]Endpoint) {
	type req struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	type flow struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Requests    []req  `json:"requests"`
	}
	type payload struct {
		Flows []flow `json:"flows"`
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	for _, flow := range p.Flows {
		item := Flow{
			ID:          flow.ID,
			Description: flow.Description,
			Requests:    []FlowRequest{},
		}
		for _, req := range flow.Requests {
			item.Requests = append(item.Requests, FlowRequest{Method: strings.ToUpper(req.Method), Path: req.Path})
			*endpoints = append(*endpoints, Endpoint{Method: strings.ToUpper(req.Method), Path: req.Path})
		}
		*flows = append(*flows, item)
	}
}

func inferFlowsFromEndpoints(endpoints []Endpoint) []Flow {
	out := []Flow{}
	for i, endpoint := range endpoints {
		out = append(out, Flow{
			ID: fmt.Sprintf("flow_%03d", i+1),
			Requests: []FlowRequest{{
				Method: endpoint.Method,
				Path:   endpoint.Path,
			}},
		})
	}
	return out
}

func sortEndpoints(endpoints []Endpoint) {
	sort.SliceStable(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
}

func sortFlows(flows []Flow) {
	sort.SliceStable(flows, func(i, j int) bool {
		return flows[i].ID < flows[j].ID
	})
}
