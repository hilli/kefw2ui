// API client for kefw2ui backend

const API_BASE = '/api';

interface APIError {
	error: string;
}

class APIClient {
	private static readonly MAX_ATTEMPTS = 3;
	private static readonly RETRY_DELAYS = [1000, 2000]; // ms between retries
	private static readonly RETRYABLE_STATUS_CODES = new Set([502, 503, 504]);

	private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
		let lastError: Error | undefined;

		for (let attempt = 0; attempt < APIClient.MAX_ATTEMPTS; attempt++) {
			try {
				const response = await fetch(`${API_BASE}${path}`, {
					headers: {
						'Content-Type': 'application/json',
						...options.headers
					},
					...options
				});

				if (!response.ok) {
					// Retry on transient server errors (502/503/504)
					if (APIClient.RETRYABLE_STATUS_CODES.has(response.status)) {
						lastError = new Error(`HTTP ${response.status}`);
						if (attempt < APIClient.MAX_ATTEMPTS - 1) {
							console.warn(
								`API ${options.method ?? 'GET'} ${path} returned ${response.status}, retrying (${attempt + 1}/${APIClient.MAX_ATTEMPTS - 1})...`
							);
							await this.delay(APIClient.RETRY_DELAYS[attempt]);
							continue;
						}
					}

					// 4xx and other errors — throw immediately, no retry
					const error: APIError = await response.json().catch(() => ({ error: 'Unknown error' }));
					throw new Error(error.error || `HTTP ${response.status}`);
				}

				return response.json();
			} catch (err) {
				// Network errors (TypeError from fetch) — retry
				if (err instanceof TypeError) {
					lastError = err;
					if (attempt < APIClient.MAX_ATTEMPTS - 1) {
						console.warn(
							`API ${options.method ?? 'GET'} ${path} network error, retrying (${attempt + 1}/${APIClient.MAX_ATTEMPTS - 1})...`
						);
						await this.delay(APIClient.RETRY_DELAYS[attempt]);
						continue;
					}
				}

				// Non-network errors (including our own thrown errors above) — rethrow
				throw err;
			}
		}

