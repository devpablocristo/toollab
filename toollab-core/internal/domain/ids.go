package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func stableHash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])[:16]
}

// EndpointID = hash(method + normalized_path_template)
func EndpointID(method, path string) string {
	return stableHash(method, NormalizePath(path))
}

// EvidenceID = hash(run_seed + endpoint_id + test_category + sequence_number)
func EvidenceID(runSeed string, endpointID string, category TestCategory, seq int) string {
	return stableHash(runSeed, endpointID, string(category), fmt.Sprintf("%d", seq))
}

// SchemaRefID = hash(endpoint_id + status_code + schema_fingerprint)
func SchemaRefID(endpointID string, status int, fingerprint string) string {
	return stableHash(endpointID, fmt.Sprintf("%d", status), fingerprint)
}

// SignatureID = hash(status_code + content_type + normalized_error_pattern)
func SignatureID(status int, contentType, normalizedPattern string) string {
	return stableHash(fmt.Sprintf("%d", status), contentType, normalizedPattern)
}

// ASTRefID produces a stable ID for an AST reference.
func ASTRefID(refType ASTRefType, parts ...string) string {
	all := append([]string{string(refType)}, parts...)
	return stableHash(all...)
}

// FindingID produces a deterministic finding ID.
func FindingID(taxonomyID string, endpointID string, evidenceIDs []string) string {
	sorted := make([]string, len(evidenceIDs))
	copy(sorted, evidenceIDs)
	sort.Strings(sorted)
	parts := append([]string{taxonomyID, endpointID}, sorted...)
	return stableHash(parts...)
}

// SchemaFingerprint produces a deterministic fingerprint from field definitions.
func SchemaFingerprint(contentType string, fields []SchemaField) string {
	sorted := make([]string, len(fields))
	for i, f := range fields {
		opt := "required"
		if f.Optional {
			opt = "optional"
		}
		sorted[i] = fmt.Sprintf("%s:%s:%s", f.Name, f.Type, opt)
	}
	sort.Strings(sorted)
	return stableHash(append([]string{contentType}, sorted...)...)
}

var (
	paramPattern = regexp.MustCompile(`:[a-zA-Z_][a-zA-Z0-9_]*`)
	uuidPattern  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	numIDPattern = regexp.MustCompile(`/\d+(/|$)`)
)

// NormalizePath converts parameter-style paths to template form.
// "/users/:id" → "/users/{id}", "/users/123" → "/users/{id}"
func NormalizePath(p string) string {
	p = strings.ReplaceAll(p, "//", "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = paramPattern.ReplaceAllStringFunc(p, func(s string) string {
		return "{" + s[1:] + "}"
	})
	return p
}

// NormalizeErrorPattern strips volatile data (UUIDs, timestamps, numbers) from error text.
func NormalizeErrorPattern(body string) string {
	s := strings.ToLower(body)
	if len(s) > 512 {
		s = s[:512]
	}
	s = uuidPattern.ReplaceAllString(s, "<uuid>")
	s = numIDPattern.ReplaceAllString(s, "/<id>/")
	return strings.TrimSpace(s)
}
