package mcp

import (
	"context"
	"strings"

	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerSpeakerTools(s *server.MCPServer) {
	s.AddTool(mcppkg.NewTool("list_speakers",
		mcppkg.WithDescription("List all known speakers with their connection status"),
	), h.handleListSpeakers)

	s.AddTool(mcppkg.NewTool("get_active_speaker",
		mcppkg.WithDescription("Get details about the currently active speaker"),
	), h.handleGetActiveSpeaker)

	s.AddTool(mcppkg.NewTool("set_active_speaker",
		mcppkg.WithDescription("Switch to a different speaker by IP address or name"),
		mcppkg.WithString("speaker_ip",
			mcppkg.Description("IP address of the speaker to activate"),
		),
		mcppkg.WithString("speaker_name",
			mcppkg.Description("Name of the speaker to activate (case-insensitive)"),
		),
	), h.handleSetActiveSpeaker)

	s.AddTool(mcppkg.NewTool("discover_speakers",
		mcppkg.WithDescription("Discover KEF speakers on the local network using mDNS (takes ~5 seconds)"),
	), h.handleDiscoverSpeakers)

	s.AddTool(mcppkg.NewTool("get_speaker_info",
		mcppkg.WithDescription("Get detailed information about the active speaker including model, firmware, and capabilities"),
	), h.handleGetSpeakerInfo)
}

func (h *Handler) handleListSpeakers(_ context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	speakers := h.manager.GetSpeakers()
	activeSpeaker := h.manager.GetActiveSpeaker()

	speakerList := make([]map[string]any, 0, len(speakers))
	for _, spk := range speakers {
		speakerList = append(speakerList, map[string]any{
			"ip":       spk.IPAddress,
			"name":     spk.Name,
			"model":    spk.Model,
			"active":   activeSpeaker != nil && spk.IPAddress == activeSpeaker.IPAddress,
			"firmware": spk.FirmwareVersion,
		})
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"speakers": speakerList,
	})), nil
}

func (h *Handler) handleGetActiveSpeaker(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
	}

	info := map[string]any{
		"ip":       spk.IPAddress,
		"name":     spk.Name,
		"model":    spk.Model,
		"firmware": spk.FirmwareVersion,
		"standby":  h.manager.IsInStandby(),
	}

	if !h.manager.IsInStandby() {
		source, _ := spk.Source(ctx)
		volume, _ := spk.GetVolume(ctx)
		muted, _ := spk.IsMuted(ctx)
		info["source"] = string(source)
		info["volume"] = volume
		info["muted"] = muted
	}

	return mcppkg.NewToolResultText(jsonString(info)), nil
}

func (h *Handler) handleSetActiveSpeaker(ctx context.Context, req mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	ip := req.GetString("speaker_ip", "")
	name := req.GetString("speaker_name", "")

	if ip == "" && name == "" {
		return mcppkg.NewToolResultError("Either speaker_ip or speaker_name is required"), nil
	}

	// If name provided, find the matching speaker
	if ip == "" && name != "" {
		speakers := h.manager.GetSpeakers()
		for _, spk := range speakers {
			if strings.EqualFold(spk.Name, name) {
				ip = spk.IPAddress
				break
			}
		}
		if ip == "" {
			return mcppkg.NewToolResultError("No speaker found with name: " + name), nil
		}
	}

	if err := h.manager.SetActiveSpeaker(ctx, ip); err != nil {
		return mcppkg.NewToolResultError("Failed to set active speaker: " + err.Error()), nil
	}

	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return mcppkg.NewToolResultError("Speaker set but not connected"), nil
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"status": "ok",
		"speaker": map[string]any{
			"ip":    spk.IPAddress,
			"name":  spk.Name,
			"model": spk.Model,
		},
	})), nil
}

func (h *Handler) handleDiscoverSpeakers(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	speakers, err := h.manager.Discover(ctx)
	if err != nil {
		return mcppkg.NewToolResultError("Discovery failed: " + err.Error()), nil
	}

	speakerList := make([]map[string]any, 0, len(speakers))
	for _, spk := range speakers {
		speakerList = append(speakerList, map[string]any{
			"ip":    spk.IPAddress,
			"name":  spk.Name,
			"model": spk.Model,
		})
	}

	return mcppkg.NewToolResultText(jsonString(map[string]any{
		"discovered": speakerList,
	})), nil
}

func (h *Handler) handleGetSpeakerInfo(ctx context.Context, _ mcppkg.CallToolRequest) (*mcppkg.CallToolResult, error) {
	spk := h.manager.GetActiveSpeaker()
	if spk == nil {
		return noSpeakerError(), nil
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
		source, _ := spk.Source(ctx)
		info["poweredOn"] = isPoweredOn
		info["status"] = string(status)
		info["source"] = string(source)
	}

	return mcppkg.NewToolResultText(jsonString(info)), nil
}
