package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hilli/go-kef-w2/kefw2"
	"github.com/hilli/kefw2ui/config"
	"github.com/hilli/kefw2ui/speaker"
)

// Options configures the server
type Options struct {
	Bind           string
	Port           int
	FrontendFS     embed.FS
	Config         *config.Config
	SpeakerManager *speaker.Manager
}

// Server is the HTTP server for kefw2ui
type Server struct {
	opts       Options
	mux        *http.ServeMux
	httpServer *http.Server
	manager    *speaker.Manager

	// SSE clients
	sseClients   map[chan []byte]struct{}
	sseClientsMu sync.RWMutex
}

// New creates a new server instance
func New(opts Options) *Server {
	s := &Server{
		opts:       opts,
		mux:        http.NewServeMux(),
		sseClients: make(map[chan []byte]struct{}),
		manager:    opts.SpeakerManager,
	}

	s.registerRoutes()

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", opts.Bind, opts.Port),
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// registerRoutes sets up all HTTP routes
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("/api/health", s.handleHealth)

	// Speaker management
	s.mux.HandleFunc("/api/speakers", s.handleSpeakers)
	s.mux.HandleFunc("/api/speakers/discover", s.handleSpeakersDiscover)
	s.mux.HandleFunc("/api/speakers/add", s.handleSpeakersAdd)
	s.mux.HandleFunc("/api/speaker", s.handleSpeaker)
	s.mux.HandleFunc("/api/speaker/logo", s.handleSpeakerLogo)

	// Player controls
	s.mux.HandleFunc("/api/player", s.handlePlayer)
	s.mux.HandleFunc("/api/player/play", s.handlePlayerPlay)
	s.mux.HandleFunc("/api/player/next", s.handlePlayerNext)
	s.mux.HandleFunc("/api/player/prev", s.handlePlayerPrev)
	s.mux.HandleFunc("/api/player/volume", s.handlePlayerVolume)
	s.mux.HandleFunc("/api/player/mute", s.handlePlayerMute)
	s.mux.HandleFunc("/api/player/source", s.handlePlayerSource)

	// Queue management (placeholder for Phase 2)
	s.mux.HandleFunc("/api/queue", s.handleQueue)

	// Playlists (placeholder for Phase 3)
	s.mux.HandleFunc("/api/playlists", s.handlePlaylists)

	// Content browsing (placeholder for Phase 4)
	s.mux.HandleFunc("/api/browse/", s.handleBrowse)

	// SSE endpoint
	s.mux.HandleFunc("/events", s.handleSSE)

	// Static frontend files
	s.mux.HandleFunc("/", s.handleFrontend)
}

// HandleSpeakerEvent is called by the speaker manager when events occur
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
				"icon":     e.Icon,
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

// broadcastSSE sends data to all connected SSE clients
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

// handleHealth is a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSpeakers returns the list of known speakers
func (s *Server) handleSpeakers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	speakers := s.manager.GetSpeakers()
	activeSpeaker := s.manager.GetActiveSpeaker()

	speakerList := make([]map[string]any, 0, len(speakers))
	for _, spk := range speakers {
		speakerInfo := map[string]any{
			"ip":       spk.IPAddress,
			"name":     spk.Name,
			"model":    spk.Model,
			"active":   activeSpeaker != nil && spk.IPAddress == activeSpeaker.IPAddress,
			"firmware": spk.FirmwareVersion,
		}
		speakerList = append(speakerList, speakerInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"speakers": speakerList,
	})
}

// handleSpeakersDiscover triggers mDNS discovery
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
	json.NewEncoder(w).Encode(map[string]any{
		"discovered": speakerList,
	})
}

// handleSpeakersAdd adds a speaker by IP address
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"speaker": map[string]any{
			"ip":       spk.IPAddress,
			"name":     spk.Name,
			"model":    spk.Model,
			"firmware": spk.FirmwareVersion,
		},
	})
}

