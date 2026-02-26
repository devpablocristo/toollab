package meta

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"toolab-core/pkg/utils"
)

const SchemaVersion = 1
const CanonicalWriterVersion = "meta-json-v1"

type Document struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	ToolabVersion string         `json:"toolab_version"`
	Seed          SeedInfo       `json:"seed"`
	Source        SourceInfo     `json:"source"`
	Hashes        HashesInfo     `json:"hashes"`
	Options       map[string]any `json:"options"`
	Capabilities  CapabilityInfo `json:"capabilities"`
	Warnings      []string       `json:"warnings"`
	Unknowns      []string       `json:"unknowns"`
	Changes       []Change       `json:"changes,omitempty"`
	Determinism   Determinism    `json:"determinism"`
	GeneratedAt   string         `json:"generated_at_utc"`
}

type SeedInfo struct {
	Provided   bool   `json:"provided"`
	InputSeed  string `json:"input_seed,omitempty"`
	Effective  string `json:"effective_seed"`
	Derivation string `json:"derivation"`
}

type SourceInfo struct {
	Primary   string   `json:"primary"`
	Secondary []string `json:"secondary"`
	Inputs    []string `json:"inputs"`
}

type HashesInfo struct {
	OpenAPISHA256       string `json:"openapi_sha256,omitempty"`
	ManifestSHA256      string `json:"manifest_sha256,omitempty"`
	ProfileSHA256       string `json:"profile_sha256,omitempty"`
	EvidenceSHA256      string `json:"evidence_sha256,omitempty"`
	BaseScenarioSHA256  string `json:"base_scenario_sha256,omitempty"`
	OutputScenarioSHA   string `json:"output_scenario_sha256,omitempty"`
	SystemMapSHA256     string `json:"system_map_sha256,omitempty"`
	UnderstandingSHA256 string `json:"understanding_sha256,omitempty"`
	DiffSHA256          string `json:"diff_sha256,omitempty"`
}

type CapabilityInfo struct {
	Declared        []string `json:"declared"`
	Used            []string `json:"used"`
	MissingRequired []string `json:"missing_required"`
}

type Change struct {
	Op         string `json:"op"`
	Path       string `json:"path"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
	BeforeHash string `json:"before_hash,omitempty"`
	AfterHash  string `json:"after_hash,omitempty"`
}

type Determinism struct {
	CanonicalWriterVersion string `json:"canonical_writer_version"`
	MetaFingerprint        string `json:"meta_fingerprint"`
}

type SeedInput struct {
	Inputs  map[string]string `json:"inputs"`
	Options map[string]any    `json:"options"`
}

func CanonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")
	query := u.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := url.Values{}
	for _, k := range keys {
		vals := append([]string(nil), query[k]...)
		sort.Strings(vals)
		for _, v := range vals {
			ordered.Add(k, v)
		}
	}
	u.RawQuery = ordered.Encode()
	return u.String()
}

func DeriveSeed(input SeedInput) (string, string, error) {
	canonical, err := utils.CanonicalJSON(input)
	if err != nil {
		return "", "", fmt.Errorf("canonical seed input: %w", err)
	}
	sum := sha256.Sum256(canonical)
	u := binary.BigEndian.Uint64(sum[0:8])
	return strconv.FormatUint(u, 10), hex.EncodeToString(sum[:]), nil
}

func WriteCanonical(doc Document) ([]byte, string, error) {
	doc.SchemaVersion = SchemaVersion
	doc.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	doc.Determinism.CanonicalWriterVersion = CanonicalWriterVersion

	tmp := doc
	tmp.GeneratedAt = ""
	tmp.Determinism.MetaFingerprint = ""
	canon, err := utils.CanonicalJSON(tmp)
	if err != nil {
		return nil, "", err
	}
	fp := utils.SHA256Hex(canon)
	doc.Determinism.MetaFingerprint = fp

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out, fp, nil
}

func SortStrings(values []string) []string {
	cp := append([]string(nil), values...)
	sort.Strings(cp)
	return cp
}

func NormalizeOutputPath(path string) string {
	if path == "" {
		return path
	}
	return filepath.Clean(path)
}
