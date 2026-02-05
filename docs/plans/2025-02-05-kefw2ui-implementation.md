# kefw2ui Implementation Plan

**Date:** 2025-02-05  
**Design Doc:** [2025-02-05-kefw2ui-design.md](./2025-02-05-kefw2ui-design.md)

## Overview

This document outlines the phased implementation approach for kefw2ui. Each phase builds on the previous one and delivers a working increment.

---

## Phase 1: Core Backend & Basic UI

**Goal:** Working speaker connection with real-time updates displayed in browser.

### Backend Tasks

- [ ] **1.1 Wire up SpeakerManager to Server**
  - Create manager instance in main.go
  - Pass manager to server
  - Start initial speaker discovery on startup

- [ ] **1.2 Implement `/api/speakers` endpoint**
  - `GET /api/speakers` - List all discovered speakers
  - `POST /api/speakers/discover` - Trigger mDNS discovery
  - `POST /api/speakers/add` - Add speaker by IP
  - Return speaker info: name, IP, model, online status

- [ ] **1.3 Implement `/api/speaker` endpoint (active speaker)**
  - `GET /api/speaker` - Get active speaker info
  - `POST /api/speaker` - Set active speaker by IP or name
  - Include current source, status

- [ ] **1.4 Implement `/api/player` endpoints**
  - `GET /api/player` - Current playback state (title, artist, album, artwork, position, duration, playing/paused)
  - `POST /api/player/play` - Play/pause toggle
  - `POST /api/player/next` - Next track
  - `POST /api/player/prev` - Previous track
  - `POST /api/player/volume` - Set volume `{ "volume": 0-100 }`
  - `POST /api/player/mute` - Toggle mute or set `{ "muted": true/false }`
  - `POST /api/player/source` - Set source `{ "source": "wifi" }`

- [ ] **1.5 SSE Event Bridge**
  - Forward kefw2 events to SSE broadcast
  - Map event types: volume, mute, player, source, playMode
  - Include full player state on connection (initial sync)

### Frontend Tasks

- [ ] **1.6 Add shadcn-svelte components**
  - Button, Slider, Card, Dialog, Command, Popover
  - Run: `bunx shadcn-svelte@latest add button slider card dialog command popover`

- [ ] **1.7 Create API client (`frontend/src/lib/api/client.ts`)**
  - Typed fetch wrapper with error handling
  - Base URL configuration (dev proxy vs production)

- [ ] **1.8 Create player API module (`frontend/src/lib/api/player.ts`)**
  - Functions for all player endpoints
  - TypeScript interfaces for responses

- [ ] **1.9 Wire SSE to Svelte stores**
  - Update player store on SSE events
  - Connection status store (connected/reconnecting/disconnected)

- [ ] **1.10 Build Now Playing component**
  - Album artwork (hero display, fallback placeholder)
  - Track info: title, artist, album
  - Progress bar (display only for Phase 1)
  - Play state indicator

- [ ] **1.11 Build Controls component**
  - Play/Pause button
  - Previous/Next buttons
  - Volume slider with mute toggle
  - Source indicator (read-only for Phase 1)

- [ ] **1.12 Build Speaker Switcher**
  - Dropdown showing discovered speakers
  - Active speaker indicator
  - Click to switch

### Verification

- [ ] Connect to real speaker
- [ ] See now playing info update in real-time
- [ ] Control play/pause, volume from UI
- [ ] Switch between speakers

---

## Phase 2: Queue Management

**Goal:** View and manage the playback queue.

### Backend Tasks

- [ ] **2.1 Implement `/api/queue` endpoints**
  - `GET /api/queue` - Current queue with track info
  - `POST /api/queue/add` - Add tracks `{ "tracks": [...] }`
  - `POST /api/queue/remove` - Remove by index `{ "index": 2 }`
  - `POST /api/queue/reorder` - Move track `{ "from": 2, "to": 0 }`
  - `POST /api/queue/clear` - Clear queue
  - `POST /api/queue/play` - Jump to track `{ "index": 3 }`

- [ ] **2.2 Implement `/api/player/mode` endpoint**
  - `GET /api/player/mode` - Current shuffle/repeat mode
  - `POST /api/player/mode` - Set mode `{ "shuffle": true, "repeat": "all" }`
  - Repeat modes: "off", "all", "one"

- [ ] **2.3 SSE queue events**
  - Broadcast queue changes
  - Include version number for conflict detection

### Frontend Tasks

- [ ] **2.4 Create queue store (`frontend/src/lib/stores/queue.ts`)**
  - Track list with current index
  - Optimistic updates for drag-drop

- [ ] **2.5 Build Queue component**
  - Track list with artwork thumbnails
  - Current track highlight
  - Click to jump to track
  - Remove button per track
  - Clear all button with confirmation

