package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ImageCache is a two-tier (memory + disk) cache for proxied images.
// Memory tier provides fast serving; disk tier persists across restarts.
type ImageCache struct {
	mu      sync.RWMutex
	entries map[string]*imageCacheEntry
	memSize int64 // current total bytes in memory
	maxMem  int64 // max memory bytes (e.g. 50MB)
	memTTL  time.Duration
	diskTTL time.Duration
	diskDir string
}

type imageCacheEntry struct {
	Data        []byte
	ContentType string
	FetchedAt   time.Time
}

// imageDiskMeta is the JSON sidecar stored alongside each cached image on disk.
type imageDiskMeta struct {
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// ImageCacheConfig configures the image cache.
type ImageCacheConfig struct {
	MaxMemBytes int64         // max memory usage (default 50MB)
	MemTTL      time.Duration // memory entry TTL (default 1h, 0 = never expire)
	DiskTTL     time.Duration // disk entry TTL (default 7d, 0 = never expire; -1 = use default)
	DiskDir     string        // disk cache directory (default auto)
}

// NewImageCache creates a new two-tier image cache.
func NewImageCache(cfg ImageCacheConfig) *ImageCache {
	if cfg.MaxMemBytes <= 0 {
		cfg.MaxMemBytes = 50 << 20 // 50MB
	}
	if cfg.MemTTL <= 0 {
		cfg.MemTTL = 1 * time.Hour
	}
	if cfg.DiskTTL < 0 {
		cfg.DiskTTL = 7 * 24 * time.Hour // 7 days
	}
	// DiskTTL == 0 means never expire (kept as-is)
	if cfg.DiskDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			cacheDir = os.TempDir()
		}
		cfg.DiskDir = filepath.Join(cacheDir, "kefw2", "images")
	}

	if err := os.MkdirAll(cfg.DiskDir, 0750); err != nil {
		log.Printf("Warning: failed to create image cache dir %s: %v", cfg.DiskDir, err)
	}

	return &ImageCache{
		entries: make(map[string]*imageCacheEntry),
		maxMem:  cfg.MaxMemBytes,
		memTTL:  cfg.MemTTL,
		diskTTL: cfg.DiskTTL,
		diskDir: cfg.DiskDir,
	}
}

// cacheKey returns a filesystem-safe key for a URL.
func cacheKey(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("%x", h)
}

// Get returns a cached image if available (memory first, then disk).
// Returns nil if not found or expired.
func (c *ImageCache) Get(rawURL string) *imageCacheEntry {
	key := cacheKey(rawURL)

	// Tier 1: memory
	if entry := c.getFromMemory(key); entry != nil {
		return entry
	}

	// Tier 2: disk
	entry := c.getFromDisk(key)
	if entry != nil {
		// Promote to memory
		c.putToMemory(key, entry)
	}
	return entry
}

// Put stores an image in both memory and disk tiers.
func (c *ImageCache) Put(rawURL string, data []byte, contentType string) {
	entry := &imageCacheEntry{
		Data:        data,
		ContentType: contentType,
		FetchedAt:   time.Now(),
	}
	key := cacheKey(rawURL)
	c.putToMemory(key, entry)
	c.putToDisk(key, rawURL, entry)
}

// getFromMemory returns an entry from the memory tier, or nil if miss/expired.
func (c *ImageCache) getFromMemory(key string) *imageCacheEntry {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil
	}
	if c.memTTL > 0 && time.Since(entry.FetchedAt) > c.memTTL {
		c.mu.Lock()
		// Re-check under write lock
		if e, ok := c.entries[key]; ok && time.Since(e.FetchedAt) > c.memTTL {
			c.memSize -= int64(len(e.Data))
			delete(c.entries, key)
		}
		c.mu.Unlock()
		return nil
	}
	return entry
}

// putToMemory stores an entry in the memory tier, evicting oldest if over limit.
func (c *ImageCache) putToMemory(key string, entry *imageCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If replacing an existing entry, subtract old size
	if old, ok := c.entries[key]; ok {
		c.memSize -= int64(len(old.Data))
	}

	c.entries[key] = entry
	c.memSize += int64(len(entry.Data))

	// Evict oldest entries if over memory limit
	for c.memSize > c.maxMem {
		c.evictOldest()
	}
}

// evictOldest removes the oldest memory entry. Must be called with mu held.
func (c *ImageCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for k, e := range c.entries {
		if first || e.FetchedAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.FetchedAt
			first = false
		}
	}
	if oldestKey != "" {
		c.memSize -= int64(len(c.entries[oldestKey].Data))
		delete(c.entries, oldestKey)
	}
}

// getFromDisk returns an entry from the disk tier, or nil if miss/expired.
func (c *ImageCache) getFromDisk(key string) *imageCacheEntry {
	dataPath := filepath.Join(c.diskDir, key+".dat")
	metaPath := filepath.Join(c.diskDir, key+".meta")

	metaBytes, err := os.ReadFile(metaPath) //nolint:gosec // path from cache key hash
	if err != nil {
		return nil
	}
	var meta imageDiskMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil
	}
	if c.diskTTL > 0 && time.Since(meta.FetchedAt) > c.diskTTL {
		// Expired — clean up lazily
		_ = os.Remove(dataPath)
		_ = os.Remove(metaPath)
		return nil
	}

	data, err := os.ReadFile(dataPath) //nolint:gosec // path from cache key hash
	if err != nil {
		return nil
	}

	return &imageCacheEntry{
		Data:        data,
		ContentType: meta.ContentType,
		FetchedAt:   meta.FetchedAt,
	}
}

// putToDisk stores an entry to the disk tier.
func (c *ImageCache) putToDisk(key, rawURL string, entry *imageCacheEntry) {
	dataPath := filepath.Join(c.diskDir, key+".dat")
	metaPath := filepath.Join(c.diskDir, key+".meta")

	meta := imageDiskMeta{
		ContentType: entry.ContentType,
		URL:         rawURL,
		FetchedAt:   entry.FetchedAt,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return
	}

	if err := os.WriteFile(dataPath, entry.Data, 0600); err != nil {
		log.Printf("Warning: failed to write image cache %s: %v", dataPath, err)
		return
	}
	if err := os.WriteFile(metaPath, metaBytes, 0600); err != nil {
		log.Printf("Warning: failed to write image cache meta %s: %v", metaPath, err)
		_ = os.Remove(dataPath) // clean up orphan
	}
}
