package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hilli/kefw2ui/config"
)

// Options configures the server
type Options struct {
	Bind       string
	Port       int
	FrontendFS embed.FS
	Config     *config.Config
}

// Server is the HTTP server for kefw2ui
type Server struct {
	opts       Options
	mux        *http.ServeMux
	httpServer *http.Server

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
	s.mux.HandleFunc("/api/speakers", s.handleSpeakers)
	s.mux.HandleFunc("/api/speaker", s.handleSpeaker)
	s.mux.HandleFunc("/api/player", s.handlePlayer)
	s.mux.HandleFunc("/api/queue", s.handleQueue)
	s.mux.HandleFunc("/api/playlists", s.handlePlaylists)
	s.mux.HandleFunc("/api/browse/", s.handleBrowse)

	// SSE endpoint
	s.mux.HandleFunc("/events", s.handleSSE)

	// Static frontend files
	s.mux.HandleFunc("/", s.handleFrontend)
}

// handleHealth is a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleSpeakers handles speaker list and discovery
func (s *Server) handleSpeakers(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement speaker discovery and listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"speakers": []any{},
	})
}

// handleSpeaker handles active speaker get/set
func (s *Server) handleSpeaker(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement active speaker management
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"active": nil,
	})
}

// handlePlayer handles playback controls
func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement player controls
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"state": "stopped",
	})
}

// handleQueue handles queue operations
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement queue management
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tracks": []any{},
	})
}

// handlePlaylists handles playlist operations
func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement playlist management
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"playlists": []any{},
	})
}

// handleBrowse handles content browsing (UPnP, Radio, Podcasts)
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement content browsing
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

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

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

// BroadcastEvent sends an event to all connected SSE clients
func (s *Server) BroadcastEvent(eventType string, data any) {
	payload, err := json.Marshal(map[string]any{
		"type": eventType,
		"data": data,
	})
	if err != nil {
		log.Printf("Error marshaling SSE event: %v", err)
		return
	}

	s.sseClientsMu.RLock()
	defer s.sseClientsMu.RUnlock()

	for clientChan := range s.sseClients {
		select {
		case clientChan <- payload:
		default:
			// Client buffer full, skip
		}
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
