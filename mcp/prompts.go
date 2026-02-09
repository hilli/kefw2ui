package mcp

import (
	"context"
	"fmt"

	mcppkg "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (h *Handler) registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcppkg.NewPrompt("speaker_assistant",
		mcppkg.WithPromptDescription("System prompt for a KEF speaker assistant that can control speakers conversationally"),
		mcppkg.WithArgument("speaker_name",
			mcppkg.ArgumentDescription("Name of the speaker to mention in the prompt (optional)"),
		),
	), h.handleSpeakerAssistantPrompt)
}

func (h *Handler) handleSpeakerAssistantPrompt(_ context.Context, req mcppkg.GetPromptRequest) (*mcppkg.GetPromptResult, error) {
	speakerName := ""
	if req.Params.Arguments != nil {
		if name, ok := req.Params.Arguments["speaker_name"]; ok {
			speakerName = name
		}
	}

	speakerRef := "the KEF speaker"
	if speakerName != "" {
		speakerRef = fmt.Sprintf("the KEF speaker '%s'", speakerName)
	}

	systemPrompt := fmt.Sprintf(`You are a helpful assistant that controls %s. You have access to MCP tools for:

**Playback Control**: play, pause, stop, next/previous track, seek, volume, mute, source selection, power on/off
**Queue Management**: view queue, play/remove/reorder items, clear queue, set shuffle/repeat
**Playlist Management**: list/create/update/delete playlists, save queue as playlist, load playlist
**Media Browsing**: browse UPnP media servers, search local library, browse internet radio and podcasts
**Speaker Management**: list/discover speakers, switch active speaker, get speaker info

Guidelines:
- When the user asks to play something, first check if it's in the queue. If not, try browsing or searching for it.
- Always confirm destructive actions (clearing queue, deleting playlists) before executing.
- When listing tracks or queue items, format them in a readable way with track number, title, and artist.
- If the speaker is in standby, power it on before trying to play anything.
- Report volume changes clearly (e.g., "Volume set to 45%%").
- When browsing media, present results concisely and offer to play or queue items.`, speakerRef)

	return mcppkg.NewGetPromptResult(
		"System prompt for KEF speaker assistant",
		[]mcppkg.PromptMessage{
			mcppkg.NewPromptMessage(mcppkg.RoleAssistant, mcppkg.NewTextContent(systemPrompt)),
		},
	), nil
}
