import {
	connectionStatus,
	updateVolume,
	updateMute,
	updatePlayerData,
	updateSource,
	updatePosition,
	updatePowerState,
	updateSpeakerHealth
} from '$lib/stores/player';
import { activeSpeaker, updateSpeakers } from '$lib/stores/speakers';
import { queueRefresh, playModeRefresh, playlistsRefresh } from '$lib/stores/queue';
import { api } from '$lib/api/client';

let eventSource: EventSource | null = null;
let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_DELAY = 30000;
let isReconnect = false;

// Heartbeat watchdog: server sends ping every 30s, we allow 45s before declaring stale
let lastHeartbeat = 0;
let heartbeatWatchdog: ReturnType<typeof setInterval> | null = null;
const HEARTBEAT_TIMEOUT = 45000;

// Visibility change handling
let visibilityHandler: (() => void) | null = null;

export function connectSSE(): () => void {
	connect();
	startHeartbeatWatchdog();
	startVisibilityHandler();

	return () => {
		disconnect();
		stopHeartbeatWatchdog();
		stopVisibilityHandler();
	};
}

/**
 * Force a reconnect of the SSE connection.
 * Useful when recovering from sleep/wake or when the connection is suspected stale.
 */
export function forceReconnect() {
	if (eventSource) {
		eventSource.close();
		eventSource = null;
	}
	if (reconnectTimeout) {
		clearTimeout(reconnectTimeout);
		reconnectTimeout = null;
	}
	// Reset attempts so we reconnect immediately
	reconnectAttempts = 0;
	isReconnect = true;
	connect();
}

function connect() {
	if (eventSource) {
		return;
	}

	connectionStatus.set('connecting');

	eventSource = new EventSource('/events');

	eventSource.onopen = () => {
		connectionStatus.set('connected');
		lastHeartbeat = Date.now();
		const wasReconnect = isReconnect || reconnectAttempts > 0;
		reconnectAttempts = 0;
		isReconnect = false;

		// After a reconnect, refresh full state to catch changes that happened while disconnected
		if (wasReconnect) {
			refreshFullState();
		}
	};

	eventSource.onerror = () => {
		connectionStatus.set('disconnected');
		eventSource?.close();
		eventSource = null;
		scheduleReconnect();
	};

	// Handle named events
	eventSource.addEventListener('connected', () => {
		connectionStatus.set('connected');
		lastHeartbeat = Date.now();
	});

	eventSource.addEventListener('ping', () => {
		// Heartbeat received, connection is alive
		lastHeartbeat = Date.now();
	});

	// Handle data events
	eventSource.onmessage = (event) => {
		try {
			const message = JSON.parse(event.data);
			lastHeartbeat = Date.now(); // Any message counts as proof of connection
			handleEvent(message);
		} catch (e) {
			console.error('Failed to parse SSE message:', e);
		}
	};
}

/**
 * Start a watchdog that checks if heartbeats are still arriving.
 * If no heartbeat/message has been received within HEARTBEAT_TIMEOUT,
 * the connection is considered stale and a reconnect is forced.
 */
function startHeartbeatWatchdog() {
	stopHeartbeatWatchdog();
	lastHeartbeat = Date.now();

	heartbeatWatchdog = setInterval(() => {
		if (eventSource && lastHeartbeat > 0 && Date.now() - lastHeartbeat > HEARTBEAT_TIMEOUT) {
			console.warn('SSE heartbeat timeout — forcing reconnect');
			forceReconnect();
		}
	}, 10000); // Check every 10 seconds
}

function stopHeartbeatWatchdog() {
	if (heartbeatWatchdog) {
		clearInterval(heartbeatWatchdog);
		heartbeatWatchdog = null;
	}
}

/**
 * Listen for page visibility changes to handle sleep/wake and tab switching.
 * When the page becomes visible again, we force reconnect the SSE connection
 * and refresh the full state, since the connection may be stale after sleep.
 */
function startVisibilityHandler() {
	stopVisibilityHandler();

	visibilityHandler = () => {
		if (document.visibilityState === 'visible') {
			// Page is visible again (wake from sleep, tab switch, etc.)
			// Force reconnect to ensure we have a fresh connection
			forceReconnect();
		}
	};

	document.addEventListener('visibilitychange', visibilityHandler);
}

