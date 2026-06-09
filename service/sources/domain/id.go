package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewRuleID generates a UUID-v4-style identifier for forwarding rules. It is the
// single shared generator for both manager-created rules and config-apply-created
// rules, so the two paths cannot drift in format. On the (practically impossible)
// event that the system CSPRNG fails, it falls back to a timestamp-based id.
func NewRuleID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}
