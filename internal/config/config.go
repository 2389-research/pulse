// ABOUTME: Configuration management for pulse with YAML config loading.
// ABOUTME: Handles social API settings, journal paths, and ~ expansion.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config stores pulse configuration loaded from ~/.config/pulse/config.yaml.
type Config struct {
	Social  SocialConfig  `yaml:"social"`
	Journal JournalConfig `yaml:"journal"`
}

// SocialConfig holds remote social media API settings.
type SocialConfig struct {
	APIKey string `yaml:"api_key"`
	TeamID string `yaml:"team_id"`
	APIURL string `yaml:"api_url"`
}

// JournalConfig holds optional path overrides for journal storage.
type JournalConfig struct {
	ProjectPath string `yaml:"project_path"`
	UserPath    string `yaml:"user_path"`
}

// HasRemote returns true if remote social posting is configured.
func (c *Config) HasRemote() bool {
	return c.Social.APIKey != "" && c.Social.TeamID != "" && c.Social.APIURL != ""
}

// GetJournalProjectPath returns the project-local journal path, defaulting to .private-journal/ in cwd.
func (c *Config) GetJournalProjectPath() (string, error) {
	if c.Journal.ProjectPath != "" {
		return ExpandPath(c.Journal.ProjectPath)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return filepath.Join(cwd, ".private-journal"), nil
}

// GetJournalUserPath returns the user-global journal path, defaulting to ~/.private-journal/.
func (c *Config) GetJournalUserPath() (string, error) {
	if c.Journal.UserPath != "" {
		return ExpandPath(c.Journal.UserPath)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".private-journal"), nil
}

// GetSocialDataDir returns the social media data directory.
func (c *Config) GetSocialDataDir() (string, error) {
	return SocialDataDir()
}

// SocialDataDir returns the default social data directory.
func SocialDataDir() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "pulse", "social"), nil
}

// GetConfigPath returns the config file path.
func GetConfigPath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "pulse", "config.yaml"), nil
}

// ExpandPath expands a leading ~ to the user's home directory.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// Load reads config from disk. Returns default config if file doesn't exist.
func Load() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Save writes config to disk.
func (c *Config) Save() error {
	path, err := GetConfigPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
