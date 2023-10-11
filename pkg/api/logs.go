package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (h handler) getAPIV1NamespaceResourceLog(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1NamespaceResourceLog")

	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	container := r.URL.Query().Get("container")
	previous, _ := strconv.ParseBool(r.URL.Query().Get("previous"))

	logFileName := fmt.Sprintf("%s.log", container)
	if previous {
		logFileName = fmt.Sprintf("%s-previous.log", container)
	}

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, "logs", namespace, name, logFileName)
	data, err := os.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			// try reading from -logs-errors.log file
			errFileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, "logs", namespace, name, fmt.Sprintf("%s-logs-errors.log", container))
			data, err = os.ReadFile(errFileName)
			if err != nil {
				if os.IsNotExist(err) {
					PlainText(w, http.StatusNotFound, []byte(fmt.Sprintf("log files not found in support-bundle.\n%v\n%v", fileName, errFileName)))
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	PlainText(w, http.StatusOK, data)
}

func PlainText(w http.ResponseWriter, responseCode int, responseBody []byte) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(responseCode)
	_, err := w.Write(responseBody)
	if err != nil {
		log.Error("Failed to write response", err)
	}
}
