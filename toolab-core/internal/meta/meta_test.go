package meta

import (
	"encoding/json"
	"testing"
)

func TestDeriveSeed_IsDeterministic(t *testing.T) {
	input := SeedInput{
		Inputs: map[string]string{
			"openapi_sha256": "abc",
			"profile_sha256": "def",
		},
		Options: map[string]any{
			"from": "toolab",
			"mode": "smoke",
		},
	}
	seedA, hashA, err := DeriveSeed(input)
	if err != nil {
		t.Fatalf("derive seed A: %v", err)
	}
	seedB, hashB, err := DeriveSeed(input)
	if err != nil {
		t.Fatalf("derive seed B: %v", err)
	}
	if seedA != seedB || hashA != hashB {
		t.Fatalf("derived seed/hash must be stable")
	}
}

func TestWriteCanonical_MetaFingerprintStable(t *testing.T) {
	doc := Document{
		Operation:     "generate",
		ToolabVersion: "0.1.0",
		Seed: SeedInfo{
			Provided:   true,
			InputSeed:  "123",
			Effective:  "123",
			Derivation: "provided",
		},
		Source: SourceInfo{
			Primary:   "openapi",
			Secondary: []string{},
			Inputs:    []string{"openapi_sha256=abc"},
		},
		Hashes: HashesInfo{OpenAPISHA256: "abc"},
		Options: map[string]any{
			"mode": "smoke",
		},
		Capabilities: CapabilityInfo{
			Declared:        []string{},
			Used:            []string{},
			MissingRequired: []string{},
		},
		Warnings: []string{},
		Unknowns: []string{},
	}
	rawA, fpA, err := WriteCanonical(doc)
	if err != nil {
		t.Fatalf("write canonical A: %v", err)
	}
	rawB, fpB, err := WriteCanonical(doc)
	if err != nil {
		t.Fatalf("write canonical B: %v", err)
	}
	if fpA != fpB {
		t.Fatalf("meta fingerprint must be stable")
	}
	var outA, outB map[string]any
	if err := json.Unmarshal(rawA, &outA); err != nil {
		t.Fatalf("decode meta A: %v", err)
	}
	if err := json.Unmarshal(rawB, &outB); err != nil {
		t.Fatalf("decode meta B: %v", err)
	}
	if outA["generated_at_utc"] == outB["generated_at_utc"] {
		// This can occasionally be equal if calls are in same second; avoid asserting inequality.
	}
	detA := outA["determinism"].(map[string]any)
	detB := outB["determinism"].(map[string]any)
	if detA["meta_fingerprint"] != detB["meta_fingerprint"] {
		t.Fatalf("embedded meta fingerprint mismatch")
	}
}
