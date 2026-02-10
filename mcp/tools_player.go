package mcp

import (
	"context"

	"github.com/hilli/go-kef-w2/kefw2"
	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerPlayerTools(s *server.MCPServer) {
	s.AddTool(mcppkg.NewTool("get_player_status",
		mcppkg.WithDescription("Get current playback status including track info, volume, source, and position"),
	), h.handleGetPlayerStatus)

	s.AddTool(mcppkg.NewTool("play",
		mcppkg.WithDescription("Intelligently resume or start playback. Resumes if paused, starts from queue if stopped (respects shuffle mode), or reports if already playing."),
	), h.handlePlay)

	s.AddTool(mcppkg.NewTool("pause",
		mcppkg.WithDescription("Pause playback on the active speaker"),
	), h.handlePause)

	s.AddTool(mcppkg.NewTool("stop",
		mcppkg.WithDescription("Stop playback on the active speaker"),
	), h.handleStop)

	s.AddTool(mcppkg.NewTool("next_track",
		mcppkg.WithDescription("Skip to the next track"),
	), h.handleNextTrack)

	s.AddTool(mcppkg.NewTool("previous_track",
		mcppkg.WithDescription("Skip to the previous track"),
	), h.handlePreviousTrack)

	s.AddTool(mcppkg.NewTool("seek",
		mcppkg.WithDescription("Seek to a position in the current track"),
		mcppkg.WithNumber("position_seconds",
			mcppkg.Required(),
			mcppkg.Description("Position to seek to in seconds"),
		),
	), h.handleSeek)

	s.AddTool(mcppkg.NewTool("set_volume",
		mcppkg.WithDescription("Set the speaker volume"),
		mcppkg.WithNumber("volume",
			mcppkg.Required(),
			mcppkg.Description("Volume level (0-100)"),
			mcppkg.Min(0),
			mcppkg.Max(100),
		),
	), h.handleSetVolume)

	s.AddTool(mcppkg.NewTool("get_volume",
		mcppkg.WithDescription("Get the current speaker volume"),
	), h.handleGetVolume)

	s.AddTool(mcppkg.NewTool("mute",
		mcppkg.WithDescription("Toggle mute or set mute state. If muted is omitted, toggles the current state."),
		mcppkg.WithBoolean("muted",
			mcppkg.Description("true to mute, false to unmute. Omit to toggle."),
		),
	), h.handleMute)

	s.AddTool(mcppkg.NewTool("set_source",
		mcppkg.WithDescription("Change the speaker input source"),
		mcppkg.WithString("source",
			mcppkg.Required(),
			mcppkg.Description("Input source to switch to"),
			mcppkg.Enum("wifi", "bluetooth", "aux", "optical", "coaxial", "tv", "usb"),
		),
	), h.handleSetSource)

	s.AddTool(mcppkg.NewTool("get_source",
		mcppkg.WithDescription("Get the current input source"),
	), h.handleGetSource)

	s.AddTool(mcppkg.NewTool("power_on",
		mcppkg.WithDescription("Power on the speaker (wakes from standby)"),
	), h.handlePowerOn)

	s.AddTool(mcppkg.NewTool("power_off",
		mcppkg.WithDescription("Power off the speaker (enter standby)"),
	), h.handlePowerOff)

	s.AddTool(mcppkg.NewTool("get_power_state",
		mcppkg.WithDescription("Get the current power state of the speaker"),
	), h.handleGetPowerState)
}

func (h *Handler) handleGetPlayerStatus(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if h.manager.IsInStandby() {
		return mcppkg.NewToolResultText(jsonString(map[string]any{
			"state":    "stopped",
			"volume":   0,
			"muted":    false,
			"source":   "standby",
			"title":    "",
			"artist":   "",
			"album":    "",
			"duration": 0,
			"position": 0,
		})), nil
	}

	playerData, err := spk.PlayerData(ctx)
	if err != nil {
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		source, _ := spk.Source(ctx)
		return mcppkg.NewToolResultText(jsonString(map[string]any{
			"state":    "stopped",
			"volume":   volume,
			"muted":    muted,
			"source":   string(source),
			"title":    "",
			"artist":   "",
			"album":    "",
			"duration": 0,
			"position": 0,
		})), nil
	}

	volume, _ := spk.GetVolume(ctx)
	muted, _ := spk.IsMuted(ctx)
	source, _ := spk.Source(ctx)
	position, _ := spk.SongProgressMS(ctx)

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"state":     playerData.State,
		"volume":    volume,
		"muted":     muted,
		"source":    string(source),
		"title":     playerData.TrackRoles.Title,
		"artist":    playerData.TrackRoles.MediaData.MetaData.Artist,
		"album":     playerData.TrackRoles.MediaData.MetaData.Album,
		"duration":  playerData.Status.Duration,
		"position":  position,
		"audioType": playerData.MediaRoles.AudioType,
	})), nil
}

func (h *Handler) handlePlay(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	airable := kefw2.NewAirableClient(spk)
	result, err := airable.PlayOrResumeFromQueue(ctx)
	if err != nil {
		return mcppkg.NewToolResultError("Play failed: " + err.Error()), nil
	}

	resp := map[string]any{
		"status": "ok",
		"action": string(result.Action),
	}
	if result.Track != nil {
		resp["track"] = result.Track.Title
		resp["index"] = result.Index
		resp["shuffled"] = result.Shuffled
	}
	return mcppkg.NewToolResultText(jsonString(resp)), nil
}

