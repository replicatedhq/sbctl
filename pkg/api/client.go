package api

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
)

func createConfigFile(endPoint string, token string, tlsCert *TLSCertificate) (string, error) {
	var configString string

	if token != "" && tlsCert != nil {
		// HTTPS with bearer token authentication
		// Include CA certificate data for TLS verification
		certData := base64.StdEncoding.EncodeToString(tlsCert.CertPEM)
		configString = fmt.Sprintf(`apiVersion: v1
kind: Config
preferences: {}
current-context: default
clusters:
- name: default
  cluster:
    server: %s
    certificate-authority-data: %s
contexts:
- name: default
  context:
    cluster: default
    user: default
users:
- name: default
  user:
    token: %s
`, endPoint, certData, token)
	} else if token != "" {
		// HTTP with bearer token (shouldn't happen, but handle gracefully)
		// Use insecure-skip-tls-verify as fallback
		configString = fmt.Sprintf(`apiVersion: v1
kind: Config
preferences: {}
current-context: default
clusters:
- name: default
  cluster:
    server: %s
    insecure-skip-tls-verify: true
contexts:
- name: default
  context:
    cluster: default
    user: default
users:
- name: default
  user:
    token: %s
`, endPoint, token)
	} else {
		// No authentication
		ctxTemplate := `
apiVersion: v1
kind: Config
preferences: {}
current-context: default
clusters:
- name: default
  cluster:
    server: %s
contexts:
- name: default
  context:
    cluster: default
    user: default
users:
- name: default
  user: {}
`
		configString = fmt.Sprintf(ctxTemplate, endPoint)
	}

	kubeconfigFile, err := os.CreateTemp("", "local-kubeconfig-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create config file")
	}
	defer kubeconfigFile.Close()

	if _, err := io.WriteString(kubeconfigFile, configString); err != nil {
		return "", errors.Wrap(err, "failed to write config file")
	}

	return kubeconfigFile.Name(), nil
}
