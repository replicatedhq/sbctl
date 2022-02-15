package cli

import (
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	describecli "k8s.io/kubectl/pkg/cmd/describe"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
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

			// This only works with generated config, so let's make sure we don't mess up user's real files.
			err := os.Setenv("KUBECONFIG", "")
			if err != nil {
				return errors.New("failed to clear out KUBECONFIG value")
			}

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

			_, err = api.StartAPIServer(clusterData)
			if err != nil {
				return errors.Wrap(err, "failed to create api server")

			}

			defer func() {
				err := os.Remove(os.Getenv("KUBECONFIG"))
				if err != nil {
					log.Printf("failed to delete %s\n", os.Getenv("KUBECONFIG"))
				}
			}()

			kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
			flags := cmd.PersistentFlags()
			kubeConfigFlags.AddFlags(flags)
			matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
			matchVersionKubeConfigFlags.AddFlags(flags)
			f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

			namespace := v.GetString("namespace")
			if namespace == "" {
				namespace = "default"
			}

			describeOpts := &describecli.DescribeOptions{
				FilenameOptions: &resource.FilenameOptions{},
				DescriberSettings: &describe.DescriberSettings{
					ShowEvents: true,
					ChunkSize:  cmdutil.DefaultChunkSize,
				},
				BuilderArgs: args,
				NewBuilder:  f.NewBuilder,
				Namespace:   namespace,
				Describer: func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
					return describe.DescriberFn(f, mapping)
				},

				CmdParent: "kubectl",
				IOStreams: genericclioptions.IOStreams{
					In:     os.Stdin,
					Out:    os.Stdout,
					ErrOut: os.Stderr,
				},
			}
			if err := describeOpts.Run(); err != nil {
				return errors.Wrap(err, "failed to run describer")
			}

			return nil
		},
	}

	return cmd
}
