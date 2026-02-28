package domain

import (
	"context"
	"net/url"
	"strings"
	"time"

	discoveryDomain "toollab-core/internal/discovery/usecases/domain"
	evidenceDomain "toollab-core/internal/evidence/usecases/domain"
	scenarioDomain "toollab-core/internal/scenario/usecases/domain"
	"toollab-core/internal/shared"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

var severityOrder = map[Severity]int{
	SeverityCritical: 0,
	SeverityHigh:     1,
	SeverityMedium:   2,
	SeverityLow:      3,
	SeverityInfo:     4,
}

func (s Severity) Order() int {
	if o, ok := severityOrder[s]; ok {
		return o
	}
	return 99
}

type Category string

const (
	CategoryAuth            Category = "auth"
	CategoryErrorLeak       Category = "error_leak"
	CategoryContractAnomaly Category = "contract_anomaly"
	CategoryStability       Category = "stability"
	CategoryContentType     Category = "content_type"
	CategorySize            Category = "size"
)

type AuditReport struct {
	SchemaVersion string    `json:"schema_version"`
	RunID         string    `json:"run_id"`
	CreatedAt     time.Time `json:"created_at"`
	Findings      []Finding `json:"findings"`
	Summary       Summary   `json:"summary"`
}

type Summary struct {
	FindingsTotal int            `json:"findings_total"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCategory    map[string]int `json:"by_category"`
}

type Finding struct {
	FindingID      string           `json:"finding_id"`
	RuleID         string           `json:"rule_id"`
	Category       Category         `json:"category"`
	Severity       Severity         `json:"severity"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	Confidence     float64          `json:"confidence"`
	ModelRefs      []shared.ModelRef `json:"model_refs,omitempty"`
	EvidenceRefs   []string         `json:"evidence_refs,omitempty"`
	Recommendation string           `json:"recommendation,omitempty"`
}

type Rule interface {
	ID() string
	Apply(ctx context.Context, in *Inputs) []Finding
}

type Inputs struct {
	ServiceModel *discoveryDomain.ServiceModel
	ScenarioPlan *scenarioDomain.ScenarioPlan
	EvidencePack *evidenceDomain.EvidencePack

	byEndpoint  map[string][]evidenceDomain.EvidenceItem
	byEvidence  map[string]evidenceDomain.EvidenceItem
	byCaseID    map[string]evidenceDomain.EvidenceItem
	endpointMap map[string]discoveryDomain.Endpoint
}

func NewInputs(
	model *discoveryDomain.ServiceModel,
	plan *scenarioDomain.ScenarioPlan,
	pack *evidenceDomain.EvidencePack,
) *Inputs {
	in := &Inputs{
		ServiceModel: model,
		ScenarioPlan: plan,
		EvidencePack: pack,
		byEndpoint:   make(map[string][]evidenceDomain.EvidenceItem),
		byEvidence:   make(map[string]evidenceDomain.EvidenceItem),
		byCaseID:     make(map[string]evidenceDomain.EvidenceItem),
		endpointMap:  make(map[string]discoveryDomain.Endpoint),
	}

	if pack != nil {
		for _, item := range pack.Items {
			in.byEvidence[item.EvidenceID] = item
			in.byCaseID[item.CaseID] = item
			key := endpointKeyFromURL(item.Request.Method, item.Request.URL)
			in.byEndpoint[key] = append(in.byEndpoint[key], item)
		}
	}

	if model != nil {
		for _, ep := range model.Endpoints {
			in.endpointMap[ep.Method+" "+ep.Path] = ep
		}
	}

	return in
}

func (in *Inputs) ItemsByEndpoint(method, path string) []evidenceDomain.EvidenceItem {
	return in.byEndpoint[method+" "+path]
}

func (in *Inputs) ItemByEvidenceID(id string) (evidenceDomain.EvidenceItem, bool) {
	item, ok := in.byEvidence[id]
	return item, ok
}

func (in *Inputs) MatchEndpoint(method, rawURL string) (discoveryDomain.Endpoint, bool) {
	path := extractPath(rawURL)
	for key, ep := range in.endpointMap {
		if ep.Method != method {
			continue
		}
		if matchPath(ep.Path, path) {
			return in.endpointMap[key], true
		}
	}
	return discoveryDomain.Endpoint{}, false
}

func endpointKeyFromURL(method, rawURL string) string {
	return method + " " + extractPath(rawURL)
}

func extractPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Path
}

func matchPath(pattern, actual string) bool {
	patParts := strings.Split(strings.Trim(pattern, "/"), "/")
	actParts := strings.Split(strings.Trim(actual, "/"), "/")
	if len(patParts) != len(actParts) {
		return false
	}
	for i, pp := range patParts {
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") {
			continue
		}
		if pp != actParts[i] {
			return false
		}
	}
	return true
}

func DeterministicFindingID(ruleID string, endpointKey string, evidenceIDs []string) string {
	raw := ruleID + "|" + endpointKey
	for _, eid := range evidenceIDs {
		raw += "|" + eid
	}
	return shared.SHA256Bytes([]byte(raw))[:16]
}
