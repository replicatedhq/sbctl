package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
		SilenceUsage:  true,
		SilenceErrors: false,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
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

					err = sbctl.ExtractBundle(os.Args[1], bundleDir)
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

			fmt.Printf("Server is running\n\n")
			fmt.Printf("export KUBECONFIG=%s\n\n", kubeConfig)

			<-make(chan struct{}, 0)

			return nil
		},
	}

	cmd.Flags().String("support-bundle-location", "", "path to support bundle archive, directory, or URL")
	cmd.Flags().String("token", "", "API token for authentication when fetching on-line bundles")
	return cmd
}

func downloadAndExtractBundle(bundleUrl string, token string) (string, error) {
	parsedUrl, err := url.Parse(bundleUrl)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse url")
	}

	_, slug := path.Split(parsedUrl.Path)

	gqlUrl := "https://g.replicated.com/graphql"
	gqlReqest := "{\"operationName\":\"supportBundleForSlug\",\"variables\":{\"slug\":\"%s\"},\"query\":\"query supportBundleForSlug($slug: String!) {\\n  supportBundleForSlug(slug: $slug) {\\n    bundle {\\n      id\\n      appId\\n      size\\n      name\\n      teamId\\n      teamName\\n      teamShareIds\\n      status\\n      createdAt\\n      collectedAt\\n      slug\\n      viewed\\n      analyzeChannelId\\n      customer {\\n        id\\n        name\\n        avatar\\n      }\\n      uri\\n      signedUri\\n      notes\\n      treeIndex\\n    }\\n    insights {\\n      level\\n      primary\\n      key\\n      detail\\n      icon\\n      icon_key\\n      desiredPosition\\n      involvedObject {\\n        kind\\n        namespace\\n        name\\n        apiVersion\\n      }\\n      labels {\\n        key\\n        value\\n      }\\n    }\\n  }\\n}\\n\"}"
	gqlReqest = fmt.Sprintf(gqlReqest, slug)

	req, err := http.NewRequest("POST", gqlUrl, strings.NewReader(gqlReqest))
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read GQL response")
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	bundleObj := struct {
		Data struct {
			SupportBundleForSlug struct {
				Bundle struct {
					SignedUri string `json:"signedUri"`
				} `json:"bundle"`
			} `json:"supportBundleForSlug"`
		} `json:"data"`
	}{}
	err = json.Unmarshal(body, &bundleObj)
	if err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal response: %s", body)
	}

	resp, err = http.Get(bundleObj.Data.SupportBundleForSlug.Bundle.SignedUri)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute signed URL request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	tmpFile, err := ioutil.TempFile("", "sbctl-bundle-")
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
