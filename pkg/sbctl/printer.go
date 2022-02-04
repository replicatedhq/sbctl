package sbctl

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/cmd/get"
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
	// 1. parse json
	// 2. convert to table
	// 3. print

	// extensionsscheme.AddToScheme(scheme.Scheme)
	// decode := scheme.Codecs.UniversalDeserializer().Decode
	// obj, _, err := decode(crd, nil, nil)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to decode crd")
	// }

	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, _, err := decode(data, nil, nil)
	if err != nil {
		return errors.Wrap(err, "unable to decode file")
	}

	// fmt.Printf("+++++decoded:%#v\n", decoded)

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
		WithNamespace: yes,
	}

	printer, err := f.ToPrinter("") // "wide"
	if err != nil {
		return errors.Wrap(err, "faile to create printer")
	}

	err = printer.PrintObj(decoded, os.Stdout)
	if err == nil {
		return errors.Wrap(err, "faile to print")
	}

	return nil
}

// return rest.NewDefaultTableConvertor(e.DefaultQualifiedResource).ConvertToTable(ctx, object, tableOptions)
