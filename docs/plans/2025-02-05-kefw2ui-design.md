# kefw2ui Design Document

**Date:** 2025-02-05  
**Status:** Approved

## Overview

kefw2ui is a web-based dashboard for controlling KEF W2 speakers (LSX II, LS50 Wireless II, LS60). It provides a slick, responsive UI with real-time updates, keyboard shortcuts, cover art display, playlist management, and content browsing across multiple services.

### Goals

- Always-on dashboard optimized for desktop, responsive for tablet/phone
- Real-time updates via Server-Sent Events (SSE)
- Full keyboard navigation with command palette (Cmd+K)
- Dark mode with hero artwork display
- Browse UPnP, Radio, and Podcasts; passthrough for AirPlay/Bluetooth
- Queue management with saved playlists (shared with CLI)
- PWA with Media Session API for OS-level media controls
- Single binary deployment with Docker and Systemd support

### Non-Goals

- Multi-user accounts or authentication (Tailscale handles remote access)
- Cross-service smart recommendations (use service-provided suggestions only)
- In-UI EQ adjustments (use CLI for that)
- Listening history or analytics

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         User Devices                            │
│   (Desktop browser, tablet, phone - via LAN or Tailscale)       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     kefw2ui Backend (Go)                        │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│  │ HTTP API     │  │ SSE endpoint │  │ Static file server    │  │
│  │ /api/*       │  │ /events      │  │ (SvelteKit build)     │  │
│  └──────────────┘  └──────────────┘  └───────────────────────┘  │
│                              │                                   │
│                    ┌─────────▼─────────┐                        │
│                    │ Speaker Manager   │                        │
│                    │ - Active speaker  │                        │
│                    │ - Event bridge    │                        │
│                    │ - Config (shared) │                        │
│                    └─────────┬─────────┘                        │
└──────────────────────────────┼──────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                     KEF Speakers (LAN)                          │
│         LSX II  ←──mDNS──→  LS50 II  ←──mDNS──→  LS60           │
└─────────────────────────────────────────────────────────────────┘
```

### Key Decisions

- **Single binary** - Backend serves both API and frontend assets (embedded)
- **Shared config** - Uses `os.UserConfigDir()/kefw2/` (same location as CLI)
- **One active speaker** - Backend maintains connection to one speaker at a time
- **SSE fan-out** - Backend subscribes to speaker events, broadcasts to all connected clients
- **Local + Tailscale** - No authentication; network access is the auth layer

---

## Backend (Go)

### API Structure

```
GET  /api/speakers              # List discovered + configured speakers
POST /api/speakers/discover     # Trigger mDNS discovery
POST /api/speakers/add          # Manually add speaker by IP
GET  /api/speaker               # Get active speaker info
POST /api/speaker               # Set active speaker (by IP or name)

GET  /api/player                # Current playback state
POST /api/player/volume         # Set volume { "volume": 50 }
POST /api/player/mute           # Toggle or set mute
POST /api/player/play           # Play/pause
POST /api/player/next           # Next track
POST /api/player/prev           # Previous track
POST /api/player/source         # Set source { "source": "wifi" }

GET  /api/queue                 # Current queue
POST /api/queue/add             # Add tracks
POST /api/queue/remove          # Remove track by index
POST /api/queue/reorder         # Reorder { "from": 2, "to": 0 }
POST /api/queue/clear           # Clear queue
POST /api/queue/mode            # Set shuffle/repeat mode

GET  /api/playlists             # List saved playlists
POST /api/playlists             # Save current queue as playlist
DELETE /api/playlists/:name     # Delete playlist
POST /api/playlists/:name/load  # Load playlist into queue

GET  /api/browse/radio/*        # Radio browsing
GET  /api/browse/podcast/*      # Podcast browsing
GET  /api/browse/upnp/*         # UPnP server browsing
POST /api/browse/*/play         # Play item from any browser

GET  /events                    # SSE endpoint - real-time updates
```

### Configuration

Location: `os.UserConfigDir()/kefw2/`

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/kefw2/` |
| Linux | `~/.config/kefw2/` (respects `XDG_CONFIG_HOME`) |
| Windows | `%AppData%/kefw2/` |

Files:
- `kefw2ui.yaml` - Server config
- `playlists/*.json` - Saved playlists (shared with CLI)

---

## Frontend (SvelteKit)

### Technology Stack

- **Framework:** SvelteKit (static SPA adapter)
- **UI Components:** shadcn-svelte (Tailwind + accessible primitives)
- **Package Manager:** Bun
- **Styling:** Tailwind CSS, dark mode only
- **State:** Svelte stores, reactive to SSE events

### Project Structure

