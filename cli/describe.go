package cli

import (
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "describe [resource] [name]",
		Args:          cobra.MinimumNArgs(2),
		Short:         "Describe resources",
		Long:          `Describe resources`,
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			bundlePath := os.Getenv("SBCTL_SUPPORT_BUNDLE_PATH")
			if bundlePath == "" {
				return errors.New("support bundle filename is required or SBCTL_SUPPORT_BUNDLE_PATH must be set")
			}

			if bundlePath == "" {
				return errors.New("support bundle filename is required or SBCTL_SUPPORT_BUNDLE_PATH must be set")
			}

			fileInfo, err := os.Stat(bundlePath)
			if err != nil {
				return errors.New("failed to stat input path")
			}

			bundleDir := bundlePath
			if !fileInfo.IsDir() {
				bundleDir, err = os.MkdirTemp("", "sbctl-")
				if err != nil {
					return errors.Wrap(err, "failed to create temp dir")
				}
				defer os.RemoveAll(bundleDir)

				err = sbctl.ExtractBundle(os.Args[1], bundleDir)
				if err != nil {
					return errors.Wrap(err, "failed to extract bundle")
				}
			}

			clusterData, err := sbctl.FindClusterData(bundleDir)
			if err != nil {
				return errors.Wrap(err, "failed to find cluster data")
			}

			namespace := v.GetString("namespace")
			resourceKind := args[0]
			resourceName := args[1]
			switch resourceKind {
			case "cronjob", "cronjobs":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "cronjobs", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print cronjobs")
				}
			case "deployment", "deployments":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "deployments", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print deployments")
				}
			case "event", "events":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "events", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print events")
				}
			case "ingress", "ingresses":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "ingress", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print ingresses")
				}
			case "job", "jobs":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "jobs", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print jobs")
				}
			case "limitrange", "limitranges":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "limitranges", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print limitranges")
				}
			case "pod", "pods":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "pods", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print pods")
				}
			case "pvc", "pvcs":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "pvcs", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print pvcs")
				}
			case "replicaset", "replicasets", "rs":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "replicasets", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print replicasets")
				}
			case "service", "services", "svc":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "services", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print services")
				}
			case "statefulset", "statefulsets":
				err := sbctl.PrintNamespacedDescribe(clusterData.ClusterResourcesDir, "statefulsets", resourceName, namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print statefulsets")
				}
			case "ns", "namespace", "namespaces":
				err := sbctl.PrintClusterDescribe(clusterData.ClusterResourcesDir, "namespaces", resourceName)
				if err != nil {
					return errors.Wrap(err, "failed to print namespaces")
				}
			case "node", "nodes":
				err := sbctl.PrintClusterDescribe(clusterData.ClusterResourcesDir, "nodes", resourceName)
				if err != nil {
					return errors.Wrap(err, "failed to print nodes")
				}
			case "pv", "pvs":
				err := sbctl.PrintClusterDescribe(clusterData.ClusterResourcesDir, "pvs", resourceName)
				if err != nil {
					return errors.Wrap(err, "failed to print pvs")
				}
			case "resource", "resources":
				err := sbctl.PrintClusterDescribe(clusterData.ClusterResourcesDir, "resources", resourceName)
				if err != nil {
					return errors.Wrap(err, "failed to print resources")
				}
			case "storageclass", "storageclasses":
				err := sbctl.PrintClusterDescribe(clusterData.ClusterResourcesDir, "storage-classes", resourceName)
				if err != nil {
					return errors.Wrap(err, "failed to print storageclasses")
				}
			default:
				return errors.Errorf("unknown resource: %s", args[0])
			}

			return nil
		},
	}

	return cmd
}
