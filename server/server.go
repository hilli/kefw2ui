package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hilli/go-kef-w2/kefw2"

	"github.com/hilli/kefw2ui/config"
	mcppkg "github.com/hilli/kefw2ui/mcp"
	"github.com/hilli/kefw2ui/playlist"
	"github.com/hilli/kefw2ui/speaker"
)

// Options configures the server.
type Options struct {
	Bind           string
	Port           int
	FrontendFS     embed.FS
	Config         *config.Config
	SpeakerManager *speaker.Manager
}

// Server is the HTTP server for kefw2ui.
type Server struct {
	opts       Options
	mux        *http.ServeMux
	httpServer *http.Server
	manager    *speaker.Manager
	playlists  *playlist.Manager

	// Shared cache for Airable content (UPnP, Radio, Podcasts)
	airableCache *kefw2.RowsCache

	// SSE clients
	sseClients   map[chan []byte]struct{}
	sseClientsMu sync.RWMutex
}

// Content type constants used across browse/queue handlers.
const (
	contentTypeContainer = "container"
	contentTypeAudio     = "audio"
	browseSourceUPnP     = "upnp"
	browseSourceRadio    = "radio"
	browseSourcePodcasts = "podcasts"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for SSE support.
func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// loggingMiddleware logs all HTTP requests with method, path, status, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(lrw, r)

		// Calculate duration
		duration := time.Since(start)

		// Skip logging for static assets and SSE (too noisy)
		path := r.URL.Path
		if strings.HasPrefix(path, "/_app/") ||
			strings.HasSuffix(path, ".js") ||
			strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".ico") ||
			path == "/api/events" {
			return
		}

		// Log the request
		log.Printf("%s %s %d %v", r.Method, path, lrw.statusCode, duration.Round(time.Millisecond))
	})
}

// New creates a new server instance.
func New(opts Options) *Server {
	// Initialize playlist manager
	playlistMgr, err := playlist.NewManager()
	if err != nil {
		log.Printf("Warning: failed to initialize playlist manager: %v", err)
	}

	// Initialize shared Airable cache (disk-persisted for performance)
	airableCache := kefw2.NewRowsCache(kefw2.DefaultDiskCacheConfig())

	s := &Server{
		opts:         opts,
		mux:          http.NewServeMux(),
		sseClients:   make(map[chan []byte]struct{}),
		manager:      opts.SpeakerManager,
		playlists:    playlistMgr,
		airableCache: airableCache,
	}

	s.registerRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", opts.Bind, opts.Port),
		Handler:      loggingMiddleware(s.mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// getAirableClient returns an AirableClient with the shared cache.
// This improves performance by caching API responses across requests.
func (s *Server) getAirableClient(spk *kefw2.KEFSpeaker) *kefw2.AirableClient {
	return kefw2.NewAirableClient(spk, kefw2.WithCache(kefw2.CacheConfig{
		Enabled: true,
		TTL:     5 * time.Minute,
	}))
}

// getCachedAirableClient returns an AirableClient with the server's shared disk cache.
// Use this for browse operations that benefit from persistent caching.
func (s *Server) getCachedAirableClient(spk *kefw2.KEFSpeaker) *kefw2.AirableClient {
	client := kefw2.NewAirableClient(spk)
	client.Cache = s.airableCache
	return client
}

// Handler returns the server's HTTP handler (with logging middleware).
// This is useful for serving the same handler on additional listeners (e.g. Tailscale).
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server without interrupting active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("/api/health", s.handleHealth)

	// Speaker management
	s.mux.HandleFunc("/api/speakers", s.handleSpeakers)
	s.mux.HandleFunc("/api/speakers/discover", s.handleSpeakersDiscover)
	s.mux.HandleFunc("/api/speakers/add", s.handleSpeakersAdd)
	s.mux.HandleFunc("/api/speakers/default", s.handleSpeakersDefault)
	s.mux.HandleFunc("/api/speaker", s.handleSpeaker)
	s.mux.HandleFunc("/api/speaker/logo", s.handleSpeakerLogo)
	s.mux.HandleFunc("/api/proxy/image", s.handleProxyImage)

	// Player controls
	s.mux.HandleFunc("/api/player", s.handlePlayer)
	s.mux.HandleFunc("/api/player/play", s.handlePlayerPlay)
	s.mux.HandleFunc("/api/player/stop", s.handlePlayerStop)
	s.mux.HandleFunc("/api/player/next", s.handlePlayerNext)
	s.mux.HandleFunc("/api/player/prev", s.handlePlayerPrev)
	s.mux.HandleFunc("/api/player/volume", s.handlePlayerVolume)
	s.mux.HandleFunc("/api/player/mute", s.handlePlayerMute)
	s.mux.HandleFunc("/api/player/source", s.handlePlayerSource)
	s.mux.HandleFunc("/api/player/seek", s.handlePlayerSeek)
	s.mux.HandleFunc("/api/player/power", s.handlePlayerPower)

	// Queue management
	s.mux.HandleFunc("/api/queue", s.handleQueue)
	s.mux.HandleFunc("/api/queue/play", s.handleQueuePlay)
	s.mux.HandleFunc("/api/queue/remove", s.handleQueueRemove)
	s.mux.HandleFunc("/api/queue/move", s.handleQueueMove)
	s.mux.HandleFunc("/api/queue/clear", s.handleQueueClear)
	s.mux.HandleFunc("/api/queue/mode", s.handleQueueMode)

	// Playlist management
	s.mux.HandleFunc("/api/playlists", s.handlePlaylists)
	s.mux.HandleFunc("/api/playlists/", s.handlePlaylist) // GET/PUT/DELETE single playlist
	s.mux.HandleFunc("/api/playlists/save-queue", s.handleSaveQueueAsPlaylist)
	s.mux.HandleFunc("/api/playlists/load/", s.handleLoadPlaylist) // Load playlist to queue

	// Content browsing
	s.mux.HandleFunc("/api/browse/", s.handleBrowse)

	// Settings
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/settings/speaker", s.handleSpeakerSettings)
	s.mux.HandleFunc("/api/settings/eq", s.handleEQSettings)
	s.mux.HandleFunc("/api/settings/upnp", s.handleUPnPSettings)
	s.mux.HandleFunc("/api/upnp/servers", s.handleUPnPServers)
	s.mux.HandleFunc("/api/upnp/containers", s.handleUPnPContainers)

	// SSE endpoint
	s.mux.HandleFunc("/events", s.handleSSE)

	// MCP server
	mcpHandler := mcppkg.NewMCPHandler(s.manager, s.playlists, s.airableCache, s.BroadcastPlaylistsChanged)
	s.mux.Handle("/api/mcp", mcpHandler)

	// Static frontend files
	s.mux.HandleFunc("/", s.handleFrontend)
}

// HandleSpeakerHealth is called by the speaker manager when speaker connectivity changes.
// It broadcasts a speakerHealth SSE event to all connected clients.
func (s *Server) HandleSpeakerHealth(connected bool) {
	payload, err := json.Marshal(map[string]any{
		"type": "speakerHealth",
		"data": map[string]any{
			"connected": connected,
		},
	})
	if err != nil {
		log.Printf("Error marshaling speakerHealth event: %v", err)
		return
	}

	s.broadcastSSE(payload)
}

// BroadcastPlaylistsChanged sends a "playlists" SSE event to all connected
// clients so they can refresh their playlist lists. Called after any playlist
// CRUD operation (create, update, delete) from both REST and MCP handlers.
func (s *Server) BroadcastPlaylistsChanged() {
	payload, err := json.Marshal(map[string]any{
		"type": "playlists",
	})
	if err != nil {
		log.Printf("Error marshaling playlists event: %v", err)
		return
	}

	s.broadcastSSE(payload)
}

// HandleSpeakerEvent is called by the speaker manager when events occur.
func (s *Server) HandleSpeakerEvent(event kefw2.Event) {
	if event == nil {
		return
	}

	var eventData map[string]any

	switch e := event.(type) {
	case *kefw2.VolumeEvent:
		eventData = map[string]any{
			"type": "volume",
			"data": map[string]any{
				"volume": e.Volume,
			},
		}
	case *kefw2.MuteEvent:
		eventData = map[string]any{
			"type": "mute",
			"data": map[string]any{
				"muted": e.Muted,
			},
		}
	case *kefw2.SourceEvent:
		eventData = map[string]any{
			"type": "source",
			"data": map[string]any{
				"source": string(e.Source),
			},
		}
		// Track standby state so the event reconnection loop can pause
		if e.Source == kefw2.SourceStandby {
			s.manager.NotifyStandby()
		} else {
			s.manager.NotifyWake()
		}
	case *kefw2.PowerEvent:
		eventData = map[string]any{
			"type": "power",
			"data": map[string]any{
				"status": string(e.Status),
			},
		}
	case *kefw2.PlayerDataEvent:
		eventData = map[string]any{
			"type": "player",
			"data": map[string]any{
				"state":    e.State,
				"title":    e.Title,
				"artist":   e.Artist,
				"album":    e.Album,
				"duration": e.Duration,
				"icon":     s.proxyIconURL(e.Icon),
			},
		}
	case *kefw2.PlayTimeEvent:
		eventData = map[string]any{
			"type": "playTime",
			"data": map[string]any{
				"position": e.PositionMS,
			},
		}
	case *kefw2.PlayModeEvent:
		eventData = map[string]any{
			"type": "playMode",
			"data": map[string]any{
				"mode": e.Mode,
			},
		}
	case *kefw2.PlaylistEvent:
		eventData = map[string]any{
			"type": "queue",
			"data": map[string]any{
				"changes": e.Changes,
				"version": e.Version,
			},
		}
	default:
		// Ignore unknown events
		return
	}

	payload, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("Error marshaling event: %v", err)
		return
	}

	s.broadcastSSE(payload)
}

