package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "0.0.1"

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sbctl",
		Version:      version,
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

	cmd.AddCommand(ServeCmd())
	cmd.AddCommand(ShellCmd())

	viper.BindPFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}

func InitAndExecute(v string) {
	version = v
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
