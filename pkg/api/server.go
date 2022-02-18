package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apisapps "k8s.io/kubernetes/pkg/apis/apps"
	apisappsv1 "k8s.io/kubernetes/pkg/apis/apps/v1"
	apisbatch "k8s.io/kubernetes/pkg/apis/batch"
	apisbatchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	apisbatchv1beta1 "k8s.io/kubernetes/pkg/apis/batch/v1beta1"
	apicore "k8s.io/kubernetes/pkg/apis/core"
	apicorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
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

// fake, kubectl can't parse this anyways
type errorResponse struct {
	Error string `json:"error"`
}

func StartAPIServer(clusterData sbctl.ClusterData) (string, error) {
	h := handler{
		clusterData: clusterData,
	}

	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	r.HandleFunc("/api", h.getAPI)
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/v1", h.getAPIV1)
	apiv1Router := apiRouter.PathPrefix("/v1").Subrouter()
	apiv1Router.HandleFunc("/{resource}", h.getAPIV1ClusterResources)
	apiv1Router.HandleFunc("/{resource}/{name}", h.getAPIV1ClusterResource)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}", h.getAPIV1NamespaceResources)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}/{name}", h.getAPIV1NamespaceResource)

	r.HandleFunc("/apis", h.getAPIs)
	apisRouter := r.PathPrefix("/apis").Subrouter()
	apisRouter.HandleFunc("/{group}/{version}", h.getAPIByGroupAndVersion)
	apisRouter.HandleFunc("/{group}/{version}/{resource}", h.getAPIsClusterResources)
	apisRouter.HandleFunc("/{group}/{version}/namespaces/{namespace}/{resource}", h.getAPIsNamespaceResources)
	apisRouter.HandleFunc("/{group}/{version}/namespaces/{namespace}/{resource}/{name}", h.getAPIsNamespaceResource)

	r.PathPrefix("/").HandlerFunc(h.getNotFound)

	srv := &http.Server{
		Handler: r,
		Addr:    localServerEndPoint,
	}
	go func() {
		panic(srv.ListenAndServe())
	}()

	time.Sleep(2) // TODO: poll until server starts

	configFile, err := createConfigFile(fmt.Sprintf("http://%s", localServerEndPoint))
	if err != nil {
		return "", errors.Wrap(err, "failed to create clientset for local endpoint")
	}

	return configFile, nil
}

func (h handler) getAPI(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPI")
	apiVersions := &metav1.APIVersions{
		Versions: []string{"v1"},
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: localServerEndPoint,
			},
		},
	}
	apiVersions.SetGroupVersionKind(schema.GroupVersionKind{
		Kind: "APIVersions",
	})

	JSON(w, http.StatusOK, apiVersions)
}

