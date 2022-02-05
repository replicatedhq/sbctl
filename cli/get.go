package cli

import (
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "get [resource]",
		Args:          cobra.MinimumNArgs(1),
		Short:         "Get resources",
		Long:          `Get resources`,
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
			switch resourceKind {
			case "cronjob", "cronjobs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "cronjobs", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print cronjobs")
				}
			case "deployment", "deployments":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "deployments", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print deployments")
				}
			case "event", "events":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "events", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print events")
				}
			case "ingress", "ingresses":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "ingress", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print ingresses")
				}
			case "job", "jobs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "jobs", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print jobs")
				}
			case "limitrange", "limitranges":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "limitranges", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print limitranges")
				}
			case "pod", "pods":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "pods", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print pods")
				}
			case "pvc", "pvcs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "pvcs", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print pvcs")
				}
			case "replicaset", "replicasets", "rs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "replicasets", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print replicasets")
				}
			case "service", "services", "svc":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "services", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print services")
				}
			case "statefulset", "statefulsets":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "statefulsets", namespace)
				if err != nil {
					return errors.Wrap(err, "failed to print statefulsets")
				}
			case "ns", "namespace", "namespaces":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "namespaces")
				if err != nil {
					return errors.Wrap(err, "failed to print namespaces")
				}
			case "node", "nodes":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "nodes")
				if err != nil {
					return errors.Wrap(err, "failed to print nodes")
				}
			case "pv", "pvs":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "pvs")
				if err != nil {
					return errors.Wrap(err, "failed to print pvs")
				}
			case "resource", "resources":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "resources")
				if err != nil {
					return errors.Wrap(err, "failed to print resources")
				}
			case "storageclass", "storageclasses":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "storage-classes")
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
