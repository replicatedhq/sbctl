package api

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func getClientset(endPoint string) (*kubernetes.Clientset, error) {
	filename, err := createConfigFile(endPoint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create config file")
	}

	cfg, err := getConfigFromfile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config from file")
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes clientset")
	}

	return clientset, nil
}

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
	kubeconfigFile, err := ioutil.TempFile("", "local-kubeconfig-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create config file")
	}
	defer kubeconfigFile.Close()

	if _, err := io.WriteString(kubeconfigFile, configString); err != nil {
		return "", errors.Wrap(err, "failed to write config file")
	}

	return kubeconfigFile.Name(), nil
}

func getConfigFromfile(filename string) (*rest.Config, error) {
	os.Setenv("KUBECONFIG", filename)

	cfg, err := config.GetConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config")
	}

	return cfg, nil
}
