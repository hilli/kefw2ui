import { writable } from 'svelte/store';

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected';

export interface PlayerState {
	title: string;
	artist: string;
	album: string;
	artwork: string | null;
	state: 'stopped' | 'playing' | 'paused';
	duration: number;
	position: number;
	volume: number;
	muted: boolean;
	source: string;
	poweredOn: boolean;
}

const initialPlayerState: PlayerState = {
	title: '',
	artist: '',
	album: '',
	artwork: null,
	state: 'stopped',
	duration: 0,
	position: 0,
	volume: 50,
	muted: false,
	source: 'wifi',
	poweredOn: true
};

export const player = writable<PlayerState>(initialPlayerState);
export const connectionStatus = writable<ConnectionStatus>('disconnected');

// Update functions for SSE events
export function updateVolume(volume: number) {
	player.update((p) => ({ ...p, volume }));
}

export function updateMute(muted: boolean) {
	player.update((p) => ({ ...p, muted }));
}

export function updatePlayerData(data: {
	title?: string;
	artist?: string;
	album?: string;
	icon?: string;
	state?: string;
	duration?: number;
	position?: number;
}) {
	player.update((p) => ({
		...p,
		title: data.title ?? p.title,
		artist: data.artist ?? p.artist,
		album: data.album ?? p.album,
		artwork: data.icon ?? p.artwork,
		state: (data.state as PlayerState['state']) ?? p.state,
		duration: data.duration ?? p.duration,
		position: data.position ?? p.position
	}));
}

export function updateSource(source: string) {
	player.update((p) => ({ ...p, source }));
}

export function updatePosition(position: number) {
	player.update((p) => ({ ...p, position }));
}

export function updatePowerState(poweredOn: boolean) {
	player.update((p) => ({ ...p, poweredOn }));
}
