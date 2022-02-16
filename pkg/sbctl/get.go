package sbctl

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/cmd/get"
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

func PrintNamespacedGet(rootDir string, resource string, namespace string) error {
	if namespace == "" {
		namespace = "default"
	}
	fileName := filepath.Join(rootDir, resource, fmt.Sprintf("%s.json", namespace))

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to load file")
	}

	err = printGet(data)
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

	err = printGet(data)
	if err != nil {
		return errors.Wrap(err, "failed to print data")
	}

	return nil
}

func printGet(data []byte) error {
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
			return errors.Wrap(err, "failed to convert replicaset")
		}
		decoded = converted
	case *appsv1.DeploymentList:
		converted := &apisapps.DeploymentList{}
		apisappsv1.Convert_v1_DeploymentList_To_apps_DeploymentList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert deployment list")
		}
		decoded = converted
	case *appsv1.Deployment:
		converted := &apisapps.Deployment{}
		apisappsv1.Convert_v1_Deployment_To_apps_Deployment(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert deployment")
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
	case *corev1.EventList:
		converted := &apicore.EventList{}
		apicorev1.Convert_v1_EventList_To_core_EventList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert event list")
		}
		decoded = converted
	case *corev1.Event:
		converted := &apicore.Event{}
		apicorev1.Convert_v1_Event_To_core_Event(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert event")
		}
		decoded = converted
	case *batchv1beta1.CronJobList:
		converted := &apisbatch.CronJobList{}
		apisbatchv1beta1.Convert_v1beta1_CronJobList_To_batch_CronJobList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert cronjob list")
		}
		decoded = converted
	case *batchv1beta1.CronJob:
		converted := &apisbatch.CronJob{}
		apisbatchv1beta1.Convert_v1beta1_CronJob_To_batch_CronJob(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert cronjob")
		}
		decoded = converted
	case *batchv1.JobList:
		converted := &apisbatch.JobList{}
		apisbatchv1.Convert_v1_JobList_To_batch_JobList(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert job list")
		}
		decoded = converted
	case *batchv1.Job:
		converted := &apisbatch.Job{}
		apisbatchv1.Convert_v1_Job_To_batch_Job(o, converted, nil)
		if err != nil {
			return errors.Wrap(err, "failed to convert job")
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