function stopVisibilityHandler() {
	if (visibilityHandler) {
		document.removeEventListener('visibilitychange', visibilityHandler);
		visibilityHandler = null;
	}
}

function disconnect() {
	if (reconnectTimeout) {
		clearTimeout(reconnectTimeout);
		reconnectTimeout = null;
	}

	if (eventSource) {
		eventSource.close();
		eventSource = null;
	}

	connectionStatus.set('disconnected');
}

function scheduleReconnect() {
	if (reconnectTimeout) {
		return;
	}

	isReconnect = true;
	const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), MAX_RECONNECT_DELAY);
	reconnectAttempts++;

	reconnectTimeout = setTimeout(() => {
		reconnectTimeout = null;
		connect();
	}, delay);
}

/**
 * Refresh full application state from the API.
 * Called after SSE reconnects to sync any changes that happened while disconnected.
 * Exported so it can be called from visibility change handlers.
 */
export async function refreshFullState() {
	try {
		// Fetch player state, speaker info, and speakers list in parallel
		const [playerData, speakersData] = await Promise.all([
			api.getPlayer().catch((e) => {
				console.error('Failed to refresh player state:', e);
				return null;
			}),
			api.getSpeakers().catch((e) => {
				console.error('Failed to refresh speakers:', e);
				return null;
			})
		]);

		if (playerData) {
			updateVolume(playerData.volume);
			updateMute(playerData.muted);
			updateSource(playerData.source);
			updatePowerState(playerData.source !== 'standby');
			updatePlayerData({
				title: playerData.title,
				artist: playerData.artist,
				album: playerData.album,
				icon: playerData.icon,
				state: playerData.state,
				duration: playerData.duration,
				position: playerData.position,
				audioType: playerData.audioType,
				live: playerData.live
			});
		}

		if (speakersData) {
			updateSpeakers(
				speakersData.speakers.map((s) => ({
					...s,
					active: s.active
				})),
				speakersData.defaultSpeaker
			);
		}

		// Trigger queue and play mode refresh
		queueRefresh.refresh();
		playModeRefresh.refresh();
	} catch (e) {
		console.error('Failed to refresh state after reconnect:', e);
	}
}

function handleEvent(message: { type: string; data: unknown }) {
	switch (message.type) {
		case 'volume':
			updateVolume((message.data as { volume: number }).volume);
			break;
		case 'mute':
			updateMute((message.data as { muted: boolean }).muted);
			break;
		case 'player':
			updatePlayerData(
				message.data as {
					title?: string;
					artist?: string;
					album?: string;
					icon?: string;
					state?: string;
					duration?: number;
					position?: number;
					audioType?: string;
					live?: boolean;
				}
			);
			// Track changed — refresh queue so currentIndex updates
			queueRefresh.refresh();
			break;
		case 'source': {
			const source = (message.data as { source: string }).source;
			updateSource(source);
			// When source changes to standby, speaker is off
			updatePowerState(source !== 'standby');
			break;
		}
		case 'power':
			updatePowerState((message.data as { status: string }).status === 'powerOn');
			break;
		case 'playTime':
			updatePosition((message.data as { position: number }).position);
			break;
		case 'speaker': {
			const data = message.data as { ip: string; name: string; model: string };
			activeSpeaker.set({
				ip: data.ip,
				name: data.name,
				model: data.model,
				active: true
			});
			// Speaker became active — refresh everything (handles startup race
			// where initial API calls returned 503 before discovery completed)
			refreshFullState();
			break;
		}
		case 'queue':
			// Queue changed, trigger a refresh
			queueRefresh.refresh();
			break;
		case 'playMode':
			// Play mode changed on the speaker, trigger a refresh
			playModeRefresh.refresh();
			break;
		case 'playlists':
			// Playlist list changed (CRUD from MCP or another client), trigger a refresh
			playlistsRefresh.refresh();
			break;
		case 'speakerHealth':
			updateSpeakerHealth((message.data as { connected: boolean }).connected);
			break;
		default:
			console.log('Unknown event type:', message.type);
	}
}
