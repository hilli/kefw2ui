// API client for kefw2ui backend

const API_BASE = '/api';

interface APIError {
	error: string;
}

class APIClient {
	private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
		const response = await fetch(`${API_BASE}${path}`, {
			headers: {
				'Content-Type': 'application/json',
				...options.headers
			},
			...options
		});

		if (!response.ok) {
			const error: APIError = await response.json().catch(() => ({ error: 'Unknown error' }));
			throw new Error(error.error || `HTTP ${response.status}`);
		}

		return response.json();
	}

	// Speaker management
	async getSpeakers(): Promise<{
		speakers: Array<{
			ip: string;
			name: string;
			model: string;
			active: boolean;
			firmware: string;
		}>;
	}> {
		return this.request('/speakers');
	}

	async discoverSpeakers(): Promise<{
		discovered: Array<{
			ip: string;
			name: string;
			model: string;
		}>;
	}> {
		return this.request('/speakers/discover', { method: 'POST' });
	}

	async addSpeaker(ip: string): Promise<{
		speaker: {
			ip: string;
			name: string;
			model: string;
			firmware: string;
		};
	}> {
		return this.request('/speakers/add', {
			method: 'POST',
			body: JSON.stringify({ ip })
		});
	}

	async getActiveSpeaker(): Promise<{
		active: {
			ip: string;
			name: string;
			model: string;
			firmware: string;
			source: string;
			volume: number;
			muted: boolean;
			status: string;
		} | null;
	}> {
		return this.request('/speaker');
	}

	async setActiveSpeaker(ip: string): Promise<{
		active: {
			ip: string;
			name: string;
			model: string;
			firmware: string;
			source: string;
			volume: number;
			muted: boolean;
			status: string;
		};
	}> {
		return this.request('/speaker', {
			method: 'POST',
			body: JSON.stringify({ ip })
		});
	}

	// Player controls
	async getPlayer(): Promise<{
		state: string;
		volume: number;
		muted: boolean;
		source: string;
		title: string;
		artist: string;
		album: string;
		icon: string;
		duration: number;
		position: number;
	}> {
		return this.request('/player');
	}

	async playPause(): Promise<{ status: string }> {
		return this.request('/player/play', { method: 'POST' });
	}

	async nextTrack(): Promise<{ status: string }> {
		return this.request('/player/next', { method: 'POST' });
	}

	async previousTrack(): Promise<{ status: string }> {
		return this.request('/player/prev', { method: 'POST' });
	}

	async setVolume(volume: number): Promise<{ volume: number }> {
		return this.request('/player/volume', {
			method: 'POST',
			body: JSON.stringify({ volume })
		});
	}

	async getVolume(): Promise<{ volume: number }> {
		return this.request('/player/volume');
	}

	async toggleMute(): Promise<{ muted: boolean }> {
		return this.request('/player/mute', { method: 'POST' });
	}

	async setMute(muted: boolean): Promise<{ muted: boolean }> {
		return this.request('/player/mute', {
			method: 'POST',
			body: JSON.stringify({ muted })
		});
	}

	async setSource(source: string): Promise<{ source: string }> {
		return this.request('/player/source', {
			method: 'POST',
			body: JSON.stringify({ source })
		});
	}

	async getSource(): Promise<{ source: string }> {
		return this.request('/player/source');
	}

	// Queue
	async getQueue(): Promise<{
		tracks: Array<{
			index: number;
			title: string;
			artist?: string;
			album?: string;
			id: string;
			path: string;
			icon?: string;
			type: string;
			duration: number;
		}>;
		currentIndex: number;
	}> {
		return this.request('/queue');
	}
}

export const api = new APIClient();
