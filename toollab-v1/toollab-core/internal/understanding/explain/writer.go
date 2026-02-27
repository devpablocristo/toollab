package explain

import (
	"encoding/json"

	"toollab-core/pkg/utils"
)

func WriteCanonical(in *Understanding) ([]byte, string, error) {
	doc := *in
	doc.SchemaVersion = 1

	tmp := doc
	tmp.GeneratedAtUTC = ""
	tmp.Determinism.UnderstandingFingerprint = ""
	canon, err := utils.CanonicalJSON(tmp)
	if err != nil {
		return nil, "", err
	}
	fp := utils.SHA256Hex(canon)
	doc.Determinism.UnderstandingFingerprint = fp

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	return raw, fp, nil
}
