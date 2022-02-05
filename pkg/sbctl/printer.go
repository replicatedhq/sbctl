package sbctl

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/cmd/get"
	apisapps "k8s.io/kubernetes/pkg/apis/apps"
	apisappsv1 "k8s.io/kubernetes/pkg/apis/apps/v1"
	apicore "k8s.io/kubernetes/pkg/apis/core"
	apicorev1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/printers"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"
)

func PrintNamespacedGet(rootDir string, resource string, namespace string) error {
	if namespace == "" {
		namespace = "default"
	}
	fileName := filepath.Join(rootDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to load file")
	}

	err = Print(data)
	if err != nil {
		return errors.Wrap(err, "failed to print data")
	}

	return nil
}

func PrintClusterGet(rootDir string, resource string) error {
	fileName := filepath.Join(rootDir, fmt.Sprintf("%s.json", resource))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to load file")
	}

	err = Print(data)
	if err != nil {
		return errors.Wrap(err, "failed to print data")
	}

	return nil
}

func Print(data []byte) error {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, _, err := decode(data, nil, nil)
	if err != nil {
		return errors.Wrap(err, "failed to decode file")
	}

	// TODO: convert more types
	switch o := decoded.(type) {
	case *corev1.PodList:
		converted := &apicore.PodList{}
		err = apicorev1.Convert_v1_PodList_To_core_PodList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert pod list")
		}
		decoded = converted
	case *corev1.Pod:
		converted := &apicore.Pod{}
		err = apicorev1.Convert_v1_Pod_To_core_Pod(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert pod")
		}
		decoded = converted
	case *appsv1.ReplicaSetList:
		converted := &apisapps.ReplicaSetList{}
		apisappsv1.Convert_v1_ReplicaSetList_To_apps_ReplicaSetList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert replicaset list")
		}
		decoded = converted
	case *appsv1.ReplicaSet:
		converted := &apisapps.ReplicaSet{}
		apisappsv1.Convert_v1_ReplicaSet_To_apps_ReplicaSet(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert replicaset list")
		}
		decoded = converted
	case *corev1.NamespaceList:
		converted := &apicore.NamespaceList{}
		apicorev1.Convert_v1_NamespaceList_To_core_NamespaceList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert namespace list")
		}
		decoded = converted
	case *corev1.Namespace:
		converted := &apicore.Namespace{}
		apicorev1.Convert_v1_Namespace_To_core_Namespace(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert namespace")
		}
		decoded = converted
	default:
		// no conversion needed
	}

	ctx := context.TODO()
	tableOptions := &metav1.TableOptions{}
	tableConvertor := printerstorage.TableConvertor{
		TableGenerator: printers.NewTableGenerator().With(printersinternal.AddHandlers),
	}
	tableObject, err := tableConvertor.ConvertToTable(ctx, decoded, tableOptions)
	if err != nil {
		return errors.Wrap(err, "failed to create table")
	}

	yes := true
	// f := &get.PrintFlags{
	// 	// JSONYamlPrintFlags *genericclioptions.JSONYamlPrintFlags
	// 	// NamePrintFlags     *genericclioptions.NamePrintFlags
	// 	// CustomColumnsFlags *CustomColumnsPrintFlags
	// 	HumanReadableFlags: &get.HumanPrintFlags{
	// 		ShowKind: &yes,
	// 		// ShowLabels   *bool
	// 		// SortBy       *string
	// 		// ColumnLabels *[]string
	// 		// NoHeaders bool
	// 		// Kind          schema.GroupKind
	// 		WithNamespace: yes,
	// 	},
	// 	// TemplateFlags      *genericclioptions.KubeTemplatePrintFlags

	// 	// NoHeaders    *bool
	// 	// OutputFormat *string
	// } //.WithTypeSetter(scheme.Scheme)
	f := &get.HumanPrintFlags{
		ShowKind: &yes,
		// ShowLabels   *bool
		// SortBy       *string
		// ColumnLabels *[]string
		// NoHeaders bool
		// Kind          schema.GroupKind
		// WithNamespace: yes,
	}

	printer, err := f.ToPrinter("") // "wide"
	if err != nil {
		return errors.Wrap(err, "failed to create printer")
	}

	err = printer.PrintObj(tableObject, os.Stdout)
	if err == nil {
		return errors.Wrap(err, "failed to print")
	}

	return nil
}
