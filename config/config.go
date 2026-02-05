package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Speakers SpeakersConfig `yaml:"speakers"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

// SpeakersConfig holds speaker-related settings
type SpeakersConfig struct {
	Default string   `yaml:"default"`
	Known   []string `yaml:"known"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Bind: "0.0.0.0",
		},
		Speakers: SpeakersConfig{},
	}
}

// ConfigDir returns the kefw2 config directory path
func ConfigDir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "kefw2"), nil
}

// ConfigPath returns the full path to kefw2ui.yaml
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kefw2ui.yaml"), nil
}

// PlaylistsDir returns the path to the playlists directory
func PlaylistsDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "playlists"), nil
}

// Load reads the config file from disk
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// Save writes the config to disk
func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