- [ ] **2.6 Add drag-and-drop reordering**
  - Svelte action for drag-drop
  - Visual feedback during drag
  - Optimistic UI update, revert on error

- [ ] **2.7 Add shuffle/repeat controls**
  - Shuffle toggle button
  - Repeat mode button (cycles: off → all → one)

### Verification

- [ ] View queue updates in real-time
- [ ] Drag to reorder tracks
- [ ] Remove tracks, clear queue
- [ ] Toggle shuffle and repeat modes

---

## Phase 3: Playlists

**Goal:** Save and load playlists (shared with CLI).

### Backend Tasks

- [ ] **3.1 Implement `/api/playlists` endpoints**
  - `GET /api/playlists` - List saved playlists (names + track counts)
  - `POST /api/playlists` - Save current queue as playlist `{ "name": "Chill Vibes" }`
  - `DELETE /api/playlists/:name` - Delete playlist
  - `GET /api/playlists/:name` - Get playlist tracks
  - `POST /api/playlists/:name/load` - Load into queue `{ "mode": "replace" | "append" }`
  - `PUT /api/playlists/:name` - Rename playlist `{ "newName": "..." }`

- [ ] **3.2 Playlist file format**
  - JSON files in `{UserConfigDir}/kefw2/playlists/`
  - Compatible with CLI format
  - Include metadata: created, modified, track count

### Frontend Tasks

- [ ] **3.3 Create playlists store (`frontend/src/lib/stores/playlists.ts`)**
  - List of playlist names
  - Load on app start

- [ ] **3.4 Build Save Playlist dialog**
  - Name input with validation
  - Overwrite warning if name exists

- [ ] **3.5 Build Load Playlist panel**
  - List of saved playlists
  - Replace or append option
  - Delete button with confirmation

- [ ] **3.6 Add quick-save keyboard shortcut**
  - Cmd+S to save current queue
  - Prompt for name

### Verification

- [ ] Save queue as named playlist
- [ ] Load playlist (replace and append modes)
- [ ] Delete playlists
- [ ] Verify CLI can read saved playlists

---

## Phase 4: Content Browsing

**Goal:** Browse and play content from UPnP, Radio, and Podcasts.

### Backend Tasks

- [ ] **4.1 Implement `/api/browse/upnp/*` endpoints**
  - `GET /api/browse/upnp` - List UPnP servers
  - `GET /api/browse/upnp/:server/*path` - Browse folder
  - Return: folders, tracks with metadata
  - Support search query parameter

- [ ] **4.2 Implement `/api/browse/radio/*` endpoints**
  - `GET /api/browse/radio` - Radio sections (Favorites, Local, Popular, etc.)
  - `GET /api/browse/radio/:section` - Section content
  - `GET /api/browse/radio/search?q=...` - Search stations
  - `POST /api/browse/radio/favorites` - Add to favorites
  - `DELETE /api/browse/radio/favorites/:id` - Remove from favorites

- [ ] **4.3 Implement `/api/browse/podcast/*` endpoints**
  - `GET /api/browse/podcast` - Podcast sections (Favorites, Popular, etc.)
  - `GET /api/browse/podcast/show/:id` - Show details with episodes
  - `GET /api/browse/podcast/search?q=...` - Search podcasts
  - `POST /api/browse/podcast/favorites` - Add show to favorites

- [ ] **4.4 Implement play actions**
  - `POST /api/browse/*/play` - Play item immediately
  - `POST /api/browse/*/queue` - Add to queue
  - Handle folders (play all) vs individual tracks

### Frontend Tasks

- [ ] **4.5 Create browse store (`frontend/src/lib/stores/browse.ts`)**
  - Current path/breadcrumbs
  - Items in current view
  - Loading states

- [ ] **4.6 Build Browser component shell**
  - Tab bar: UPnP | Radio | Podcasts
  - Breadcrumb navigation
  - Loading indicator

- [ ] **4.7 Build UPnP browser**
  - Server selector (if multiple)
  - Folder/file list with icons
  - Play and Add to Queue actions
  - Search within current server

- [ ] **4.8 Build Radio browser**
  - Section navigation (Favorites, Local, Popular, Genres, Countries)
  - Station cards with logos
  - Play, Add to Favorites actions
  - Search stations

- [ ] **4.9 Build Podcast browser**
  - Popular/Trending shows
  - Show detail → episode list
  - Play episode, Add show to favorites
  - Search podcasts

- [ ] **4.10 Add keyboard navigation for browser**
  - J/K to navigate items
  - Enter to play
  - A to add to queue
  - Backspace to go up

### Verification

- [ ] Browse UPnP server and play tracks
- [ ] Browse radio stations by genre/country
- [ ] Play podcasts, add shows to favorites
- [ ] Keyboard navigation works

---

## Phase 5: Keyboard & Command Palette

