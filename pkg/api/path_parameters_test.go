package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
)

func TestValidatePathParameters(t *testing.T) {
	tests := []struct {
		name  string
		value string
		code  int
	}{
		{name: "valid", value: "pods", code: http.StatusOK},
		{name: "parent directory", value: "..", code: http.StatusBadRequest},
		{name: "current directory", value: ".", code: http.StatusBadRequest},
		{name: "empty", value: "", code: http.StatusBadRequest},
		{name: "slash", value: "pods/logs", code: http.StatusBadRequest},
		{name: "backslash", value: `pods\\logs`, code: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = mux.SetURLVars(req, map[string]string{"resource": tt.value})
			recorder := httptest.NewRecorder()
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			validatePathParameters(next).ServeHTTP(recorder, req)

			if nextCalled != (tt.code == http.StatusOK) {
				t.Fatalf("next handler called = %t, want %t", nextCalled, tt.code == http.StatusOK)
			}
			if recorder.Code != tt.code {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.code)
			}
		})
	}
}

func TestGetAPIV1NamespaceResourceLogRejectsContainerTraversal(t *testing.T) {
	h := handler{clusterData: sbctl.ClusterData{ClusterResourcesDir: t.TempDir()}}
	req := httptest.NewRequest(http.MethodGet, "/?container=../../secret", nil)
	req = mux.SetURLVars(req, map[string]string{
		"namespace": "default",
		"resource":  "pods",
		"name":      "pod",
	})
	recorder := httptest.NewRecorder()

	h.getAPIV1NamespaceResourceLog(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestGetAPIV1NamespaceResourceLogReadsValidContainer(t *testing.T) {
	resourcesDir := t.TempDir()
	logDir := filepath.Join(resourcesDir, "pods", "logs", "default", "pod")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte("log output"), 0o600); err != nil {
		t.Fatal(err)
	}

	h := handler{clusterData: sbctl.ClusterData{ClusterResourcesDir: resourcesDir}}
	req := httptest.NewRequest(http.MethodGet, "/?container=app", nil)
	req = mux.SetURLVars(req, map[string]string{
		"namespace": "default",
		"resource":  "pods",
		"name":      "pod",
	})
	recorder := httptest.NewRecorder()

	h.getAPIV1NamespaceResourceLog(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "log output" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "log output")
	}
}
