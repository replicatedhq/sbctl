package tests

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Authentication", func() {
	var (
		authToken        string
		tlsCert          *api.TLSCertificate
		kubeConfig       string
		authenticatedURL string
		httpClient       *http.Client
	)

	BeforeEach(func() {
		// Load cluster data
		clusterData, err := sbctl.FindClusterData("./support-bundle")
		Expect(err).NotTo(HaveOccurred())

		// Generate auth token and TLS certificate
		authToken, err = api.GenerateToken()
		Expect(err).NotTo(HaveOccurred())
		Expect(authToken).NotTo(BeEmpty())

		tlsCert, err = api.GenerateSelfSignedCert()
		Expect(err).NotTo(HaveOccurred())
		Expect(tlsCert).NotTo(BeNil())

		// Start API server with authentication
		kubeConfig, err = api.StartAPIServer(clusterData, os.Stderr, authToken, tlsCert)
		Expect(err).NotTo(HaveOccurred())

		// Get the endpoint
		endpoint, err := getAPIEndpoint(kubeConfig)
		Expect(err).NotTo(HaveOccurred())
		authenticatedURL = endpoint

		// Create HTTP client that skips TLS verification (for self-signed cert)
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		httpClient = &http.Client{
			Transport: transport,
		}
	})

	AfterEach(func() {
		if kubeConfig != "" {
			os.RemoveAll(kubeConfig)
		}
	})

	Context("When authentication is enabled", func() {
		It("should reject requests without authorization header", func() {
			req, err := http.NewRequest("GET", authenticatedURL+"/api/v1", nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))

			var errorResp map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(errorResp["error"]).To(ContainSubstring("missing authorization header"))
		})

		It("should reject requests with invalid token", func() {
			req, err := http.NewRequest("GET", authenticatedURL+"/api/v1", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer invalid-token")

			resp, err := httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))

			var errorResp map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResp)
			Expect(err).NotTo(HaveOccurred())
			Expect(errorResp["error"]).To(ContainSubstring("invalid token"))
		})

		It("should reject requests with malformed authorization header", func() {
			testCases := []string{
				"invalid-token",       // No Bearer prefix
				"Basic " + authToken,  // Wrong auth type
				"bearer " + authToken, // Lowercase bearer
				"Bearer",              // No token
				"Bearer ",             // Empty token
			}

			for _, authHeader := range testCases {
				req, err := http.NewRequest("GET", authenticatedURL+"/api/v1", nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", authHeader)

				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized),
					fmt.Sprintf("Expected 401 for auth header: %q", authHeader))
			}
		})

		It("should accept requests with valid token", func() {
			req, err := http.NewRequest("GET", authenticatedURL+"/api/v1", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer "+authToken)
			req.Header.Set("Accept", "application/json")

			resp, err := httpClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Verify we get actual data back
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(body)).To(BeNumerically(">", 0))
		})

		It("should protect all endpoints with authentication", func() {
			endpoints := []string{
				"/api",
				"/api/v1",
				"/api/v1/pods",
				"/api/v1/namespaces/default/pods",
				"/apis",
				"/version",
			}

			for _, endpoint := range endpoints {
				// Without token - should fail
				req, err := http.NewRequest("GET", authenticatedURL+endpoint, nil)
				Expect(err).NotTo(HaveOccurred())

				resp, err := httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				resp.Body.Close()

				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized),
					fmt.Sprintf("Endpoint %s should require authentication", endpoint))

				// With token - should succeed (or at least not return 401)
				req, err = http.NewRequest("GET", authenticatedURL+endpoint, nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", "Bearer "+authToken)

				resp, err = httpClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				resp.Body.Close()

				Expect(resp.StatusCode).NotTo(Equal(http.StatusUnauthorized),
					fmt.Sprintf("Endpoint %s should accept valid token", endpoint))
			}
		})

		It("should include token in generated kubeconfig", func() {
			configData, err := os.ReadFile(kubeConfig)
			Expect(err).NotTo(HaveOccurred())

			var config map[string]interface{}
			err = yaml.Unmarshal(configData, &config)
			Expect(err).NotTo(HaveOccurred())

			// Check users section has token
			users, ok := config["users"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(users)).To(BeNumerically(">", 0))

			user := users[0].(map[interface{}]interface{})
			userData := user["user"].(map[interface{}]interface{})

			token, hasToken := userData["token"]
			Expect(hasToken).To(BeTrue(), "Token should be in kubeconfig")
			Expect(token).To(Equal(authToken))
		})

		It("should include TLS certificate in generated kubeconfig", func() {
			configData, err := os.ReadFile(kubeConfig)
			Expect(err).NotTo(HaveOccurred())

			var config map[string]interface{}
			err = yaml.Unmarshal(configData, &config)
			Expect(err).NotTo(HaveOccurred())

			// Check clusters section has certificate-authority-data
			clusters, ok := config["clusters"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(clusters)).To(BeNumerically(">", 0))

			cluster := clusters[0].(map[interface{}]interface{})
			clusterData := cluster["cluster"].(map[interface{}]interface{})

			certData, hasCert := clusterData["certificate-authority-data"]
			Expect(hasCert).To(BeTrue(), "Certificate data should be in kubeconfig")
			Expect(certData).NotTo(BeEmpty())
		})

		It("should use HTTPS when authentication is enabled", func() {
			configData, err := os.ReadFile(kubeConfig)
			Expect(err).NotTo(HaveOccurred())

			var config map[string]interface{}
			err = yaml.Unmarshal(configData, &config)
			Expect(err).NotTo(HaveOccurred())

			clusters, ok := config["clusters"].([]interface{})
			Expect(ok).To(BeTrue())

			cluster := clusters[0].(map[interface{}]interface{})
			clusterData := cluster["cluster"].(map[interface{}]interface{})

			server := clusterData["server"].(string)
			Expect(server).To(HavePrefix("https://"), "Server should use HTTPS when auth is enabled")
		})
	})

	Context("When authentication is disabled", func() {
		var (
			noAuthKubeConfig string
			noAuthURL        string
		)

		BeforeEach(func() {
			clusterData, err := sbctl.FindClusterData("./support-bundle")
			Expect(err).NotTo(HaveOccurred())

			// Start API server WITHOUT authentication
			noAuthKubeConfig, err = api.StartAPIServer(clusterData, os.Stderr, "", nil)
			Expect(err).NotTo(HaveOccurred())

			endpoint, err := getAPIEndpoint(noAuthKubeConfig)
			Expect(err).NotTo(HaveOccurred())
			noAuthURL = endpoint
		})

		AfterEach(func() {
			if noAuthKubeConfig != "" {
				os.RemoveAll(noAuthKubeConfig)
			}
		})

		It("should allow requests without authorization header", func() {
			req, err := http.NewRequest("GET", noAuthURL+"/api/v1", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Accept", "application/json")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("should not include token in generated kubeconfig", func() {
			configData, err := os.ReadFile(noAuthKubeConfig)
			Expect(err).NotTo(HaveOccurred())

			var config map[string]interface{}
			err = yaml.Unmarshal(configData, &config)
			Expect(err).NotTo(HaveOccurred())

			users, ok := config["users"].([]interface{})
			Expect(ok).To(BeTrue())

			user := users[0].(map[interface{}]interface{})
			userData := user["user"].(map[interface{}]interface{})

			_, hasToken := userData["token"]
			Expect(hasToken).To(BeFalse(), "Token should not be in kubeconfig when auth is disabled")
		})

		It("should use HTTP when authentication is disabled", func() {
			configData, err := os.ReadFile(noAuthKubeConfig)
			Expect(err).NotTo(HaveOccurred())

			var config map[string]interface{}
			err = yaml.Unmarshal(configData, &config)
			Expect(err).NotTo(HaveOccurred())

			clusters, ok := config["clusters"].([]interface{})
			Expect(ok).To(BeTrue())

			cluster := clusters[0].(map[interface{}]interface{})
			clusterData := cluster["cluster"].(map[interface{}]interface{})

			server := clusterData["server"].(string)
			Expect(server).To(HavePrefix("http://"), "Server should use HTTP when auth is disabled")
		})
	})
})

// Made with Bob
