# kefw2ui

A slick, responsive web UI for controlling KEF W2 speakers (LSX II, LS50 Wireless II, LS60).

## Features

- Real-time updates via Server-Sent Events (SSE)
- Dark mode with hero album artwork display
- Full keyboard navigation with command palette (Cmd+K)
- Browse UPnP, Radio, and Podcasts
- Queue management with saved playlists
- PWA with Media Session API for OS-level media controls
- Single binary deployment

## Requirements

- Go 1.22+
- Bun 1.0+
- Task (go-task.github.io)

## Quick Start

### Development

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
./kefw2ui --port 8080 --bind 0.0.0.0
```

## Deployment

### Docker

```bash
# Build and run
docker compose up -d

# View logs
docker compose logs -f
```

Note: `network_mode: host` is required for mDNS speaker discovery.

### Systemd

```bash
# Install binary
sudo task install

# Create service user
sudo useradd -r -s /bin/false kefw2ui
sudo mkdir -p /home/kefw2ui/.config/kefw2
sudo chown -R kefw2ui:kefw2ui /home/kefw2ui

# Install and enable service
sudo cp kefw2ui.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now kefw2ui

# View logs
sudo journalctl -u kefw2ui -f
```

## Configuration

Configuration is stored in the OS-specific config directory:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/kefw2/` |
| Linux | `~/.config/kefw2/` |
| Windows | `%AppData%/kefw2/` |

Files:
- `kefw2ui.yaml` - Server configuration
- `playlists/*.json` - Saved playlists (shared with CLI)

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Space` | Play/Pause |
| `←` / `→` | Previous / Next track |
| `↑` / `↓` | Volume up / down |
| `M` | Toggle mute |
| `Cmd/Ctrl + K` | Open command palette |
| `S` | Speaker switcher |
| `Q` | Focus queue |
| `B` | Focus browser |

## Remote Access via Tailscale

kefw2ui works great over Tailscale. Run the server on your home network, access it from anywhere on your tailnet.

```bash
# On your home server
./kefw2ui --bind 0.0.0.0 --port 8080

# Access from anywhere
# http://your-server.tail12345.ts.net:8080
```

## License

MIT
