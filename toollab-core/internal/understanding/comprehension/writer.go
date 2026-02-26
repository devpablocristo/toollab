package comprehension

import (
	"encoding/json"

	"toollab-core/pkg/utils"
)

func WriteCanonical(r *Report) ([]byte, string, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, "", err
	}
	fp := utils.SHA256Hex(raw)
	return raw, fp, nil
}
