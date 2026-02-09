package mcp

import (
	"context"

	"github.com/hilli/go-kef-w2/kefw2"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerBrowseTools(s *server.MCPServer) {
	s.AddTool(mcppkg.NewTool("browse_media",
		mcppkg.WithDescription("Browse UPnP media servers and their content. Call without path to list servers, or with a path to browse a container."),
		mcppkg.WithString("path",
			mcppkg.Description("Path to browse. Omit to list media servers."),
		),
	), h.handleBrowseMedia)

	s.AddTool(mcppkg.NewTool("search_media",
		mcppkg.WithDescription("Search the local UPnP media library. Requires a pre-built search index (kefw2 upnp index). Supports prefix queries like 'artist:Name' or 'album:Name'."),
		mcppkg.WithString("query",
			mcppkg.Required(),
			mcppkg.Description("Search query. Use 'artist:Name' or 'album:Name' for filtered searches."),
		),
	), h.handleSearchMedia)

	s.AddTool(mcppkg.NewTool("browse_radio",
		mcppkg.WithDescription("Browse internet radio stations by category or search"),
		mcppkg.WithString("category",
			mcppkg.Description("Radio category to browse"),
			mcppkg.Enum("menu", "favorites", "local", "popular", "trending", "hq", "new"),
		),
		mcppkg.WithString("query",
			mcppkg.Description("Search query for radio stations"),
		),
		mcppkg.WithString("path",
			mcppkg.Description("Direct path to browse (overrides category)"),
		),
	), h.handleBrowseRadio)

	s.AddTool(mcppkg.NewTool("browse_podcasts",
		mcppkg.WithDescription("Browse podcasts by category or search"),
		mcppkg.WithString("category",
			mcppkg.Description("Podcast category to browse"),
			mcppkg.Enum("menu", "favorites", "popular", "trending", "history"),
		),
		mcppkg.WithString("query",
			mcppkg.Description("Search query for podcasts"),
		),
		mcppkg.WithString("path",
			mcppkg.Description("Direct path to browse (overrides category)"),
		),
	), h.handleBrowsePodcasts)

	s.AddTool(mcppkg.NewTool("play_media_item",
		mcppkg.WithDescription("Play a media item from browsing results"),
		mcppkg.WithString("path",
			mcppkg.Required(),
			mcppkg.Description("The path of the item to play (from browse results)"),
		),
		mcppkg.WithString("source",
			mcppkg.Required(),
			mcppkg.Description("The source type"),
			mcppkg.Enum("upnp", "radio", "podcasts"),
		),
		mcppkg.WithString("type",
			mcppkg.Description("Item type (audio or container)"),
			mcppkg.Enum("audio", "container"),
		),
		mcppkg.WithString("title",
			mcppkg.Description("Item title"),
		),
		mcppkg.WithString("container_path",
			mcppkg.Description("Parent container path (needed for podcast episodes)"),
		),
	), h.handlePlayMediaItem)

	s.AddTool(mcppkg.NewTool("add_to_queue",
		mcppkg.WithDescription("Add a media item to the play queue without playing it"),
		mcppkg.WithString("path",
			mcppkg.Required(),
			mcppkg.Description("The path of the item to add (from browse results)"),
		),
		mcppkg.WithString("source",
			mcppkg.Required(),
			mcppkg.Description("The source type"),
			mcppkg.Enum("upnp", "radio", "podcasts"),
		),
		mcppkg.WithString("type",
			mcppkg.Description("Item type (audio or container)"),
			mcppkg.Enum("audio", "container"),
		),
		mcppkg.WithString("title",
			mcppkg.Description("Item title"),
		),
	), h.handleAddToQueue)
}

func (h *Handler) handleBrowseMedia(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := h.getCachedAirableClient(spk)
	path := req.GetString("path", "")

	var resp *kefw2.RowsResponse
	var err error

	if path == "" {
		resp, err = airable.GetMediaServers()
	} else {
		resp, err = airable.BrowseContainer(path)
	}

	if err != nil {
		return mcppkg.NewToolResultError("Failed to browse: " + err.Error()), nil
	}

	items := make([]map[string]any, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		if row.Type == "query" {
			continue
		}
		item := map[string]any{
			"title":    row.Title,
			"type":     row.Type,
			"path":     row.Path,
			"playable": row.Type == "audio" || row.ContainerPlayable,
		}
		if row.MediaData != nil {
			item["artist"] = row.MediaData.MetaData.Artist
			item["album"] = row.MediaData.MetaData.Album
			if len(row.MediaData.Resources) > 0 {
				item["duration"] = row.MediaData.Resources[0].Duration
			}
		}
		items = append(items, item)
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "upnp",
	})), nil
}

