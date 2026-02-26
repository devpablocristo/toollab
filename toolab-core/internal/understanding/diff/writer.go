package diff

import (
	"encoding/json"

	"toolab-core/pkg/utils"
)

func WriteCanonical(in *Diff) ([]byte, string, error) {
	doc := *in
	doc.SchemaVersion = 1

	tmp := doc
	tmp.GeneratedAtUTC = ""
	tmp.Determinism.DiffFingerprint = ""
	canon, err := utils.CanonicalJSON(tmp)
	if err != nil {
		return nil, "", err
	}
	fp := utils.SHA256Hex(canon)
	doc.Determinism.DiffFingerprint = fp

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	return raw, fp, nil
}
