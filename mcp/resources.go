package mcp

import (
	"context"
	"strings"

	"github.com/hilli/go-kef-w2/kefw2"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerResources(s *server.MCPServer) {
	// Static resources
	s.AddResource(mcppkg.NewResource(
		"kefw2://speaker/status",
		"Speaker Status",
		mcppkg.WithResourceDescription("Current playback state including track, volume, source, and position"),
		mcppkg.WithMIMEType("application/json"),
	), h.handleResourceSpeakerStatus)

	s.AddResource(mcppkg.NewResource(
		"kefw2://speaker/info",
		"Speaker Info",
		mcppkg.WithResourceDescription("Active speaker details including model, firmware, and MAC address"),
		mcppkg.WithMIMEType("application/json"),
	), h.handleResourceSpeakerInfo)

	s.AddResource(mcppkg.NewResource(
		"kefw2://queue",
		"Play Queue",
		mcppkg.WithResourceDescription("Current play queue with all tracks"),
		mcppkg.WithMIMEType("application/json"),
	), h.handleResourceQueue)

	s.AddResource(mcppkg.NewResource(
		"kefw2://playlists",
		"Playlists",
		mcppkg.WithResourceDescription("All saved playlists with metadata"),
		mcppkg.WithMIMEType("application/json"),
	), h.handleResourcePlaylists)

	// Resource templates
	s.AddResourceTemplate(mcppkg.NewResourceTemplate(
		"kefw2://playlists/{id}",
		"Playlist",
		mcppkg.WithTemplateDescription("A specific playlist with its tracks"),
		mcppkg.WithTemplateMIMEType("application/json"),
	), h.handleResourcePlaylist)

	s.AddResourceTemplate(mcppkg.NewResourceTemplate(
		"kefw2://speakers/{ip}",
		"Speaker",
		mcppkg.WithTemplateDescription("Details for a specific speaker by IP address"),
		mcppkg.WithTemplateMIMEType("application/json"),
	), h.handleResourceSpeaker)
}

func (h *Handler) handleResourceSpeakerStatus(ctx context.Context, _ mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://speaker/status",
				MIMEType: "application/json",
				Text:     `{"error":"No active speaker"}`,
			},
		}, nil
	}

	var data map[string]any

	if h.manager.IsInStandby() {
		data = map[string]any{
			"state":  "stopped",
			"source": "standby",
		}
	} else {
		playerData, err := spk.PlayerData(ctx)
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		source, _ := spk.Source(ctx)

		if err != nil {
			data = map[string]any{
				"state":  "stopped",
				"volume": volume,
				"muted":  muted,
				"source": string(source),
			}
		} else {
			position, _ := spk.SongProgressMS(ctx)
			data = map[string]any{
				"state":    playerData.State,
				"volume":   volume,
				"muted":    muted,
				"source":   string(source),
				"title":    playerData.TrackRoles.Title,
				"artist":   playerData.TrackRoles.MediaData.MetaData.Artist,
				"album":    playerData.TrackRoles.MediaData.MetaData.Album,
				"duration": playerData.Status.Duration,
				"position": position,
			}
		}
	}

	return []mcppkg.ResourceContents{
		mcppkg.TextResourceContents{
			URI:      "kefw2://speaker/status",
			MIMEType: "application/json",
			Text:     jsonString(data),
		},
	}, nil
}

func (h *Handler) handleResourceSpeakerInfo(ctx context.Context, _ mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://speaker/info",
				MIMEType: "application/json",
				Text:     `{"error":"No active speaker"}`,
			},
		}, nil
	}

	info := map[string]any{
		"ip":         spk.IPAddress,
		"name":       spk.Name,
		"model":      spk.Model,
		"firmware":   spk.FirmwareVersion,
		"macAddress": spk.MacAddress,
		"maxVolume":  spk.MaxVolume,
		"standby":    h.manager.IsInStandby(),
	}

	if !h.manager.IsInStandby() {
		isPoweredOn, _ := spk.IsPoweredOn(ctx)
		status, _ := spk.SpeakerState(ctx)
		info["poweredOn"] = isPoweredOn
		info["status"] = string(status)
	}

	return []mcppkg.ResourceContents{
		mcppkg.TextResourceContents{
			URI:      "kefw2://speaker/info",
			MIMEType: "application/json",
			Text:     jsonString(info),
		},
	}, nil
}

