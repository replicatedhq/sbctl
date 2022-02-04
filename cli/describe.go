package cli

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "describe [resource]",
		Args:          cobra.MinimumNArgs(1),
		Short:         "Describe resources",
		Long:          `Describe resources`,
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// v := viper.GetViper()
			return errors.New("not implemented")
		},
	}

	return cmd
}
