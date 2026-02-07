import { type PlayerState } from '$lib/stores/player';
import { api } from '$lib/api/client';

/**
 * Media Session API integration for OS-level media controls.
 * Enables laptop media keys (F7/F8/F9) and lock screen controls on mobile.
 */

let isInitialized = false;

/**
 * Initialize the Media Session API with action handlers.
 * Call this once when the app mounts.
 */
export function initMediaSession() {
	if (isInitialized) return;
	if (!('mediaSession' in navigator)) {
		console.log('Media Session API not supported');
		return;
	}

	// Set up action handlers
	try {
		navigator.mediaSession.setActionHandler('play', async () => {
			try {
				await api.playPause();
			} catch (error) {
				console.error('Media Session play failed:', error);
			}
		});

		navigator.mediaSession.setActionHandler('pause', async () => {
			try {
				await api.playPause();
			} catch (error) {
				console.error('Media Session pause failed:', error);
			}
		});

		navigator.mediaSession.setActionHandler('previoustrack', async () => {
			try {
				await api.previousTrack();
			} catch (error) {
				console.error('Media Session previous failed:', error);
			}
		});

		navigator.mediaSession.setActionHandler('nexttrack', async () => {
			try {
				await api.nextTrack();
			} catch (error) {
				console.error('Media Session next failed:', error);
			}
		});

		navigator.mediaSession.setActionHandler('stop', async () => {
			try {
				await api.stop();
			} catch (error) {
				console.error('Media Session stop failed:', error);
			}
		});

		// Seek actions (if supported by the speaker in future)
		// navigator.mediaSession.setActionHandler('seekto', (details) => { ... });
		// navigator.mediaSession.setActionHandler('seekbackward', (details) => { ... });
		// navigator.mediaSession.setActionHandler('seekforward', (details) => { ... });

		isInitialized = true;
		console.log('Media Session API initialized');
	} catch (error) {
		console.error('Failed to initialize Media Session:', error);
	}
}

/**
 * Update the Media Session metadata with current player state.
 * Call this whenever player state changes.
 */
export function updateMediaSessionMetadata(playerState: PlayerState) {
	if (!('mediaSession' in navigator)) return;

	// Only update if we have actual content
	if (!playerState.title && !playerState.artist) {
		// Clear metadata when nothing is playing
		navigator.mediaSession.metadata = null;
		navigator.mediaSession.playbackState = 'none';
		return;
	}

	// Build artwork array
	const artwork: MediaImage[] = [];
	if (playerState.artwork) {
		// The artwork URL might be relative or absolute
		const artworkUrl = playerState.artwork.startsWith('http')
			? playerState.artwork
			: playerState.artwork;
		
		artwork.push(
			{ src: artworkUrl, sizes: '96x96', type: 'image/png' },
			{ src: artworkUrl, sizes: '128x128', type: 'image/png' },
			{ src: artworkUrl, sizes: '192x192', type: 'image/png' },
			{ src: artworkUrl, sizes: '256x256', type: 'image/png' },
			{ src: artworkUrl, sizes: '384x384', type: 'image/png' },
			{ src: artworkUrl, sizes: '512x512', type: 'image/png' }
		);
	}

	// Set metadata
	navigator.mediaSession.metadata = new MediaMetadata({
		title: playerState.title || 'Unknown Track',
		artist: playerState.artist || 'Unknown Artist',
		album: playerState.album || '',
		artwork
	});

	// Set playback state
	switch (playerState.state) {
		case 'playing':
			navigator.mediaSession.playbackState = 'playing';
			break;
		case 'paused':
			navigator.mediaSession.playbackState = 'paused';
			break;
		default:
			navigator.mediaSession.playbackState = 'none';
	}

	// Update position state if we have duration info
	if (playerState.duration > 0) {
		try {
			navigator.mediaSession.setPositionState({
				duration: playerState.duration / 1000, // Convert ms to seconds
				playbackRate: 1,
				position: Math.min(playerState.position / 1000, playerState.duration / 1000)
			});
		} catch (error) {
			// Position state might not be supported in all browsers
			console.debug('Failed to set position state:', error);
		}
	}
}

/**
 * Clean up Media Session (call on unmount if needed)
 */
export function cleanupMediaSession() {
	if (!('mediaSession' in navigator)) return;

	navigator.mediaSession.metadata = null;
	navigator.mediaSession.playbackState = 'none';

	// Clear action handlers
	try {
		navigator.mediaSession.setActionHandler('play', null);
		navigator.mediaSession.setActionHandler('pause', null);
		navigator.mediaSession.setActionHandler('previoustrack', null);
		navigator.mediaSession.setActionHandler('nexttrack', null);
		navigator.mediaSession.setActionHandler('stop', null);
	} catch {
		// Some browsers don't support setting handlers to null
	}

	isInitialized = false;
}
