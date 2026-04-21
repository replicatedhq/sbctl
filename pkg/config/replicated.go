package config

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type ReplicatedConfig struct {
	Profiles       map[string]Profile `yaml:"profiles"`
	DefaultProfile string             `yaml:"defaultProfile"`
}

type Profile struct {
	APIToken string `yaml:"apiToken"`
}

func ReplicatedConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".replicated", "config.yaml")
}

// GetTokenFromReplicatedConfig reads the API token from the Replicated CLI
// config file at ~/.replicated/config.yaml. If profileName is empty, it uses
// the defaultProfile from the config.
func GetTokenFromReplicatedConfig(profileName string) (string, error) {
	configPath := ReplicatedConfigPath()
	if configPath == "" {
		return "", errors.New("could not determine home directory")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read replicated config")
	}

	var cfg ReplicatedConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", errors.Wrap(err, "failed to parse replicated config")
	}

	if profileName == "" {
		profileName = cfg.DefaultProfile
	}
	if profileName == "" {
		return "", errors.New("no profile specified and no defaultProfile set in replicated config")
	}

	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return "", errors.Errorf("profile %q not found in replicated config", profileName)
	}

	if profile.APIToken == "" {
		return "", errors.Errorf("profile %q has no apiToken", profileName)
	}

	return profile.APIToken, nil
}
