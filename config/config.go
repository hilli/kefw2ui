package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// SpeakerConfig matches the kefw2 CLI speaker configuration format.
type SpeakerConfig struct {
	IPAddress       string `yaml:"ip_address"`
	Name            string `yaml:"name"`
	Model           string `yaml:"model"`
	FirmwareVersion string `yaml:"firmware_version"`
	MacAddress      string `yaml:"mac_address"`
	ID              string `yaml:"id"`
	MaxVolume       int    `yaml:"max_volume"`
}

// UPnPConfig holds UPnP/DLNA media server configuration.
// This matches the CLI config format for compatibility.
type UPnPConfig struct {
	// DefaultServer is the display name of the default media server
	DefaultServer string `yaml:"default_server,omitempty"`

	// DefaultServerPath is the API path to the default server
	DefaultServerPath string `yaml:"default_server_path,omitempty"`

	// BrowseContainer is the container path to start browsing from
	// When set, users won't see parent containers or other servers
	BrowseContainer string `yaml:"browse_container,omitempty"`

	// IndexContainer is the container path for search indexing scope
	// Tip: Use "By Folder" structure for best results
	IndexContainer string `yaml:"index_container,omitempty"`
}

// Config holds the application configuration (compatible with kefw2 CLI).
type Config struct {
	mu             sync.RWMutex    `yaml:"-"`
	DefaultSpeaker string          `yaml:"defaultspeaker,omitempty"`
	Speakers       []SpeakerConfig `yaml:"speakers,omitempty"`
	UPnP           UPnPConfig      `yaml:"upnp,omitempty"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Speakers: []SpeakerConfig{},
	}
}

// Dir returns the kefw2 config directory path.
func Dir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "kefw2"), nil
}

// Path returns the full path to kefw2.yaml (shared with CLI).
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "kefw2.yaml"), nil
}

// PlaylistsDir returns the path to the playlists directory.
func PlaylistsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "playlists"), nil
}

// Load reads the config file from disk.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is from our own config directory
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

// Save writes the config to disk.
func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	path, err := Path()
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

// GetDefaultSpeaker returns the default speaker IP.
func (c *Config) GetDefaultSpeaker() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.DefaultSpeaker
}

// SetDefaultSpeaker sets the default speaker IP and saves config.
func (c *Config) SetDefaultSpeaker(ip string) error {
	c.mu.Lock()
	c.DefaultSpeaker = ip
	c.mu.Unlock()
	return c.Save()
}

// GetSpeakers returns all configured speakers.
func (c *Config) GetSpeakers() []SpeakerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]SpeakerConfig, len(c.Speakers))
	copy(result, c.Speakers)
	return result
}

// FindSpeaker finds a speaker by IP address.
func (c *Config) FindSpeaker(ip string) *SpeakerConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.Speakers {
		if c.Speakers[i].IPAddress == ip {
			spk := c.Speakers[i]
			return &spk
		}
	}
	return nil
}

// AddOrUpdateSpeaker adds a new speaker or updates existing one and saves config.
func (c *Config) AddOrUpdateSpeaker(spk SpeakerConfig) error {
	c.mu.Lock()

	// Check if speaker already exists
	found := false
	for i := range c.Speakers {
		if c.Speakers[i].IPAddress == spk.IPAddress {
			c.Speakers[i] = spk
			found = true
			break
		}
	}

	if !found {
		c.Speakers = append(c.Speakers, spk)
	}

	c.mu.Unlock()
	return c.Save()
}

// RemoveSpeaker removes a speaker by IP and saves config.
func (c *Config) RemoveSpeaker(ip string) error {
	c.mu.Lock()

	for i := range c.Speakers {
		if c.Speakers[i].IPAddress == ip {
			c.Speakers = append(c.Speakers[:i], c.Speakers[i+1:]...)
			break
		}
	}

	// Clear default if removed speaker was the default
	if c.DefaultSpeaker == ip {
		c.DefaultSpeaker = ""
	}

	c.mu.Unlock()
	return c.Save()
}

// GetUPnPConfig returns the UPnP configuration.
func (c *Config) GetUPnPConfig() UPnPConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.UPnP
}

// SetUPnPConfig updates the entire UPnP configuration and saves.
func (c *Config) SetUPnPConfig(upnp UPnPConfig) error {
	c.mu.Lock()
	c.UPnP = upnp
	c.mu.Unlock()
	return c.Save()
}

// SetDefaultServer sets the default UPnP server and saves.
func (c *Config) SetDefaultServer(name, path string) error {
	c.mu.Lock()
	c.UPnP.DefaultServer = name
	c.UPnP.DefaultServerPath = path
	c.mu.Unlock()
	return c.Save()
}

// SetBrowseContainer sets the browse container path and saves.
func (c *Config) SetBrowseContainer(containerPath string) error {
	c.mu.Lock()
	c.UPnP.BrowseContainer = containerPath
	c.mu.Unlock()
	return c.Save()
}

// SetIndexContainer sets the index container path and saves.
func (c *Config) SetIndexContainer(containerPath string) error {
	c.mu.Lock()
	c.UPnP.IndexContainer = containerPath
	c.mu.Unlock()
	return c.Save()
}

// HasDefaultServer returns true if a default server is configured.
func (c *Config) HasDefaultServer() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.UPnP.DefaultServerPath != ""
}