// broadcastSSE sends data to all connected SSE clients.
func (s *Server) broadcastSSE(data []byte) {
	s.sseClientsMu.RLock()
	defer s.sseClientsMu.RUnlock()

	for clientChan := range s.sseClients {
		select {
		case clientChan <- data:
		default:
			// Client buffer full, skip
		}
	}
}

// broadcastCurrentState sends the current speaker/player state to all SSE clients.
// Called when the active speaker changes so all clients get the new state.
func (s *Server) broadcastCurrentState() {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		return
	}

	ctx := context.Background()

	// Broadcast speaker info
	if speakerData, err := json.Marshal(map[string]any{
		"type": "speaker",
		"data": map[string]any{
			"ip":    spk.IPAddress,
			"name":  spk.Name,
			"model": spk.Model,
		},
	}); err == nil {
		s.broadcastSSE(speakerData)
	}

	// Broadcast current volume
	if volume, err := spk.GetVolume(ctx); err == nil {
		if volumeData, err := json.Marshal(map[string]any{
			"type": "volume",
			"data": map[string]any{"volume": volume},
		}); err == nil {
			s.broadcastSSE(volumeData)
		}
	}

	// Broadcast mute state
	if muted, err := spk.IsMuted(ctx); err == nil {
		if muteData, err := json.Marshal(map[string]any{
			"type": "mute",
			"data": map[string]any{"muted": muted},
		}); err == nil {
			s.broadcastSSE(muteData)
		}
	}

	// Broadcast source
	if source, err := spk.Source(ctx); err == nil {
		if sourceData, err := json.Marshal(map[string]any{
			"type": "source",
			"data": map[string]any{"source": string(source)},
		}); err == nil {
			s.broadcastSSE(sourceData)
		}
	}

	// Broadcast power state
	if status, err := spk.SpeakerState(ctx); err == nil {
		if powerData, err := json.Marshal(map[string]any{
			"type": "power",
			"data": map[string]any{"status": string(status)},
		}); err == nil {
			s.broadcastSSE(powerData)
		}
	}

	// Broadcast player data
	if playerData, err := spk.PlayerData(ctx); err == nil {
		position, _ := spk.SongProgressMS(ctx)
		if playerEventData, err := json.Marshal(map[string]any{
			"type": "player",
			"data": map[string]any{
				"state":     playerData.State,
				"title":     playerData.TrackRoles.Title,
				"artist":    playerData.TrackRoles.MediaData.MetaData.Artist,
				"album":     playerData.TrackRoles.MediaData.MetaData.Album,
				"icon":      s.proxyIconURL(playerData.TrackRoles.Icon),
				"duration":  playerData.Status.Duration,
				"position":  position,
				"audioType": playerData.MediaRoles.AudioType,
				"live":      playerData.MediaRoles.MediaData.MetaData.Live,
			},
		}); err == nil {
			s.broadcastSSE(playerEventData)
		}
	}
}

// handleHealth is a simple health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "ok",
		"speakerConnected": s.manager.IsSpeakerConnected(),
	})
}

// handleSpeakers returns the list of known speakers.
func (s *Server) handleSpeakers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	speakers := s.manager.GetSpeakers()
	activeSpeaker := s.manager.GetActiveSpeaker()

	// Get default speaker from config
	var defaultSpeakerIP string
	if s.opts.Config != nil {
		defaultSpeakerIP = s.opts.Config.GetDefaultSpeaker()
	}

	speakerList := make([]map[string]any, 0, len(speakers))
	for _, spk := range speakers {
		speakerInfo := map[string]any{
			"ip":        spk.IPAddress,
			"name":      spk.Name,
			"model":     spk.Model,
			"active":    activeSpeaker != nil && spk.IPAddress == activeSpeaker.IPAddress,
			"isDefault": spk.IPAddress == defaultSpeakerIP,
			"firmware":  spk.FirmwareVersion,
		}
		speakerList = append(speakerList, speakerInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"speakers":       speakerList,
		"defaultSpeaker": defaultSpeakerIP,
	})
}

