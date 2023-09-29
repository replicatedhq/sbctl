package tests

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/antzucaro/matchr"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	yaml "gopkg.in/yaml.v2"
)

var (
	apiServerEndpoint string
)

var (
	describeHeaders = map[string]string{
		"Content-Type": "application/json",
	}
	getHeaders = map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io,application/json",
	}
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = BeforeSuite(func() {
	clusterData, err := sbctl.FindClusterData("./support-bundle")
	Expect(err).NotTo(HaveOccurred())

	Expect(clusterData.ClusterResourcesDir).To(Equal("support-bundle/cluster-resources"))
	Expect(clusterData.ClusterInfoFile).To(Equal("support-bundle/cluster-info/cluster_version.json"))

	kubeConfig, err := api.StartAPIServer(clusterData)
	Expect(err).NotTo(HaveOccurred())
	cleanup := func() error {
		return os.RemoveAll(kubeConfig)
	}
	DeferCleanup(cleanup)

	endpoint, err := getAPIEndpoint(kubeConfig)
	Expect(err).NotTo(HaveOccurred())
	apiServerEndpoint = endpoint
})

var _ = AfterSuite(func() {
})

func getAPIEndpoint(filename string) (string, error) {
	configData, err := os.ReadFile(filename)
	if err != nil {
		return "", errors.Wrap(err, "failed to read config data")
	}

	config := struct {
		Clusters []struct {
			Cluster struct {
				Server string `yaml:"server"`
			} `yaml:"cluster"`
		} `yaml:"clusters"`
	}{}
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal config data")
	}

	return config.Clusters[0].Cluster.Server, nil
}

func Similar(expected string) types.GomegaMatcher {
	return &similarMatcher{
		expected: expected,
	}
}

type similarMatcher struct {
	expected string
}

func (matcher *similarMatcher) Match(actual interface{}) (success bool, err error) {
	actualString, ok := actual.(string)
	if !ok {
		return false, errors.Errorf("cannot match %T, only stings are supported", actual)
	}
	score := matchr.Jaro(actualString, matcher.expected)
	fmt.Printf("Jaro score is %v\n", score)
	return score > 0.80, nil // 0.80 found by trial and error and may need to be adjusted
}

func (matcher *similarMatcher) FailureMessage(actual interface{}) (message string) {
	actualString, actualOK := actual.(string)
	if actualOK {
		return format.MessageWithDiff(actualString, "to equal", matcher.expected)
	}

	return format.Message(actual, "to equal", matcher.expected)
}

func (matcher *similarMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "not to equal", matcher.expected)
}
