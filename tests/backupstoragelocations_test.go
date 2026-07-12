package tests

import (
	_ "embed"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:embed results/backupstoragelocations_velero_namespace.json
var expectedGetBackupStorageLocationsResult string

var _ = Describe("GET /apis/velero.io/v1/namespaces/{namespace}/backupstoragelocations", func() {
	Context("When getting a custom resource whose CRD defines additionalPrinterColumns", func() {
		It("Renders the CRD-defined columns instead of only NAME/AGE", func() {
			resp, statusCode, err := HTTPExec(
				"GET",
				fmt.Sprintf("%s/apis/velero.io/v1/namespaces/velero/backupstoragelocations", apiServerEndpoint),
				getHeaders,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))

			// The CRD's additionalPrinterColumns must surface as table columns. Without
			// them a custom resource falls back to the built-in NAME/AGE table.
			Expect(resp).To(ContainSubstring(`"name":"Phase"`))
			Expect(resp).To(ContainSubstring(`"name":"Last Validated"`))
			Expect(resp).To(ContainSubstring(`"name":"Default"`))

			Expect(resp).To(Similar(expectedGetBackupStorageLocationsResult))
		})
	})
})
