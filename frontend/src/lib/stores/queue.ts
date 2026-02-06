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
