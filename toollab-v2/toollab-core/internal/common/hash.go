package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func SHA256String(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func MustStableJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