// handleSpeaker gets or sets the active speaker
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
		json.NewEncoder(w).Encode(map[string]any{
			"active": nil,
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
	json.NewEncoder(w).Encode(map[string]any{
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

	// Return the new active speaker info
	s.handleSpeakerGet(w, r)
}

// handleSpeakerLogo proxies the KEF logo from the active speaker
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
	io.Copy(w, resp.Body)
}

// handlePlayer returns the current player state
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

	ctx := r.Context()

	// Get player data
	playerData, err := spk.PlayerData(ctx)
	if err != nil {
		// If we can't get player data, return basic state
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		source, _ := spk.Source(ctx)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
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
	json.NewEncoder(w).Encode(map[string]any{
		"state":    playerData.State,
		"volume":   volume,
		"muted":    muted,
		"source":   string(source),
		"title":    playerData.TrackRoles.Title,
		"artist":   playerData.TrackRoles.MediaData.MetaData.Artist,
		"album":    playerData.TrackRoles.MediaData.MetaData.Album,
		"icon":     playerData.TrackRoles.Icon,
		"duration": playerData.Status.Duration,
		"position": position,
	})
}

// handlePlayerPlay toggles play/pause
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerNext skips to next track
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerPrev goes to previous track
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
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handlePlayerVolume gets or sets volume
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
		json.NewEncoder(w).Encode(map[string]any{"volume": volume})

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
		json.NewEncoder(w).Encode(map[string]any{"volume": req.Volume})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerMute gets or sets mute state
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
		json.NewEncoder(w).Encode(map[string]any{"muted": muted})

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

			if req.Muted == nil {
				// Toggle
				muted, _ := spk.IsMuted(r.Context())
				if muted {
					spk.Unmute(r.Context())
				} else {
					spk.Mute(r.Context())
				}
			} else if *req.Muted {
				if err := spk.Mute(r.Context()); err != nil {
					s.jsonError(w, "Failed to mute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				if err := spk.Unmute(r.Context()); err != nil {
					s.jsonError(w, "Failed to unmute: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		muted, _ := spk.IsMuted(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"muted": muted})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlayerSource gets or sets the audio source
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
		json.NewEncoder(w).Encode(map[string]any{"source": string(source)})

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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"source": req.Source})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleQueue handles queue operations
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
		json.NewEncoder(w).Encode(map[string]any{
			"tracks":       []any{},
			"currentIndex": -1,
		})
		return
	}

	// Get current player data to identify current track
	playerData, playerErr := spk.PlayerData(ctx)
	currentTitle := ""
	if playerErr == nil {
		currentTitle = playerData.TrackRoles.Title
	}

	// Convert to simplified track list
	tracks := make([]map[string]any, 0, len(queueResp.Rows))
	currentIndex := -1
	for i, item := range queueResp.Rows {
		track := map[string]any{
			"index":    i,
			"title":    item.Title,
			"id":       item.ID,
			"path":     item.Path,
			"icon":     item.Icon,
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

		// Try to match current track
		if currentTitle != "" && item.Title == currentTitle {
			currentIndex = i
		}

		tracks = append(tracks, track)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tracks":       tracks,
		"currentIndex": currentIndex,
	})
}

// handlePlaylists handles playlist operations (placeholder for Phase 3)
func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"playlists": []any{},
	})
}

// handleBrowse handles content browsing (placeholder for Phase 4)
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"items": []any{},
	})
}

// handleSSE handles Server-Sent Events connections
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

// sendInitialState sends the current speaker/player state to a newly connected SSE client
func (s *Server) sendInitialState(w http.ResponseWriter, flusher http.Flusher) {
	// Send connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	spk := s.manager.GetActiveSpeaker()
	if spk == nil {
		return
	}

	ctx := context.Background()

	// Send speaker info
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

	// Send player data
	if playerData, err := spk.PlayerData(ctx); err == nil {
		position, _ := spk.SongProgressMS(ctx)
		playerEventData, _ := json.Marshal(map[string]any{
			"type": "player",
			"data": map[string]any{
				"state":    playerData.State,
				"title":    playerData.TrackRoles.Title,
				"artist":   playerData.TrackRoles.MediaData.MetaData.Artist,
				"album":    playerData.TrackRoles.MediaData.MetaData.Album,
				"icon":     playerData.TrackRoles.Icon,
				"duration": playerData.Status.Duration,
				"position": position,
			},
		})
		fmt.Fprintf(w, "data: %s\n\n", playerEventData)
		flusher.Flush()
	}
}

// handleFrontend serves the embedded frontend files
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
	file.Close()

	fileServer.ServeHTTP(w, r)
}

// jsonError sends a JSON error response
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