func (h *Handler) handlePause(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.PlayPause(ctx); err != nil {
		return mcppkg.NewToolResultError("Pause failed: " + err.Error()), nil
	}
	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleStop(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.Stop(ctx); err != nil {
		return mcppkg.NewToolResultError("Stop failed: " + err.Error()), nil
	}
	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleNextTrack(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.NextTrack(ctx); err != nil {
		return mcppkg.NewToolResultError("Next track failed: " + err.Error()), nil
	}
	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handlePreviousTrack(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.PreviousTrack(ctx); err != nil {
		return mcppkg.NewToolResultError("Previous track failed: " + err.Error()), nil
	}
	return mcppkg.NewToolResultText(`{"status":"ok"}`), nil
}

func (h *Handler) handleSeek(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	seconds, err := req.RequireFloat("position_seconds")
	if err != nil {
		return mcppkg.NewToolResultError("position_seconds is required"), nil
	}

	if seconds < 0 {
		return mcppkg.NewToolResultError("Position must be non-negative"), nil
	}

	positionMS := int64(seconds * 1000)
	if err := spk.SeekTo(ctx, positionMS); err != nil {
		return mcppkg.NewToolResultError("Seek failed: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"status":           "ok",
		"position_seconds": seconds,
	})), nil
}

func (h *Handler) handleSetVolume(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	vol, err := req.RequireInt("volume")
	if err != nil {
		return mcppkg.NewToolResultError("volume is required"), nil
	}

	if vol < 0 || vol > 100 {
		return mcppkg.NewToolResultError("Volume must be between 0 and 100"), nil
	}

	if err := spk.SetVolume(ctx, vol); err != nil {
		return mcppkg.NewToolResultError("Failed to set volume: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"volume": vol})), nil
}

func (h *Handler) handleGetVolume(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	volume, err := spk.GetVolume(ctx)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to get volume: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"volume": volume})), nil
}

func (h *Handler) handleMute(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	// Check if muted parameter was explicitly provided
	args := req.GetArguments()
	mutedVal, hasExplicit := args["muted"]

	if hasExplicit && mutedVal != nil {
		wantMuted := req.GetBool("muted", false)
		if wantMuted {
			if err := spk.Mute(ctx); err != nil {
				return mcppkg.NewToolResultError("Failed to mute: " + err.Error()), nil
			}
		} else {
			if err := spk.Unmute(ctx); err != nil {
				return mcppkg.NewToolResultError("Failed to unmute: " + err.Error()), nil
			}
		}
	} else {
		// Toggle
		muted, _ := spk.IsMuted(ctx)
		if muted {
			if err := spk.Unmute(ctx); err != nil {
				return mcppkg.NewToolResultError("Failed to unmute: " + err.Error()), nil
			}
		} else {
			if err := spk.Mute(ctx); err != nil {
				return mcppkg.NewToolResultError("Failed to mute: " + err.Error()), nil
			}
		}
	}

	muted, _ := spk.IsMuted(ctx)
	return mcppkg.NewToolResultText(jsonString(map[string]any{"muted": muted})), nil
}

var sourceMap = map[string]kefw2.Source{
	"wifi":      kefw2.SourceWiFi,
	"bluetooth": kefw2.SourceBluetooth,
	"aux":       kefw2.SourceAux,
	"optical":   kefw2.SourceOptical,
	"coaxial":   kefw2.SourceCoaxial,
	"tv":        kefw2.SourceTV,
	"usb":       kefw2.SourceUSB,
}

func (h *Handler) handleSetSource(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	sourceName, err := req.RequireString("source")
	if err != nil {
		return mcppkg.NewToolResultError("source is required"), nil
	}

	source, ok := sourceMap[sourceName]
	if !ok {
		return mcppkg.NewToolResultError("Unknown source: " + sourceName + ". Valid sources: wifi, bluetooth, aux, optical, coaxial, tv, usb"), nil
	}

	if err := spk.SetSource(ctx, source); err != nil {
		return mcppkg.NewToolResultError("Failed to set source: " + err.Error()), nil
	}

	h.manager.NotifyWake()
	return mcppkg.NewToolResultText(jsonString(map[string]any{"source": sourceName})), nil
}

func (h *Handler) handleGetSource(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	source, err := spk.Source(ctx)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to get source: " + err.Error()), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{"source": string(source)})), nil
}

func (h *Handler) handlePowerOn(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.SetSource(ctx, kefw2.SourceWiFi); err != nil {
		return mcppkg.NewToolResultError("Failed to power on: " + err.Error()), nil
	}

	h.manager.NotifyWake()
	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"poweredOn": true,
		"status":    "powerOn",
	})), nil
}

func (h *Handler) handlePowerOff(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	if err := spk.PowerOff(ctx); err != nil {
		return mcppkg.NewToolResultError("Failed to power off: " + err.Error()), nil
	}

	h.manager.NotifyStandby()
	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"poweredOn": false,
		"status":    "standby",
	})), nil
}

func (h *Handler) handleGetPowerState(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	isPoweredOn, err := spk.IsPoweredOn(ctx)
	if err != nil {
		return mcppkg.NewToolResultError("Failed to get power state: " + err.Error()), nil
	}

	status, _ := spk.SpeakerState(ctx)
	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"poweredOn": isPoweredOn,
		"status":    string(status),
	})), nil
}
