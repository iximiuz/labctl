package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	defaultBaseURL = "https://labs.iximiuz.com"

	defaultAPIBaseURL = defaultBaseURL + "/api"

	defaultSSHIdentityFile = "iximiuz_labs_user"
)

type Config struct {
	mu sync.RWMutex

	FilePath string `yaml:"-"`

	BaseURL string `yaml:"base_url"`

	APIBaseURL string `yaml:"api_base_url"`

	SessionID string `yaml:"session_id"`

	AccessToken string `yaml:"access_token"`

	PlaysDir string `yaml:"plays_dir"`

	// Deprecated: use SSHIdentityFile instead
	SSHDir string `yaml:"ssh_dir,omitempty"`

	SSHIdentityFile string `yaml:"ssh_identity_file"`
}

func (c *Config) WebSocketOrigin() string {
	return "https://cli." + strings.TrimPrefix(c.BaseURL, "https://")
}

func ConfigFilePath(homeDir string) string {
	return filepath.Join(homeDir, ".iximiuz", "labctl", "config.yaml")
}

func Default(homeDir string) *Config {
	configFilePath := ConfigFilePath(homeDir)

	cfg := &Config{
		FilePath:        configFilePath,
		BaseURL:         defaultBaseURL,
		APIBaseURL:      defaultAPIBaseURL,
		PlaysDir:        filepath.Join(filepath.Dir(configFilePath), "plays"),
		SSHIdentityFile: filepath.Join(homeDir, ".ssh", defaultSSHIdentityFile),
	}

	// Override with environment variables if present
	if sessionID := os.Getenv("IXIMIUZ_SESSION_ID"); sessionID != "" {
		cfg.SessionID = sessionID
	}
	if accessToken := os.Getenv("IXIMIUZ_ACCESS_TOKEN"); accessToken != "" {
		cfg.AccessToken = accessToken
	}

	return cfg
}

func Load(homeDir string) (*Config, error) {
	path := ConfigFilePath(homeDir)

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Default(homeDir), nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to open config file: %s", err)
	}
	defer file.Close()

	var cfg Config
	if err := yaml.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config from YAML: %s", err)
	}

	if cfg.SSHDir != "" && cfg.SSHIdentityFile == "" {
		cfg.SSHIdentityFile = filepath.Join(cfg.SSHDir, defaultSSHIdentityFile)
	}

	if cfg.SSHIdentityFile == "" {
		cfg.SSHIdentityFile = filepath.Join(homeDir, ".ssh", defaultSSHIdentityFile)
	}

	// Migrations
	if cfg.BaseURL == "" {
		cfg.BaseURL = strings.TrimSuffix(cfg.APIBaseURL, "/api")
	}

	// Override with environment variables if present
	if sessionID := os.Getenv("IXIMIUZ_SESSION_ID"); sessionID != "" {
		cfg.SessionID = sessionID
	}
	if accessToken := os.Getenv("IXIMIUZ_ACCESS_TOKEN"); accessToken != "" {
		cfg.AccessToken = accessToken
	}

	cfg.FilePath = path

	return &cfg, nil
}

func (c *Config) Dump() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(c.FilePath), 0o700); err != nil {
		return fmt.Errorf("unable to create config directory: %s", err)
	}

	file, err := os.OpenFile(c.FilePath+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("unable to open config file: %s", err)
	}
	defer file.Close()

	if err := yaml.NewEncoder(file).Encode(c); err != nil {
		return fmt.Errorf("unable to encode config to YAML: %s", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("unable to close config file: %s", err)
	}

	if err := os.Rename(c.FilePath+".tmp", c.FilePath); err != nil {
		return fmt.Errorf("unable to rename config file: %s", err)
	}

	return nil
}
