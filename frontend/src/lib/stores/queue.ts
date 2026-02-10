import { writable } from 'svelte/store';

// Simple counter-based store that triggers refreshes.
// When refresh() is called the value increments, notifying all subscribers.
function createRefreshStore() {
	const { subscribe, update } = writable(0);

	return {
		subscribe,
		refresh: () => update(n => n + 1)
	};
}

// Trigger for queue refreshes (speaker notifies us via SSE)
export const queueRefresh = createRefreshStore();

// Trigger for play mode refreshes (speaker notifies us via SSE)
export const playModeRefresh = createRefreshStore();

// Trigger for playlist list refreshes (server notifies us via SSE after CRUD)
export const playlistsRefresh = createRefreshStore();
