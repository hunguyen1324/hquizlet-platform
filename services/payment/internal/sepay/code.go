package sepay

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GenerateOrderCode generates a unique deposit order code: "DEP" + 8 hex chars = 11 chars.
// The prefix must match the Payment code structure configured on my.sepay.vn.
func GenerateOrderCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "DEP" + strings.ToUpper(hex.EncodeToString(b))
}
