package cli

import (
	"fmt"
	"os"

	"github.com/replicatedhq/sbctl/pkg/config"
	"github.com/spf13/viper"
)

// resolveToken returns the API token by checking, in order:
// 1. --token flag / SBCTL_TOKEN env var
// 2. ~/.replicated/config.yaml (using --profile flag or defaultProfile)
func resolveToken(v *viper.Viper) (string, error) {
	if token := v.GetString("token"); token != "" {
		return token, nil
	}

	replToken := os.Getenv("REPLICATED_API_TOKEN")
	if replToken != "" {
		return replToken, nil
	}

	profileName := v.GetString("profile")
	token, err := config.GetTokenFromReplicatedConfig(profileName)
	if err != nil {
		if profileName != "" {
			return "", fmt.Errorf("token not provided and failed to read profile %q from replicated config: %w", profileName, err)
		}
		return "", fmt.Errorf("token is required when downloading bundle (set --token, SBCTL_TOKEN, or configure ~/.replicated/config.yaml)")
	}

	return token, nil
}
