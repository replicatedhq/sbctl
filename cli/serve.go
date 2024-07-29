package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/sbctl/pkg/api"
	"github.com/replicatedhq/sbctl/pkg/sbctl"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start API server",
		Long:          `Start API server`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var kubeConfig string
			var bundleDir string
			deleteBundleDir := false

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
			if len(args) > 0 && args[0] != "" {
				bundleLocation = args[0]
			}
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

			kubeConfig, err = api.StartAPIServer(clusterData, os.Stderr)
			if err != nil {
				return errors.Wrap(err, "failed to create api server")

			}
			defer os.RemoveAll(kubeConfig)

			fmt.Printf("Server is running\n\n")
			fmt.Printf("export KUBECONFIG=%s\n\n", kubeConfig)

			<-make(chan struct{})

			return nil
		},
	}

	cmd.Flags().StringP("support-bundle-location", "s", "", "path to support bundle archive, directory, or URL")
	cmd.Flags().StringP("token", "t", "", "API token for authentication when fetching on-line bundles")
	cmd.Flags().Bool("debug", false, "enable debug logging. This will include HTTP response bodies in logs.")
	return cmd
}

func downloadAndExtractBundle(bundleUrl string, token string) (string, error) {
	parsedUrl, err := url.Parse(bundleUrl)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse url")
	}

	_, slug := path.Split(parsedUrl.Path)
	if slug == "" {
		return "", errors.New("failed to extract slug from URL")
	}
	sbEndpoint := fmt.Sprintf("https://api.replicated.com/vendor/v3/supportbundle/%s", slug)
	req, err := http.NewRequest("GET", sbEndpoint, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to create HTTP request")
	}

	req.Header.Add("Authorization", token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read GQL response")
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	bundleObj := struct {
		Bundle struct {
			SignedUri string `json:"signedUri"`
		} `json:"bundle"`
	}{}
	err = json.Unmarshal(body, &bundleObj)
	if err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal response: %s", body)
	}

	resp, err = http.Get(bundleObj.Bundle.SignedUri)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute signed URL request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "sbctl-bundle-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	defer tmpFile.Close()

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to copy bundle to tmp file")
	}

	_ = tmpFile.Close()

	bundleDir, err := os.MkdirTemp("", "sbctl-")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}

	err = sbctl.ExtractBundle(tmpFile.Name(), bundleDir)
	if err != nil {
		return "", errors.Wrap(err, "failed to extract bundle")
	}

	return bundleDir, nil
}
