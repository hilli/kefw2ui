import { writable } from 'svelte/store';

export interface ReindexState {
	/** Whether a reindex is currently active */
	active: boolean;
	/** Current status: idle, progress, complete, error */
	status: 'idle' | 'progress' | 'complete' | 'error';
	/** Number of containers scanned so far */
	containersScanned: number;
	/** Number of tracks found so far */
	tracksFound: number;
	/** Name of the container currently being scanned */
	currentContainer: string;
	/** Total track count on completion */
	trackCount?: number;
	/** Server name on completion */
	serverName?: string;
	/** Error message on failure */
	error?: string;
}

const initialState: ReindexState = {
	active: false,
	status: 'idle',
	containersScanned: 0,
	tracksFound: 0,
	currentContainer: ''
};

export const reindexState = writable<ReindexState>(initialState);

export function updateReindex(data: {
	status: string;
	containersScanned?: number;
	tracksFound?: number;
	currentContainer?: string;
	trackCount?: number;
	serverName?: string;
	error?: string;
}) {
	switch (data.status) {
		case 'progress':
			reindexState.set({
				active: true,
				status: 'progress',
				containersScanned: data.containersScanned ?? 0,
				tracksFound: data.tracksFound ?? 0,
				currentContainer: data.currentContainer ?? ''
			});
			break;
		case 'complete':
			reindexState.set({
				active: false,
				status: 'complete',
				containersScanned: 0,
				tracksFound: 0,
				currentContainer: '',
				trackCount: data.trackCount,
				serverName: data.serverName
			});
			break;
		case 'error':
			reindexState.set({
				active: false,
				status: 'error',
				containersScanned: 0,
				tracksFound: 0,
				currentContainer: '',
				error: data.error
			});
			break;
	}
}

export function resetReindex() {
	reindexState.set(initialState);
}