func (h *Handler) handleSearchMedia(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	query, err := req.RequireString("query")
	if err != nil {
		return mcppkg.NewToolResultError("query is required"), nil
	}

	index, loadErr := kefw2.LoadTrackIndexCached()
	if loadErr != nil || index == nil {
		return mcppkg.NewToolResultError("No media index found. Use 'kefw2 upnp index' to build the search index."), nil
	}

	results := kefw2.SearchTracks(index, query, 100)
	if len(results) == 0 {
		return mcppkg.NewToolResultText(jsonString(map[string]any{
			"items":      []any{},
			"totalCount": 0,
			"message":    "No results found",
		})), nil
	}

	items := make([]map[string]any, 0, len(results))
	for _, track := range results {
		items = append(items, map[string]any{
			"title":    track.Title,
			"artist":   track.Artist,
			"album":    track.Album,
			"path":     track.Path,
			"duration": track.Duration,
			"type":     "audio",
			"playable": true,
		})
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"items":      items,
		"totalCount": len(items),
		"source":     "upnp",
		"search":     true,
	})), nil
}

func (h *Handler) handleBrowseRadio(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := h.getCachedAirableClient(spk)
	query := req.GetString("query", "")
	path := req.GetString("path", "")
	category := req.GetString("category", "menu")

	var resp *kefw2.RowsResponse
	var err error

	switch {
	case query != "":
		resp, err = airable.SearchRadio(query)
	case path != "":
		resp, err = airable.BrowseRadioByItemPath(path)
	default:
		switch category {
		case "favorites":
			// Don't cache favorites
			uncached := kefw2.NewAirableClient(spk)
			resp, err = uncached.GetRadioFavorites()
		case "local":
			resp, err = airable.GetRadioLocal()
		case "popular":
			resp, err = airable.GetRadioPopular()
		case "trending":
			resp, err = airable.GetRadioTrending()
		case "hq":
			resp, err = airable.GetRadioHQ()
		case "new":
			resp, err = airable.GetRadioNew()
		default:
			resp, err = airable.GetRadioMenu()
		}
	}

	if err != nil {
		return mcppkg.NewToolResultError("Failed to browse radio: " + err.Error()), nil
	}

	items := make([]map[string]any, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		item := map[string]any{
			"title":    row.Title,
			"type":     row.Type,
			"path":     row.Path,
			"playable": row.Type == "audio" || row.ContainerPlayable || row.AudioType == "audioBroadcast",
		}
		if row.LongDescription != "" {
			item["description"] = row.LongDescription
		}
		items = append(items, item)
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "radio",
	})), nil
}

func (h *Handler) handleBrowsePodcasts(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := h.getCachedAirableClient(spk)
	query := req.GetString("query", "")
	path := req.GetString("path", "")
	category := req.GetString("category", "menu")

	var resp *kefw2.RowsResponse
	var err error

	switch {
	case query != "":
		resp, err = airable.SearchPodcasts(query)
	case path != "":
		// Don't cache favorites/history
		resp, err = airable.GetRows(path, 0, 100)
	default:
		switch category {
		case "favorites":
			uncached := kefw2.NewAirableClient(spk)
			resp, err = uncached.GetPodcastFavorites()
		case "popular":
			resp, err = airable.GetPodcastPopular()
		case "trending":
			resp, err = airable.GetPodcastTrending()
		case "history":
			resp, err = airable.GetPodcastHistory()
		default:
			resp, err = airable.GetPodcastMenu()
		}
	}

	if err != nil {
		return mcppkg.NewToolResultError("Failed to browse podcasts: " + err.Error()), nil
	}

	items := make([]map[string]any, 0, len(resp.Rows))
	for _, row := range resp.Rows {
		item := map[string]any{
			"title":    row.Title,
			"type":     row.Type,
			"path":     row.Path,
			"playable": row.Type == "audio",
		}
		if row.LongDescription != "" {
			item["description"] = row.LongDescription
		}
		if row.MediaData != nil && len(row.MediaData.Resources) > 0 {
			item["duration"] = row.MediaData.Resources[0].Duration
		}
		items = append(items, item)
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"items":      items,
		"totalCount": resp.RowsCount,
		"source":     "podcasts",
	})), nil
}

