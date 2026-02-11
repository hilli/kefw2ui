# kefw2ui

A responsive web UI for controlling KEF W2 speakers (LS50 Wireless II, LSX II / LSX II LT, LS60 Wireless). Single binary with embedded frontend, real-time updates, and full playback control.

## Installation

### Homebrew

```bash
brew install hilli/tap/kefw2ui
```

### Docker

```bash
docker pull ghcr.io/hilli/kefw2ui:latest
docker run -p 8080:8080 ghcr.io/hilli/kefw2ui:latest
```

Or use Docker Compose (see [Docker Compose](#docker-compose) below).

### Download Binary

Grab a pre-built binary from the [Releases](https://github.com/hilli/kefw2ui/releases) page for your platform (Linux/macOS, amd64/arm64).

## Quick Start

```bash
# Run with defaults (binds to 0.0.0.0:8080)
kefw2ui

# Custom port
kefw2ui --port 3000

# Skip mDNS and specify speaker IPs directly
kefw2ui --speaker-ips 192.168.1.10,192.168.1.11 --no-discovery

# Print version
kefw2ui --version
```

Then open http://localhost:8080 in your browser.

## Features

<details>
<summary><strong>Playback Controls</strong></summary>

- Play, pause, stop, next, previous track
- Volume control (0-100%) with fine-grained adjustment
- Mute/unmute toggle
- Seek within tracks (click or drag on progress bar, mouse and touch)
- Input source selection: WiFi, Bluetooth, TV, Optical, USB
- Shuffle mode (on/off) and repeat mode (off/one/all)
- Power on / standby
- Live stream and radio detection (adapts UI: stop instead of pause)

</details>

<details>
<summary><strong>Media Browsing</strong></summary>

- **UPnP/DLNA**: Browse media servers on your network, navigate folder hierarchies, play tracks or entire containers
- **Internet Radio**: Browse by category (favorites, local, popular, trending, HQ, new) or search by name
- **Podcasts**: Browse by category (favorites, popular, trending, history) or search by name
- **Track Search**: Fast search of your local UPnP library using a pre-built index. Supports prefix queries (`artist:Name`, `album:Name`)
- **Quick Search from Now Playing**: Click on artist or album name to search for more from that artist/album
- **Rebuild Search Index**: One-click reindex from Settings with live SSE progress (folders scanned, tracks found, current container)

</details>

<details>
<summary><strong>Queue Management</strong></summary>

- View the current play queue with track metadata and artwork
- Play any item in the queue by clicking it
- Remove individual tracks from the queue
- Reorder queue items via drag-and-drop
- Clear the entire queue
- Set shuffle and repeat modes

</details>

<details>
<summary><strong>Playlists</strong></summary>

- Create, rename, and delete playlists
- Save the current speaker queue as a playlist
- Load a playlist to the speaker queue (replace or append)
- Add and remove individual tracks
- Reorder tracks within playlists via drag-and-drop
- Playlists are stored as JSON files and shared with the CLI tool

</details>

<details>
<summary><strong>Speaker Management</strong></summary>

- Automatic speaker discovery via mDNS
- Manual speaker addition by IP address
- Multi-speaker support with quick switching (number keys 1-9 or command palette)
- Set a default speaker that persists across restarts
- View speaker details: model, firmware version, MAC address, max volume
- Speaker health monitoring with real-time connectivity status

</details>

<details>
<summary><strong>Real-time Updates (SSE)</strong></summary>

All state changes are pushed to the browser instantly via Server-Sent Events:

- Volume, mute, and source changes
- Track changes with metadata (title, artist, album, artwork)
- Playback position updates
- Power state changes
- Queue modifications
- Shuffle/repeat mode changes
- Speaker connectivity health
- Reindex progress (folders scanned, tracks found)

The SSE client handles reconnection with exponential backoff, a heartbeat watchdog, and automatic state refresh on reconnect or tab visibility change.

</details>

<details>
<summary><strong>MCP Server (Model Context Protocol)</strong></summary>

A built-in MCP server at `/api/mcp` (Streamable HTTP transport) exposes the full speaker API for AI assistants:

**Player Tools** (14): `get_player_status`, `play`, `pause`, `stop`, `next_track`, `previous_track`, `seek`, `set_volume`, `get_volume`, `mute`, `set_source`, `get_source`, `power_on`, `power_off`

**Queue Tools** (6): `get_queue`, `play_queue_item`, `remove_from_queue`, `move_queue_item`, `clear_queue`, `set_play_mode`

**Playlist Tools** (9): `list_playlists`, `get_playlist`, `create_playlist`, `update_playlist`, `delete_playlist`, `save_queue_as_playlist`, `add_tracks_to_playlist`, `remove_tracks_from_playlist`, `load_playlist`

**Browse Tools** (6): `browse_media`, `search_media`, `browse_radio`, `browse_podcasts`, `play_media_item`, `add_to_queue`

**Speaker Tools** (5): `list_speakers`, `get_active_speaker`, `set_active_speaker`, `discover_speakers`, `get_speaker_info`

**Resources**: `kefw2://speaker/status`, `kefw2://speaker/info`, `kefw2://queue`, `kefw2://playlists`, `kefw2://playlists/{id}`, `kefw2://speakers/{ip}`

**Prompts**: `speaker_assistant` - a system prompt for building a conversational KEF speaker assistant

</details>

<details>
<summary><strong>UI Features</strong></summary>

- **Dark mode** with consistent zinc-900 theme throughout
- **Hero album artwork** display with animated playing indicator
- **Command palette** (Cmd/Ctrl+K) with fuzzy search across all actions
- **PWA** (Progressive Web App) with offline support, installable on mobile and desktop
- **Media Session API** integration for OS-level media controls (media keys, lock screen controls)
- **Toast notifications** for all operations (success, error, warning, info)
- **Connection status indicator** with automatic reconnection
- **Responsive design** optimized for mobile and desktop

</details>

<details>
<summary><strong>Caching</strong></summary>

- **Image proxy cache**: Two-tier (memory + disk) cache for album art and media server images. Configurable memory cap and disk TTL. Proxies images from private network IPs so they work over Tailscale.
- **Airable content cache**: In-memory cache with 5-minute TTL for UPnP, radio, and podcast browse results. Favorites and history bypass cache for freshness.
- **Track search index**: Disk-persisted index of all UPnP tracks for fast search. Rebuildable from the Settings UI with live progress.
- **Service Worker cache**: Precaches static assets, stale-while-revalidate for resources, network-first for API calls, offline fallback.

</details>

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play / Pause |
| `←` / `→` | Previous / Next track |
| `↑` / `↓` | Volume up / down (5%) |
| `Shift + ↑` / `Shift + ↓` | Volume up / down (1%) |
| `M` | Toggle mute |
| `Cmd/Ctrl + K` | Open command palette |
| `1` - `9` | Switch to speaker by index |
| `/` | Focus media search |
| `?` | Show keyboard shortcuts help |
| `Escape` | Close modal / panel |

## Configuration

### CLI Flags and Environment Variables

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--bind` | `KEFW2UI_BIND` | `0.0.0.0` | Address to bind to |
| `--port` | - | `8080` | Port to listen on |
| `--speaker-ips` | `KEFW2UI_SPEAKER_IPS` | - | Comma-separated speaker IP addresses |
| `--no-discovery` | `KEFW2UI_NO_DISCOVERY` | `false` | Skip mDNS speaker discovery |
| `--image-cache-ttl` | `KEFW2UI_IMAGE_CACHE_TTL` | `7d` | Image cache disk TTL (`0` = never expire, e.g. `1h`, `7d`, `30d`) |
| `--image-cache-mem-mb` | `KEFW2UI_IMAGE_CACHE_MEM_MB` | `50` | Max memory for image cache in MB |
| `--tailscale` | `TS_ENABLED` | `false` | Enable Tailscale listener |
| `--tailscale-hostname` | `TS_HOSTNAME` | `kefw2ui` | Hostname on the tailnet |
| `--tailscale-authkey` | `TS_AUTHKEY` | - | Tailscale auth key for headless login |
| `--tailscale-dir` | `TS_STATE_DIR` | - | Directory for Tailscale state persistence |
| `--version` | - | - | Print version and exit |

### Config Files

Configuration is stored in the OS-specific config directory:

| OS | Config | Cache |
|----|--------|-------|
| macOS | `~/Library/Application Support/kefw2/` | `~/Library/Caches/kefw2/` |
| Linux | `~/.config/kefw2/` | `~/.cache/kefw2/` |

Files:
- `kefw2ui.yaml` - Server configuration (speakers, UPnP settings)
- `playlists/*.json` - Saved playlists (shared with CLI)

Cache contents (auto-managed):
- `images/` - Proxied album art and media server images
- `track_index.json` - UPnP track search index
- `airable_cache/` - Browse result cache

## Deployment

<details>
<summary><strong>Docker Compose</strong></summary>

Create a `compose.yaml`:

```yaml
services:
  kefw2ui:
    image: ghcr.io/hilli/kefw2ui:latest
    container_name: kefw2ui
    ports:
      - "8080:8080"
    volumes:
      # Persist config and cache across restarts
      - kefw2ui-state:/home/kefw2ui
    restart: unless-stopped
    environment:
      - TZ=${TZ:-UTC}
      # Speaker IPs (comma-separated)
      - KEFW2UI_SPEAKER_IPS=192.168.1.10,192.168.1.11
      - KEFW2UI_NO_DISCOVERY=true
      # Image cache (optional, shown with defaults)
      # - KEFW2UI_IMAGE_CACHE_TTL=7d
      # - KEFW2UI_IMAGE_CACHE_MEM_MB=50

volumes:
  kefw2ui-state:
```

**Speaker discovery**: If you want mDNS speaker discovery instead of specifying IPs, use `network_mode: host` (Linux only) and remove the `ports:` mapping. On macOS, Docker does not support `network_mode: host` properly - use `KEFW2UI_SPEAKER_IPS` instead.

**Tailscale in Docker**: Add the Tailscale environment variables and a volume for state:

```yaml
services:
  kefw2ui:
    image: ghcr.io/hilli/kefw2ui:latest
    container_name: kefw2ui
    ports:
      - "8080:8080"
    volumes:
      - kefw2ui-state:/home/kefw2ui
      - tailscale-state:/data/tailscale
    restart: unless-stopped
    environment:
      - TZ=${TZ:-UTC}
      - KEFW2UI_SPEAKER_IPS=192.168.1.10
      - KEFW2UI_NO_DISCOVERY=true
      - TS_ENABLED=true
      - TS_HOSTNAME=kefw2ui
      - TS_AUTHKEY=tskey-auth-...
      - TS_STATE_DIR=/data/tailscale

volumes:
  kefw2ui-state:
  tailscale-state:
```

Run with:

```bash
docker compose up -d
docker compose logs -f
```

</details>

<details>
<summary><strong>Systemd</strong></summary>

```bash
# Install binary
sudo cp kefw2ui /usr/local/bin/

# Create service user and directories
sudo useradd -r -m -s /bin/false kefw2ui
sudo mkdir -p /home/kefw2ui/.config/kefw2 /home/kefw2ui/.cache/kefw2
sudo chown -R kefw2ui:kefw2ui /home/kefw2ui

# Install and enable service
sudo cp kefw2ui.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now kefw2ui

# View logs
sudo journalctl -u kefw2ui -f
```

The included `kefw2ui.service` runs as a dedicated user with security hardening (NoNewPrivileges, ProtectSystem, PrivateTmp, etc.).

To use specific speaker IPs instead of mDNS discovery, override the `ExecStart` line:

```bash
sudo systemctl edit kefw2ui
```

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/kefw2ui --bind 0.0.0.0 --port 8080 --speaker-ips 192.168.1.10,192.168.1.11 --no-discovery
```

</details>

<details>
<summary><strong>Tailscale</strong></summary>

kefw2ui has built-in Tailscale support using [tsnet](https://tailscale.com/kb/1244/tsnet). When enabled, it joins your tailnet directly and serves over HTTPS with automatic TLS certificates - no separate Tailscale client needed.

```bash
# First run (interactive login - opens browser for auth)
kefw2ui --tailscale

# Headless / Docker (use an auth key)
kefw2ui --tailscale --tailscale-authkey tskey-auth-...
```

Once running, access it at `https://kefw2ui.<your-tailnet>.ts.net` from any device on your tailnet.

The local listener on port 8080 runs in parallel, so you get both local and remote access simultaneously. The image proxy automatically rewrites private network IPs so album art and media server images work over Tailscale.

</details>

## Development

### Requirements

- Go 1.25+
- Bun 1.0+ or Node.js 22+
- [Task](https://taskfile.dev)

### Running locally

```bash
# Install dependencies
task frontend:install

# Run development servers (backend + frontend with hot reload)
task dev
```

The frontend dev server runs on http://localhost:5173 with proxy to backend on :8080.

### Production Build

```bash
# Build single binary with embedded frontend
task build

# Run
./kefw2ui --port 8080
```

## License

MIT
