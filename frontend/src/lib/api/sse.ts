import {
	connectionStatus,
	updateVolume,
	updateMute,
	updatePlayerData,
	updateSource,
	updatePosition,
	updatePowerState
} from '$lib/stores/player';
import { activeSpeaker } from '$lib/stores/speakers';
import { queueRefresh } from '$lib/stores/queue';

let eventSource: EventSource | null = null;
let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
const MAX_RECONNECT_DELAY = 30000;

export function connectSSE(): () => void {
	connect();

	return () => {
		disconnect();
	};
}

function connect() {
	if (eventSource) {
		return;
	}

	connectionStatus.set('connecting');

	eventSource = new EventSource('/events');

	eventSource.onopen = () => {
		connectionStatus.set('connected');
		reconnectAttempts = 0;
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
	});

	eventSource.addEventListener('ping', () => {
		// Heartbeat received, connection is alive
	});

	// Handle data events
	eventSource.onmessage = (event) => {
		try {
			const message = JSON.parse(event.data);
			handleEvent(message);
		} catch (e) {
			console.error('Failed to parse SSE message:', e);
		}
	};
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

	const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), MAX_RECONNECT_DELAY);
	reconnectAttempts++;

	reconnectTimeout = setTimeout(() => {
		reconnectTimeout = null;
		connect();
	}, delay);
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
				}
			);
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
			break;
		}
		case 'queue':
			// Queue changed, trigger a refresh
			queueRefresh.refresh();
			break;
		default:
			console.log('Unknown event type:', message.type);
	}
}