func (h *Handler) handlePlayMediaItem(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcppkg.NewToolResultError("path is required"), nil
	}

	source, err := req.RequireString("source")
	if err != nil {
		return mcppkg.NewToolResultError("source is required"), nil
	}

	itemType := req.GetString("type", "audio")
	airable := kefw2.NewAirableClient(spk)

	switch source {
	case "upnp":
		if itemType == "container" {
			err = airable.PlayUPnPContainer(path)
		} else {
			err = airable.PlayUPnPByPath(path)
		}
	case "radio":
		station, getErr := airable.GetRadioStationDetails(path)
		if getErr != nil {
			return mcppkg.NewToolResultError("Failed to get station details: " + getErr.Error()), nil
		}
		err = airable.ResolveAndPlayRadioStation(station)
	case "podcasts":
		title := req.GetString("title", "")
		containerPath := req.GetString("container_path", "")
		episode := &kefw2.ContentItem{
			Path:  path,
			Title: title,
			Type:  itemType,
		}
		if containerPath != "" {
			episode.Context = &kefw2.Context{
				Path: containerPath,
			}
		}
		err = airable.PlayPodcastEpisode(episode)
	default:
		return mcppkg.NewToolResultError("Unknown source: " + source), nil
	}

	if err != nil {
		return mcppkg.NewToolResultError("Failed to play: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleAddToQueue(_ context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcppkg.NewToolResultError("path is required"), nil
	}

	source, err := req.RequireString("source")
	if err != nil {
		return mcppkg.NewToolResultError("source is required"), nil
	}

	itemType := req.GetString("type", "audio")
	title := req.GetString("title", "")
	airable := kefw2.NewAirableClient(spk)
	tracksAdded := 0

	switch source {
	case "upnp":
		if itemType == "container" {
			tracks, getErr := airable.GetContainerTracksRecursive(path)
			if getErr != nil {
				return mcppkg.NewToolResultError("Failed to get container tracks: " + getErr.Error()), nil
			}
			if len(tracks) == 0 {
				return mcppkg.NewToolResultError("No tracks found in container"), nil
			}
			err = airable.AddToQueue(tracks, false)
			tracksAdded = len(tracks)
		} else {
			resp, getErr := airable.GetRows(path, 0, 1)
			if getErr != nil {
				return mcppkg.NewToolResultError("Failed to get track details: " + getErr.Error()), nil
			}
			var track *kefw2.ContentItem
			switch {
			case resp.Roles != nil:
				track = resp.Roles
			case len(resp.Rows) > 0:
				track = &resp.Rows[0]
			default:
				return mcppkg.NewToolResultError("Track not found"), nil
			}
			err = airable.AddToQueue([]kefw2.ContentItem{*track}, false)
			tracksAdded = 1
		}
	case "radio":
		station, getErr := airable.GetRadioStationDetails(path)
		if getErr != nil {
			station = &kefw2.ContentItem{
				Title: title,
				Type:  "audio",
				Path:  path,
			}
		}
		err = airable.AddToQueue([]kefw2.ContentItem{*station}, false)
		tracksAdded = 1
	case "podcasts":
		episode, getErr := airable.GetPodcastDetails(path)
		if getErr != nil {
			episode = &kefw2.ContentItem{
				Title: title,
				Type:  "audio",
				Path:  path,
			}
		}
		err = airable.AddToQueue([]kefw2.ContentItem{*episode}, false)
		tracksAdded = 1
	default:
		return mcppkg.NewToolResultError("Unknown source: " + source), nil
	}

	if err != nil {
		return mcppkg.NewToolResultError("Failed to add to queue: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"status":      "ok",
		"tracksAdded": tracksAdded,
	})), nil
}