**Goal:** Full keyboard control with Cmd+K command palette.

### Frontend Tasks

- [ ] **5.1 Create keyboard handler action**
  - Global keyboard event listener
  - Shortcut registry with customization
  - Prevent conflicts with inputs

- [ ] **5.2 Implement global shortcuts**
  - Space: Play/Pause
  - ←/→: Previous/Next
  - ↑/↓: Volume up/down (5%)
  - Shift+↑/↓: Volume fine (1%)
  - M: Toggle mute
  - Escape: Close modal/go back
  - S: Open speaker switcher
  - 1-9: Quick switch speaker
  - /: Focus search

- [ ] **5.3 Implement panel shortcuts**
  - Q: Focus queue
  - B: Focus browser
  - N: Focus now playing
  - J/K: Navigate lists
  - Enter: Select/play
  - A: Add to queue

- [ ] **5.4 Build Command Palette component**
  - Cmd+K to open
  - Search across: commands, playlists, queue, recent
  - Quick actions: "volume 50", "mute", "shuffle on"
  - Speaker switching: "switch to bedroom"
  - Source selection: "source bluetooth"

- [ ] **5.5 Add shortcut customization**
  - Store shortcuts in localStorage
  - Settings modal for editing
  - Reset to defaults option

- [ ] **5.6 Create keyboard shortcut help**
  - ? to show shortcuts overlay
  - Grouped by category

### Verification

- [ ] All keyboard shortcuts work
- [ ] Command palette finds items and executes commands
- [ ] Custom shortcuts persist

---

## Phase 6: PWA & Polish

**Goal:** Installable PWA with OS-level media controls and final polish.

### Frontend Tasks

- [ ] **6.1 Create Service Worker**
  - Cache app shell (HTML, CSS, JS)
  - Network-first for API calls
  - Offline fallback page
  - Auto-update detection

- [ ] **6.2 Implement Media Session API**
  - Set metadata: title, artist, album, artwork
  - Handle actions: play, pause, previoustrack, nexttrack
  - Update on track change

- [ ] **6.3 Add install prompt**
  - Detect beforeinstallprompt event
  - Show install button when available
  - Hide after installation

- [ ] **6.4 Polish Now Playing**
  - Progress bar with seeking (click to seek)
  - Time display (elapsed / total)
  - Smooth progress updates
  - Album art loading states

- [ ] **6.5 Polish responsive layout**
  - Tablet: Stack queue below now-playing
  - Mobile: Full-screen now-playing, swipe gestures
  - Bottom navigation for mobile

- [ ] **6.6 Add loading states and error handling**
  - Skeleton loaders
  - Error toasts with retry
  - Offline indicator

- [ ] **6.7 Add accessibility**
  - ARIA labels
  - Focus management
  - Keyboard trap in modals
  - Announce state changes

- [ ] **6.8 Performance optimization**
  - Lazy load browser panels
  - Virtual scrolling for long lists
  - Image optimization (WebP, lazy load)
  - Bundle size analysis

### Verification

- [ ] App installable as PWA
- [ ] Media keys work on laptop/desktop
- [ ] Works offline (graceful degradation)
- [ ] Responsive on phone/tablet
- [ ] Lighthouse score ≥90

---

## Implementation Notes

### Development Workflow

```bash
# Start development (backend + frontend hot reload)
task dev

# In separate terminal, if needed
cd frontend && bun run dev

# Run tests
task test

# Build production binary
task build

# Build and run Docker
task docker
```

### API Design Principles

1. **RESTful** - Standard HTTP methods and status codes
2. **JSON** - All requests and responses are JSON
3. **Consistent errors** - `{ "error": "message", "code": "ERROR_CODE" }`
4. **Optimistic UI** - Frontend assumes success, reverts on error

### Testing Strategy

- **Backend:** Go unit tests for handlers and manager
- **Frontend:** Vitest for stores and utils, Playwright for E2E
- **Integration:** Test against real speaker in CI (optional)

### Code Style

- **Go:** Standard gofmt, golint
- **TypeScript:** Prettier + ESLint
- **Commits:** Conventional commits (feat:, fix:, docs:, etc.)

---

## Timeline Estimate

| Phase | Effort | Dependencies |
|-------|--------|--------------|
| Phase 1 | 3-4 days | None |
| Phase 2 | 2-3 days | Phase 1 |
| Phase 3 | 1-2 days | Phase 2 |
| Phase 4 | 3-4 days | Phase 1 |
| Phase 5 | 2-3 days | Phase 4 |
| Phase 6 | 2-3 days | All phases |
| **Total** | **13-19 days** | |

Phases 4 and 5 can be done in parallel with Phase 2-3 if resources allow.

---

## Current Status

**Phase:** Scaffolding Complete  
**Next:** Phase 1 - Wire up SpeakerManager and implement basic endpoints
