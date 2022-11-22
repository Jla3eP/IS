package utils

import (
	"encoding/hex"
)

func ToHex(bytes []byte) string {
	return hex.EncodeToString(bytes)
}
