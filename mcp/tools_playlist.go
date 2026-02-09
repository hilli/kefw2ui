package mcp

import (
	"context"
	"strings"
	"time"

	"github.com/hilli/go-kef-w2/kefw2"
	"github.com/hilli/kefw2ui/playlist"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerPlaylistTools(s *server.MCPServer) {
	s.AddTool(mcppkg.NewTool("list_playlists",
		mcppkg.WithDescription("List all saved playlists with metadata"),
	), h.handleListPlaylists)

	s.AddTool(mcppkg.NewTool("get_playlist",
		mcppkg.WithDescription("Get a playlist with its tracks"),
		mcppkg.WithString("playlist_id",
			mcppkg.Required(),
			mcppkg.Description("The playlist ID"),
		),
	), h.handleGetPlaylist)

	s.AddTool(mcppkg.NewTool("create_playlist",
		mcppkg.WithDescription("Create a new empty playlist"),
		mcppkg.WithString("name",
			mcppkg.Required(),
			mcppkg.Description("Playlist name"),
		),
		mcppkg.WithString("description",
			mcppkg.Description("Optional playlist description"),
		),
	), h.handleCreatePlaylist)

	s.AddTool(mcppkg.NewTool("update_playlist",
		mcppkg.WithDescription("Update a playlist's name or description"),
		mcppkg.WithString("playlist_id",
			mcppkg.Required(),
			mcppkg.Description("The playlist ID"),
		),
		mcppkg.WithString("name",
			mcppkg.Description("New playlist name"),
		),
		mcppkg.WithString("description",
			mcppkg.Description("New playlist description"),
		),
	), h.handleUpdatePlaylist)

	s.AddTool(mcppkg.NewTool("delete_playlist",
		mcppkg.WithDescription("Delete a playlist"),
		mcppkg.WithString("playlist_id",
			mcppkg.Required(),
			mcppkg.Description("The playlist ID to delete"),
		),
	), h.handleDeletePlaylist)

	s.AddTool(mcppkg.NewTool("save_queue_as_playlist",
		mcppkg.WithDescription("Save the current play queue as a new playlist"),
		mcppkg.WithString("name",
			mcppkg.Required(),
			mcppkg.Description("Name for the new playlist"),
		),
		mcppkg.WithString("description",
			mcppkg.Description("Optional playlist description"),
		),
	), h.handleSaveQueueAsPlaylist)

	s.AddTool(mcppkg.NewTool("load_playlist",
		mcppkg.WithDescription("Load a playlist into the speaker's play queue"),
		mcppkg.WithString("playlist_id",
			mcppkg.Required(),
			mcppkg.Description("The playlist ID to load"),
		),
		mcppkg.WithBoolean("append",
			mcppkg.Description("If true, append to existing queue instead of replacing it"),
		),
	), h.handleLoadPlaylist)
}

func (h *Handler) handleListPlaylists(_ context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	playlists, err := h.playlists.List()
	if err != nil {
		return mcppkg.NewToolResultError("Failed to list playlists: " + err.Error()), nil
	}

	result := make([]map[string]any, len(playlists))
	for i, pl := range playlists {
		count, _ := h.playlists.TrackCount(pl.ID)
		result[i] = map[string]any{
			"id":          pl.ID,
			"name":        pl.Name,
			"description": pl.Description,
			"trackCount":  count,
			"createdAt":   pl.CreatedAt,
			"updatedAt":   pl.UpdatedAt,
		}
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"playlists": result})), nil
}

func (h *Handler) handleGetPlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	id, err := req.RequireString("playlist_id")
	if err != nil {
		return mcppkg.NewToolResultError("playlist_id is required"), nil
	}

	pl, err := h.playlists.Get(id)
	if err != nil {
		return mcppkg.NewToolResultError("Playlist not found: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(pl)), nil
}

func (h *Handler) handleCreatePlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	name, err := req.RequireString("name")
	if err != nil {
		return mcppkg.NewToolResultError("name is required"), nil
	}

	description := req.GetString("description", "")

	pl, err := h.playlists.Create(name, description, nil)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to create playlist: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"playlist": pl})), nil
}

func (h *Handler) handleUpdatePlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	id, err := req.RequireString("playlist_id")
	if err != nil {
		return mcppkg.NewToolResultError("playlist_id is required"), nil
	}

	// Get existing playlist to preserve values not being updated
	existing, err := h.playlists.Get(id)
	if err != nil {
		return mcppkg.NewToolResultError("Playlist not found: " + err.Error()), nil
	}

	name := req.GetString("name", existing.Name)
	description := req.GetString("description", existing.Description)

	pl, err := h.playlists.Update(id, name, description, existing.Tracks)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to update playlist: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"playlist": pl})), nil
}

func (h *Handler) handleDeletePlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	id, err := req.RequireString("playlist_id")
	if err != nil {
		return mcppkg.NewToolResultError("playlist_id is required"), nil
	}

	if err := h.playlists.Delete(id); err != nil {
		return mcppkg.NewToolResultError("Failed to delete playlist: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleSaveQueueAsPlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	name, err := req.RequireString("name")
	if err != nil {
		return mcppkg.NewToolResultError("name is required"), nil
	}

	description := req.GetString("description", "")

	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		return mcppkg.NewToolResultError("Failed to get queue: " + err.Error()), nil
	}

	if len(queueResp.Rows) == 0 {
		return mcppkg.NewToolResultError("Queue is empty"), nil
	}

	tracks := make([]playlist.Track, 0, len(queueResp.Rows))
	for _, item := range queueResp.Rows {
		if item.Type == "container" {
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
		tracks = append(tracks, track)
	}

	pl, err := h.playlists.Create(name, description, tracks)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to create playlist: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"playlist":   pl,
		"trackCount": len(tracks),
	})), nil
}

func (h *Handler) handleLoadPlaylist(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	if h.playlists == nil {
		return mcppkg.NewToolResultError("Playlist manager not available"), nil
	}

	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	id, err := req.RequireString("playlist_id")
	if err != nil {
		return mcppkg.NewToolResultError("playlist_id is required"), nil
	}

	appendMode := req.GetBool("append", false)

	pl, err := h.playlists.Get(id)
	if err != nil {
		return mcppkg.NewToolResultError("Playlist not found: " + err.Error()), nil
	}

	if len(pl.Tracks) == 0 {
		return mcppkg.NewToolResultError("Playlist is empty"), nil
	}

	airable := kefw2.NewAirableClient(spk)

	if !appendMode {
		if err := airable.ClearPlaylist(); err != nil {
			return mcppkg.NewToolResultError("Failed to clear queue: " + err.Error()), nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	contentItems := make([]kefw2.ContentItem, 0, len(pl.Tracks))
	skipped := 0
	for _, track := range pl.Tracks {
		if track.Type == "container" || track.URI == "" {
			skipped++
			continue
		}

		serviceID := track.ServiceID
		if serviceID == "" {
			serviceID = "UPnP"
		}

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
		return mcppkg.NewToolResultError("No playable tracks in playlist"), nil
	}

	if err := airable.AddToQueue(contentItems, true); err != nil {
		return mcppkg.NewToolResultError("Failed to add tracks to queue: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"status":     "ok",
		"trackCount": len(contentItems),
		"skipped":    skipped,
	})), nil
}
