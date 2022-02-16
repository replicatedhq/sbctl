package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	localServerEndPoint = "127.0.0.1:31180" // TODO: find random port
)

var (
	errorNotFound = errorResponse{
		Error: "not found",
	}
)

type handler struct {
	clusterData sbctl.ClusterData
}

type errorResponse struct {
	Error string `json:"error"`
}

func StartAPIServer(clusterData sbctl.ClusterData) (*rest.Config, error) {
	h := handler{
		clusterData: clusterData,
	}

	r := mux.NewRouter()

	r.HandleFunc("/api", h.getAPI)
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/v1", h.getAPIV1)
	apiv1Router := apiRouter.PathPrefix("/v1").Subrouter()
	// apiv1Router.HandleFunc("/{objects}", h.getObjects)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}", h.getAPIV1Objects)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}/{name}", h.getAPIV1Object)

	r.HandleFunc("/apis", h.getAPIs)
	apisRouter := r.PathPrefix("/apis").Subrouter()
	apisRouter.HandleFunc("/{group}/{version}", h.getAPIByGroupAndVersion)
	apisRouter.HandleFunc("/{group}/{version}/namespaces/{namespace}/{resource}/{name}", h.getAPIsObject)

	r.PathPrefix("/").HandlerFunc(h.getNotFound)

	srv := &http.Server{
		Handler: r,
		Addr:    localServerEndPoint,
	}
	go func() {
		panic(srv.ListenAndServe())
	}()

	time.Sleep(2) // TODO: poll until server starts

	config, err := getConfig(fmt.Sprintf("http://%s", localServerEndPoint))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create clientset for local endpoint")
	}

	return config, nil
}

func (h handler) getAPI(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPI:%s\n", r.RequestURI)
	apiVersions := &metav1.APIVersions{
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: localServerEndPoint,
			},
		},
		Versions: []string{"v1"},
	}

	JSON(w, http.StatusOK, apiVersions)
}

func (h handler) getAPIV1(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIV1:%s\n", r.RequestURI)

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load data").Error(),
		})
		return
	}

	type simpleResource struct {
		Kind         string      `json:"kind"`
		APIVersion   string      `json:"apiVersion"`
		GroupVersion string      `json:"groupVersion"`
		Resources    interface{} `json:"resources"`
	}
	allResources := []simpleResource{}

	err = json.Unmarshal(data, &allResources)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to unmarshal data").Error(),
		})
		return
	}

	for _, resources := range allResources {
		fmt.Printf("+++++resources:%s, %s\n", resources.APIVersion, resources.GroupVersion)
		if resources.APIVersion == "" && resources.GroupVersion == "v1" {
			JSON(w, http.StatusOK, resources)
			return
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIV1Objects(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIV1Objects:%s\n", r.RequestURI)
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load file").Error(),
		})
		return
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, gvk, err := decode(data, nil, nil)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to decode file").Error(),
		})
		return
	}

	// TODO: filter list by selector?
	// selector := r.URL.Query().Get("fieldSelector")

	switch o := decoded.(type) {
	case *corev1.EventList:
		JSON(w, http.StatusOK, o)
		return
	default:
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Errorf("wrong gvk is found: %s", gvk).Error(),
		})
		return
	}
}

func (h handler) getAPIV1Object(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIV1Object:%s\n", r.RequestURI)
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load file").Error(),
		})
		return
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, gvk, err := decode(data, nil, nil)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to decode file").Error(),
		})
		return
	}

	switch o := decoded.(type) {
	case *corev1.PodList:
		for _, pod := range o.Items {
			if pod.Name == name {
				JSON(w, http.StatusOK, pod)
				return
			}
		}
	default:
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Errorf("wrong gvk is found: %s", gvk).Error(),
		})
		return
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getObjects(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getObjects:%s\n", r.RequestURI)
	//	apiv1Router.HandleFunc("/{objects}", func(w http.ResponseWriter, r *http.Request) {
	objects := mux.Vars(r)["objects"]
	fmt.Printf("++++++in GET handler, objects:%s from %s\n", objects, h.clusterData)
	JSON(w, http.StatusOK, map[string]string{})
}

func (h handler) getAPIs(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIs:%s\n", r.RequestURI)

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "groups.json"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load data").Error(),
		})
		return
	}

	var allGroups interface{}
	err = json.Unmarshal(data, &allGroups)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to unmarshal data").Error(),
		})
		return
	}

	groupList := map[string]interface{}{
		"kind":       "APIGroupList",
		"apiVersion": "v1",
		"groups":     allGroups,
	}

	JSON(w, http.StatusOK, groupList)
}

func (h handler) getAPIByGroupAndVersion(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIByGroupAndVersion:%s\n", r.RequestURI)

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load data").Error(),
		})
		return
	}

	type simpleResource struct {
		Kind         string      `json:"kind"`
		APIVersion   string      `json:"apiVersion"`
		GroupVersion string      `json:"groupVersion"`
		Resources    interface{} `json:"resources"`
	}
	allResources := []simpleResource{}

	err = json.Unmarshal(data, &allResources)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to unmarshal data").Error(),
		})
		return
	}

	groupVersion := fmt.Sprintf("%s/%s", group, version)
	for _, resources := range allResources {
		if resources.GroupVersion == groupVersion {
			fmt.Printf("+++++found:%s, %s\n", resources.APIVersion, resources.GroupVersion)
			JSON(w, http.StatusOK, resources)
			return
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIsObject(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++getAPIsObject:%s\n", r.RequestURI)
	// group := mux.Vars(r)["group"]
	// version := mux.Vars(r)["version"]
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to load file").Error(),
		})
		return
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, gvk, err := decode(data, nil, nil)
	if err != nil {
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Wrap(err, "failed to decode file").Error(),
		})
		return
	}

	switch o := decoded.(type) {
	case *appsv1.ReplicaSetList:
		for _, rs := range o.Items {
			if rs.Name == name {
				JSON(w, http.StatusOK, rs)
				return
			}
		}
	case *appsv1.DeploymentList:
		for _, d := range o.Items {
			if d.Name == name {
				JSON(w, http.StatusOK, d)
				return
			}
		}
	case *appsv1.DaemonSetList:
		for _, ds := range o.Items {
			if ds.Name == name {
				JSON(w, http.StatusOK, ds)
				return
			}
		}
	case *batchv1.JobList:
		for _, j := range o.Items {
			if j.Name == name {
				JSON(w, http.StatusOK, j)
				return
			}
		}
	default:
		JSON(w, http.StatusInternalServerError, errorResponse{
			Error: errors.Errorf("wrong gvk is found: %s", gvk).Error(),
		})
		return
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getNotFound(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("++++++404 getNotFound:%s\n", r.RequestURI)

	var b bytes.Buffer
	_, err := io.Copy(&b, r.Body)
	fmt.Printf("++++++err:%v, body:%s\n", err, b.Bytes())

	http.Error(w, "", http.StatusNotFound)
	return
}

func JSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %v\n", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func JSONBytes(w http.ResponseWriter, code int, response []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
