package generate

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"toolab-core/internal/discovery"
	"toolab-core/internal/gen"
	"toolab-core/internal/scenario"
)

type OpenAPIOptions struct {
	Mode          string
	BaseURL       string
	EffectiveSeed string
	Input         string
}

func BuildFromOpenAPI(ctx context.Context, fetcher *discovery.OpenAPIFetcher, auth *discovery.AuthConfig, opts OpenAPIOptions) (*scenario.Scenario, string, []string, error) {
	doc, openapiHash, _, warnings, err := fetcher.Fetch(ctx, opts.Input, auth)
	if err != nil {
		return nil, "", warnings, err
	}
	scn, extraWarnings, err := BuildFromOpenAPIDoc(doc, opts)
	if err != nil {
		return nil, "", append(warnings, extraWarnings...), err
	}
	return scn, openapiHash, append(warnings, extraWarnings...), nil
}

func BuildFromOpenAPIDoc(doc *gen.OpenAPIDoc, opts OpenAPIOptions) (*scenario.Scenario, []string, error) {
	if opts.Mode == "" {
		opts.Mode = "smoke"
	}
	profile := "none"
	switch opts.Mode {
	case "smoke", "load":
		profile = "none"
	case "chaos":
		profile = "light"
	default:
		return nil, nil, fmt.Errorf("invalid mode %q", opts.Mode)
	}

	if opts.EffectiveSeed == "" {
		return nil, nil, fmt.Errorf("effective seed is required")
	}
	chaosSeed := deriveSeed(opts.EffectiveSeed, "chaos")
	source := opts.Input
	if source == "" {
		tmp, err := os.CreateTemp("", "toolab-openapi-*.yaml")
		if err != nil {
			return nil, nil, err
		}
		defer os.Remove(tmp.Name())
		raw, err := marshalOpenAPI(doc)
		if err != nil {
			return nil, nil, err
		}
		if _, err := tmp.Write(raw); err != nil {
			return nil, nil, err
		}
		if err := tmp.Close(); err != nil {
			return nil, nil, err
		}
		source = tmp.Name()
	}

	yamlBytes, err := gen.Generate(source, gen.Options{
		Profile:     profile,
		Concurrency: defaultConcurrency(opts.Mode),
		DurationS:   defaultDuration(opts.Mode),
		Schedule:    "closed_loop",
		RunSeed:     opts.EffectiveSeed,
		ChaosSeed:   chaosSeed,
	})
	if err != nil {
		return nil, nil, err
	}
	scn, _, err := scenario.LoadBytes(source, yamlBytes)
	if err != nil {
		return nil, nil, err
	}
	if opts.BaseURL != "" {
		scn.Target.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	}

	warnings := []string{}
	filtered := make([]scenario.RequestSpec, 0, len(scn.Workload.Requests))
	for _, req := range scn.Workload.Requests {
		if strings.HasPrefix(req.Path, "/_toolab/") {
			warnings = append(warnings, "excluded /_toolab endpoint from generated workload")
			continue
		}
		normalizeRequestContentType(&req)
		filtered = append(filtered, req)
	}
	if len(filtered) == 0 {
		return nil, warnings, fmt.Errorf("no business endpoints found after filtering")
	}
	scn.Workload.Requests = dedupeRequests(filtered)
	return scn, warnings, nil
}

func defaultConcurrency(mode string) int {
	switch mode {
	case "load":
		return 8
	case "chaos":
		return 4
	default:
		return 1
	}
}

func defaultDuration(mode string) int {
	switch mode {
	case "load":
		return 60
	case "chaos":
		return 30
	default:
		return 15
	}
}

func deriveSeed(seed, scope string) string {
	sum := sha256.Sum256([]byte(seed + ":" + scope))
	v := binary.BigEndian.Uint64(sum[0:8])
	return fmt.Sprintf("%d", v)
}

func normalizeRequestContentType(req *scenario.RequestSpec) {
	if req.Headers == nil {
		req.Headers = map[string]string{}
	}
	if len(req.JSONBody) > 0 {
		if _, ok := req.Headers["Content-Type"]; !ok {
			req.Headers["Content-Type"] = "application/json"
		}
	}
}

func requestFingerprint(req scenario.RequestSpec) string {
	queryKeys := make([]string, 0, len(req.Query))
	for k := range req.Query {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	queryParts := make([]string, 0, len(queryKeys))
	for _, k := range queryKeys {
		queryParts = append(queryParts, k+"="+req.Query[k])
	}
	contentType := req.Headers["Content-Type"]
	bodyHash := "empty"
	if req.Body != nil {
		bodyHash = hash6([]byte(*req.Body))
	}
	if len(req.JSONBody) > 0 {
		bodyHash = hash6(req.JSONBody)
	}
	return strings.Join([]string{
		req.Method,
		req.Path,
		strings.Join(queryParts, "&"),
		contentType,
		bodyHash,
	}, "|")
}

func dedupeRequests(requests []scenario.RequestSpec) []scenario.RequestSpec {
	seen := map[string]scenario.RequestSpec{}
	keys := make([]string, 0, len(requests))
	for _, req := range requests {
		fp := requestFingerprint(req)
		if _, ok := seen[fp]; ok {
			continue
		}
		seen[fp] = req
		keys = append(keys, fp)
	}
	sort.Strings(keys)
	out := make([]scenario.RequestSpec, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func hash6(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:3])
}

func marshalOpenAPI(doc *gen.OpenAPIDoc) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("openapi doc is nil")
	}
	return yaml.Marshal(doc)
}
