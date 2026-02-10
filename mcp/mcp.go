// Package mcp provides an MCP (Model Context Protocol) server for AI assistant
// control of KEF W2 speakers. It exposes speaker controls, playlist management,
// queue operations, media browsing, and multi-speaker management as MCP tools,
// resources, and prompts.
package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/hilli/go-kef-w2/kefw2"
	"github.com/hilli/kefw2ui/playlist"
	"github.com/hilli/kefw2ui/speaker"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Handler holds the shared dependencies needed by all MCP tool/resource handlers.
type Handler struct {
	manager          *speaker.Manager
	playlists        *playlist.Manager
	airableCache     *kefw2.RowsCache
	onPlaylistChange func() // called after playlist CRUD to notify SSE clients
}

// NewMCPHandler creates a fully-configured MCP server with all tools, resources,
// and prompts registered, and returns it as an http.Handler suitable for mounting
// on an existing ServeMux. The onPlaylistChange callback is invoked after any
// playlist mutation so the caller can broadcast updates to connected clients.
func NewMCPHandler(mgr *speaker.Manager, pl *playlist.Manager, cache *kefw2.RowsCache, onPlaylistChange func()) http.Handler {
	h := &Handler{
		manager:          mgr,
		playlists:        pl,
		airableCache:     cache,
		onPlaylistChange: onPlaylistChange,
	}

	s := server.NewMCPServer("kef-speakers", "1.0.0",
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
		server.WithInstructions("MCP server for controlling KEF W2 wireless speakers (LSX II, LS50 Wireless II, LS60). "+
			"Provides tools for playback control, volume, source selection, queue management, playlist management, "+
			"media browsing (UPnP, internet radio, podcasts), and multi-speaker management."),
	)

	// Register tools
	h.registerPlayerTools(s)
	h.registerPlaylistTools(s)
	h.registerQueueTools(s)
	h.registerBrowseTools(s)
	h.registerSpeakerTools(s)

	// Register resources
	h.registerResources(s)

	// Register prompts
	h.registerPrompts(s)

	return server.NewStreamableHTTPServer(s)
}

// getCachedAirableClient returns an AirableClient with the shared disk cache.
func (h *Handler) getCachedAirableClient(spk *kefw2.KEFSpeaker) *kefw2.AirableClient {
	client := kefw2.NewAirableClient(spk)
	client.Cache = h.airableCache
	return client
}

// jsonString marshals v to a JSON string, returning "{}" on error.
func jsonString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// noSpeakerError returns a standard MCP tool error for when no speaker is connected.
func noSpeakerError() *mcppkg.CallToolResult {
	return mcppkg.NewToolResultError("No active speaker. Use list_speakers or discover_speakers to find speakers, then set_active_speaker to connect.")
}