// handleSpeakersDiscover triggers mDNS discovery.
func (s *Server) handleSpeakersDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	speakers, err := s.manager.Discover(ctx)
	if err != nil {
		s.jsonError(w, "Discovery failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	speakerList := make([]map[string]any, 0, len(speakers))
	for _, spk := range speakers {
		speakerList = append(speakerList, map[string]any{
			"ip":    spk.IPAddress,
			"name":  spk.Name,
			"model": spk.Model,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"discovered": speakerList,
	})
}

// handleSpeakersAdd adds a speaker by IP address.
func (s *Server) handleSpeakersAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.IP == "" {
		s.jsonError(w, "IP address required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	spk, err := s.manager.AddSpeaker(ctx, req.IP)
	if err != nil {
		s.jsonError(w, "Failed to add speaker: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Save speaker to config for persistence
	if s.opts.Config != nil {
		if err := s.opts.Config.AddOrUpdateSpeaker(config.SpeakerConfig{
			IPAddress:       spk.IPAddress,
			Name:            spk.Name,
			Model:           spk.Model,
			FirmwareVersion: spk.FirmwareVersion,
			MacAddress:      spk.MacAddress,
			MaxVolume:       spk.MaxVolume,
		}); err != nil {
			log.Printf("Warning: could not save speaker to config: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"speaker": map[string]any{
			"ip":       spk.IPAddress,
			"name":     spk.Name,
			"model":    spk.Model,
			"firmware": spk.FirmwareVersion,
		},
	})
}

// handleSpeakersDefault gets or sets the default speaker.
func (s *Server) handleSpeakersDefault(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get current default speaker
		var defaultIP string
		if s.opts.Config != nil {
			defaultIP = s.opts.Config.GetDefaultSpeaker()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"defaultSpeaker": defaultIP,
		})

	case http.MethodPost:
		// Set default speaker
		var req struct {
			IP string `json:"ip"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if s.opts.Config == nil {
			s.jsonError(w, "Config not available", http.StatusInternalServerError)
			return
		}

		if err := s.opts.Config.SetDefaultSpeaker(req.IP); err != nil {
			s.jsonError(w, "Failed to save default speaker: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"defaultSpeaker": req.IP,
			"message":        "Default speaker updated",
		})

	case http.MethodDelete:
		// Clear default speaker
		if s.opts.Config != nil {
			if err := s.opts.Config.SetDefaultSpeaker(""); err != nil {
				s.jsonError(w, "Failed to clear default speaker: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Default speaker cleared",
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSpeaker gets or sets the active speaker.
func (s *Server) handleSpeaker(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleSpeakerGet(w, r)
	case http.MethodPost:
		s.handleSpeakerSet(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSpeakerGet(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": nil,
		})
		return
	}

	// If the speaker is in standby, return cached info without querying it.
	if s.manager.IsInStandby() {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"active": map[string]any{
				"ip":       spk.IPAddress,
				"name":     spk.Name,
				"model":    spk.Model,
				"firmware": spk.FirmwareVersion,
				"source":   "standby",
				"volume":   0,
				"muted":    false,
				"status":   "standby",
			},
		})
		return
	}

	ctx := r.Context()

	// Get current state
	source, _ := spk.Source(ctx)
	volume, _ := spk.GetVolume(ctx)
	muted, _ := spk.IsMuted(ctx)
	status, _ := spk.SpeakerState(ctx)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"active": map[string]any{
			"ip":       spk.IPAddress,
			"name":     spk.Name,
			"model":    spk.Model,
			"firmware": spk.FirmwareVersion,
			"source":   string(source),
			"volume":   volume,
			"muted":    muted,
			"status":   string(status),
		},
	})
}

func (s *Server) handleSpeakerSet(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.IP == "" {
		s.jsonError(w, "IP address required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := s.manager.SetActiveSpeaker(ctx, req.IP); err != nil {
		s.jsonError(w, "Failed to set active speaker: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Broadcast the new speaker's state to all SSE clients
	go s.broadcastCurrentState()

	// Return the new active speaker info
	s.handleSpeakerGet(w, r)
}

// handleSpeakerLogo proxies the KEF logo from the active speaker.
func (s *Server) handleSpeakerLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		http.Error(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	// Fetch the logo from the speaker
	logoURL := fmt.Sprintf("http://%s/style/images/logo-kef.png", spk.IPAddress)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logoURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch logo", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Logo not available", resp.StatusCode)
		return
	}

	// Copy headers and body
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	_, _ = io.Copy(w, resp.Body)
}

// proxyIconURL rewrites icon URLs that point to private/local IPs
// to use the /api/proxy/image endpoint instead. External URLs pass through unchanged.
// This allows the frontend to load images when accessed remotely via Tailscale.
func (s *Server) proxyIconURL(iconURL string) string {
	if iconURL == "" {
		return ""
	}
	parsed, err := url.Parse(iconURL)
	if err != nil {
		return iconURL
	}
	host := parsed.Hostname()
	if host == "" {
		return iconURL
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return iconURL
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return "/api/proxy/image?url=" + url.QueryEscape(iconURL)
	}
	return iconURL
}

// proxyPlaylistIcons returns a copy of the playlist with icon URLs rewritten for the proxy.
func (s *Server) proxyPlaylistIcons(pl *playlist.Playlist) *playlist.Playlist {
	proxied := *pl
	proxied.Tracks = make([]playlist.Track, len(pl.Tracks))
	copy(proxied.Tracks, pl.Tracks)
	for i := range proxied.Tracks {
		proxied.Tracks[i].Icon = s.proxyIconURL(proxied.Tracks[i].Icon)
	}
	return &proxied
}

// handleProxyImage proxies image requests to speaker-local URLs.
// This allows the frontend to load images from private IPs when accessed remotely via Tailscale.
func (s *Server) handleProxyImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "Missing url parameter", http.StatusBadRequest)
		return
	}

	parsed, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid url parameter", http.StatusBadRequest)
		return
	}

	// Security: only proxy requests to private/local IPs
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()) {
		http.Error(w, "Only private IP addresses can be proxied", http.StatusForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to fetch image", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Upstream error", resp.StatusCode)
		return
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Limit response to 10MB to prevent abuse
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 10<<20))
}

// handlePlayer returns the current player state.
func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	// If the speaker is in standby, return a cached standby response
	// instead of querying it (which would wake it up).
	if s.manager.IsInStandby() {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":    "stopped",
			"volume":   0,
			"muted":    false,
			"source":   "standby",
			"title":    "",
			"artist":   "",
			"album":    "",
			"icon":     "",
			"duration": 0,
			"position": 0,
		})
		return
	}

	ctx := r.Context()

	// Get player data
	playerData, err := spk.PlayerData(ctx)
	if err != nil {
		// If we can't get player data, return basic state
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		source, _ := spk.Source(ctx)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":    "stopped",
			"volume":   volume,
			"muted":    muted,
			"source":   string(source),
			"title":    "",
			"artist":   "",
			"album":    "",
			"icon":     "",
			"duration": 0,
			"position": 0,
		})
		return
	}

	volume, _ := spk.GetVolume(ctx)
	muted, _ := spk.IsMuted(ctx)
	source, _ := spk.Source(ctx)
	position, _ := spk.SongProgressMS(ctx)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state":     playerData.State,
		"volume":    volume,
		"muted":     muted,
		"source":    string(source),
		"title":     playerData.TrackRoles.Title,
		"artist":    playerData.TrackRoles.MediaData.MetaData.Artist,
		"album":     playerData.TrackRoles.MediaData.MetaData.Album,
		"icon":      s.proxyIconURL(playerData.TrackRoles.Icon),
		"duration":  playerData.Status.Duration,
		"position":  position,
		"audioType": playerData.MediaRoles.AudioType,
		"live":      playerData.MediaRoles.MediaData.MetaData.Live,
	})
}

// handlePlayerPlay toggles play/pause.
func (s *Server) handlePlayerPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	if err := spk.PlayPause(r.Context()); err != nil {
		s.jsonError(w, "Play/pause failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerStop stops playback (for radio/streaming where pause doesn't apply).
func (s *Server) handlePlayerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	if err := spk.Stop(r.Context()); err != nil {
		s.jsonError(w, "Stop failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerNext skips to next track.
func (s *Server) handlePlayerNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	if err := spk.NextTrack(r.Context()); err != nil {
		s.jsonError(w, "Next track failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerPrev goes to previous track.
func (s *Server) handlePlayerPrev(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	if err := spk.PreviousTrack(r.Context()); err != nil {
		s.jsonError(w, "Previous track failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerSeek seeks to a specific position in the current track.
func (s *Server) handlePlayerSeek(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		PositionMS int `json:"positionMs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.PositionMS < 0 {
		s.jsonError(w, "Position must be non-negative", http.StatusBadRequest)
		return
	}

	if err := spk.SeekTo(r.Context(), int64(req.PositionMS)); err != nil {
		s.jsonError(w, "Seek failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"positionMs": req.PositionMS,
	})
}

// handlePlayerPower gets or sets power state (on/standby).
func (s *Server) handlePlayerPower(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		isPoweredOn, err := spk.IsPoweredOn(ctx)
		if err != nil {
			s.jsonError(w, "Failed to get power state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		status, _ := spk.SpeakerState(ctx)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"poweredOn": isPoweredOn,
			"status":    string(status),
		})

	case http.MethodPost:
		var req struct {
			PowerOn *bool `json:"powerOn"`
		}

		body, _ := io.ReadAll(r.Body)

		// wantPowerOn tracks the intended result so we can return it without
		// querying the speaker again (which could wake it from standby).
		var wantPowerOn bool

		// Determine the desired action
		isToggle := len(body) == 0 || string(body) == "{}"
		if !isToggle {
			if err := json.Unmarshal(body, &req); err != nil {
				s.jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			isToggle = req.PowerOn == nil
		}

		if isToggle {
			// Toggle: read current state, then flip
			isPoweredOn, _ := spk.IsPoweredOn(ctx)
			wantPowerOn = !isPoweredOn
		} else {
			wantPowerOn = *req.PowerOn
		}

		// Execute the action
		if wantPowerOn {
			if err := spk.SetSource(ctx, kefw2.SourceWiFi); err != nil {
				s.jsonError(w, "Failed to power on: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.manager.NotifyWake()
		} else {
			if err := spk.PowerOff(ctx); err != nil {
				s.jsonError(w, "Failed to power off: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.manager.NotifyStandby()
		}

		// Return the intended state directly — no readback query needed.
		// After PowerOff the speaker is entering standby; querying it via HTTP
		// would wake it right back up, defeating the purpose.
		resultStatus := "standby"
		if wantPowerOn {
			resultStatus = "powerOn"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"poweredOn": wantPowerOn,
			"status":    resultStatus,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerVolume gets or sets volume.
func (s *Server) handlePlayerVolume(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		volume, err := spk.GetVolume(r.Context())
		if err != nil {
			s.jsonError(w, "Failed to get volume: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"volume": volume})

	case http.MethodPost:
		var req struct {
			Volume int `json:"volume"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Volume < 0 || req.Volume > 100 {
			s.jsonError(w, "Volume must be between 0 and 100", http.StatusBadRequest)
			return
		}

		if err := spk.SetVolume(r.Context(), req.Volume); err != nil {
			s.jsonError(w, "Failed to set volume: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"volume": req.Volume})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerMute gets or sets mute state.
func (s *Server) handlePlayerMute(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		muted, err := spk.IsMuted(r.Context())
		if err != nil {
			s.jsonError(w, "Failed to get mute state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"muted": muted})

	case http.MethodPost:
		var req struct {
			Muted *bool `json:"muted"`
		}
		body, _ := io.ReadAll(r.Body)

		// If body is empty or {}, toggle mute
		if len(body) == 0 || string(body) == "{}" {
			muted, _ := spk.IsMuted(r.Context())
			if muted {
				if err := spk.Unmute(r.Context()); err != nil {
					s.jsonError(w, "Failed to unmute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				if err := spk.Mute(r.Context()); err != nil {
					s.jsonError(w, "Failed to mute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else {
			if err := json.Unmarshal(body, &req); err != nil {
				s.jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			switch {
			case req.Muted == nil:
				// Toggle
				muted, _ := spk.IsMuted(r.Context())
				if muted {
					if err := spk.Unmute(r.Context()); err != nil {
						s.jsonError(w, "Failed to unmute: "+err.Error(), http.StatusInternalServerError)
						return
					}
				} else {
					if err := spk.Mute(r.Context()); err != nil {
						s.jsonError(w, "Failed to mute: "+err.Error(), http.StatusInternalServerError)
						return
					}
				}
			case *req.Muted:
				if err := spk.Mute(r.Context()); err != nil {
					s.jsonError(w, "Failed to mute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				if err := spk.Unmute(r.Context()); err != nil {
					s.jsonError(w, "Failed to unmute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		muted, _ := spk.IsMuted(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"muted": muted})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerSource gets or sets the audio source.
func (s *Server) handlePlayerSource(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		source, err := spk.Source(r.Context())
		if err != nil {
			s.jsonError(w, "Failed to get source: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"source": string(source)})

	case http.MethodPost:
		var req struct {
			Source string `json:"source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		source := kefw2.Source(req.Source)
		if err := spk.SetSource(r.Context(), source); err != nil {
			s.jsonError(w, "Failed to set source: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Notify manager of standby/wake so it can pause/resume event reconnection
		if source == kefw2.SourceStandby {
			s.manager.NotifyStandby()
		} else {
			s.manager.NotifyWake()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"source": req.Source})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleQueue handles queue operations.
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	// Create Airable client to get the play queue
	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		// Queue might be empty or not available
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tracks":       []any{},
			"currentIndex": -1,
		})
		return
	}

	// Get current player data to identify current track
	playerData, playerErr := spk.PlayerData(ctx)
	nowPlayingPath := ""
	nowPlayingTitle := ""
	if playerErr == nil {
		nowPlayingPath = playerData.TrackRoles.Path
		nowPlayingTitle = playerData.TrackRoles.Title
	}

	// Convert to simplified track list and find current track by matching path
	currentIndex := -1
	tracks := make([]map[string]any, 0, len(queueResp.Rows))
	for i, item := range queueResp.Rows {
		track := map[string]any{
			"index":    i,
			"title":    item.Title,
			"id":       item.ID,
			"path":     item.Path,
			"icon":     s.proxyIconURL(item.Icon),
			"type":     item.Type,
			"duration": 0,
		}

		// Extract artist/album from media data if available
		if item.MediaData != nil {
			track["artist"] = item.MediaData.MetaData.Artist
			track["album"] = item.MediaData.MetaData.Album
			// Get duration from resources if available
			if len(item.MediaData.Resources) > 0 {
				track["duration"] = item.MediaData.Resources[0].Duration
			}
		}

		// Match current track by path (speaker returns the item's path in TrackRoles.Path)
		if currentIndex == -1 && nowPlayingPath != "" && item.Path == nowPlayingPath {
			currentIndex = i
		}

		tracks = append(tracks, track)
	}

	// Fallback: match by title if path didn't match (e.g. non-queue playback)
	if currentIndex == -1 && nowPlayingTitle != "" {
		for i, item := range queueResp.Rows {
			if item.Title == nowPlayingTitle {
				currentIndex = i
				break
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"tracks":       tracks,
		"currentIndex": currentIndex,
	})
}

// handleQueuePlay plays a specific track in the queue.
func (s *Server) handleQueuePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Index int `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get the queue to find the track at the specified index
	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		s.jsonError(w, "Failed to get queue: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Index < 0 || req.Index >= len(queueResp.Rows) {
		s.jsonError(w, "Index out of range", http.StatusBadRequest)
		return
	}

	track := queueResp.Rows[req.Index]
	if err := airable.PlayQueueIndex(req.Index, &track); err != nil {
		s.jsonError(w, "Failed to play track: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleQueueRemove removes tracks from the queue.
func (s *Server) handleQueueRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Indices []int `json:"indices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Indices) == 0 {
		s.jsonError(w, "No indices provided", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.RemoveFromQueue(req.Indices); err != nil {
		s.jsonError(w, "Failed to remove tracks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleQueueMove moves a track within the queue.
func (s *Server) handleQueueMove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.MoveQueueItem(req.From, req.To); err != nil {
		s.jsonError(w, "Failed to move track: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleQueueClear clears the entire queue.
func (s *Server) handleQueueClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.ClearPlaylist(); err != nil {
		s.jsonError(w, "Failed to clear queue: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleQueueMode gets or sets the play mode (shuffle/repeat).
func (s *Server) handleQueueMode(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	airable := kefw2.NewAirableClient(spk)

	switch r.Method {
	case http.MethodGet:
		mode, err := airable.GetPlayMode()
		if err != nil {
			s.jsonError(w, "Failed to get play mode: "+err.Error(), http.StatusInternalServerError)
			return
		}

		shuffle, _ := airable.IsShuffleEnabled()
		repeat, _ := airable.GetRepeatMode()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":    mode,
			"shuffle": shuffle,
			"repeat":  repeat,
		})

	case http.MethodPost:
		var req struct {
			Mode    *string `json:"mode"`
			Shuffle *bool   `json:"shuffle"`
			Repeat  *string `json:"repeat"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Handle direct mode setting
		if req.Mode != nil {
			if err := airable.SetPlayMode(*req.Mode); err != nil {
				s.jsonError(w, "Failed to set play mode: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Handle shuffle toggle
		if req.Shuffle != nil {
			if err := airable.SetShuffle(*req.Shuffle); err != nil {
				s.jsonError(w, "Failed to set shuffle: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// Handle repeat setting
		if req.Repeat != nil {
			if err := airable.SetRepeat(*req.Repeat); err != nil {
				s.jsonError(w, "Failed to set repeat: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		// Return updated state
		mode, _ := airable.GetPlayMode()
		shuffle, _ := airable.IsShuffleEnabled()
		repeat, _ := airable.GetRepeatMode()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"mode":    mode,
			"shuffle": shuffle,
			"repeat":  repeat,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlaylists handles listing and creating playlists.
func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	if s.playlists == nil {
		s.jsonError(w, "Playlist manager not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// List all playlists
		playlists, err := s.playlists.List()
		if err != nil {
			s.jsonError(w, "Failed to list playlists: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Add track counts
		playlistsWithCount := make([]map[string]any, len(playlists))
		for i, pl := range playlists {
			count, _ := s.playlists.TrackCount(pl.ID)
			playlistsWithCount[i] = map[string]any{
				"id":          pl.ID,
				"name":        pl.Name,
				"description": pl.Description,
				"trackCount":  count,
				"createdAt":   pl.CreatedAt,
				"updatedAt":   pl.UpdatedAt,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"playlists": playlistsWithCount,
		})

	case http.MethodPost:
		// Create new playlist
		var req struct {
			Name        string           `json:"name"`
			Description string           `json:"description"`
			Tracks      []playlist.Track `json:"tracks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.Name == "" {
			s.jsonError(w, "Playlist name is required", http.StatusBadRequest)
			return
		}

		pl, err := s.playlists.Create(req.Name, req.Description, req.Tracks)
		if err != nil {
			s.jsonError(w, "Failed to create playlist: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"playlist": s.proxyPlaylistIcons(pl),
		})
		s.BroadcastPlaylistsChanged()

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlaylist handles operations on a single playlist.
func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	if s.playlists == nil {
		s.jsonError(w, "Playlist manager not available", http.StatusServiceUnavailable)
		return
	}

	// Extract playlist ID from path: /api/playlists/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/playlists/")
	if id == "" || strings.Contains(id, "/") {
		s.jsonError(w, "Invalid playlist ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get playlist with tracks
		pl, err := s.playlists.Get(id)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"playlist": s.proxyPlaylistIcons(pl),
		})

	case http.MethodPut:
		// Update playlist
		var req struct {
			Name        string           `json:"name"`
			Description string           `json:"description"`
			Tracks      []playlist.Track `json:"tracks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		pl, err := s.playlists.Update(id, req.Name, req.Description, req.Tracks)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"playlist": s.proxyPlaylistIcons(pl),
		})
		s.BroadcastPlaylistsChanged()

	case http.MethodDelete:
		// Delete playlist
		if err := s.playlists.Delete(id); err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		s.BroadcastPlaylistsChanged()

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSaveQueueAsPlaylist saves the current queue as a new playlist.
func (s *Server) handleSaveQueueAsPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.playlists == nil {
		s.jsonError(w, "Playlist manager not available", http.StatusServiceUnavailable)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.jsonError(w, "Playlist name is required", http.StatusBadRequest)
		return
	}

	// Get current queue
	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		s.jsonError(w, "Failed to get queue: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(queueResp.Rows) == 0 {
		s.jsonError(w, "Queue is empty", http.StatusBadRequest)
		return
	}

	// Convert queue items to playlist tracks
	tracks := make([]playlist.Track, 0, len(queueResp.Rows))
	for _, item := range queueResp.Rows {
		// Skip non-playable items (containers, etc.)
		if item.Type == contentTypeContainer {
			continue
		}

		track := playlist.Track{
			Title: item.Title,
			ID:    item.ID,
			Path:  item.Path,
			Icon:  item.Icon,
			Type:  item.Type,
		}
		if item.MediaData != nil {
			track.Artist = item.MediaData.MetaData.Artist
			track.Album = item.MediaData.MetaData.Album
			track.ServiceID = item.MediaData.MetaData.ServiceID
			if len(item.MediaData.Resources) > 0 {
				track.Duration = item.MediaData.Resources[0].Duration
				track.URI = item.MediaData.Resources[0].URI
				track.MimeType = item.MediaData.Resources[0].MimeType
			}
		}

		// Note: queue items may have ephemeral paths like "playlists:item/N" that
		// can't be re-resolved later. When loading back, the URI is used instead.

		tracks = append(tracks, track)
	}

	// Create playlist
	pl, err := s.playlists.Create(req.Name, req.Description, tracks)
	if err != nil {
		s.jsonError(w, "Failed to create playlist: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"playlist": s.proxyPlaylistIcons(pl),
	})
	s.BroadcastPlaylistsChanged()
}

// handleLoadPlaylist loads a playlist to the speaker's queue.
func (s *Server) handleLoadPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.playlists == nil {
		s.jsonError(w, "Playlist manager not available", http.StatusServiceUnavailable)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	// Extract playlist ID from path: /api/playlists/load/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/playlists/load/")
	if id == "" {
		s.jsonError(w, "Playlist ID is required", http.StatusBadRequest)
		return
	}

	// Optional: check if we should append or replace
	var req struct {
		Append bool `json:"append"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // Ignore decode error, use defaults

	// Get playlist
	pl, err := s.playlists.Get(id)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	if len(pl.Tracks) == 0 {
		s.jsonError(w, "Playlist is empty", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)

	// Clear queue if not appending
	if !req.Append {
		if err := airable.ClearPlaylist(); err != nil {
			s.jsonError(w, "Failed to clear queue: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Give the speaker time to process the clear before adding new items
		time.Sleep(500 * time.Millisecond)
	}

	// Convert playlist tracks to ContentItems, filtering out non-playable items.
	// For UPnP tracks that have a browsable path but no stream URI, resolve the
	// full track details from the speaker API (the speaker returns the stream URL).
	contentItems := make([]kefw2.ContentItem, 0, len(pl.Tracks))
	skipped := 0
	for _, track := range pl.Tracks {
		// Skip containers (albums, folders) — they can't be played as individual tracks
		if track.Type == contentTypeContainer {
			skipped++
			continue
		}

		// Skip tracks with no playback URI and no browsable path
		if track.URI == "" && track.Path == "" {
			skipped++
			continue
		}

		// If the track has a browsable path but no stream URI, resolve it
		// from the speaker API to get the full ContentItem with stream URL.
		// This handles UPnP tracks that were added to playlists by path only.
		if track.URI == "" && track.Path != "" {
			resp, resolveErr := airable.GetRows(track.Path, 0, 1)
			if resolveErr == nil {
				var resolved *kefw2.ContentItem
				switch {
				case resp.Roles != nil:
					resolved = resp.Roles
				case len(resp.Rows) > 0:
					resolved = &resp.Rows[0]
				}
				if resolved != nil {
					contentItems = append(contentItems, *resolved)
					continue
				}
			}
			// Resolution failed — fall through to manual construction
			skipped++
			continue
		}

		// Determine service ID, default to UPnP for local media
		serviceID := track.ServiceID
		if serviceID == "" {
			serviceID = "UPnP"
		}

		// Fix paths: queue-internal paths like "playlists:item/N" are ephemeral
		// and can't be resolved by the speaker. Use the URI as the path instead,
		// which works for addexternalitems since the speaker plays from the URI.
		path := track.Path
		if strings.HasPrefix(path, "playlists:item/") || path == "" {
			path = track.URI
		}

		contentItems = append(contentItems, kefw2.ContentItem{
			Title: track.Title,
			ID:    track.ID,
			Path:  path,
			Icon:  track.Icon,
			Type:  track.Type,
			MediaData: &kefw2.MediaData{
				MetaData: kefw2.MediaMetaData{
					Artist:    track.Artist,
					Album:     track.Album,
					ServiceID: serviceID,
				},
				Resources: []kefw2.MediaResource{
					{
						URI:      track.URI,
						MimeType: track.MimeType,
						Duration: track.Duration,
					},
				},
			},
		})
	}

	if len(contentItems) == 0 {
		s.jsonError(w, "No playable tracks in playlist", http.StatusBadRequest)
		return
	}

	// Add tracks to queue and start playing
	if err := airable.AddToQueue(contentItems, true); err != nil {
		s.jsonError(w, "Failed to add tracks to queue: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     "ok",
		"trackCount": len(contentItems),
		"skipped":    skipped,
	})
}

// BrowseItem represents a browsable content item for the API response.
type BrowseItem struct {
	Title         string           `json:"title"`
	Type          string           `json:"type"` // "container", "audio", "query"
	Path          string           `json:"path"`
	Icon          string           `json:"icon,omitempty"`
	Artist        string           `json:"artist,omitempty"`
	Album         string           `json:"album,omitempty"`
	Duration      int              `json:"duration,omitempty"` // milliseconds
	ID            string           `json:"id,omitempty"`
	Description   string           `json:"description,omitempty"`
	Playable      bool             `json:"playable,omitempty"`
	AudioType     string           `json:"audioType,omitempty"`     // "audioBroadcast" for radio
	MediaData     *kefw2.MediaData `json:"mediaData,omitempty"`     // Required for queue playback of airable content
	ContainerPath string           `json:"containerPath,omitempty"` // Parent container path for podcast episodes
	SearchQuery   string           `json:"searchQuery,omitempty"`   // If set, clicking triggers this search instead of browsing
}

// handleBrowse handles content browsing for UPnP, Radio, and Podcasts.
// Routes:.
//   - GET /api/browse/sources - List available content sources
//   - GET /api/browse/upnp - List UPnP media servers
//   - GET /api/browse/upnp/{path...} - Browse UPnP container
//   - GET /api/browse/radio - Radio menu
//   - GET /api/browse/radio/search?q=query - Search radio stations
//   - GET /api/browse/podcasts - Podcast menu
//   - GET /api/browse/podcasts/search?q=query - Search podcasts
//   - POST /api/browse/play - Play an item
//   - POST /api/browse/queue - Add an item to the queue
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	// Extract path after /api/browse/
	path := strings.TrimPrefix(r.URL.Path, "/api/browse")
	path = strings.TrimPrefix(path, "/")

	// Handle POST actions
	if r.Method == http.MethodPost {
		switch path {
		case "play":
			s.handleBrowsePlay(w, r)
			return
		case "queue":
			s.handleBrowseAddToQueue(w, r)
			return
		case "favorite":
			s.handleBrowseFavorite(w, r)
			return
		default:
			s.jsonError(w, "Unknown action", http.StatusNotFound)
			return
		}
	}

	// Only GET for browsing
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Route based on path
	switch {
	case path == "" || path == "sources":
		s.handleBrowseSources(w, r)
	case path == browseSourceUPnP:
		s.handleBrowseUPnP(w, r, "")
	case strings.HasPrefix(path, browseSourceUPnP+"/"):
		s.handleBrowseUPnP(w, r, strings.TrimPrefix(path, browseSourceUPnP+"/"))
	case path == browseSourceRadio:
		s.handleBrowseRadio(w, r, "")
	case strings.HasPrefix(path, browseSourceRadio+"/"):
		s.handleBrowseRadio(w, r, strings.TrimPrefix(path, browseSourceRadio+"/"))
	case path == browseSourcePodcasts:
		s.handleBrowsePodcasts(w, r, "")
	case strings.HasPrefix(path, browseSourcePodcasts+"/"):
		s.handleBrowsePodcasts(w, r, strings.TrimPrefix(path, browseSourcePodcasts+"/"))
	default:
		s.jsonError(w, "Unknown browse path", http.StatusNotFound)
	}
}

// handleBrowseSources returns available content sources.
func (s *Server) handleBrowseSources(w http.ResponseWriter, _ *http.Request) {
	sources := []map[string]any{
		{
			"id":          "upnp",
			"name":        "Media Servers",
			"description": "UPnP/DLNA media servers on your network",
			"icon":        "server",
		},
		{
			"id":          "radio",
			"name":        "Internet Radio",
			"description": "Browse and search internet radio stations",
			"icon":        "radio",
		},
		{
			"id":          "podcasts",
			"name":        "Podcasts",
			"description": "Browse and search podcasts",
			"icon":        "podcast",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"sources": sources,
	})
}

// handleBrowseUPnP handles UPnP media server browsing.
//
//nolint:gocyclo // UPnP browsing inherently requires many conditional branches
func (s *Server) handleBrowseUPnP(w http.ResponseWriter, r *http.Request, subpath string) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	airable := s.getCachedAirableClient(spk)

	var resp *kefw2.RowsResponse
	var err error

	// Check for search query first
	searchQuery := r.URL.Query().Get("q")
	if searchQuery != "" {
		// Search the UPnP track index
		index, loadErr := kefw2.LoadTrackIndexCached()
		if loadErr != nil || index == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":      []BrowseItem{},
				"totalCount": 0,
				"source":     "upnp",
				"search":     true,
				"message":    "No media index found. Use 'kefw2 upnp index' to build the search index.",
			})
			return
		}

		results := kefw2.SearchTracks(index, searchQuery, 100)
		if len(results) == 0 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items":      []BrowseItem{},
				"totalCount": 0,
				"source":     "upnp",
				"search":     true,
				"message":    "No results found.",
			})
			return
		}

		// Convert search results to BrowseItems
		items := make([]BrowseItem, 0, len(results))

		// For artist searches, prepend synthetic album headers so albums are easy to find
		if strings.HasPrefix(strings.ToLower(searchQuery), "artist:") {
			albums := kefw2.AlbumsForArtist(results)
			for i, album := range albums {
				item := BrowseItem{
					Title:       album.Album,
					Type:        contentTypeContainer,
					Path:        fmt.Sprintf("album-header-%d", i),
					Icon:        s.proxyIconURL(album.Icon),
					Artist:      album.Artist,
					Description: fmt.Sprintf("%d tracks", album.TrackCount),
					SearchQuery: fmt.Sprintf(`album:"%s"`, album.Album),
				}
				items = append(items, item)
			}
		}

		for _, track := range results {
			item := BrowseItem{
				Title:    track.Title,
				Type:     contentTypeAudio,
				Path:     track.Path,
				Icon:     s.proxyIconURL(track.Icon),
				Artist:   track.Artist,
				Album:    track.Album,
				Duration: track.Duration,
				Playable: true,
				MediaData: &kefw2.MediaData{
					MetaData: kefw2.MediaMetaData{
						Artist:    track.Artist,
						Album:     track.Album,
						ServiceID: "UPnP",
					},
					Resources: []kefw2.MediaResource{
						{
							URI:      track.URI,
							MimeType: track.MimeType,
							Duration: track.Duration,
						},
					},
				},
			}
			items = append(items, item)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":      items,
			"totalCount": len(items),
			"source":     "upnp",
			"search":     true,
			"indexInfo": map[string]any{
				"serverName": index.ServerName,
				"trackCount": len(index.Tracks),
				"indexedAt":  index.IndexedAt,
			},
		})
		return
	}

	// Check for direct path navigation first (used when clicking into containers)
	itemPath := r.URL.Query().Get("path")

	// If no path provided, check for configured browse container
	if itemPath == "" && subpath == "" {
		if s.opts.Config != nil {
			upnp := s.opts.Config.GetUPnPConfig()
			if upnp.DefaultServerPath != "" && upnp.BrowseContainer != "" {
				// Resolve the human-readable path to an API path
				resolvedPath, _, resolveErr := kefw2.FindContainerByPath(airable, upnp.DefaultServerPath, upnp.BrowseContainer)
				if resolveErr == nil && resolvedPath != "" {
					itemPath = resolvedPath
				}
				// If resolve fails, fall through to listing servers
			}
		}
	}

	switch {
	case itemPath != "":
		// Direct path navigation takes priority
		resp, err = airable.BrowseContainer(itemPath)
	case subpath == "":
		// List media servers (fallback if no browse container configured)
		resp, err = airable.GetMediaServers()
	case strings.HasPrefix(subpath, "upnp:"):
		// Full API path in subpath
		resp, err = airable.BrowseContainer(subpath)
	default:
		// Try to match by server name
		resp, err = airable.GetMediaServers()
		if err == nil && len(resp.Rows) > 0 {
			for _, server := range resp.Rows {
				if server.Title == subpath && server.Type != "query" {
					resp, err = airable.BrowseContainer(server.Path)
					break
				}
			}
		}
	}

	if err != nil {
		s.jsonError(w, "Failed to browse: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	items := make([]BrowseItem, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		item := BrowseItem{
			Title:     row.Title,
			Type:      row.Type,
			Path:      row.Path,
			Icon:      s.proxyIconURL(row.GetThumbnail()),
			ID:        row.ID,
			Playable:  row.Type == contentTypeAudio || row.ContainerPlayable,
			AudioType: row.AudioType,
			MediaData: row.MediaData, // Include full media data for queue playback
		}

		// Add metadata if available
		if row.MediaData != nil {
			item.Artist = row.MediaData.MetaData.Artist
			item.Album = row.MediaData.MetaData.Album
			if len(row.MediaData.Resources) > 0 {
				item.Duration = row.MediaData.Resources[0].Duration
			}
		}

		// Skip search entries for now (type="query")
		if row.Type != "query" {
			items = append(items, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "upnp",
	})
}

// handleBrowseRadio handles internet radio browsing.
func (s *Server) handleBrowseRadio(w http.ResponseWriter, r *http.Request, subpath string) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	airable := s.getCachedAirableClient(spk)

	var resp *kefw2.RowsResponse
	var err error

	// Check for search query
	searchQuery := r.URL.Query().Get("q")
	// Check for direct path navigation (used when clicking into containers)
	itemPath := r.URL.Query().Get("path")

	switch {
	case searchQuery != "":
		resp, err = airable.SearchRadio(searchQuery)
	case itemPath != "":
		// Direct path navigation takes priority
		// For favorites, don't cache as it changes frequently
		if strings.HasSuffix(itemPath, "/favorites") {
			uncachedAirable := kefw2.NewAirableClient(spk)
			resp, err = uncachedAirable.GetRows(itemPath, 0, 100)
		} else {
			resp, err = airable.BrowseRadioByItemPath(itemPath)
		}
	case subpath == "" || subpath == "menu":
		resp, err = airable.GetRadioMenu()
	case subpath == "favorites":
		resp, err = airable.GetRadioFavorites()
	case subpath == "local":
		resp, err = airable.GetRadioLocal()
	case subpath == "popular":
		resp, err = airable.GetRadioPopular()
	case subpath == "trending":
		resp, err = airable.GetRadioTrending()
	case subpath == "hq":
		resp, err = airable.GetRadioHQ()
	case subpath == "new":
		resp, err = airable.GetRadioNew()
	default:
		// Try display path navigation
		resp, err = airable.BrowseRadioByDisplayPath(subpath)
	}

	if err != nil {
		s.jsonError(w, "Failed to browse radio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	items := make([]BrowseItem, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		item := BrowseItem{
			Title:       row.Title,
			Type:        row.Type,
			Path:        row.Path,
			Icon:        s.proxyIconURL(row.GetThumbnail()),
			ID:          row.ID,
			Description: row.LongDescription,
			Playable:    row.Type == contentTypeAudio || row.ContainerPlayable || row.AudioType == "audioBroadcast",
			AudioType:   row.AudioType,
			MediaData:   row.MediaData, // Include full media data for queue playback
		}

		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "radio",
	})
}

// handleBrowsePodcasts handles podcast browsing.
func (s *Server) handleBrowsePodcasts(w http.ResponseWriter, r *http.Request, subpath string) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	airable := s.getCachedAirableClient(spk)

	var resp *kefw2.RowsResponse
	var err error

	// Check for search query
	searchQuery := r.URL.Query().Get("q")
	// Check for direct path navigation (used when clicking into containers)
	itemPath := r.URL.Query().Get("path")

	switch {
	case searchQuery != "":
		resp, err = airable.SearchPodcasts(searchQuery)
	case itemPath != "":
		// Direct path navigation takes priority
		// For favorites and history, don't cache as these change frequently
		if strings.HasSuffix(itemPath, "/favorites") || strings.HasSuffix(itemPath, "/history") {
			// Use uncached client for dynamic content
			uncachedAirable := kefw2.NewAirableClient(spk)
			resp, err = uncachedAirable.GetRows(itemPath, 0, 100)
		} else {
			resp, err = airable.GetRows(itemPath, 0, 100)
		}
	case subpath == "" || subpath == "menu":
		resp, err = airable.GetPodcastMenu()
	case subpath == "favorites":
		resp, err = airable.GetPodcastFavorites()
	case subpath == "popular":
		resp, err = airable.GetPodcastPopular()
	case subpath == "trending":
		resp, err = airable.GetPodcastTrending()
	case subpath == "history":
		resp, err = airable.GetPodcastHistory()
	default:
		// Try display path navigation
		resp, err = airable.BrowsePodcastByDisplayPath(subpath)
	}

	if err != nil {
		s.jsonError(w, "Failed to browse podcasts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	items := make([]BrowseItem, 0, len(resp.Rows))

	// If we're browsing an episodes container, store the container path for playback
	// Episodes need their parent container path to play correctly
	var episodesContainerPath string
	if itemPath != "" && strings.HasSuffix(itemPath, "/episodes") {
		episodesContainerPath = itemPath
	}

	for _, row := range resp.Rows {
		item := BrowseItem{
			Title:       row.Title,
			Type:        row.Type,
			Path:        row.Path,
			Icon:        s.proxyIconURL(row.GetThumbnail()),
			ID:          row.ID,
			Description: row.LongDescription,
			Playable:    row.Type == contentTypeAudio,
			AudioType:   row.AudioType,
			MediaData:   row.MediaData, // Include full media data for queue playback
		}

		// For podcast episodes, add the container path to Context for playback
		if episodesContainerPath != "" && row.Type == contentTypeAudio {
			item.ContainerPath = episodesContainerPath
		}

		// Add duration if available
		if row.MediaData != nil && len(row.MediaData.Resources) > 0 {
			item.Duration = row.MediaData.Resources[0].Duration
		}

		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "podcasts",
	})
}

// handleBrowsePlay handles playing a browsed item.
func (s *Server) handleBrowsePlay(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Path          string           `json:"path"`
		Source        string           `json:"source"` // "upnp", "radio", "podcasts"
		Type          string           `json:"type"`   // "audio", "container"
		AudioType     string           `json:"audioType,omitempty"`
		Title         string           `json:"title,omitempty"`
		Icon          string           `json:"icon,omitempty"`
		ID            string           `json:"id,omitempty"`
		MediaData     *kefw2.MediaData `json:"mediaData,omitempty"`     // For podcasts: full media data for playback
		ContainerPath string           `json:"containerPath,omitempty"` // For podcast episodes: parent container path for playback
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		s.jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	var err error

	switch req.Source {
	case browseSourceUPnP:
		if req.Type == contentTypeContainer {
			err = airable.PlayUPnPContainer(req.Path)
		} else {
			err = airable.PlayUPnPByPath(req.Path)
		}
	case browseSourceRadio:
		// Get station details and play
		station, getErr := airable.GetRadioStationDetails(req.Path)
		if getErr != nil {
			s.jsonError(w, "Failed to get station details: "+getErr.Error(), http.StatusInternalServerError)
			return
		}
		err = airable.ResolveAndPlayRadioStation(station)
	case browseSourcePodcasts:
		// Build ContentItem from request data - include mediaData if available for direct playback
		episode := &kefw2.ContentItem{
			Path:      req.Path,
			Title:     req.Title,
			Type:      req.Type,
			Icon:      req.Icon,
			ID:        req.ID,
			AudioType: req.AudioType,
			MediaData: req.MediaData, // Include media data for playback
		}
		// If we have the container path from browsing, set it in Context
		// This allows PlayPodcastEpisode to use the container for playback
		if req.ContainerPath != "" {
			episode.Context = &kefw2.Context{
				Path: req.ContainerPath,
			}
		}
		err = airable.PlayPodcastEpisode(episode)
	default:
		s.jsonError(w, "Unknown source type", http.StatusBadRequest)
		return
	}

	if err != nil {
		s.jsonError(w, "Failed to play: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
	})
}

// handleBrowseAddToQueue adds a browsed item to the queue without playing.
func (s *Server) handleBrowseAddToQueue(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Path      string           `json:"path"`
		Source    string           `json:"source"` // "upnp", "radio", "podcasts"
		Type      string           `json:"type"`   // "audio", "container"
		Title     string           `json:"title,omitempty"`
		Icon      string           `json:"icon,omitempty"`
		Artist    string           `json:"artist,omitempty"`
		Album     string           `json:"album,omitempty"`
		AudioType string           `json:"audioType,omitempty"`
		MediaData *kefw2.MediaData `json:"mediaData,omitempty"` // Full media data for queue playback
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		s.jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	var err error
	var tracksAdded int

	switch req.Source {
	case browseSourceUPnP:
		if req.Type == contentTypeContainer {
			// Get all tracks from container recursively and add to queue
			tracks, getErr := airable.GetContainerTracksRecursive(req.Path)
			if getErr != nil {
				s.jsonError(w, "Failed to get container tracks: "+getErr.Error(), http.StatusInternalServerError)
				return
			}
			if len(tracks) == 0 {
				s.jsonError(w, "No tracks found in container", http.StatusBadRequest)
				return
			}
			err = airable.AddToQueue(tracks, false)
			tracksAdded = len(tracks)
		} else {
			// Single track - browse to get full details from API
			resp, getErr := airable.GetRows(req.Path, 0, 1)
			if getErr != nil {
				s.jsonError(w, "Failed to get track details: "+getErr.Error(), http.StatusInternalServerError)
				return
			}
			var track *kefw2.ContentItem
			switch {
			case resp.Roles != nil:
				track = resp.Roles
			case len(resp.Rows) > 0:
				track = &resp.Rows[0]
			default:
				s.jsonError(w, "Track not found", http.StatusNotFound)
				return
			}
			err = airable.AddToQueue([]kefw2.ContentItem{*track}, false)
			tracksAdded = 1
		}
	case browseSourceRadio:
		// For radio stations, use mediaData from request if available
		// Otherwise try to fetch full details
		var station *kefw2.ContentItem
		if req.MediaData != nil && len(req.MediaData.Resources) > 0 {
			// Use provided media data (from browser)
			station = &kefw2.ContentItem{
				Title:     req.Title,
				Type:      contentTypeAudio,
				AudioType: req.AudioType,
				Path:      req.Path,
				Icon:      req.Icon,
				MediaData: req.MediaData,
			}
		} else {
			// Try to fetch full details
			var getErr error
			station, getErr = airable.GetRadioStationDetails(req.Path)
			if getErr != nil {
				// Fallback: construct minimal ContentItem if fetch fails
				station = &kefw2.ContentItem{
					Title:     req.Title,
					Type:      contentTypeAudio,
					AudioType: req.AudioType,
					Path:      req.Path,
					Icon:      req.Icon,
				}
			}
		}
		err = airable.AddToQueue([]kefw2.ContentItem{*station}, false)
		tracksAdded = 1
	case browseSourcePodcasts:
		// For podcast episodes, use mediaData from request if available
		// Podcast episode paths cannot be fetched directly, so we rely on browser data
		var episode *kefw2.ContentItem
		if req.MediaData != nil && len(req.MediaData.Resources) > 0 {
			// Use provided media data (from browser)
			episode = &kefw2.ContentItem{
				Title:     req.Title,
				Type:      contentTypeAudio,
				Path:      req.Path,
				Icon:      req.Icon,
				MediaData: req.MediaData,
			}
		} else {
			// Try to fetch full details (may fail for episode paths)
			var getErr error
			episode, getErr = airable.GetPodcastDetails(req.Path)
			if getErr != nil {
				// Fallback: construct minimal ContentItem if fetch fails
				episode = &kefw2.ContentItem{
					Title: req.Title,
					Type:  contentTypeAudio,
					Path:  req.Path,
					Icon:  req.Icon,
				}
			}
		}
		err = airable.AddToQueue([]kefw2.ContentItem{*episode}, false)
		tracksAdded = 1
	default:
		s.jsonError(w, "Unknown source type", http.StatusBadRequest)
		return
	}

	if err != nil {
		s.jsonError(w, "Failed to add to queue: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"tracksAdded": tracksAdded,
	})
}

// handleBrowseFavorite adds or removes an item from favorites.
func (s *Server) handleBrowseFavorite(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Path   string `json:"path"`
		Source string `json:"source"` // "radio", "podcasts"
		ID     string `json:"id"`
		Title  string `json:"title"`
		Add    bool   `json:"add"` // true = add to favorites, false = remove
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		s.jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}

	airable := kefw2.NewAirableClient(spk)
	var err error

	// Create a ContentItem from the request
	item := &kefw2.ContentItem{
		Path:  req.Path,
		ID:    req.ID,
		Title: req.Title,
	}

	switch req.Source {
	case browseSourceRadio:
		if req.Add {
			err = airable.AddRadioFavorite(item)
		} else {
			err = airable.RemoveRadioFavorite(item)
		}
	case browseSourcePodcasts:
		if req.Add {
			err = airable.AddPodcastFavorite(item)
		} else {
			err = airable.RemovePodcastFavorite(item)
		}
	default:
		s.jsonError(w, "Favorites only supported for radio and podcasts", http.StatusBadRequest)
		return
	}

	if err != nil {
		action := "add to"
		if !req.Add {
			action = "remove from"
		}
		s.jsonError(w, "Failed to "+action+" favorites: "+err.Error(), http.StatusInternalServerError)
		return
	}

	action := "added to"
	if !req.Add {
		action = "removed from"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"message": "Successfully " + action + " favorites",
	})
}

// handleSettings returns app-level settings.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return app settings
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"version": "1.0.0",
		"server": map[string]any{
			"port": s.opts.Port,
			"bind": s.opts.Bind,
		},
	})
}

// handleSpeakerSettings returns and updates speaker-specific settings.
func (s *Server) handleSpeakerSettings(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Get speaker settings
		maxVolume, _ := spk.GetMaxVolume(ctx)
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		source, _ := spk.Source(ctx)
		isPoweredOn, _ := spk.IsPoweredOn(ctx)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"speaker": map[string]any{
				"ip":         spk.IPAddress,
				"name":       spk.Name,
				"model":      spk.Model,
				"firmware":   spk.FirmwareVersion,
				"macPrimary": spk.MacAddress,
			},
			"settings": map[string]any{
				"maxVolume": maxVolume,
				"volume":    volume,
				"muted":     muted,
				"source":    string(source),
				"poweredOn": isPoweredOn,
			},
		})

	case http.MethodPut, http.MethodPost:
		// Update speaker settings
		var req struct {
			MaxVolume *int `json:"maxVolume,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Update max volume if provided
		if req.MaxVolume != nil {
			if *req.MaxVolume < 0 || *req.MaxVolume > 100 {
				s.jsonError(w, "Max volume must be between 0 and 100", http.StatusBadRequest)
				return
			}
			if err := spk.SetMaxVolume(ctx, *req.MaxVolume); err != nil {
				s.jsonError(w, "Failed to set max volume: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
		})

	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEQSettings returns and updates EQ/DSP settings.
func (s *Server) handleEQSettings(w http.ResponseWriter, r *http.Request) {
	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		// Get EQ profile
		eqProfile, err := spk.GetEQProfileV2(ctx)
		if err != nil {
			s.jsonError(w, "Failed to get EQ profile: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"eq": map[string]any{
				"profileName":     eqProfile.ProfileName,
				"bassExtension":   eqProfile.BassExtension,
				"deskMode":        eqProfile.DeskMode,
				"deskModeSetting": eqProfile.DeskModeSetting,
				"wallMode":        eqProfile.WallMode,
				"wallModeSetting": eqProfile.WallModeSetting,
				"trebleAmount":    eqProfile.TrebleAmount,
				"balance":         eqProfile.Balance,
				"phaseCorrection": eqProfile.PhaseCorrection,
				"isExpertMode":    eqProfile.IsExpertMode,
			},
			"subwoofer": map[string]any{
				"enabled":      eqProfile.SubwooferOut,
				"count":        eqProfile.SubwooferCount,
				"gain":         eqProfile.SubwooferGain,
				"polarity":     eqProfile.SubwooferPolarity,
				"preset":       eqProfile.SubwooferPreset,
				"lowPassFreq":  eqProfile.SubOutLPFreq,
				"stereo":       eqProfile.SubEnableStereo,
				"highPassMode": eqProfile.HighPassMode,
				"highPassFreq": eqProfile.HighPassModeFreq,
			},
		})

	default:
		// EQ settings are read-only for now (requires complex setData calls)
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUPnPSettings returns and updates UPnP/media server settings.
func (s *Server) handleUPnPSettings(w http.ResponseWriter, r *http.Request) {
	if s.opts.Config == nil {
		s.jsonError(w, "Config not available", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		upnp := s.opts.Config.GetUPnPConfig()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"defaultServer":     upnp.DefaultServer,
			"defaultServerPath": upnp.DefaultServerPath,
			"browseContainer":   upnp.BrowseContainer,
			"indexContainer":    upnp.IndexContainer,
		})

	case http.MethodPut, http.MethodPost:
		var req struct {
			DefaultServer     *string `json:"defaultServer,omitempty"`
			DefaultServerPath *string `json:"defaultServerPath,omitempty"`
			BrowseContainer   *string `json:"browseContainer,omitempty"`
			IndexContainer    *string `json:"indexContainer,omitempty"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		upnp := s.opts.Config.GetUPnPConfig()

		// Update fields if provided
		if req.DefaultServer != nil {
			upnp.DefaultServer = *req.DefaultServer
		}
		if req.DefaultServerPath != nil {
			upnp.DefaultServerPath = *req.DefaultServerPath
		}
		if req.BrowseContainer != nil {
			// Validate browse container requires server
			if *req.BrowseContainer != "" && upnp.DefaultServerPath == "" {
				s.jsonError(w, "Cannot set browse container without a default server", http.StatusBadRequest)
				return
			}
			upnp.BrowseContainer = *req.BrowseContainer
		}
		if req.IndexContainer != nil {
			// Validate index container requires server
			if *req.IndexContainer != "" && upnp.DefaultServerPath == "" {
				s.jsonError(w, "Cannot set index container without a default server", http.StatusBadRequest)
				return
			}
			upnp.IndexContainer = *req.IndexContainer
		}

		if err := s.opts.Config.SetUPnPConfig(upnp); err != nil {
			s.jsonError(w, "Failed to save settings: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":            "ok",
			"defaultServer":     upnp.DefaultServer,
			"defaultServerPath": upnp.DefaultServerPath,
			"browseContainer":   upnp.BrowseContainer,
			"indexContainer":    upnp.IndexContainer,
		})

	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUPnPServers returns available UPnP media servers.
func (s *Server) handleUPnPServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	client := s.getAirableClient(spk)
	servers, err := client.GetMediaServers()
	if err != nil {
		s.jsonError(w, "Failed to get servers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to simplified response
	result := make([]map[string]string, 0, len(servers.Rows))
	for _, server := range servers.Rows {
		result = append(result, map[string]string{
			"name": server.Title,
			"path": server.Path,
			"icon": s.proxyIconURL(server.Icon),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"servers": result,
	})
}

// handleUPnPContainers returns containers at a given path (for folder picker).
func (s *Server) handleUPnPContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		s.jsonError(w, "No active speaker", http.StatusServiceUnavailable)
		return
	}

	// Get query parameters
	serverPath := r.URL.Query().Get("server")
	containerPath := r.URL.Query().Get("path")

	if serverPath == "" {
		// Use default server from config if available
		if s.opts.Config != nil {
			upnp := s.opts.Config.GetUPnPConfig()
			serverPath = upnp.DefaultServerPath
		}
		if serverPath == "" {
			s.jsonError(w, "Server path required (use ?server=... or set default server)", http.StatusBadRequest)
			return
		}
	}

	client := s.getAirableClient(spk)

	// List containers at the path
	containers, err := kefw2.ListContainersAtPath(client, serverPath, containerPath)
	if err != nil {
		s.jsonError(w, "Failed to list containers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"path":       containerPath,
		"containers": containers,
	})
}

// handleSSE handles Server-Sent Events connections.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Create client channel
	clientChan := make(chan []byte, 10)

	// Register client
	s.sseClientsMu.Lock()
	s.sseClients[clientChan] = struct{}{}
	s.sseClientsMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		s.sseClientsMu.Lock()
		delete(s.sseClients, clientChan)
		s.sseClientsMu.Unlock()
		close(clientChan)
	}()

	// Send initial connection event with current state
	s.sendInitialState(w, flusher)

	// Heartbeat ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

// sendInitialState sends the current speaker/player state to a newly connected SSE client.
func (s *Server) sendInitialState(w http.ResponseWriter, flusher http.Flusher) {
	// Send connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// Send speaker health status
	speakerHealthData, _ := json.Marshal(map[string]any{
		"type": "speakerHealth",
		"data": map[string]any{
			"connected": s.manager.IsSpeakerConnected(),
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", speakerHealthData)
	flusher.Flush()

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		return
	}

	// Send speaker info (this is static metadata, safe even during standby)
	speakerData, _ := json.Marshal(map[string]any{
		"type": "speaker",
		"data": map[string]any{
			"ip":    spk.IPAddress,
			"name":  spk.Name,
			"model": spk.Model,
		},
	})
	fmt.Fprintf(w, "data: %s\n\n", speakerData)
	flusher.Flush()

	// If the speaker is in standby, send standby state without querying it.
	// Querying via HTTP would wake it from standby.
	if s.manager.IsInStandby() {
		sourceData, _ := json.Marshal(map[string]any{
			"type": "source",
			"data": map[string]any{"source": "standby"},
		})
		fmt.Fprintf(w, "data: %s\n\n", sourceData)
		flusher.Flush()

		powerData, _ := json.Marshal(map[string]any{
			"type": "power",
			"data": map[string]any{"status": "standby"},
		})
		fmt.Fprintf(w, "data: %s\n\n", powerData)
		flusher.Flush()
		return
	}

	ctx := context.Background()

	// Send current volume
	if volume, err := spk.GetVolume(ctx); err == nil {
		volumeData, _ := json.Marshal(map[string]any{
			"type": "volume",
			"data": map[string]any{"volume": volume},
		})
		fmt.Fprintf(w, "data: %s\n\n", volumeData)
		flusher.Flush()
	}

	// Send mute state
	if muted, err := spk.IsMuted(ctx); err == nil {
		muteData, _ := json.Marshal(map[string]any{
			"type": "mute",
			"data": map[string]any{"muted": muted},
		})
		fmt.Fprintf(w, "data: %s\n\n", muteData)
		flusher.Flush()
	}

	// Send source
	if source, err := spk.Source(ctx); err == nil {
		sourceData, _ := json.Marshal(map[string]any{
			"type": "source",
			"data": map[string]any{"source": string(source)},
		})
		fmt.Fprintf(w, "data: %s\n\n", sourceData)
		flusher.Flush()
	}

	// Send power state
	if status, err := spk.SpeakerState(ctx); err == nil {
		powerData, _ := json.Marshal(map[string]any{
			"type": "power",
			"data": map[string]any{"status": string(status)},
		})
		fmt.Fprintf(w, "data: %s\n\n", powerData)
		flusher.Flush()
	}

	// Send player data
	if playerData, err := spk.PlayerData(ctx); err == nil {
		position, _ := spk.SongProgressMS(ctx)
		playerEventData, _ := json.Marshal(map[string]any{
			"type": "player",
			"data": map[string]any{
				"state":     playerData.State,
				"title":     playerData.TrackRoles.Title,
				"artist":    playerData.TrackRoles.MediaData.MetaData.Artist,
				"album":     playerData.TrackRoles.MediaData.MetaData.Album,
				"icon":      s.proxyIconURL(playerData.TrackRoles.Icon),
				"duration":  playerData.Status.Duration,
				"position":  position,
				"audioType": playerData.MediaRoles.AudioType,
				"live":      playerData.MediaRoles.MediaData.MetaData.Live,
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", playerEventData)
		flusher.Flush()
	}
}

// handleFrontend serves the embedded frontend files.
func (s *Server) handleFrontend(w http.ResponseWriter, r *http.Request) {
	// Try to get the frontend build subdirectory
	frontendBuild, err := fs.Sub(s.opts.FrontendFS, "frontend/build")
	if err != nil {
		// In development, frontend might not be built yet
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		// Serve a simple message for development
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>kefw2ui</title></head>
<body>
<h1>kefw2ui</h1>
<p>Frontend not built. Run <code>task frontend:build</code> first.</p>
<p>Or in development, run <code>cd frontend && bun run dev</code> and access port 5173.</p>
</body>
</html>`)
		return
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(frontendBuild))

	// For SPA routing, serve index.html for non-file requests
	path := r.URL.Path
	if path == "/" {
		fileServer.ServeHTTP(w, r)
		return
	}

	// Check if file exists
	file, err := frontendBuild.Open(strings.TrimPrefix(path, "/"))
	if err != nil {
		// File not found, serve index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
		return
	}
	_ = file.Close()

	fileServer.ServeHTTP(w, r)
}

// jsonError sends a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
