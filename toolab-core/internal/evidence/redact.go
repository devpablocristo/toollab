package evidence

import (
	"encoding/json"
	"strings"

	"toolab-core/pkg/utils"
)

func RedactHeaders(headers map[string]string, sensitive []string, mask string) map[string]string {
	redacted := map[string]string{}
	sensitiveSet := map[string]struct{}{}
	for _, h := range sensitive {
		sensitiveSet[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
	}
	for k, v := range headers {
		if _, ok := sensitiveSet[strings.ToLower(k)]; ok {
			redacted[k] = mask
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func RedactBodyPreview(body []byte, jsonPaths []string, mask string, maxBytes int) string {
	if len(body) == 0 {
		return ""
	}

	trimmed := body
	if maxBytes > 0 && len(trimmed) > maxBytes {
		trimmed = trimmed[:maxBytes]
	}

	var doc any
	if err := json.Unmarshal(trimmed, &doc); err == nil {
		for _, p := range jsonPaths {
			applyJSONPathMask(doc, p, mask)
		}
		if canonical, err := utils.CanonicalJSON(doc); err == nil {
			return string(canonical)
		}
	}
	return string(trimmed)
}

func applyJSONPathMask(doc any, path string, mask string) {
	if !strings.HasPrefix(path, "$.") {
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "$."), ".")
	if len(parts) == 0 {
		return
	}
	current, ok := doc.(map[string]any)
	if !ok {
		return
	}
	for i := 0; i < len(parts)-1; i++ {
		next, exists := current[parts[i]]
		if !exists {
			return
		}
		nextObj, ok := next.(map[string]any)
		if !ok {
			return
		}
		current = nextObj
	}
	leaf := parts[len(parts)-1]
	if _, exists := current[leaf]; exists {
		current[leaf] = mask
	}
}
