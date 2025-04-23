package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sbctl",
		Short:        "Run commands against a support bundle",
		Long:         `Run commands against a support bundle`,
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
	}

	cobra.OnInitialize(func() {
		viper.SetEnvPrefix("SBCTL")
		viper.AutomaticEnv()
	})

	cmd.AddCommand(ServeCmd())
	cmd.AddCommand(ShellCmd())
	cmd.AddCommand(DownloadCmd())
	cmd.AddCommand(VersionCmd())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
