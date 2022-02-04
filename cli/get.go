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
			if len(os.Args) == 2 {
				bundlePath = os.Args[1]
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

			switch args[0] {
			case "cronjob", "cronjobs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "cronjobs", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print cronjobs")
				}
			case "deployment", "deployments":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "deployments", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print deployments")
				}
			case "event", "events":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "events", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print events")
				}
			case "ingress", "ingresses":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "ingress", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print ingresses")
				}
			case "job", "jobs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "jobs", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print jobs")
				}
			case "limitrange", "limitranges":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "limitranges", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print limitranges")
				}
			case "pod", "pods":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "pods", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print pods")
				}
			case "pvc", "pvcs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "pvcs", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print pvcs")
				}
			case "replicaset", "replicasets", "rs":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "replicasets", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print replicasets")
				}
			case "service", "services", "svc":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "services", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to print services")
				}
			case "statefulset", "statefulsets":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "statefulsets", v.GetString("namespace"))
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
