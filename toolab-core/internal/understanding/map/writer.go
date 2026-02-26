package mapmodel

import (
	"encoding/json"

	"toolab-core/pkg/utils"
)

func WriteCanonical(in *SystemMap) ([]byte, string, error) {
	doc := *in
	doc.SchemaVersion = 1

	tmp := doc
	tmp.GeneratedAtUTC = ""
	tmp.Determinism.SystemMapFingerprint = ""

	canon, err := utils.CanonicalJSON(tmp)
	if err != nil {
		return nil, "", err
	}
	fingerprint := utils.SHA256Hex(canon)
	doc.Determinism.SystemMapFingerprint = fingerprint

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	return raw, fingerprint, nil
}