		// All retries exhausted
		throw lastError ?? new Error(`Request failed after ${APIClient.MAX_ATTEMPTS} attempts`);
	}

	private delay(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	// Speaker management
	async getSpeakers(): Promise<{
		speakers: Array<{
			ip: string;
			name: string;
			model: string;
			active: boolean;
			isDefault: boolean;
			firmware: string;
		}>;
		defaultSpeaker: string;
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

	async getDefaultSpeaker(): Promise<{ defaultSpeaker: string }> {
		return this.request('/speakers/default');
	}

	async setDefaultSpeaker(ip: string): Promise<{ defaultSpeaker: string; message: string }> {
		return this.request('/speakers/default', {
			method: 'POST',
			body: JSON.stringify({ ip })
		});
	}

	async clearDefaultSpeaker(): Promise<{ message: string }> {
		return this.request('/speakers/default', { method: 'DELETE' });
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
		audioType: string;
		live: boolean;
	}> {
		return this.request('/player');
	}

	async playPause(): Promise<{ status: string }> {
		return this.request('/player/play', { method: 'POST' });
	}

	async stop(): Promise<{ status: string }> {
		return this.request('/player/stop', { method: 'POST' });
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

	async seek(positionMs: number): Promise<{ status: string; positionMs: number }> {
		return this.request('/player/seek', {
			method: 'POST',
			body: JSON.stringify({ positionMs })
		});
	}

	async getPower(): Promise<{ poweredOn: boolean; status: string }> {
		return this.request('/player/power');
	}

	async setPower(powerOn: boolean): Promise<{ poweredOn: boolean; status: string }> {
		return this.request('/player/power', {
			method: 'POST',
			body: JSON.stringify({ powerOn })
		});
	}

	async togglePower(): Promise<{ poweredOn: boolean; status: string }> {
		return this.request('/player/power', { method: 'POST' });
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

	async playQueueTrack(index: number): Promise<{ status: string }> {
		return this.request('/queue/play', {
			method: 'POST',
			body: JSON.stringify({ index })
		});
	}

	async removeFromQueue(indices: number[]): Promise<{ status: string }> {
		return this.request('/queue/remove', {
			method: 'POST',
			body: JSON.stringify({ indices })
		});
	}

	async moveQueueItem(from: number, to: number): Promise<{ status: string }> {
		return this.request('/queue/move', {
			method: 'POST',
			body: JSON.stringify({ from, to })
		});
	}

	async clearQueue(): Promise<{ status: string }> {
		return this.request('/queue/clear', { method: 'POST' });
	}

	async getPlayMode(): Promise<{
		mode: string;
		shuffle: boolean;
		repeat: 'off' | 'one' | 'all';
	}> {
		return this.request('/queue/mode');
	}

	async setPlayMode(options: {
		mode?: string;
		shuffle?: boolean;
		repeat?: 'off' | 'one' | 'all';
	}): Promise<{
		mode: string;
		shuffle: boolean;
		repeat: 'off' | 'one' | 'all';
	}> {
		return this.request('/queue/mode', {
			method: 'POST',
			body: JSON.stringify(options)
		});
	}

	async toggleShuffle(): Promise<{
		mode: string;
		shuffle: boolean;
		repeat: 'off' | 'one' | 'all';
	}> {
		const current = await this.getPlayMode();
		return this.setPlayMode({ shuffle: !current.shuffle });
	}

	async cycleRepeat(): Promise<{
		mode: string;
		shuffle: boolean;
		repeat: 'off' | 'one' | 'all';
	}> {
		const current = await this.getPlayMode();
		const nextRepeat: Record<string, 'off' | 'one' | 'all'> = {
			off: 'all',
			all: 'one',
			one: 'off'
		};
		return this.setPlayMode({ repeat: nextRepeat[current.repeat] || 'off' });
	}

	// Playlists
	async getPlaylists(): Promise<{
		playlists: Array<{
			id: string;
			name: string;
			description?: string;
			trackCount: number;
			createdAt: string;
			updatedAt: string;
		}>;
	}> {
		return this.request('/playlists');
	}

	async getPlaylist(id: string): Promise<{
		playlist: {
			id: string;
			name: string;
			description?: string;
			tracks: Array<{
				title: string;
				artist?: string;
				album?: string;
				duration?: number;
				icon?: string;
				path?: string;
				id?: string;
				type?: string;
			}>;
			createdAt: string;
			updatedAt: string;
		};
	}> {
		return this.request(`/playlists/${id}`);
	}

	async createPlaylist(
		name: string,
		description?: string,
		tracks?: Array<{
			title: string;
			artist?: string;
			album?: string;
			duration?: number;
			icon?: string;
			path?: string;
			id?: string;
			type?: string;
		}>
	): Promise<{
		playlist: {
			id: string;
			name: string;
			description?: string;
			tracks: Array<{
				title: string;
				artist?: string;
				album?: string;
			}>;
			createdAt: string;
			updatedAt: string;
		};
	}> {
		return this.request('/playlists', {
			method: 'POST',
			body: JSON.stringify({ name, description, tracks: tracks || [] })
		});
	}

	async updatePlaylist(
		id: string,
		updates: {
			name?: string;
			description?: string;
			tracks?: Array<{
				title: string;
				artist?: string;
				album?: string;
				duration?: number;
				icon?: string;
				path?: string;
				id?: string;
				type?: string;
				uri?: string;
				mimeType?: string;
				serviceId?: string;
			}>;
		}
	): Promise<{
		playlist: {
			id: string;
			name: string;
			description?: string;
			tracks: Array<{
				title: string;
				artist?: string;
				album?: string;
			}>;
			createdAt: string;
			updatedAt: string;
		};
	}> {
		return this.request(`/playlists/${id}`, {
			method: 'PUT',
			body: JSON.stringify(updates)
		});
	}

	async deletePlaylist(id: string): Promise<{ status: string }> {
		return this.request(`/playlists/${id}`, { method: 'DELETE' });
	}

	async saveQueueAsPlaylist(
		name: string,
		description?: string
	): Promise<{
		playlist: {
			id: string;
			name: string;
			description?: string;
			tracks: Array<{
				title: string;
				artist?: string;
				album?: string;
			}>;
			createdAt: string;
			updatedAt: string;
		};
	}> {
		return this.request('/playlists/save-queue', {
			method: 'POST',
			body: JSON.stringify({ name, description })
		});
	}

	async loadPlaylist(
		id: string,
		append?: boolean
	): Promise<{ status: string; trackCount: number }> {
		return this.request(`/playlists/load/${id}`, {
			method: 'POST',
			body: JSON.stringify({ append: append || false })
		});
	}

	// Content Browsing
	async getBrowseSources(): Promise<{
		sources: Array<{
			id: string;
			name: string;
			description: string;
			icon: string;
		}>;
	}> {
		return this.request('/browse/sources');
	}

	async browseUPnP(
		path?: string,
		options?: { query?: string }
	): Promise<{
		items: BrowseItem[];
		totalCount: number;
		source: string;
		search?: boolean;
		message?: string;
	}> {
		const params = new URLSearchParams();
		if (path) {
			params.set('path', path);
		}
		if (options?.query) {
			params.set('q', options.query);
		}
		const url = params.toString() ? `/browse/upnp?${params.toString()}` : '/browse/upnp';
		return this.request(url);
	}

	async browseRadio(
		endpoint?: string,
		options?: { path?: string; query?: string }
	): Promise<{
		items: BrowseItem[];
		totalCount: number;
		source: string;
	}> {
		let url = '/browse/radio';
		if (endpoint) {
			url += `/${endpoint}`;
		}
		const params = new URLSearchParams();
		if (options?.path) {
			params.set('path', options.path);
		}
		if (options?.query) {
			params.set('q', options.query);
		}
		if (params.toString()) {
			url += `?${params.toString()}`;
		}
		return this.request(url);
	}

	async browsePodcasts(
		endpoint?: string,
		options?: { path?: string; query?: string }
	): Promise<{
		items: BrowseItem[];
		totalCount: number;
		source: string;
	}> {
		let url = '/browse/podcasts';
		if (endpoint) {
			url += `/${endpoint}`;
		}
		const params = new URLSearchParams();
		if (options?.path) {
			params.set('path', options.path);
		}
		if (options?.query) {
			params.set('q', options.query);
		}
		if (params.toString()) {
			url += `?${params.toString()}`;
		}
		return this.request(url);
	}

	async playBrowseItem(item: {
		path: string;
		source: string;
		type: string;
		audioType?: string;
		title?: string;
		icon?: string;
		id?: string;
		containerPath?: string; // For podcast episodes: parent container path
	}): Promise<{ status: string }> {
		return this.request('/browse/play', {
			method: 'POST',
			body: JSON.stringify(item)
		});
	}

	async addBrowseItemToQueue(item: {
		path: string;
		source: string;
		type: string;
		title?: string;
		icon?: string;
		artist?: string;
		album?: string;
		audioType?: string;
		mediaData?: MediaData; // Required for queue playback of airable content
	}): Promise<{ status: string; tracksAdded: number }> {
		return this.request('/browse/queue', {
			method: 'POST',
			body: JSON.stringify(item)
		});
	}

	async toggleFavorite(item: {
		path: string;
		source: string;
		id?: string;
		title?: string;
		add: boolean;
	}): Promise<{ status: string; message: string }> {
		return this.request('/browse/favorite', {
			method: 'POST',
			body: JSON.stringify(item)
		});
	}

	// Settings
	async getAppSettings(): Promise<{
		version: string;
		server: {
			port: number;
			bind: string;
		};
	}> {
		return this.request('/settings');
	}

	async getSpeakerSettings(): Promise<{
		speaker: {
			ip: string;
			name: string;
			model: string;
			firmware: string;
			macPrimary: string;
		};
		settings: {
			maxVolume: number;
			volume: number;
			muted: boolean;
			source: string;
			poweredOn: boolean;
		};
	}> {
		return this.request('/settings/speaker');
	}

	async updateSpeakerSettings(settings: {
		maxVolume?: number;
	}): Promise<{ status: string }> {
		return this.request('/settings/speaker', {
			method: 'PUT',
			body: JSON.stringify(settings)
		});
	}

	async getEQSettings(): Promise<{
		eq: {
			profileName: string;
			bassExtension: string;
			deskMode: boolean;
			deskModeSetting: number;
			wallMode: boolean;
			wallModeSetting: number;
			trebleAmount: number;
			balance: number;
			phaseCorrection: boolean;
			isExpertMode: boolean;
		};
		subwoofer: {
			enabled: boolean;
			count: number;
			gain: number;
			polarity: string;
			preset: string;
			lowPassFreq: number;
			stereo: boolean;
			highPassMode: boolean;
			highPassFreq: number;
		};
	}> {
		return this.request('/settings/eq');
	}

	// UPnP Settings
	async getUPnPSettings(): Promise<{
		defaultServer: string;
		defaultServerPath: string;
		browseContainer: string;
		indexContainer: string;
	}> {
		return this.request('/settings/upnp');
	}

	async updateUPnPSettings(settings: {
		defaultServer?: string;
		defaultServerPath?: string;
		browseContainer?: string;
		indexContainer?: string;
	}): Promise<{
		status: string;
		defaultServer: string;
		defaultServerPath: string;
		browseContainer: string;
		indexContainer: string;
	}> {
		return this.request('/settings/upnp', {
			method: 'PUT',
			body: JSON.stringify(settings)
		});
	}

	async getUPnPServers(): Promise<{
		servers: Array<{
			name: string;
			path: string;
			icon: string;
		}>;
	}> {
		return this.request('/upnp/servers');
	}

	async getUPnPContainers(
		serverPath?: string,
		containerPath?: string
	): Promise<{
		path: string;
		containers: string[];
	}> {
		const params = new URLSearchParams();
		if (serverPath) params.set('server', serverPath);
		if (containerPath) params.set('path', containerPath);
		const query = params.toString();
		return this.request(`/upnp/containers${query ? '?' + query : ''}`);
	}

	async reindexMedia(): Promise<{ status: string }> {
		return this.request('/upnp/reindex', { method: 'POST' });
	}
}

// Types for content browsing
export interface MediaResource {
	uri: string;
	mimeType: string;
	bitRate?: number;
	codec?: string;
	duration?: number;
	sampleFrequency?: number;
}

export interface MediaMetaData {
	artist?: string;
	album?: string;
	genre?: string;
	composer?: string;
	serviceID?: string;
	live?: boolean;
	contentPlayContextPath?: string;
	prePlayPath?: string;
	maximumRetryCount?: number;
}

export interface MediaData {
	metaData?: MediaMetaData;
	resources?: MediaResource[];
}

export interface BrowseItem {
	title: string;
	type: string; // "container", "audio", "query"
	path: string;
	icon?: string;
	artist?: string;
	album?: string;
	duration?: number; // milliseconds
	id?: string;
	description?: string;
	playable?: boolean;
	audioType?: string;
	mediaData?: MediaData; // Required for queue playback of airable content
	containerPath?: string; // Parent container path for podcast episodes
	searchQuery?: string; // If set, clicking triggers this search instead of browsing
}

export const api = new APIClient();
