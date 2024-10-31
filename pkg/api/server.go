package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	stdLog "log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/gddo/httputil/header"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/k8s"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	sbctlutil "github.com/replicatedhq/sbctl/pkg/util"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/registry/generic"
	apisapps "k8s.io/kubernetes/pkg/apis/apps"
	apisappsv1 "k8s.io/kubernetes/pkg/apis/apps/v1"
	apisbatch "k8s.io/kubernetes/pkg/apis/batch"
	apisbatchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	apisbatchv1beta1 "k8s.io/kubernetes/pkg/apis/batch/v1beta1"
	apicore "k8s.io/kubernetes/pkg/apis/core"
	apicorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	networking "k8s.io/kubernetes/pkg/apis/networking"
	apinetworkingv1 "k8s.io/kubernetes/pkg/apis/networking/v1"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
)

const (
	localServerEndPoint = "127.0.0.1"
)

var (
	errorNotFound = errorResponse{
		Error: "not found",
	}
)

type handler struct {
	clusterData sbctl.ClusterData
}
type clusterVersion struct {
	Info   *version.Info `json:"info"`
	String string        `json:"string"`
}

// fake, kubectl can't parse this anyways
type errorResponse struct {
	Error string `json:"error"`
}

func StartAPIServer(clusterData sbctl.ClusterData, logOutput io.Writer) (string, error) {
	h := handler{
		clusterData: clusterData,
	}

	r := mux.NewRouter()
	r.Use(dumpRequestResponse)

	r.HandleFunc("/api", h.getAPI)
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/v1", h.getAPIV1)
	apiv1Router := apiRouter.PathPrefix("/v1").Subrouter()
	apiv1Router.HandleFunc("/{resource}", h.getAPIV1ClusterResources)
	apiv1Router.HandleFunc("/{resource}/{name}", h.getAPIV1ClusterResource)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}", h.getAPIV1NamespaceResources)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}/{name}", h.getAPIV1NamespaceResource)
	apiv1Router.HandleFunc("/namespaces/{namespace}/{resource}/{name}/log", h.getAPIV1NamespaceResourceLog)

	r.HandleFunc("/apis", h.getAPIs)
	apisRouter := r.PathPrefix("/apis").Subrouter()
	apisRouter.HandleFunc("/{group}/{version}", h.getAPIByGroupAndVersion)
	apisRouter.HandleFunc("/{group}/{version}/{resource}", h.getAPIsClusterResources)
	apisRouter.HandleFunc("/{group}/{version}/{resource}/{name}", h.getAPIsClusterResource)
	apisRouter.HandleFunc("/{group}/{version}/namespaces/{namespace}/{resource}", h.getAPIsNamespaceResources)
	apisRouter.HandleFunc("/{group}/{version}/namespaces/{namespace}/{resource}/{name}", h.getAPIsNamespaceResource)

	r.HandleFunc("/version", h.getVersion)

	r.PathPrefix("/").HandlerFunc(h.getNotFound)

	// Pipe the error server logs to the standard logger
	srvLogsPipe := log.StandardLogger().WriterLevel(log.ErrorLevel)
	srv := &http.Server{
		Handler:           handlers.LoggingHandler(logOutput, r), // Handler with logging
		Addr:              localServerEndPoint,
		ReadHeaderTimeout: 3 * time.Second,
		ErrorLog:          stdLog.New(srvLogsPipe, "", 0),
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:", localServerEndPoint))
	if err != nil {
		return "", errors.Wrap(err, "listening on port")
	}

	go func(server *http.Server, logsPipe *io.PipeWriter) {
		defer logsPipe.Close()

		err := server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			log.Panic(err)
		}
	}(srv, srvLogsPipe)

	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

WAIT_FOR_SERVER:
	for {
		select {
		case <-time.After(1):
			resp, err := http.Get(fmt.Sprintf("http://%s/api/v1", listener.Addr()))
			if err == nil && resp.StatusCode == http.StatusOK {
				break WAIT_FOR_SERVER
			}
		case <-ctx.Done():
			return "", errors.New("timeout waiting for server to start")
		}
	}

	configFile, err := createConfigFile(fmt.Sprintf("http://%s", listener.Addr()))
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

