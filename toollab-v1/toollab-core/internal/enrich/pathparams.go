package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"toollab-core/internal/scenario"
)

var pathParamRe = regexp.MustCompile(`\{([^}]+)\}`)

// ResolvePathParams queries list endpoints on the target API to discover
// real values for path parameters like {name}, {id}, etc., and rewrites
// request paths in-place. Returns the list of changes made.
func ResolvePathParams(ctx context.Context, scn *scenario.Scenario) ([]Change, []string) {
	changes := []Change{}
	warnings := []string{}

	paramValues := map[string]string{}
	client := &http.Client{Timeout: 5 * time.Second}

	listEndpoints := discoverListEndpoints(scn.Workload.Requests)

	for param, listPath := range listEndpoints {
		if _, ok := paramValues[param]; ok {
			continue
		}
		value, err := fetchParamValue(ctx, client, scn, listPath, param)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("path param {%s}: list endpoint %s failed: %s", param, listPath, err))
			continue
		}
		if value != "" {
			paramValues[param] = value
		}
	}

	for i := range scn.Workload.Requests {
		req := &scn.Workload.Requests[i]
		if !pathParamRe.MatchString(req.Path) {
			continue
		}
		original := req.Path
		resolved := pathParamRe.ReplaceAllStringFunc(req.Path, func(match string) string {
			param := match[1 : len(match)-1]
			if v, ok := paramValues[param]; ok {
				return v
			}
			return match
		})
		if resolved != original {
			req.Path = resolved
			changes = append(changes, Change{
				Op:     "replace",
				Path:   fmt.Sprintf("/workload/requests/%d/path", i),
				Reason: fmt.Sprintf("resolved path params: %s -> %s", original, resolved),
				Source: "live_api",
			})
		}
	}

	return changes, warnings
}

// discoverListEndpoints maps path params to their probable list endpoint.
// e.g. /v1/tools/{name}/policies -> {name} -> /v1/tools
//      /v1/incidents/{id}/close  -> {id}   -> /v1/incidents
func discoverListEndpoints(requests []scenario.RequestSpec) map[string]string {
	result := map[string]string{}

	paramPaths := map[string][]string{}
	for _, req := range requests {
		matches := pathParamRe.FindAllStringIndex(req.Path, -1)
		for _, match := range matches {
			param := req.Path[match[0]+1 : match[1]-1]
			prefix := strings.TrimRight(req.Path[:match[0]], "/")
			if prefix != "" {
				paramPaths[param] = append(paramPaths[param], prefix)
			}
		}
	}

	for param, prefixes := range paramPaths {
		sort.Slice(prefixes, func(i, j int) bool {
			return len(prefixes[i]) < len(prefixes[j])
		})
		if len(prefixes) > 0 {
			result[param] = prefixes[0]
		}
	}
	return result
}

func fetchParamValue(ctx context.Context, client *http.Client, scn *scenario.Scenario, listPath, param string) (string, error) {
	url := strings.TrimRight(scn.Target.BaseURL, "/") + listPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	applyScenarioAuth(req, scn.Target.Auth)
	for k, v := range scn.Target.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	return extractFirstValue(body, param)
}

func extractFirstValue(body []byte, param string) (string, error) {
	// Try as JSON array of objects.
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		return pickFromObject(arr[0], param), nil
	}

	// Try as { "items": [...] } or { "data": [...] } wrapper.
	var wrapper map[string]any
	if err := json.Unmarshal(body, &wrapper); err == nil {
		for _, key := range []string{"items", "data", "results", "tools", "incidents", "events", "runs", "proposals", "actions", "policies", "secrets", "rules"} {
			if raw, ok := wrapper[key]; ok {
				if items, ok := raw.([]any); ok && len(items) > 0 {
					if obj, ok := items[0].(map[string]any); ok {
						return pickFromObject(obj, param), nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("could not extract value for {%s}", param)
}

func pickFromObject(obj map[string]any, param string) string {
	candidates := []string{param, "id", "name", "slug"}
	for _, key := range candidates {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
		}
	}
	return ""
}

func applyScenarioAuth(req *http.Request, auth scenario.Auth) {
	switch auth.Type {
	case "api_key":
		key := envValue(auth.APIKeyEnv)
		if key == "" {
			return
		}
		if auth.In == "header" {
			req.Header.Set(auth.Name, key)
		} else if auth.In == "query" {
			q := req.URL.Query()
			q.Set(auth.Name, key)
			req.URL.RawQuery = q.Encode()
		}
	case "bearer":
		token := envValue(auth.BearerTokenEnv)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
}

func envValue(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
