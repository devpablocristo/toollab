package generate

import (
	"encoding/json"
	"testing"
)

func TestParseInvariants_FiltersInvalidShapes(t *testing.T) {
	raw := json.RawMessage(`{
		"invariants": [
			{"type":"no_5xx_allowed"},
			{"type":"max_4xx_rate","max":0.2},
			{"type":"status_code_rate","status":429,"max":0.05},
			{"type":"idempotent_key_identical_response"},
			{"type":"idempotent_key_identical_response","request_id":"bad key"},
			{"type":"idempotent_key_identical_response","request_id":"idemp_1"},
			{"type":"max_4xx_rate","max":1.5},
			{"type":"status_code_rate","status":700,"max":0.1},
			{"type":"custom","max":0.1}
		]
	}`)

	got := parseInvariants(raw)
	if len(got) != 4 {
		t.Fatalf("expected 4 valid invariants, got %d", len(got))
	}

	if got[0].Type != "idempotent_key_identical_response" || got[0].RequestID != "idemp_1" {
		t.Fatalf("unexpected invariant[0]: %#v", got[0])
	}
	if got[1].Type != "max_4xx_rate" || got[1].Max != 0.2 {
		t.Fatalf("unexpected invariant[1]: %#v", got[1])
	}
	if got[2].Type != "no_5xx_allowed" {
		t.Fatalf("unexpected invariant[2]: %#v", got[2])
	}
	if got[3].Type != "status_code_rate" || got[3].Status != 429 || got[3].Max != 0.05 {
		t.Fatalf("unexpected invariant[3]: %#v", got[3])
	}
}

func TestParseInvariants_DeduplicatesDeterministically(t *testing.T) {
	raw := json.RawMessage(`{
		"invariants": [
			{"type":"no_5xx_allowed"},
			{"type":"no_5xx_allowed"},
			{"type":"max_4xx_rate","max":0.1},
			{"type":"max_4xx_rate","max":0.1}
		]
	}`)
	got := parseInvariants(raw)
	if len(got) != 2 {
		t.Fatalf("expected 2 deduped invariants, got %d", len(got))
	}
	if got[0].Type != "max_4xx_rate" || got[1].Type != "no_5xx_allowed" {
		t.Fatalf("unexpected invariant order/content: %#v", got)
	}
}