func (h handler) getVersion(w http.ResponseWriter, r *http.Request) {
	log.Println("called getVersion")
	data, err := readFileAndLog(h.clusterData.ClusterInfoFile)
	if err != nil {
		log.Error("failed to load data: ", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	var obj clusterVersion
	err = json.Unmarshal(data, &obj)
	if err != nil {
		log.Errorf("unable to parse the server version: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	JSON(w, http.StatusOK, obj.Info)
}

func (h handler) getAPIV1(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1")

	data, err := readFileAndLog(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		log.Error("failed to load data: ", err)
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
		log.Error("failed to unmarshal data: ", err)
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

	fieldSelector, err := fields.ParseSelector(r.URL.Query().Get("fieldSelector"))
	if err != nil {
		log.Error("failed to parse fieldSelector ", r.URL.Query().Get("fieldSelector"), ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	labelSelector, err := fields.ParseSelector(r.URL.Query().Get("labelSelector"))
	if err != nil {
		log.Error("failed to parse labelSelector ", r.URL.Query().Get("labelSelector"), ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var result runtime.Object
	filenames := []string{}
	switch resource {
	case "namespaces", "nodes", "persistentvolumes", "clusterroles", "clusterrolebindings":
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))}
	case "pods":
		result = k8s.GetEmptyPodList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get pod files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "events":
		result = k8s.GetEmptyEventList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get event files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "limitranges":
		result = k8s.GetEmptyLimitRangeList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get event files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "services":
		result = k8s.GetEmptyServiceList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get service files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "persistentvolumeclaims":
		result = k8s.GetEmptyPersistentVolumeClaimList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get persistentvolumeclaim files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "configmaps":
		result = k8s.GetEmptyConfigMapList()
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get configmap files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	for _, fileName := range filenames {
		// If we know the file does not exist, just respond with an empty list
		if !fileExists(fileName) {
			continue
		}

		data, err := readFileAndLog(fileName)
		if err != nil {
			log.Error("failed to load file: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, _, err := sbctl.Decode(resource, data)
		if err != nil {
			log.Error("failed to decode wrapped ", resource, ": ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: is this an AND or an OR
		decoded, err = filterObjectsByLabels(decoded, labelSelector)
		if err != nil {
			log.Error("failed to filter by labels: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded = filterObjectsByFields(decoded, fieldSelector)

		// The switch below is incomplete, so let's skip it if we are only dealing with 1 list of items
		if len(filenames) == 1 {
			result = decoded
			break
		}

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
		case *corev1.PersistentVolumeClaimList:
			r := result.(*corev1.PersistentVolumeClaimList)
			r.Items = append(r.Items, o.Items...)
		case *corev1.ConfigMapList:
			r := result.(*corev1.ConfigMapList)
			r.Items = append(r.Items, o.Items...)
		default:
			result, err = sbctl.ToUnstructuredList(decoded)
			if err != nil {
				log.Error("failed to convert type to unstructured: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	if result == nil {
		obj := unstructured.UnstructuredList{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mux.Vars(r)["group"],
			Version: mux.Vars(r)["version"],
			Kind:    resource,
		})

		result = &obj
	}

	if asTable {
		table, err := toTable(result, r)
		if err != nil {
			log.Error("could not convert to table: ", err)
		} else {
			result = table
		}
	}

	JSON(w, http.StatusOK, result)
}

func (h handler) getAPIV1ClusterResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIV1ClusterResource")

	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]

	filename := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))
	data, err := readFileAndLog(filename)
	if err != nil {
		log.Error("failed to load file: ", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, _, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Error("failed to decode wrapped ", resource, ": ", err)
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

	fieldSelector, err := fields.ParseSelector(r.URL.Query().Get("fieldSelector"))
	if err != nil {
		log.Error("failed to parse fieldSelector ", fieldSelector, ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	labelSelector, err := fields.ParseSelector(r.URL.Query().Get("labelSelector"))
	if err != nil {
		log.Error("failed to parse labelSelector ", r.URL.Query().Get("labelSelector"), ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource), fmt.Sprintf("%s.json", namespace))

	var decoded runtime.Object
	// If we know the file does not exist, just respond with an empty list
	if !fileExists(fileName) {
		obj := unstructured.UnstructuredList{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mux.Vars(r)["group"],
			Version: mux.Vars(r)["version"],
			Kind:    resource,
		})
		decoded = &obj
	} else {
		data, err := readFileAndLog(fileName)
		if err != nil {
			log.Error("failed to load file: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, _, err = sbctl.Decode(resource, data)
		if err != nil {
			log.Error("failed to decode wrapped ", resource, ": ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// TODO: is this an AND or an OR
		decoded, err = filterObjectsByLabels(decoded, labelSelector)
		if err != nil {
			log.Error("failed to filter by labels: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded = filterObjectsByFields(decoded, fieldSelector)
	}

	if asTable {
		table, err := toTable(decoded, r)
		if err != nil {
			log.Warn("could not convert to table: ", err)
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
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource), fmt.Sprintf("%s.json", namespace))

	data, err := readFileAndLog(fileName)
	if err != nil {
		log.Error("failed to load file: ", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, gvk, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Error("failed to decode wrapped ", resource, ": ", err)
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
	case *corev1.PersistentVolumeClaimList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *corev1.ConfigMapList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	default:
		uObjList, err := sbctl.ToUnstructuredList(decoded)
		if err != nil {
			log.Error("failed to convert type to unstructured: ", gvk)
			return
		} else {
			for _, item := range uObjList.Items {
				if item.GetName() == name {
					JSON(w, http.StatusOK, item)
					return
				}
			}
		}
	}

	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIs(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIs")

	data, err := readFileAndLog(filepath.Join(h.clusterData.ClusterResourcesDir, "groups.json"))
	if err != nil {
		log.Error("failed to load data: ", err)
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
		log.Error("failed to unmarshal data: ", err)
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

	data, err := readFileAndLog(filepath.Join(h.clusterData.ClusterResourcesDir, "resources.json"))
	if err != nil {
		log.Error("failed to load data: ", err)
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
		log.Error("failed to unmarshal data: ", err)
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

// This one below here needs to stay complete:
func (h handler) getAPIsClusterResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsClusterResources")

	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	var result runtime.Object
	var err error
	var filenames []string
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
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get job files from dir: ", err)
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
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get cronjob files from dir: ", err)
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
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get deployment files from dir: ", err)
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
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get replicaset files from dir: ", err)
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
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, resource)
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get replicaset files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "storageclasses":
		result = &storagev1.StorageClassList{
			Items: []storagev1.StorageClass{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "StorageClassList",
		})
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))}
		if err != nil {
			log.Error("failed to get storageclasses files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "customresourcedefinitions":
		result = &extensionsv1.CustomResourceDefinitionList{
			Items: []extensionsv1.CustomResourceDefinition{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "CustomResourceDefinitionList",
		})
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))}
		if err != nil {
			log.Error("failed to get customresourcedefinitions files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "ingresses":
		result = &networkingv1.IngressList{
			Items: []networkingv1.Ingress{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "IngressList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get ingresses files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "roles":
		result = &rbacv1.RoleList{
			Items: []rbacv1.Role{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "RoleList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get roles files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "clusterroles":
		result = &rbacv1.ClusterRoleList{
			Items: []rbacv1.ClusterRole{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "ClusterRoleList",
		})
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))}
		if err != nil {
			log.Error("failed to get clusterrole files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "rolebindings":
		result = &rbacv1.RoleBindingList{
			Items: []rbacv1.RoleBinding{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "RoleBindingList",
		})
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))
		filenames, err = getJSONFileListFromDir(dirName)
		if err != nil {
			log.Error("failed to get rolebindings files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "clusterrolebindings":
		result = &rbacv1.ClusterRoleBindingList{
			Items: []rbacv1.ClusterRoleBinding{},
		}
		result.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    "ClusterRoleBindingList",
		})
		filenames = []string{filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))}
		if err != nil {
			log.Error("failed to get cluster-role-binding files from dir: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "selfsubjectaccessreviews":
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("failed to read request body: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		decoded, gvk, err := sbctl.Decode(resource, body)
		if err != nil {
			log.Error("failed to decode wrapped ", resource, ": ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if selfReview, ok := decoded.(*authorizationv1.SelfSubjectAccessReview); ok {
			selfReview.Status.Allowed = true // In sbctl, we always allow self access reviews
			JSON(w, http.StatusOK, selfReview)
		} else {
			log.Warnf("We do not know gvk: %s\n", gvk)
			JSON(w, http.StatusNotFound, errorNotFound)
		}
		return
	default:
		dirName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource))

		// Check if its in custom resources dir
		if !pathExists(dirName) {
			g := fmt.Sprintf("%s.%s", resource, mux.Vars(r)["group"])
			dirName = filepath.Join(h.clusterData.ClusterResourcesDir, "custom-resources", g)
		}

		filenames, _ = getJSONFileListFromDir(dirName)

		// cluster-scoped resources have no directory
		// e.g.
		/* 		├── clusterconfigs.k0s.k0sproject.io
		   		│   ├── kube-system.json
		   		│   └── kube-system.yaml
		   		├── installations.embeddedcluster.replicated.com.json
		   		├── installations.embeddedcluster.replicated.com.yaml */
		if len(filenames) == 0 {
			filename := dirName + ".json"
			filenames = append(filenames, filename)
		}

	}

	for _, fileName := range filenames {
		// If we know the file does not exist, just respond with an empty list
		if !fileExists(fileName) {
			continue
		}

		data, err := readFileAndLog(fileName)
		if err != nil {
			log.Error("failed to load file: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, _, err := sbctl.Decode(resource, data)
		if err != nil {
			log.Error("failed to decode wrapped ", resource, ": ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// No need to do type conversions if only one file is returned.
		// This will always be the case for cluster level resources, and sometimes for namespaced resources.
		if len(filenames) == 1 {
			if asTable {
				if list, ok := decoded.(*unstructured.UnstructuredList); ok {
					sbctl.SortUnstructuredList(list)
					decoded = list
				}

				table, err := toTable(decoded, r)
				if err != nil {
					log.Warn("could not convert to table:", err)
				} else {
					decoded = table
				}
			}
			JSON(w, http.StatusOK, decoded)
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
		case *storagev1.StorageClassList:
			r := result.(*storagev1.StorageClassList)
			r.Items = append(r.Items, o.Items...)
		case *networkingv1.IngressList:
			r := result.(*networkingv1.IngressList)
			r.Items = append(r.Items, o.Items...)
		case *rbacv1.RoleList:
			r := result.(*rbacv1.RoleList)
			r.Items = append(r.Items, o.Items...)
		case *rbacv1.RoleBindingList:
			r := result.(*rbacv1.RoleBindingList)
			r.Items = append(r.Items, o.Items...)
		default:
			result, err = sbctl.ToUnstructuredList(decoded)
			if err != nil {
				log.Error("failed to convert type to unstructured list: ", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	if result == nil {
		obj := unstructured.UnstructuredList{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mux.Vars(r)["group"],
			Version: mux.Vars(r)["version"],
			Kind:    resource,
		})

		result = &obj
	}

	if asTable {
		if list, ok := result.(*unstructured.UnstructuredList); ok {
			sbctl.SortUnstructuredList(list)
			result = list
		}

		table, err := toTable(result, r)
		if err != nil {
			log.Warn("could not convert to table:", err)
		} else {
			result = table
		}
	}

	JSON(w, http.StatusOK, result)
}

func (h handler) getAPIsClusterResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsClusterResource")

	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	group := mux.Vars(r)["group"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing
	setResponse := func(d runtime.Object) {
		if asTable {
			table, err := toTable(d, r)
			if err != nil {
				log.Warn("could not convert to table: ", err)
			} else {
				d = table
			}
		}
		JSON(w, http.StatusOK, d)
	}
	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, fmt.Sprintf("%s.json", sbctlutil.GetSBCompatibleResourceName(resource)))

	if !fileExists(fileName) {
		fileName = filepath.Join(h.clusterData.ClusterResourcesDir, "custom-resources", fmt.Sprintf("%s.%s.json", resource, group))
	}

	data, err := readFileAndLog(fileName)
	if err != nil {
		log.Error("failed to load file", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, gvk, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Error("failed to decode wrapped", resource, ":", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch o := decoded.(type) {
	case *storagev1.StorageClassList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	case *extensionsv1.CustomResourceDefinitionList:
		for _, item := range o.Items {
			if item.Name == name {
				JSON(w, http.StatusOK, item)
				return
			}
		}
	default:
		uObjList, err := sbctl.ToUnstructuredList(decoded)
		if err != nil {
			log.Error("failed to convert type to unstructured: ", gvk)
			return
		}
		for _, item := range uObjList.Items {
			if item.GetName() == name {
				item := item
				setResponse(&item)
				return
			}
		}
	}
	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getAPIsNamespaceResources(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsNamespaceResources")

	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource), fmt.Sprintf("%s.json", namespace))

	// Check if its in custom resources dir
	if !fileExists(fileName) {
		dirName := fmt.Sprintf("%s.%s", resource, mux.Vars(r)["group"])
		fileName = filepath.Join(h.clusterData.ClusterResourcesDir, "custom-resources", dirName, fmt.Sprintf("%s.json", namespace))
	}

	var decoded runtime.Object
	// If the file does not exist, return an empty list
	if fileExists(fileName) {
		data, err := readFileAndLog(fileName)
		if err != nil {
			log.Error("failed to load file: ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		decoded, _, err = sbctl.Decode(resource, data)
		if err != nil {
			log.Error("failed to decode wrapped ", resource, ": ", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		obj := unstructured.UnstructuredList{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mux.Vars(r)["group"],
			Version: mux.Vars(r)["version"],
			Kind:    resource,
		})

		decoded = &obj
	}

	if asTable {
		table, err := toTable(decoded, r)
		if err != nil {
			log.Warn("could not convert to table: ", err)
		} else {
			decoded = table
		}
	}

	JSON(w, http.StatusOK, decoded)
}

func (h handler) getAPIsNamespaceResource(w http.ResponseWriter, r *http.Request) {
	log.Println("called getAPIsNamespaceResource")

	// It's important to respond with correct group and version here.  If the request is for batch/v1beta1/cronjobs,
	// we cannot return a batch/v1/cronjobs object.
	group := mux.Vars(r)["group"]
	version := mux.Vars(r)["version"]
	namespace := mux.Vars(r)["namespace"]
	resource := mux.Vars(r)["resource"]
	name := mux.Vars(r)["name"]
	asTable := strings.Contains(r.Header.Get("Accept"), "as=Table") // who needs parsing

	setResponse := func(d runtime.Object) {
		if asTable {
			table, err := toTable(d, r)
			if err != nil {
				log.Warn("could not convert to table: ", err)
			} else {
				d = table
			}
		}
		JSON(w, http.StatusOK, d)
	}

	fileName := filepath.Join(h.clusterData.ClusterResourcesDir, sbctlutil.GetSBCompatibleResourceName(resource), fmt.Sprintf("%s.json", namespace))

	// Check if its in custom resources dir
	if !fileExists(fileName) {
		dirName := fmt.Sprintf("%s.%s", resource, mux.Vars(r)["group"])
		fileName = filepath.Join(h.clusterData.ClusterResourcesDir, "custom-resources", dirName, fmt.Sprintf("%s.json", namespace))
	}

	data, err := readFileAndLog(fileName)
	if err != nil {
		log.Error("failed to load file: ", err)
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	decoded, _, err := sbctl.Decode(resource, data)
	if err != nil {
		log.Error("failed to decode wrapped ", resource, ": ", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if group == "apps" && version == "v1" {
		switch o := decoded.(type) {
		case *appsv1.ReplicaSetList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		case *appsv1.DeploymentList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		case *appsv1.DaemonSetList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		case *appsv1.StatefulSetList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		}
	}

	if group == "batch" && version == "v1" {
		switch o := decoded.(type) {
		case *batchv1.JobList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		case *batchv1.CronJobList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		}
	}

	if group == "batch" && version == "v1beta1" {
		switch o := decoded.(type) { // nolint: gocritic
		case *batchv1beta1.CronJobList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		}
	}

	if group == "networking" && version == "v1" {
		switch o := decoded.(type) { // nolint: gocritic
		case *networkingv1.IngressList:
			for _, item := range o.Items {
				if item.Name == name {
					item := item
					setResponse(&item)
					return
				}
			}
		}
	}

	uObjList, err := sbctl.ToUnstructuredList(decoded)
	if err != nil {
		log.Error("failed to convert type to unstructured list: ", err)
		return
	} else {
		for _, item := range uObjList.Items {
			if item.GetName() == name {
				item := item
				setResponse(&item)
				return
			}
		}
	}

	log.Printf("unknown type in group=%s version=%s: %T\n", group, version, decoded)
	JSON(w, http.StatusNotFound, errorNotFound)
}

func (h handler) getNotFound(w http.ResponseWriter, r *http.Request) {
	log.Println("called getNotFound")

	var b bytes.Buffer
	_, _ = io.Copy(&b, r.Body)

	body := b.Bytes()
	if len(body) > 0 {
		log.Printf("body: %s\n", body)
	}

	http.Error(w, "", http.StatusNotFound)
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func pathExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func JSON(w http.ResponseWriter, code int, payload interface{}) {
	if obj, ok := interface{}(payload).(runtime.Object); ok {
		log.Printf("Reponse GVK: (%s)\n", obj.GetObjectKind().GroupVersionKind())
	}

	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to marshal payload: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_, err = w.Write(response)
	if err != nil {
		log.Errorf("Failed to write response: %v\n", err)
	}
}

func getJSONFileListFromDir(dir string) ([]string, error) {
	filenames := []string{}

	files, err := os.ReadDir(dir)
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

func filterObjectsByLabels(object runtime.Object, selector fields.Selector) (runtime.Object, error) {
	if selector.Empty() {
		return object, nil
	}

	switch o := object.(type) {
	case *corev1.EventList:
		r := k8s.GetEmptyEventList()
		for _, i := range o.Items {
			if selector.Matches(labels.Set(i.GetObjectMeta().GetLabels())) {
				r.Items = append(r.Items, i)
			}
		}
		return r, nil
	case *corev1.PodList:
		r := k8s.GetEmptyPodList()
		for _, i := range o.Items {
			if selector.Matches(labels.Set(i.GetObjectMeta().GetLabels())) {
				r.Items = append(r.Items, i)
			}
		}
		return r, nil
	case *corev1.LimitRangeList:
		r := k8s.GetEmptyLimitRangeList()
		for _, i := range o.Items {
			if selector.Matches(labels.Set(i.GetObjectMeta().GetLabels())) {
				r.Items = append(r.Items, i)
			}
		}
		return r, nil
	case *corev1.ServiceList:
		r := k8s.GetEmptyServiceList()
		for _, i := range o.Items {
			if selector.Matches(labels.Set(i.GetObjectMeta().GetLabels())) {
				r.Items = append(r.Items, i)
			}
		}
		return r, nil
	case *corev1.PersistentVolumeClaimList:
		r := k8s.GetEmptyPersistentVolumeClaimList()
		for _, i := range o.Items {
			if selector.Matches(labels.Set(i.GetObjectMeta().GetLabels())) {
				r.Items = append(r.Items, i)
			}
		}
		return r, nil
	default:
		return nil, errors.Errorf("cannot filter type %v", object.GetObjectKind().GroupVersionKind())
	}
}

func filterObjectsByFields(object runtime.Object, selector fields.Selector) runtime.Object {
	if selector.Empty() {
		return object
	}

	switch o := object.(type) {
	case *corev1.EventList:
		filtered := &corev1.EventList{}
		for _, item := range o.Items {
			item := item
			if selector.Matches(eventToSelectableFields(&item)) {
				filtered.Items = append(filtered.Items, *item.DeepCopy())
			}
		}
		return filtered
	case *corev1.PodList:
		filtered := &corev1.PodList{}
		for _, item := range o.Items {
			item := item
			if selector.Matches(podToSelectableFields(&item)) {
				filtered.Items = append(filtered.Items, *item.DeepCopy())
			}
		}
		return filtered
	default:
		// TODO: do more
	}

	return object
}

// ToSelectableFields is available in "k8s.io/kubernetes/pkg/registry/core/core/event"
// This function is used to find object specific events for the describe commands
func eventToSelectableFields(event *corev1.Event) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&event.ObjectMeta, true)
	source := event.Source.Component
	if source == "" {
		source = event.ReportingController
	}
	specificFieldsSet := fields.Set{
		"involvedObject.kind":            event.InvolvedObject.Kind,
		"involvedObject.namespace":       event.InvolvedObject.Namespace,
		"involvedObject.name":            event.InvolvedObject.Name,
		"involvedObject.uid":             string(event.InvolvedObject.UID),
		"involvedObject.apiVersion":      event.InvolvedObject.APIVersion,
		"involvedObject.resourceVersion": event.InvolvedObject.ResourceVersion,
		"involvedObject.fieldPath":       event.InvolvedObject.FieldPath,
		"reason":                         event.Reason,
		"reportingComponent":             event.ReportingController, // use the core/v1 field name
		"source":                         source,
		"type":                           event.Type,
	}
	return generic.MergeFieldsSets(specificFieldsSet, objectMetaFieldsSet)
}

// podToSelectableFields extracts fields from a Pod object to be used for selection or filtering
func podToSelectableFields(pod *corev1.Pod) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&pod.ObjectMeta, true)
	specificFieldsSet := fields.Set{
		"spec.nodeName": pod.Spec.NodeName,
		"status.phase":  string(pod.Status.Phase),
	}
	return generic.MergeFieldsSets(specificFieldsSet, objectMetaFieldsSet)
}

func toTable(object runtime.Object, r *http.Request) (runtime.Object, error) {
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
	case *appsv1.StatefulSet:
		converted := &apisapps.StatefulSet{}
		err := apisappsv1.Convert_v1_StatefulSet_To_apps_StatefulSet(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert statefulset")
		}
		object = converted
	case *appsv1.StatefulSetList:
		converted := &apisapps.StatefulSetList{}
		err := apisappsv1.Convert_v1_StatefulSetList_To_apps_StatefulSetList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert statefulset list")
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
	case *corev1.PersistentVolumeClaimList:
		converted := &apicore.PersistentVolumeClaimList{}
		err := apicorev1.Convert_v1_PersistentVolumeClaimList_To_core_PersistentVolumeClaimList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert persistentvolumeclaim list")
		}
		object = converted
	case *corev1.PersistentVolumeClaim:
		converted := &apicore.PersistentVolumeClaim{}
		err := apicorev1.Convert_v1_PersistentVolumeClaim_To_core_PersistentVolumeClaim(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert persistentvolumeclaim")
		}
		object = converted
	case *corev1.PersistentVolumeList:
		converted := &apicore.PersistentVolumeList{}
		err := apicorev1.Convert_v1_PersistentVolumeList_To_core_PersistentVolumeList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert persistentvolume list")
		}
		object = converted
	case *corev1.PersistentVolume:
		converted := &apicore.PersistentVolume{}
		err := apicorev1.Convert_v1_PersistentVolume_To_core_PersistentVolume(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert persistentvolume")
		}
		object = converted
	case *corev1.NodeList:
		converted := &apicore.NodeList{}
		err := apicorev1.Convert_v1_NodeList_To_core_NodeList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert node list")
		}
		object = converted
	case *corev1.Node:
		converted := &apicore.Node{}
		err := apicorev1.Convert_v1_Node_To_core_Node(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert node")
		}
		object = converted
	case *corev1.ServiceList:
		converted := &apicore.ServiceList{}
		err := apicorev1.Convert_v1_ServiceList_To_core_ServiceList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert service list")
		}
		object = converted
	case *corev1.Service:
		converted := &apicore.Service{}
		err := apicorev1.Convert_v1_Service_To_core_Service(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert service")
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
	case *networkingv1.IngressList:
		converted := &networking.IngressList{}
		err := apinetworkingv1.Convert_v1_IngressList_To_networking_IngressList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert ingress list")
		}
		object = converted
	case *corev1.ConfigMapList:
		converted := &apicore.ConfigMapList{}
		err := apicorev1.Convert_v1_ConfigMapList_To_core_ConfigMapList(o, converted, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert configmap list")
		}
		object = converted
	}

	ctx := context.TODO()
	tableOptions := &metav1.TableOptions{}
	tableConvertor := printerstorage.TableConvertor{
		TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers),
	}
	table, err := tableConvertor.ConvertToTable(ctx, object, tableOptions)
	if err != nil {
		return nil, err
	}

	// TODO: github.com/golang/gddo is no longer maintained. We should
	// replace it with something else. https://github.com/golang/go/issues/44417
	// tracks a proposal to add this functionality to the standard library.
	_, accepts := header.ParseValueAndParams(r.Header, "Accept")
	g := accepts["g"]
	if g == "" {
		g = "meta.k8s.io"
	}

	v := accepts["v"]
	if v == "" {
		v = "v1"
	}

	table.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   g,
		Version: v,
		Kind:    "Table",
	})
	for i := range table.Rows {
		row := &table.Rows[i]
		m, err := meta.Accessor(row.Object.Object)
		if err != nil {
			return nil, err
		}
		// TODO: turn this into an internal type and do conversion in order to get object kind automatically set?
		partial := meta.AsPartialObjectMetadata(m)
		partial.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
			Group:   g,
			Version: v,
			Kind:    "PartialObjectMetadata",
		})
		row.Object.Object = partial
	}

	return table, nil
}

func readFileAndLog(filename string) ([]byte, error) {
	log.Printf("Reading %s file", filename)
	return os.ReadFile(filename)
}
