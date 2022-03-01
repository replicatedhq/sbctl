package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

func (h handler) getAPIV1NamespaceResourceLog(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1NamespaceResourceLog")

	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	container := r.URL.Query().Get("container")

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, "logs", namespace, name, fmt.Sprintf("%s.log", container))
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			// try reading from -logs-errors.log file
			errFileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, "logs", namespace, name, fmt.Sprintf("%s-logs-errors.log", container))
			data, err = ioutil.ReadFile(errFileName)
			if err != nil {
				if os.IsNotExist(err) {
					w.Write([]byte(fmt.Sprintf("log files not found in support-bundle.\n%v\n%v", fileName, errFileName)))
					w.WriteHeader(http.StatusNotFound)
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	w.Write(data)
}
