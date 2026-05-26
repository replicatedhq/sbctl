package api

import (
	"encoding/base64"
	"testing"
)

func TestGenerateToken_Success(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() failed: %v", err)
	}
	if token == "" {
		t.Error("GenerateToken() returned empty token")
	}
}

func TestGenerateToken_Length(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() failed: %v", err)
	}

	// Decode the base64 token to check the underlying byte length
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	expectedLength := 32 // 256 bits = 32 bytes
	if len(decoded) != expectedLength {
		t.Errorf("Token length = %d bytes, want %d bytes", len(decoded), expectedLength)
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	// Generate multiple tokens and ensure they're all unique
	tokens := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() failed on iteration %d: %v", i, err)
		}

		if tokens[token] {
			t.Errorf("GenerateToken() produced duplicate token: %s", token)
		}
		tokens[token] = true
	}

	if len(tokens) != iterations {
		t.Errorf("Expected %d unique tokens, got %d", iterations, len(tokens))
	}
}

func TestGenerateToken_Base64URLEncoding(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() failed: %v", err)
	}

	// Verify it's valid base64 URL encoding
	_, err = base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("Token is not valid base64 URL encoding: %v", err)
	}

	// Verify it doesn't contain characters invalid for URL encoding
	// Base64 URL encoding uses: A-Z, a-z, 0-9, -, _
	for _, char := range token {
		if !((char >= 'A' && char <= 'Z') ||
			(char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '=') {
			t.Errorf("Token contains invalid character for URL encoding: %c", char)
		}
	}
}

func TestGenerateToken_NotEmpty(t *testing.T) {
	for i := 0; i < 10; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken() failed: %v", err)
		}
		if len(token) == 0 {
			t.Error("GenerateToken() returned empty string")
		}
	}
}

// Made with Bob