```
frontend/
├── src/
│   ├── lib/
│   │   ├── api/              # Backend API client
│   │   │   ├── client.ts     # Fetch wrapper, error handling
│   │   │   ├── player.ts     # Player endpoints
│   │   │   ├── browse.ts     # Content browsing
│   │   │   └── sse.ts        # SSE connection manager
│   │   ├── stores/           # Svelte stores (reactive state)
│   │   │   ├── player.ts     # Volume, track, playback state
│   │   │   ├── queue.ts      # Current queue
│   │   │   ├── speakers.ts   # Speaker list, active speaker
│   │   │   └── ui.ts         # Modals, command palette, panels
│   │   ├── components/
│   │   │   ├── NowPlaying/   # Hero artwork, track info, progress
│   │   │   ├── Controls/     # Play/pause, prev/next, volume
│   │   │   ├── Queue/        # Queue list with drag-drop
│   │   │   ├── Browser/      # Content browser (UPnP, Radio, Podcast)
│   │   │   ├── CommandPalette/
│   │   │   ├── SpeakerSwitcher/
│   │   │   └── common/       # Buttons, icons, modals
│   │   └── actions/          # Svelte actions (keyboard, drag-drop)
│   ├── routes/
│   │   ├── +layout.svelte    # Main layout, SSE setup, keyboard handler
│   │   └── +page.svelte      # Single-page dashboard
│   └── app.css               # Dark theme, CSS variables
├── static/
├── package.json
├── bun.lockb
└── svelte.config.js
```

---

## Real-time Updates (SSE)

### Event Flow

```
┌─────────────┐     Event Client      ┌─────────────┐       SSE        ┌─────────────┐
│ KEF Speaker │ ──── (polling) ─────▶ │   Backend   │ ─── (fan-out) ──▶│   Browser   │
└─────────────┘                       └─────────────┘                  └─────────────┘
```

### Event Types

| Event | Data | UI Update |
|-------|------|-----------|
| `volume` | `{ volume: number }` | Volume slider, display |
| `mute` | `{ muted: boolean }` | Mute button state |
| `player` | `{ title, artist, album, icon, state, duration, position }` | Now playing, progress, artwork |
| `source` | `{ source: string }` | Source indicator |
| `queue` | `{ tracks: [...], version: number }` | Queue list |
| `speaker` | `{ name, status }` | Connection status |
| `playMode` | `{ shuffle, repeat }` | Mode toggles |

### Connection Management

- Auto-reconnect with exponential backoff
- Heartbeat ping every 30 seconds
- Connection status indicator in UI (green/yellow/red dot)

---

## UI Layout

### Desktop (≥1024px)

```
┌──────────────────────────────────────────────────────────────────────────┐
│  ┌─ Speaker ─┐                                        ┌─ Source ─┐  🔌  │
│  │ Living Rm ▼│                                       │  WiFi   ▼│      │
└──────────────────────────────────────────────────────────────────────────┘
│                                                                          │
│   ┌─────────────────────────────────┐    ┌────────────────────────────┐  │
│   │                                 │    │  QUEUE                  ≡  │  │
│   │         ┌───────────────┐       │    │───────────────────────────│  │
│   │         │               │       │    │  1. Current Track     ►   │  │
│   │         │  Album Art    │       │    │  2. Next Song             │  │
│   │         │   (Hero)      │       │    │  3. Another Track         │  │
│   │         │               │       │    │  ...                      │  │
│   │         └───────────────┘       │    │───────────────────────────│  │
│   │                                 │    │  ↻ Repeat  ⤮ Shuffle     │  │
│   │      Song Title                 │    │  💾 Save   📂 Load       │  │
│   │      Artist - Album             │    └────────────────────────────┘  │
│   │                                 │                                    │
│   │   ○━━━━━━━━━━━●━━━━━━━━━━━○     │    ┌────────────────────────────┐  │
│   │   1:42              3:56        │    │  BROWSE              ▼    │  │
│   │                                 │    │  UPnP │ Radio │ Podcasts  │  │
│   │      ◄◄    ▶︎    ►►             │    │───────────────────────────│  │
│   │                                 │    │  📁 Music Library         │  │
│   │    🔊 ━━━━━━━━━━●━━━━━━━        │    │  📁 Playlists             │  │
│   │                                 │    │  📁 Recently Added        │  │
│   └─────────────────────────────────┘    └────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
```

### Responsive Behavior

- **Tablet (768-1023px):** Stack queue below now-playing, browser in slide-out panel
- **Mobile (<768px):** Full-screen now-playing, swipe to queue, bottom nav for browse

---

## Keyboard Navigation

### Global Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `←` / `→` | Previous / Next track |
| `↑` / `↓` | Volume up / down (5% steps) |
| `Shift + ↑/↓` | Volume fine control (1%) |
| `M` | Toggle mute |
| `Cmd/Ctrl + K` | Open command palette |
| `Escape` | Close modal / go back |
| `S` | Open speaker switcher |
| `1-9` | Quick switch to speaker 1-9 |
| `/` | Focus search |

### Panel Navigation

