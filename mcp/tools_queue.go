package mcp

import (
	"context"

	"github.com/hilli/go-kef-w2/kefw2"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerQueueTools(s *server.MCPServer) {
	s.AddTool(mcppkg.NewTool("get_queue",
		mcppkg.WithDescription("Get the current play queue with track info and current playing index"),
	), h.handleGetQueue)

	s.AddTool(mcppkg.NewTool("play_queue_item",
		mcppkg.WithDescription("Play a specific item in the queue by index"),
		mcppkg.WithNumber("index",
			mcppkg.Required(),
			mcppkg.Description("Zero-based index of the queue item to play"),
		),
	), h.handlePlayQueueItem)

	s.AddTool(mcppkg.NewTool("remove_from_queue",
		mcppkg.WithDescription("Remove an item from the queue by index"),
		mcppkg.WithNumber("index",
			mcppkg.Required(),
			mcppkg.Description("Zero-based index of the queue item to remove"),
		),
	), h.handleRemoveFromQueue)

	s.AddTool(mcppkg.NewTool("move_queue_item",
		mcppkg.WithDescription("Move a queue item from one position to another"),
		mcppkg.WithNumber("from",
			mcppkg.Required(),
			mcppkg.Description("Current zero-based index of the item"),
		),
		mcppkg.WithNumber("to",
			mcppkg.Required(),
			mcppkg.Description("Target zero-based index to move the item to"),
		),
	), h.handleMoveQueueItem)

	s.AddTool(mcppkg.NewTool("clear_queue",
		mcppkg.WithDescription("Clear the entire play queue"),
	), h.handleClearQueue)

	s.AddTool(mcppkg.NewTool("set_play_mode",
		mcppkg.WithDescription("Set shuffle and/or repeat mode"),
		mcppkg.WithBoolean("shuffle",
			mcppkg.Description("Enable or disable shuffle"),
		),
		mcppkg.WithString("repeat",
			mcppkg.Description("Repeat mode"),
			mcppkg.Enum("off", "one", "all"),
		),
	), h.handleSetPlayMode)
}

func (h *Handler) handleGetQueue(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		return mcppkg.NewToolResultText(jsonString(map[string]any{
			"tracks":       []any{},
			"currentIndex": -1,
		})), nil
	}

	playerData, playerErr := spk.PlayerData(ctx)
	nowPlayingPath := ""
	nowPlayingTitle := ""
	if playerErr == nil {
		nowPlayingPath = playerData.TrackRoles.Path
		nowPlayingTitle = playerData.TrackRoles.Title
	}

	currentIndex := -1
	tracks := make([]map[string]any, 0, len(queueResp.Rows))
	for i, item := range queueResp.Rows {
		track := map[string]any{
			"index":    i,
			"title":    item.Title,
			"id":       item.ID,
			"path":     item.Path,
			"type":     item.Type,
			"duration": 0,
		}

		if item.MediaData != nil {
			track["artist"] = item.MediaData.MetaData.Artist
			track["album"] = item.MediaData.MetaData.Album
			if len(item.MediaData.Resources) > 0 {
				track["duration"] = item.MediaData.Resources[0].Duration
			}
		}

		if currentIndex == -1 && nowPlayingPath != "" && item.Path == nowPlayingPath {
			currentIndex = i
		}

		tracks = append(tracks, track)
	}

	// Fallback: match by title if path didn't match
	if currentIndex == -1 && nowPlayingTitle != "" {
		for i, item := range queueResp.Rows {
			if item.Title == nowPlayingTitle {
				currentIndex = i
				break
			}
		}
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"tracks":       tracks,
		"currentIndex": currentIndex,
	})), nil
}

func (h *Handler) handlePlayQueueItem(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	index, err := req.RequireInt("index")
	if err != nil {
		return mcppkg.NewToolResultError("index is required"), nil
	}

	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		return mcppkg.NewToolResultError("Failed to get queue: " + err.Error()), nil
	}

	if index < 0 || index >= len(queueResp.Rows) {
		return mcppkg.NewToolResultError("Index out of range"), nil
	}

	track := queueResp.Rows[index]
	if err := airable.PlayQueueIndex(index, &track); err != nil {
		return mcppkg.NewToolResultError("Failed to play track: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleRemoveFromQueue(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	index, err := req.RequireInt("index")
	if err != nil {
		return mcppkg.NewToolResultError("index is required"), nil
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.RemoveFromQueue([]int{index}); err != nil {
		return mcppkg.NewToolResultError("Failed to remove from queue: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleMoveQueueItem(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	from, err := req.RequireInt("from")
	if err != nil {
		return mcppkg.NewToolResultError("from is required"), nil
	}

	to, err := req.RequireInt("to")
	if err != nil {
		return mcppkg.NewToolResultError("to is required"), nil
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.MoveQueueItem(from, to); err != nil {
		return mcppkg.NewToolResultError("Failed to move queue item: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleClearQueue(_ context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := kefw2.NewAirableClient(spk)
	if err := airable.ClearPlaylist(); err != nil {
		return mcppkg.NewToolResultError("Failed to clear queue: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleSetPlayMode(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := kefw2.NewAirableClient(spk)

	args := req.GetArguments()

	if _, ok := args["shuffle"]; ok {
		shuffle := req.GetBool("shuffle", false)
		if err := airable.SetShuffle(shuffle); err != nil {
			return mcppkg.NewToolResultError("Failed to set shuffle: " + err.Error()), nil
		}
	}

	if _, ok := args["repeat"]; ok {
		repeat := req.GetString("repeat", "off")
		if err := airable.SetRepeat(repeat); err != nil {
			return mcppkg.NewToolResultError("Failed to set repeat: " + err.Error()), nil
		}
	}

	mode, _ := airable.GetPlayMode()
	shuffle, _ := airable.IsShuffleEnabled()
	repeat, _ := airable.GetRepeatMode()

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"mode":    mode,
		"shuffle": shuffle,
		"repeat":  repeat,
	})), nil
}
