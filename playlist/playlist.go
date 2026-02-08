// Package playlist manages saved playlists for kefw2ui.
package playlist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hilli/kefw2ui/config"
)

// Track represents a single track in a playlist.
type Track struct {
	Title     string `json:"title"`
	Artist    string `json:"artist,omitempty"`
	Album     string `json:"album,omitempty"`
	Duration  int    `json:"duration,omitempty"` // milliseconds
	Icon      string `json:"icon,omitempty"`
	Path      string `json:"path,omitempty"` // Airable path for playback
	ID        string `json:"id,omitempty"`
	Type      string `json:"type,omitempty"`
	URI       string `json:"uri,omitempty"`       // Direct playback URI (e.g., http://server/file.flac)
	MimeType  string `json:"mimeType,omitempty"`  // Content type (e.g., audio/flac)
	ServiceID string `json:"serviceId,omitempty"` // Service identifier (e.g., "UPnP", "airableRadios")
}

// Playlist represents a saved playlist.
type Playlist struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Tracks      []Track   `json:"tracks"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Manager handles playlist storage and retrieval.
type Manager struct {
	dir string
}

// NewManager creates a new playlist manager.
func NewManager() (*Manager, error) {
	dir, err := config.PlaylistsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get playlists directory: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create playlists directory: %w", err)
	}

	return &Manager{dir: dir}, nil
}

// List returns all saved playlists (metadata only, without tracks).
func (m *Manager) List() ([]Playlist, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Playlist{}, nil
		}
		return nil, fmt.Errorf("failed to read playlists directory: %w", err)
	}

	var playlists []Playlist
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		playlist, err := m.Get(id)
		if err != nil {
			continue // Skip invalid playlists
		}

		// Return metadata only (no tracks) for listing
		playlists = append(playlists, Playlist{
			ID:          playlist.ID,
			Name:        playlist.Name,
			Description: playlist.Description,
			Tracks:      nil, // Don't include tracks in list
			CreatedAt:   playlist.CreatedAt,
			UpdatedAt:   playlist.UpdatedAt,
		})
	}

	// Sort by updated time, newest first
	sort.Slice(playlists, func(i, j int) bool {
		return playlists[i].UpdatedAt.After(playlists[j].UpdatedAt)
	})

	return playlists, nil
}

// Get retrieves a playlist by ID (including all tracks).
func (m *Manager) Get(id string) (*Playlist, error) {
	path := filepath.Join(m.dir, id+".json")

	data, err := os.ReadFile(path) //nolint:gosec // path is constructed from our own playlist directory
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("playlist not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read playlist: %w", err)
	}

	var playlist Playlist
	if err := json.Unmarshal(data, &playlist); err != nil {
		return nil, fmt.Errorf("failed to parse playlist: %w", err)
	}

	return &playlist, nil
}

// Create creates a new playlist.
func (m *Manager) Create(name string, description string, tracks []Track) (*Playlist, error) {
	id := generateID(name)

	// Check if ID already exists, append timestamp if so
	if _, err := m.Get(id); err == nil {
		id = fmt.Sprintf("%s-%d", id, time.Now().Unix())
	}

	now := time.Now()
	playlist := &Playlist{
		ID:          id,
		Name:        name,
		Description: description,
		Tracks:      tracks,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.save(playlist); err != nil {
		return nil, err
	}

	return playlist, nil
}

// Update updates an existing playlist.
func (m *Manager) Update(id string, name string, description string, tracks []Track) (*Playlist, error) {
	playlist, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	if name != "" {
		playlist.Name = name
	}
	playlist.Description = description
	if tracks != nil {
		playlist.Tracks = tracks
	}
	playlist.UpdatedAt = time.Now()

	if err := m.save(playlist); err != nil {
		return nil, err
	}

	return playlist, nil
}

// Delete removes a playlist.
func (m *Manager) Delete(id string) error {
	path := filepath.Join(m.dir, id+".json")

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("playlist not found: %s", id)
		}
		return fmt.Errorf("failed to delete playlist: %w", err)
	}

	return nil
}

// AddTracks adds tracks to an existing playlist.
func (m *Manager) AddTracks(id string, tracks []Track) (*Playlist, error) {
	playlist, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	playlist.Tracks = append(playlist.Tracks, tracks...)
	playlist.UpdatedAt = time.Now()

	if err := m.save(playlist); err != nil {
		return nil, err
	}

	return playlist, nil
}

// RemoveTracks removes tracks at specified indices from a playlist.
func (m *Manager) RemoveTracks(id string, indices []int) (*Playlist, error) {
	playlist, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	// Create a map of indices to remove
	toRemove := make(map[int]bool)
	for _, idx := range indices {
		toRemove[idx] = true
	}

	// Filter out removed tracks
	var newTracks []Track
	for i, track := range playlist.Tracks {
		if !toRemove[i] {
			newTracks = append(newTracks, track)
		}
	}

	playlist.Tracks = newTracks
	playlist.UpdatedAt = time.Now()

	if err := m.save(playlist); err != nil {
		return nil, err
	}

	return playlist, nil
}

// save writes a playlist to disk.
func (m *Manager) save(playlist *Playlist) error {
	path := filepath.Join(m.dir, playlist.ID+".json")

	data, err := json.MarshalIndent(playlist, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal playlist: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write playlist: %w", err)
	}

	return nil
}

// generateID creates a URL-safe ID from a playlist name.
func generateID(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")

	// Remove non-alphanumeric characters (except hyphens)
	var result strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}

	id = result.String()

	// Remove consecutive hyphens and trim
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	id = strings.Trim(id, "-")

	if id == "" {
		id = "playlist"
	}

	return id
}

// TrackCount returns the number of tracks in a playlist without loading them all.
func (m *Manager) TrackCount(id string) (int, error) {
	playlist, err := m.Get(id)
	if err != nil {
		return 0, err
	}
	return len(playlist.Tracks), nil
}
