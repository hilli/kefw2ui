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
- Built-in Tailscale support for secure remote access

## Installation

### Homebrew

```bash
brew install hilli/tap/kefw2ui
```

### Docker

```bash
docker pull ghcr.io/hilli/kefw2ui:latest
docker run --network host ghcr.io/hilli/kefw2ui:latest
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

# Print version
kefw2ui --version
```

Then open http://localhost:8080 in your browser.

## Configuration

### CLI Flags and Environment Variables

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--bind` | `KEFW2UI_BIND` | `0.0.0.0` | Address to bind to |
| `--port` | — | `8080` | Port to listen on |
| `--version` | — | — | Print version and exit |
| `--tailscale` | `TS_ENABLED` | `false` | Enable Tailscale listener |
| `--tailscale-hostname` | `TS_HOSTNAME` | `kefw2ui` | Hostname on the tailnet |
| `--tailscale-authkey` | `TS_AUTHKEY` | — | Tailscale auth key for headless login |
| `--tailscale-dir` | `TS_STATE_DIR` | — | Directory for Tailscale state persistence |

### Config Files

Configuration is stored in the OS-specific config directory:

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/kefw2/` |
| Linux | `~/.config/kefw2/` |
| Windows | `%AppData%/kefw2/` |

Files:
- `kefw2ui.yaml` - Server configuration
- `playlists/*.json` - Saved playlists (shared with CLI)

## Deployment

### Docker Compose

```bash
docker compose up -d
docker compose logs -f
```

Note: `network_mode: host` is required for mDNS speaker discovery.

To enable Tailscale in Docker, add the environment variables to your `compose.yaml`:

```yaml
services:
  kefw2ui:
    image: ghcr.io/hilli/kefw2ui:latest
    network_mode: host
    restart: unless-stopped
    environment:
      - TS_ENABLED=true
      - TS_HOSTNAME=kefw2ui
      - TS_AUTHKEY=tskey-auth-...   # from https://login.tailscale.com/admin/settings/keys
      - TS_STATE_DIR=/data/tailscale
    volumes:
      - tailscale-state:/data/tailscale

volumes:
  tailscale-state:
```

### Systemd

```bash
# Install binary
sudo cp kefw2ui /usr/local/bin/

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

## Remote Access via Tailscale

kefw2ui has built-in Tailscale support using [tsnet](https://tailscale.com/kb/1244/tsnet). When enabled, it joins your tailnet directly and serves over HTTPS with automatic TLS certificates — no separate Tailscale client needed.

```bash
# First run (interactive login — opens browser for auth)
kefw2ui --tailscale

# Headless / Docker (use an auth key)
kefw2ui --tailscale --tailscale-authkey tskey-auth-...
```

Once running, access it at `https://kefw2ui.<your-tailnet>.ts.net` from any device on your tailnet.

The local listener on port 8080 runs in parallel, so you get both local and remote access simultaneously.

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
