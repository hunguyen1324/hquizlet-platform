package sepay

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GenerateOrderCode generates a unique deposit order code: "DEP" + 16 hex chars = 19 chars.
// The prefix must match the Payment code structure configured on my.sepay.vn.
func GenerateOrderCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "DEP" + strings.ToUpper(hex.EncodeToString(b))
}
