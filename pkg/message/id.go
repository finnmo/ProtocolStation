package message

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID generates a unique ID for messages
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