| Key | Action |
|-----|--------|
| `Q` | Focus queue panel |
| `B` | Focus browse panel |
| `N` | Focus now playing |
| `J` / `K` | Navigate list items (vim-style) |
| `Enter` | Play / Select item |
| `A` | Add to queue (in browse) |

### Command Palette

- Search across playlists, favorites, recent, queue
- Quick actions: "volume 50", "mute", "shuffle on"
- Speaker switching: "switch to bedroom"
- Source selection: "source bluetooth"

### Customization

Shortcuts stored in localStorage, editable via Settings modal.

---

## Content Browsing

### UPnP Browser (Primary)

- Server selector (if multiple UPnP servers)
- Hierarchical folder navigation with breadcrumbs
- Play folder or individual tracks
- Add to queue (single or bulk)
- Leverages CLI's local index for fast search

### Radio Browser

- Sections: Favorites, Local, Popular, Trending, Browse by genre/country
- Search with results
- One-click play, add to favorites

### Podcasts Browser

- Sections: Favorites, Popular, Trending, History
- Show → Episode list → Play
- Add show to favorites

### Recommendations

Service-provided only (no custom logic):
- Trending Radio stations
- Popular Podcasts
- Recently Added (UPnP)

---

## Queue & Playlists

### Queue Features

- View current queue with track info
- Drag-to-reorder tracks
- Remove individual tracks
- Clear queue (with confirmation)
- Jump to track (click to play)
- Repeat modes: Off → All → One
- Shuffle toggle

### Playlist Management

- Save current queue as named playlist
- Load saved playlist (replace or append)
- Delete playlists
- Rename playlists
- Storage: `{UserConfigDir}/kefw2/playlists/*.json`
- Shared with CLI

---

## PWA Features

### Manifest

- Name: "KEF Controller"
- Display: standalone
- Theme: dark (#0a0a0a)
- Icons: 192px and 512px

### Service Worker

- Cache app shell for instant loading
- Offline graceful degradation
- Auto-updates in background
- Notification permission ready (opt-in)

### Media Session API

- OS-level now-playing controls
- Laptop media keys (F7/F8/F9) work
- Lock screen controls on mobile
- Actions: play, pause, previoustrack, nexttrack, seekto
- Metadata: title, artist, album, artwork

---

## Build & Deployment

### Project Structure

```
kefw2ui/
├── main.go                 # Entry point
├── server/                 # HTTP server, routes, SSE
├── speaker/                # Speaker manager, event bridge
├── config/                 # Config loading
├── frontend/               # SvelteKit app
│   ├── src/
│   ├── static/
│   ├── bun.lockb
│   ├── package.json
│   └── svelte.config.js
├── Dockerfile              # Multi-stage build
├── compose.yaml            # Docker Compose
├── kefw2ui.service         # Systemd unit
├── Makefile                # Build commands
├── README.md
└── go.mod
```

### Build Commands

```bash
# Development (hot reload)
make dev          # Runs backend + Vite dev server (bun)

# Production - Binary
make build        # Single binary with embedded frontend
./kefw2ui --port 8080 --bind 0.0.0.0

# Production - Docker
docker compose up -d

# Production - Systemd
sudo cp kefw2ui.service /etc/systemd/system/
sudo systemctl enable --now kefw2ui
```

### Dockerfile

```dockerfile
# Stage 1: Build frontend
FROM oven/bun:1 AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/bun.lockb ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

# Stage 2: Build Go binary
FROM golang:1.22 AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/build ./frontend/build
RUN CGO_ENABLED=0 go build -o kefw2ui .

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/kefw2ui /usr/local/bin/
EXPOSE 8080
CMD ["kefw2ui"]
```

### compose.yaml

```yaml
services:
  kefw2ui:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ~/.config/kefw2:/root/.config/kefw2
    network_mode: host  # Required for mDNS speaker discovery
    restart: unless-stopped
```

### Systemd Unit (kefw2ui.service)

```ini
[Unit]
Description=KEF W2 Speaker Controller UI
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/kefw2ui --bind 0.0.0.0 --port 8080
Restart=on-failure
RestartSec=5
User=kefw2ui
Group=kefw2ui

[Install]
WantedBy=multi-user.target
```

---

## Summary

| Aspect | Decision |
|--------|----------|
| Architecture | Go backend + SvelteKit SPA, single binary |
| Frontend | shadcn-svelte + Tailwind, dark mode, hero artwork |
| Real-time | SSE from backend, Svelte stores react to events |
| Keyboard | Full navigation + Cmd+K command palette, customizable |
| Speakers | Auto-discover + manual add, one active at a time |
| Content | UPnP primary, Radio, Podcasts, passthrough for AirPlay/BT |
| Queue | Drag-drop reorder, shuffle/repeat, saved playlists |
| PWA | Installable, Media Session API for OS media controls |
| Deployment | Binary, Docker (compose.yaml), Systemd unit |
| Config | `os.UserConfigDir()/kefw2/` |
| Network | Local + Tailscale (no auth needed) |
