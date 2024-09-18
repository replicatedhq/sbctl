package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download bundle from url",
		Long:  "Download bundle from url",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			// This only works with generated config, so let's make sure we don't mess up user's real files.
			bundleLocation := v.GetString("support-bundle-location")
			if len(args) > 0 && args[0] != "" {
				bundleLocation = args[0]
			}
			if bundleLocation == "" {
				return errors.New("support-bundle-location is required")
			}

			token := v.GetString("token")
			if token == "" {
				return errors.New("token is required when downloading bundle")
			}

			fmt.Println("Downloading bundle...")

			file, err := downloadBundleToDisk(bundleLocation, token)
			if err != nil {
				return err
			}
			fmt.Println(file)

			return nil
		},
	}

	return cmd
}

func downloadBundleToDisk(bundleUrl string, token string) (string, error) {
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

	sbFile, err := os.Create("support-bundle.tgz")
	if err != nil {
		return "", errors.Wrap(err, "failed to create file")
	}
	defer sbFile.Close()

	_, err = io.Copy(sbFile, resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to copy bundle to file")
	}

	return sbFile.Name(), nil
}
