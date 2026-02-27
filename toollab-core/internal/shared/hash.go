package shared

import (
	"crypto/sha256"
	"encoding/hex"
)

func SHA256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
