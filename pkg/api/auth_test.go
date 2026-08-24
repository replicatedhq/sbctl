package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	expectedToken := "test-token-12345"
	called := false

	// Create a test handler that should be called
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with auth middleware
	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	// Create request with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+expectedToken)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Next handler was not called with valid token")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	expectedToken := "correct-token"
	called := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Next handler was called with invalid token")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error != "invalid token" {
		t.Errorf("Error message = %q, want %q", response.Error, "invalid token")
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	expectedToken := "test-token"
	called := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header set
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Next handler was called without authorization header")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var response errorResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Error != "missing authorization header" {
		t.Errorf("Error message = %q, want %q", response.Error, "missing authorization header")
	}
}

func TestAuthMiddleware_MalformedBearer(t *testing.T) {
	testCases := []struct {
		name   string
		header string
	}{
		{"No Bearer prefix", "test-token"},
		{"Wrong prefix", "Basic test-token"},
		{"Lowercase bearer", "bearer test-token"},
		{"No space", "Bearertest-token"},
		{"Multiple spaces", "Bearer  test-token"},
		{"Empty after Bearer", "Bearer "},
		{"Just Bearer", "Bearer"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedToken := "test-token"
			called := false

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			})

			middleware := authMiddleware(expectedToken)
			handler := middleware(nextHandler)

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if called {
				t.Errorf("Next handler was called with malformed header: %q", tc.header)
			}

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Status code = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	expectedToken := "test-token"
	called := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Next handler was called with empty token")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_CaseSensitiveToken(t *testing.T) {
	expectedToken := "TestToken123"
	called := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	// Try with different case
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer testtoken123")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if called {
		t.Error("Next handler was called with wrong case token (tokens should be case-sensitive)")
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ContentType(t *testing.T) {
	expectedToken := "test-token"

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestAuthMiddleware_MultipleRequests(t *testing.T) {
	expectedToken := "test-token"
	callCount := 0

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	// Make multiple requests with valid token
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+expectedToken)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Status code = %d, want %d", i, w.Code, http.StatusOK)
		}
	}

	if callCount != 5 {
		t.Errorf("Handler called %d times, want 5", callCount)
	}
}

func TestAuthMiddleware_PreservesRequestContext(t *testing.T) {
	expectedToken := "test-token"
	contextPreserved := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if we can still access request properties
		if r.Method == "GET" && r.URL.Path == "/test" {
			contextPreserved = true
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := authMiddleware(expectedToken)
	handler := middleware(nextHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+expectedToken)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if !contextPreserved {
		t.Error("Request context was not preserved through middleware")
	}
}

// Made with Bob
