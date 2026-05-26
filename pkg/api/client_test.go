package api

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestCreateConfigFile_WithoutAuth(t *testing.T) {
	endpoint := "http://127.0.0.1:12345"

	configPath, err := createConfigFile(endpoint, "", nil)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Parse as YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config as YAML: %v", err)
	}

	// Verify structure
	if config["apiVersion"] != "v1" {
		t.Errorf("apiVersion = %v, want v1", config["apiVersion"])
	}

	if config["kind"] != "Config" {
		t.Errorf("kind = %v, want Config", config["kind"])
	}

	// Check clusters
	clusters, ok := config["clusters"].([]interface{})
	if !ok || len(clusters) == 0 {
		t.Fatal("clusters not found or empty")
	}

	cluster := clusters[0].(map[interface{}]interface{})
	clusterData := cluster["cluster"].(map[interface{}]interface{})

	if clusterData["server"] != endpoint {
		t.Errorf("server = %v, want %v", clusterData["server"], endpoint)
	}

	// Verify no token in users
	users, ok := config["users"].([]interface{})
	if !ok || len(users) == 0 {
		t.Fatal("users not found or empty")
	}

	user := users[0].(map[interface{}]interface{})
	userData := user["user"].(map[interface{}]interface{})

	if _, hasToken := userData["token"]; hasToken {
		t.Error("Token should not be present when auth is disabled")
	}
}

func TestCreateConfigFile_WithAuth(t *testing.T) {
	endpoint := "https://127.0.0.1:12345"
	token := "test-token-12345"

	// Generate a test TLS certificate
	tlsCert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate TLS cert: %v", err)
	}

	configPath, err := createConfigFile(endpoint, token, tlsCert)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Parse as YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config as YAML: %v", err)
	}

	// Check clusters with TLS cert
	clusters, ok := config["clusters"].([]interface{})
	if !ok || len(clusters) == 0 {
		t.Fatal("clusters not found or empty")
	}

	cluster := clusters[0].(map[interface{}]interface{})
	clusterData := cluster["cluster"].(map[interface{}]interface{})

	if clusterData["server"] != endpoint {
		t.Errorf("server = %v, want %v", clusterData["server"], endpoint)
	}

	// Verify certificate-authority-data is present
	if _, hasCert := clusterData["certificate-authority-data"]; !hasCert {
		t.Error("certificate-authority-data should be present when TLS is enabled")
	}

	// Verify token in users
	users, ok := config["users"].([]interface{})
	if !ok || len(users) == 0 {
		t.Fatal("users not found or empty")
	}

	user := users[0].(map[interface{}]interface{})
	userData := user["user"].(map[interface{}]interface{})

	actualToken, hasToken := userData["token"]
	if !hasToken {
		t.Error("Token should be present when auth is enabled")
	}

	if actualToken != token {
		t.Errorf("token = %v, want %v", actualToken, token)
	}
}

func TestCreateConfigFile_TLSCertIncluded(t *testing.T) {
	endpoint := "https://127.0.0.1:12345"
	token := "test-token"

	tlsCert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate TLS cert: %v", err)
	}

	configPath, err := createConfigFile(endpoint, token, tlsCert)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify the certificate data is base64 encoded and present
	if !strings.Contains(string(data), "certificate-authority-data:") {
		t.Error("certificate-authority-data not found in config")
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config as YAML: %v", err)
	}

	clusters := config["clusters"].([]interface{})
	cluster := clusters[0].(map[interface{}]interface{})
	clusterData := cluster["cluster"].(map[interface{}]interface{})

	certData, ok := clusterData["certificate-authority-data"].(string)
	if !ok {
		t.Fatal("certificate-authority-data is not a string")
	}

	if len(certData) == 0 {
		t.Error("certificate-authority-data is empty")
	}
}

func TestCreateConfigFile_TokenIncluded(t *testing.T) {
	endpoint := "https://127.0.0.1:12345"
	expectedToken := "my-secret-token-12345"

	tlsCert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("Failed to generate TLS cert: %v", err)
	}

	configPath, err := createConfigFile(endpoint, expectedToken, tlsCert)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify token is in the file
	if !strings.Contains(string(data), expectedToken) {
		t.Error("Token not found in config file")
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config as YAML: %v", err)
	}

	users := config["users"].([]interface{})
	user := users[0].(map[interface{}]interface{})
	userData := user["user"].(map[interface{}]interface{})

	actualToken := userData["token"].(string)
	if actualToken != expectedToken {
		t.Errorf("token = %q, want %q", actualToken, expectedToken)
	}
}

func TestCreateConfigFile_FileCreated(t *testing.T) {
	endpoint := "http://127.0.0.1:12345"

	configPath, err := createConfigFile(endpoint, "", nil)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify file is readable
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("Config file is not readable: %v", err)
	}

	if len(data) == 0 {
		t.Error("Config file is empty")
	}
}

func TestCreateConfigFile_ValidYAML(t *testing.T) {
	testCases := []struct {
		name     string
		endpoint string
		token    string
		hasTLS   bool
	}{
		{"No auth", "http://127.0.0.1:8080", "", false},
		{"With auth", "https://127.0.0.1:8443", "token123", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tlsCert *TLSCertificate
			if tc.hasTLS {
				var err error
				tlsCert, err = GenerateSelfSignedCert()
				if err != nil {
					t.Fatalf("Failed to generate TLS cert: %v", err)
				}
			}

			configPath, err := createConfigFile(tc.endpoint, tc.token, tlsCert)
			if err != nil {
				t.Fatalf("createConfigFile() failed: %v", err)
			}
			defer os.Remove(configPath)

			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("Failed to read config file: %v", err)
			}

			// Verify it's valid YAML
			var config map[string]interface{}
			if err := yaml.Unmarshal(data, &config); err != nil {
				t.Errorf("Config is not valid YAML: %v", err)
			}
		})
	}
}

func TestCreateConfigFile_CurrentContext(t *testing.T) {
	endpoint := "http://127.0.0.1:12345"

	configPath, err := createConfigFile(endpoint, "", nil)
	if err != nil {
		t.Fatalf("createConfigFile() failed: %v", err)
	}
	defer os.Remove(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config as YAML: %v", err)
	}

	// Verify current-context is set
	currentContext, ok := config["current-context"]
	if !ok {
		t.Error("current-context not found in config")
	}

	if currentContext != "default" {
		t.Errorf("current-context = %v, want default", currentContext)
	}
}

// Made with Bob
