package cli

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/creack/pty"
	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

func ShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "shell",
		Short:         "Start interractive shell",
		Long:          `Start interractive shell`,
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var kubeConfig string
			var bundleDir string
			deleteBundleDir := false

			logFile, err := ioutil.TempFile("", "sbctl-server-logs-")
			if err == nil {
				fmt.Printf("API server logs will be written to %s\n", logFile.Name())
				defer logFile.Close()
				defer os.RemoveAll(logFile.Name())
				log.SetOutput(logFile)
			}

			go func() {
				signalChan := make(chan os.Signal, 1)
				signal.Notify(signalChan, os.Interrupt)
				<-signalChan
				if kubeConfig != "" {
					_ = os.RemoveAll(kubeConfig)
				}
				if deleteBundleDir && bundleDir != "" {
					os.RemoveAll(bundleDir)
				}
				os.Exit(0)
			}()

			v := viper.GetViper()

			// This only works with generated config, so let's make sure we don't mess up user's real files.
			bundleLocation := v.GetString("support-bundle-location")
			if bundleLocation == "" {
				return errors.New("support-bundle-location is required")
			}

			if strings.HasPrefix(bundleLocation, "http") {
				token := v.GetString("token")
				if token == "" {
					return errors.New("token is required when downloading bundle")
				}

				fmt.Printf("Downloading bundle\n")

				dir, err := downloadAndExtractBundle(bundleLocation, token)
				if err != nil {
					return errors.Wrap(err, "failed to stat input path")
				}
				bundleDir = dir
				deleteBundleDir = true
			} else {
				fileInfo, err := os.Stat(bundleLocation)
				if err != nil {
					return errors.Wrap(err, "failed to stat input path")
				}

				bundleDir = bundleLocation
				if !fileInfo.IsDir() {
					deleteBundleDir = true
					bundleDir, err = os.MkdirTemp("", "sbctl-")
					if err != nil {
						return errors.Wrap(err, "failed to create temp dir")
					}

					err = sbctl.ExtractBundle(bundleLocation, bundleDir)
					if err != nil {
						return errors.Wrap(err, "failed to extract bundle")
					}
				}
			}

			clusterData, err := sbctl.FindClusterData(bundleDir)
			if err != nil {
				return errors.Wrap(err, "failed to find cluster data")
			}

			kubeConfig, err = api.StartAPIServer(clusterData)
			if err != nil {
				return errors.Wrap(err, "failed to create api server")
			}
			defer os.RemoveAll(kubeConfig)

			shellCmd := os.Getenv("SHELL")
			if shellCmd == "" {
				return errors.New("SHELL environment is required for shell command")
			}

			shellExec := exec.Command(shellCmd)
			shellExec.Env = os.Environ()
			fmt.Printf("Starting new shell with KUBECONFIG. Press Ctl-D when done to end the shell and the sbctl server\n")
			shellPty, err := pty.Start(shellExec)

			// Handle pty size.
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, syscall.SIGWINCH)
			go func() {
				for range ch {
					if err := pty.InheritSize(os.Stdin, shellPty); err != nil {
						log.Printf("error resizing pty: %s", err)
					}
				}
			}()
			ch <- syscall.SIGWINCH // Initial resize.
			defer func() { signal.Stop(ch); close(ch) }()

			// Set stdin to raw mode.
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				panic(err)
			}
			defer func() {
				_ = term.Restore(int(os.Stdin.Fd()), oldState)
				fmt.Printf("sbctl shell exited\n")
			}()

			// Setup the shell
			setupCmd := fmt.Sprintf("export KUBECONFIG=%s\n", kubeConfig)
			io.WriteString(shellPty, setupCmd)
			io.CopyN(io.Discard, shellPty, 2*int64(len(setupCmd))) // Don't print to screen, terminal will echo anyway

			// Copy stdin to the pty and the pty to stdout.
			go func() { _, _ = io.Copy(shellPty, os.Stdin) }()
			go func() { _, _ = io.Copy(os.Stdout, shellPty) }()

			shellExec.Wait()
			return nil
		},
	}

	cmd.Flags().StringP("support-bundle-location", "s", "", "path to support bundle archive, directory, or URL")
	cmd.Flags().StringP("token", "t", "", "API token for authentication when fetching on-line bundles")
	return cmd
}
