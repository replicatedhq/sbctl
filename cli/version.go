package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version represents the version of sbctl
var Version string = "dev"

// GoVersion represents the Go version used to build the binary
var GoVersion string = runtime.Version()

// VersionCmd returns a command that displays the version of sbctl
func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of sbctl",
		Long:  `Print the version of sbctl`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sbctl version %s\ngo version %s\n", Version, GoVersion)
		},
	}

	return cmd
}
