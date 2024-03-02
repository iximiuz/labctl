package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v2"
)

const (
	defaultAPIBaseURL = "https://labs-dev.iximiuz.com/api"
)

func configFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".iximiuz", "labctl", "config.yaml"), nil
}

type Config struct {
	mu sync.RWMutex

	APIBaseURL string `yaml:"api_base_url"`

	SessionID string `yaml:"session_id"`

	AccessToken string `yaml:"access_token"`
}

func Load() (*Config, error) {
	filename, err := configFilePath()
	if err != nil {
		return nil, fmt.Errorf("unable to get config file path: %s", err)
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open config file: %s", err)
	}
	defer file.Close()

	cfg := Default()
	if err := yaml.NewDecoder(file).Decode(cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config from YAML: %s", err)
	}

	return cfg, nil
}

func Default() *Config {
	return &Config{
		APIBaseURL: defaultAPIBaseURL,
	}
}

func (cfg *Config) Dump() error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	filename, err := configFilePath()
	if err != nil {
		return fmt.Errorf("unable to get config file path: %s", err)
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		return fmt.Errorf("unable to create config directory: %s", err)
	}

	file, err := os.OpenFile(filename+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("unable to open config file: %s", err)
	}
	defer file.Close()

	if err := yaml.NewEncoder(file).Encode(cfg); err != nil {
		return fmt.Errorf("unable to encode config to YAML: %s", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("unable to close config file: %s", err)
	}

	if err := os.Rename(filename+".tmp", filename); err != nil {
		return fmt.Errorf("unable to rename config file: %s", err)
	}

	return nil
}