func (h handler) getAPIV1(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1")

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		log.Println("failed to load data", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	type simpleResource struct {
		Kind         string      `json:"kind"`
		APIVersion   *string     `json:"apiVersion,omitempty"`
		GroupVersion string      `json:"groupVersion"`
		Resources    interface{} `json:"resources"`
	}
	allResources := []simpleResource{}

	err = json.Unmarshal(data, &allResources)
	if err != nil {
		log.Println("failed to unmarshal data", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, resources := range allResources {
		if resources.APIVersion == nil && resources.GroupVersion == "v1" {
			JSON(w, http.StatusOK, resources)
			return
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIV1ClusterResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1ClusterResources")

	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	var result runtime.Object
	var err error
	filenames := []string{}
	switch resource {
	case "namespaces", "nodes", "pvs":
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", resource))}
	case "pods":
		result = &corev1.PodList{
			Items: []corev1.Pod{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "PodList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get pod files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "events":
		result = &corev1.EventList{
			Items: []corev1.Event{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "EventList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get event files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "limitranges":
		result = &corev1.LimitRangeList{
			Items: []corev1.LimitRange{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "LimitRangeList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get event files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "services":
		result = &corev1.ServiceList{
			Items: []corev1.Service{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Version: "v1",
			Kind:    "ServiceList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get service files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	for _, fileName := range filenames {
		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Println("failed to load file", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, gvk, err := sbctl.Decode(resource, data)
		if err != nil {
			log.Println("failed to decode wrapped", resource, ":", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// No need to do type conversions if only one file is returned.
		// This will always be the case for cluster level resources, and sometimes for namespaced resources.
		if len(filenames) == 1 {
			JSON(w, http.StatusOK, decoded)
			return
		}

		// TODO: filter list by selector
		// selector := r.URL.Query().Get("fieldSelector")

		switch o := decoded.(type) {
		case *corev1.EventList:
			r := result.(*corev1.EventList)
			r.Items = append(r.Items, o.Items...)
		case *corev1.PodList:
			r := result.(*corev1.PodList)
			r.Items = append(r.Items, o.Items...)
		case *corev1.LimitRangeList:
			r := result.(*corev1.LimitRangeList)
			r.Items = append(r.Items, o.Items...)
		case *corev1.ServiceList:
			r := result.(*corev1.ServiceList)
			r.Items = append(r.Items, o.Items...)
		default:
			log.Println("wrong gvk is found", gvk)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if asTable {
		table, err := toTable(result)
		if err != nil {
			log.Println("could not convert to table", err)
		} else {
			result = table
		}
	}

	JSON(w, http.StatusOK, result)
}

func (h handler) getAPIV1ClusterResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1ClusterResources")

	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]

	filename := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", resource))
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, _, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Println("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: filter list by selector
	// selector := r.URL.Query().Get("fieldSelector")

	switch o := decoded.(type) {
	case *corev1.NamespaceList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.NodeList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.PersistentVolumeList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIV1NamespaceResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1NamespaceResources")

	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, _, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Println("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: filter list by selector
	// selector := r.URL.Query().Get("fieldSelector")

	if asTable {
		table, err := toTable(decoded)
		if err != nil {
			log.Println("could not convert to table", err)
		} else {
			decoded = table
		}
	}

	JSON(w, http.StatusOK, decoded)
}

func (h handler) getAPIV1NamespaceResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1NamespaceResource")

	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, gvk, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Println("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch o := decoded.(type) {
	case *corev1.EventList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.PodList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.LimitRangeList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.ServiceList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	default:
		log.Println("wrong gvk is found", gvk)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIs(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIs")

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "groups.json"))
	if err != nil {
		log.Println("failed to load data", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	allGroups := []metav1.APIGroup{}
	err = json.Unmarshal(data, &allGroups)
	if err != nil {
		log.Println("failed to unmarshal data", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	filteredGroups := []metav1.APIGroup{}
	for _, group := range allGroups {
		// kubectl automatically adds v1 group. not filetring these out causes a duplicate resource error on the client side.
		if len(group.Versions) == 1 && group.Versions[0].GroupVersion == "v1" {
			continue
		}
		filteredGroups = append(filteredGroups, group)
	}
	groupList := map[string]interface{}{
		"kind":       "APIGroupList",
		"apiVersion": "v1",
		"groups":     filteredGroups,
	}

	JSON(w, http.StatusOK, groupList)
}

func (h handler) getAPIByGroupAndVersion(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIByGroupAndVersion")

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]

	data, err := ioutil.ReadFile(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		log.Println("failed to load data", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
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
		log.Println("failed to unmarshal data", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	groupVersion := fmt.Sprintf("%s/%s", group, version)
	for _, resources := range allResources {
		if resources.GroupVersion == groupVersion {
			JSON(w, http.StatusOK, resources)
			return
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIsClusterResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsClusterResources")

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	var result runtime.Object
	var err error
	filenames := []string{}
	switch resource {
	case "jobs":
		result = &batchv1.JobList{
			Items: []batchv1.Job{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "JobList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get job files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "cronjobs":
		result = &batchv1beta1.CronJobList{
			Items: []batchv1beta1.CronJob{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "CronJobList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get cronjob files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "deployments":
		result = &appsv1.DeploymentList{
			Items: []appsv1.Deployment{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "DeploymentList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get deployment files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "replicasets":
		result = &appsv1.ReplicaSetList{
			Items: []appsv1.ReplicaSet{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "ReplicaSetList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get replicaset files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "statefulsets":
		result = &appsv1.StatefulSetList{
			Items: []appsv1.StatefulSet{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "StatefulSetList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s", resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Println("failed to get replicaset files from dir", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	case "ingresses":
		log.Println("get ingresses is not implemented")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, fileName := range filenames {
		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			log.Println("failed to load file", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, gvk, err := sbctl.Decode(resource, data)
		if err != nil {
			log.Println("failed to decode wrapped", resource, ":", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: filter list by selector
		// selector := r.URL.Query().Get("fieldSelector")

		switch o := decoded.(type) {
		case *batchv1.JobList:
			r := result.(*batchv1.JobList)
			r.Items = append(r.Items, o.Items...)
		case *batchv1beta1.CronJobList:
			r := result.(*batchv1beta1.CronJobList)
			r.Items = append(r.Items, o.Items...)
		case *appsv1.DeploymentList:
			r := result.(*appsv1.DeploymentList)
			r.Items = append(r.Items, o.Items...)
		case *appsv1.ReplicaSetList:
			r := result.(*appsv1.ReplicaSetList)
			r.Items = append(r.Items, o.Items...)
		case *appsv1.StatefulSetList:
			r := result.(*appsv1.StatefulSetList)
			r.Items = append(r.Items, o.Items...)
		default:
			log.Println("wrong gvk is found", gvk)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	if asTable {
		table, err := toTable(result)
		if err != nil {
			log.Println("could not convert to table", err)
		} else {
			result = table
		}
	}

	JSON(w, http.StatusOK, result)
	return
}

func (h handler) getAPIsNamespaceResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsNamespaceResources")

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, gvk, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Println("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if gvk.Group != group && gvk.Version != version {
		log.Println("wrong gvk is found", gvk)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if asTable {
		table, err := toTable(decoded)
		if err != nil {
			log.Println("could not convert to table", err)
		} else {
			decoded = table
		}
	}

	JSON(w, http.StatusOK, decoded)
}

func (h handler) getAPIsNamespaceResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsNamespaceResource")

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing
	if asTable {
		log.Printf("TODO: as=Table is not yet implemeted")
	}

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, resource, fmt.Sprintf("%s.json", namespace))
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Println("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, gvk, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Println("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if gvk.Group != group && gvk.Version != version {
		log.Println("wrong gvk is found", gvk)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch o := decoded.(type) {
	case *appsv1.ReplicaSetList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *appsv1.DeploymentList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *appsv1.DaemonSetList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *appsv1.StatefulSetList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *batchv1.JobList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getNotFound(w http.ResponseWriter, r *http.Request) {
	log.Println("called getNotFound")

	var b bytes.Buffer
	_, _ = io.Copy(&b, r.Body)

	body := b.Bytes()
	if len(body) > 0 {
		log.Printf("body:%s\n", body)
	}

	http.Error(w, "", http.StatusNotFound)
	return
}

func JSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(code)
	w.Write(response)
}

func getJSONFileListFromDir(dir string) ([]string, error) {
	filenames := []string{}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read dir")
	}

	for _, file := range files {
		if file.IsDir() || strings.ToLower(filepath.Ext(file.Name())) != ".json" {
			continue
		}
		filenames = append(filenames, filepath.Join(dir, file.Name()))
	}

	return filenames, nil
}

func toTable(object runtime.Object) (runtime.Object, error) {
	switch o := object.(type) {
	case *corev1.PodList:
		converted := &apicore.PodList{}
		err := apicorev1.Convert_v1_PodList_To_core_PodList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert pod list")
		}
		object = converted
	case *corev1.Pod:
		converted := &apicore.Pod{}
		err := apicorev1.Convert_v1_Pod_To_core_Pod(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert pod")
		}
		object = converted
	case *appsv1.ReplicaSetList:
		converted := &apisapps.ReplicaSetList{}
		err := apisappsv1.Convert_v1_ReplicaSetList_To_apps_ReplicaSetList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert replicaset list")
		}
		object = converted
	case *appsv1.ReplicaSet:
		converted := &apisapps.ReplicaSet{}
		err := apisappsv1.Convert_v1_ReplicaSet_To_apps_ReplicaSet(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert replicaset")
		}
		object = converted
	case *appsv1.DeploymentList:
		converted := &apisapps.DeploymentList{}
		err := apisappsv1.Convert_v1_DeploymentList_To_apps_DeploymentList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert deployment list")
		}
		object = converted
	case *appsv1.Deployment:
		converted := &apisapps.Deployment{}
		err := apisappsv1.Convert_v1_Deployment_To_apps_Deployment(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert deployment")
		}
		object = converted
	case *corev1.NamespaceList:
		converted := &apicore.NamespaceList{}
		err := apicorev1.Convert_v1_NamespaceList_To_core_NamespaceList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert namespace list")
		}
		object = converted
	case *corev1.Namespace:
		converted := &apicore.Namespace{}
		err := apicorev1.Convert_v1_Namespace_To_core_Namespace(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert namespace")
		}
		object = converted
	case *corev1.EventList:
		converted := &apicore.EventList{}
		err := apicorev1.Convert_v1_EventList_To_core_EventList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert event list")
		}
		object = converted
	case *corev1.Event:
		converted := &apicore.Event{}
		err := apicorev1.Convert_v1_Event_To_core_Event(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert event")
		}
		object = converted
	case *batchv1beta1.CronJobList:
		converted := &apisbatch.CronJobList{}
		err := apisbatchv1beta1.Convert_v1beta1_CronJobList_To_batch_CronJobList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert cronjob list")
		}
		object = converted
	case *batchv1beta1.CronJob:
		converted := &apisbatch.CronJob{}
		err := apisbatchv1beta1.Convert_v1beta1_CronJob_To_batch_CronJob(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert cronjob")
		}
		object = converted
	case *batchv1.JobList:
		converted := &apisbatch.JobList{}
		err := apisbatchv1.Convert_v1_JobList_To_batch_JobList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert job list")
		}
		object = converted
	case *batchv1.Job:
		converted := &apisbatch.Job{}
		err := apisbatchv1.Convert_v1_Job_To_batch_Job(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert job")
		}
		object = converted
	default:
		// no conversion needed
	}

	ctx := context.TODO()
	tableOptions := &metav1.TableOptions{}
	tableConvertor := printerstorage.TableConvertor{
		TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers),
	}
	table, err := tableConvertor.ConvertToTable(ctx, object, tableOptions)
	if err != nil {
		return nil, errors.Wrap(err, "could not convert to table")
	}

	table.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "meta.k8s.io",
		Version: "v1",
		Kind:    "Table",
	})
	for i := range table.Rows {
		row := &table.Rows[i]
		row.Object.Object.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "meta.k8s.io",
			Version: "v1",
			Kind:    "PartialObjectMetadata",
		})
	}

	return table, nil
}
