package util

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

var (
	// sbResourceCompatibilityMap
	sbResourceCompatibilityMap = map[string]string{
		"persistentvolumeclaims":    "pvcs",
		"persistentvolumes":         "pvs",
		"storageclasses":            "storage-classes",
		"ingresses":                 "ingress",
		"customresourcedefinitions": "custom-resource-definitions",
		"clusterrolebindings":       "clusterRoleBindings",
	}
)

// GetSBCompatibleResourceName returns SupportBundle compatible resource name if exists else the same resource name
func GetSBCompatibleResourceName(resource string) string {
	if val, ok := sbResourceCompatibilityMap[resource]; ok {
		return val
	}
	return resource
}

func GetOpenAIKey() (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAPI_API_KEY environment variable is not set")
	}
	return apiKey, nil
}

func GetGithubIssue() string {
	filePath := "github.yaml"
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	return string(content)
}
