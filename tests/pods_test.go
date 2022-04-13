package tests

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed results/pods_all_namespaces.json
var expectedGetAllPodsResult string

//go:embed results/pods_velero_namespace.json
var expectedGetVeleroPodsResult string

//go:embed results/pods_restic_only.json
var expectedGetResticPodsResult string

var _ = Describe("GET /api/v1/pods", func() {
	Context("When getting pods in all namespaces", func() {
		It("Returns all pods", func() {
			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf("%s/api/v1/pods", apiServerEndpoint), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetAllPodsResult))
		})
	})

	Context("When getting pods in all namespaces by label", func() {
		It("Returns restic pods", func() {
			v := url.Values{}
			v.Set("labelSelector", "name=restic")

			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf(`%s/api/v1/pods?%s`, apiServerEndpoint, v.Encode()), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetResticPodsResult))
		})
	})
})

var _ = Describe("GET /api/v1/namespaces/{namespace}/pods", func() {
	Context("When getting pods in velero namespace", func() {
		It("Returns velero pods", func() {
			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf("%s/api/v1/namespaces/velero/pods", apiServerEndpoint), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetVeleroPodsResult))
		})
	})

	Context("When getting pods in velero namespace by label", func() {
		It("Returns restic pods", func() {
			v := url.Values{}
			v.Set("labelSelector", "name=restic")

			resp, statusCode, err := HTTPExec("GET", fmt.Sprintf("%s/api/v1/namespaces/velero/pods?%s", apiServerEndpoint, v.Encode()), getHeaders)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))
			Expect(resp).To(Similar(expectedGetResticPodsResult))
		})
	})
})
