package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateToken generates a cryptographically secure random token.
// The token is 256 bits (32 bytes) encoded as base64 URL-safe string.
func GenerateToken() (string, error) {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
