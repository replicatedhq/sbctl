package tests

import (
	_ "embed"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed results/configmaps_all_namespaces.json
var expectedGetAllConfigmapsResult string

//go:embed results/configmaps_kube_public_namespace.json
var expectedGetKubePublicConfigmapsResult string

var _ = Describe("GET /api/v1/configmaps", func() {
	Context("When getting configmaps in all namespaces", func() {
		It("Returns all configmaps", func() {
			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf("%s/api/v1/configmaps", apiServerEndpoint), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetAllConfigmapsResult))
		})
	})
})

var _ = Describe("GET /api/v1/namespaces/{namespace}/configmaps", func() {
	Context("When getting configmaps in kube-public namespace", func() {
		It("Returns kube-public configmaps", func() {
			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf("%s/api/v1/namespaces/kube-public/configmaps", apiServerEndpoint), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetKubePublicConfigmapsResult))
		})
	})
})