func (h *Handler) handleResourceQueue(_ context.Context, _ mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://queue",
				MIMEType: "application/json",
				Text:     `{"tracks":[],"totalCount":0}`,
			},
		}, nil
	}

	airable := kefw2.NewAirableClient(spk)
	queueResp, err := airable.GetPlayQueue()
	if err != nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://queue",
				MIMEType: "application/json",
				Text:     `{"tracks":[],"totalCount":0}`,
			},
		}, nil
	}

	tracks := make([]map[string]any, 0, len(queueResp.Rows))
	for i, item := range queueResp.Rows {
		track := map[string]any{
			"index": i,
			"title": item.Title,
			"path":  item.Path,
			"type":  item.Type,
		}
		if item.MediaData != nil {
			track["artist"] = item.MediaData.MetaData.Artist
			track["album"] = item.MediaData.MetaData.Album
			if len(item.MediaData.Resources) > 0 {
				track["duration"] = item.MediaData.Resources[0].Duration
			}
		}
		tracks = append(tracks, track)
	}

	return []mcppkg.ResourceContents{
		mcppkg.TextResourceContents{
			URI:      "kefw2://queue",
			MIMEType: "application/json",
			Text: jsonString(map[string]any{
				"tracks":     tracks,
				"totalCount": len(tracks),
			}),
		},
	}, nil
}

func (h *Handler) handleResourcePlaylists(_ context.Context, _ mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	if h.playlists == nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://playlists",
				MIMEType: "application/json",
				Text:     `{"playlists":[]}`,
			},
		}, nil
	}

	playlists, err := h.playlists.List()
	if err != nil {
		return []mcppkg.ResourceContents{
			mcppkg.TextResourceContents{
				URI:      "kefw2://playlists",
				MIMEType: "application/json",
				Text:     `{"playlists":[],"error":"` + err.Error() + `"}`,
			},
		}, nil
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

	return []mcppkg.ResourceContents{
		mcppkg.TextResourceContents{
			URI:      "kefw2://playlists",
			MIMEType: "application/json",
			Text:     jsonString(map[string]any{"playlists": result}),
		},
	}, nil
}

func (h *Handler) handleResourcePlaylist(_ context.Context, req mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	if h.playlists == nil {
		return nil, nil
	}

	// Extract ID from URI: kefw2://playlists/{id}
	uri := req.Params.URI
	id := strings.TrimPrefix(uri, "kefw2://playlists/")

	pl, err := h.playlists.Get(id)
	if err != nil {
		return nil, err
	}

	return []mcppkg.ResourceContents{
		mcppkg.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     jsonString(pl),
		},
	}, nil
}

func (h *Handler) handleResourceSpeaker(ctx context.Context, req mcppkg.ReadResourceRequest) ([]mcppkg.ResourceContents, error) {
	uri := req.Params.URI
	ip := strings.TrimPrefix(uri, "kefw2://speakers/")

	speakers := h.manager.GetSpeakers()
	for _, spk := range speakers {
		if spk.IPAddress == ip {
			info := map[string]any{
				"ip":         spk.IPAddress,
				"name":       spk.Name,
				"model":      spk.Model,
				"firmware":   spk.FirmwareVersion,
				"macAddress": spk.MacAddress,
				"maxVolume":  spk.MaxVolume,
			}

			activeSpeaker := h.manager.GetActiveSpeaker()
			info["active"] = activeSpeaker != nil && spk.IPAddress == activeSpeaker.IPAddress

			return []mcppkg.ResourceContents{
				mcppkg.TextResourceContents{
					URI:      uri,
					MIMEType: "application/json",
					Text:     jsonString(info),
				},
			}, nil
		}
	}

	return nil, nil
}
