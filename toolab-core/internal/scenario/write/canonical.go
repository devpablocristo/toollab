package write

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"toolab-core/internal/scenario"
	"toolab-core/pkg/utils"
)

const CanonicalWriterVersion = "scenario-yaml-v1"

var topLevelOrder = []string{
	"version",
	"mode",
	"target",
	"workload",
	"chaos",
	"expectations",
	"seeds",
	"observability",
	"redaction",
}

func WriteCanonicalScenario(s *scenario.Scenario) ([]byte, string, error) {
	if s == nil {
		return nil, "", fmt.Errorf("scenario is nil")
	}
	normalized := cloneScenario(*s)
	canonicalizeScenario(&normalized)

	rootNode, err := scenarioToOrderedNode(normalized)
	if err != nil {
		return nil, "", err
	}
	out, err := yaml.Marshal(rootNode)
	if err != nil {
		return nil, "", fmt.Errorf("marshal yaml: %w", err)
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	canonicalJSON, err := utils.CanonicalJSON(normalized)
	if err != nil {
		return nil, "", fmt.Errorf("canonical json: %w", err)
	}
	return out, utils.SHA256Hex(canonicalJSON), nil
}

func canonicalizeScenario(s *scenario.Scenario) {
	sort.Strings(s.Redaction.Headers)
	sort.Strings(s.Redaction.JSONPaths)

	for i := range s.Workload.Requests {
		req := &s.Workload.Requests[i]
		if req.Query == nil {
			req.Query = map[string]string{}
		}
		if req.Headers == nil {
			req.Headers = map[string]string{}
		}
	}
	sort.SliceStable(s.Workload.Requests, func(i, j int) bool {
		leftFP := requestFingerprint(s.Workload.Requests[i])
		rightFP := requestFingerprint(s.Workload.Requests[j])
		if leftFP != rightFP {
			return leftFP < rightFP
		}
		return strings.ToLower(s.Workload.Requests[i].ID) < strings.ToLower(s.Workload.Requests[j].ID)
	})

	sort.SliceStable(s.Expectations.Invariants, func(i, j int) bool {
		a := invariantSortKey(s.Expectations.Invariants[i])
		b := invariantSortKey(s.Expectations.Invariants[j])
		return a < b
	})
}

func invariantSortKey(inv scenario.InvariantConfig) string {
	return fmt.Sprintf("%s|%03d|%.12f|%s", inv.Type, inv.Status, inv.Max, inv.RequestID)
}

func requestFingerprint(req scenario.RequestSpec) string {
	queryKeys := make([]string, 0, len(req.Query))
	for k := range req.Query {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	queryParts := make([]string, 0, len(queryKeys))
	for _, key := range queryKeys {
		queryParts = append(queryParts, key+"="+req.Query[key])
	}
	contentType := req.Headers["Content-Type"]
	bodyShape := "empty"
	switch {
	case len(req.JSONBody) > 0:
		sum := sha256.Sum256(req.JSONBody)
		bodyShape = fmt.Sprintf("%x", sum[:3])
	case req.Body != nil:
		sum := sha256.Sum256([]byte(*req.Body))
		bodyShape = fmt.Sprintf("%x", sum[:3])
	}
	return strings.Join([]string{
		strings.ToUpper(req.Method),
		req.Path,
		strings.Join(queryParts, "&"),
		contentType,
		bodyShape,
	}, "|")
}

func scenarioToOrderedNode(s scenario.Scenario) (*yaml.Node, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("marshal scenario json: %w", err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode scenario json: %w", err)
	}
	root, err := toOrderedYAMLNode(decoded, topLevelOrder)
	if err != nil {
		return nil, err
	}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	return doc, nil
}

func toOrderedYAMLNode(v any, preferred []string) (*yaml.Node, error) {
	switch t := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	case bool:
		if t {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}, nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}, nil
	case float64:
		if math.Trunc(t) == t {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%.0f", t)}, nil
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", t), "0"), ".")}, nil
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range t {
			child, err := toOrderedYAMLNode(item, nil)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, child)
		}
		return n, nil
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		keys := orderedKeys(t, preferred)
		for _, k := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}
			valNode, err := toOrderedYAMLNode(t[k], nil)
			if err != nil {
				return nil, err
			}
			n.Content = append(n.Content, keyNode, valNode)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("unsupported node type %T", v)
	}
}

func orderedKeys(m map[string]any, preferred []string) []string {
	if len(preferred) == 0 {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	seen := map[string]struct{}{}
	keys := make([]string, 0, len(m))
	for _, p := range preferred {
		if _, ok := m[p]; ok {
			keys = append(keys, p)
			seen[p] = struct{}{}
		}
	}
	rest := make([]string, 0, len(m))
	for k := range m {
		if _, ok := seen[k]; !ok {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(keys, rest...)
}

func cloneScenario(in scenario.Scenario) scenario.Scenario {
	raw, _ := json.Marshal(in)
	var out scenario.Scenario
	_ = json.Unmarshal(raw, &out)
	return out
}
