package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetTokenFromReplicatedConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".replicated")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `profiles:
  prod:
    apiToken: prod-token-123
  staging:
    apiToken: staging-token-456
  empty:
    apiToken: ""
defaultProfile: prod
`
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Override the config path for testing
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	tests := []struct {
		name        string
		profile     string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "default profile",
			profile:   "",
			wantToken: "prod-token-123",
		},
		{
			name:      "explicit profile",
			profile:   "staging",
			wantToken: "staging-token-456",
		},
		{
			name:        "nonexistent profile",
			profile:     "nonexistent",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "empty token profile",
			profile:     "empty",
			wantErr:     true,
			errContains: "no apiToken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GetTokenFromReplicatedConfig(tt.profile)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantToken {
				t.Fatalf("got token %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestGetTokenFromReplicatedConfig_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := GetTokenFromReplicatedConfig("")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
