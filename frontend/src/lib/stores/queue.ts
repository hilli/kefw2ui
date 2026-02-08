import { writable } from 'svelte/store';

// Simple store to trigger queue refreshes
// When the value changes, any subscriber will be notified
function createQueueRefreshStore() {
	const { subscribe, update } = writable(0);

	return {
		subscribe,
		// Call this to trigger a queue refresh
		refresh: () => update(n => n + 1)
	};
}

export const queueRefresh = createQueueRefreshStore();

// Trigger for play mode refreshes (speaker notifies us via SSE)
function createPlayModeRefreshStore() {
	const { subscribe, update } = writable(0);

	return {
		subscribe,
		refresh: () => update(n => n + 1)
	};
}

export const playModeRefresh = createPlayModeRefreshStore();
