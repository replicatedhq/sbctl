package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var (
	kubernetesConfigFlags *genericclioptions.ConfigFlags
)

func init() {
	kubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
}

func addKubectlFlags(flags *flag.FlagSet) {
	kubernetesConfigFlags.AddFlags(flags)
}

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sbctl",
		Short:        "Run commands against a support bundle",
		Long:         `Run commands against a support bundle`,
		SilenceUsage: true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
	}

	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("SBCTL")
		viper.AutomaticEnv()
	})

	addKubectlFlags(cmd.PersistentFlags())

	cmd.AddCommand(GetCmd())
	cmd.AddCommand(DescribeCmd())
	cmd.AddCommand(APICmd())

	viper.BindPFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// addKubectlFlags(cmd.Flags())

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
