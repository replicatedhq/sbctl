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
			case "pod", "pods":
				err := sbctl.PrintNamespacedGet(clusterData.ClusterResourcesDir, "pods", v.GetString("namespace"))
				if err != nil {
					return errors.Wrap(err, "failed to pring pods")
				}
			case "ns", "namespace", "namespaces":
				err := sbctl.PrintClusterGet(clusterData.ClusterResourcesDir, "namespaces")
				if err != nil {
					return errors.Wrap(err, "failed to pring namespaces")
				}
			default:
				return errors.Errorf("unknown resource: %s", args[0])
			}

			return nil
		},
	}

	return cmd
}
