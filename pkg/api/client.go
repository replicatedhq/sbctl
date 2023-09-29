package api

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
)

func createConfigFile(endPoint string) (string, error) {
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

	configString := fmt.Sprintf(ctxTemplate, endPoint)
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
