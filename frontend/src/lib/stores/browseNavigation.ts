import { writable } from 'svelte/store';

export type BrowseNavigationType = 'artist' | 'album' | 'focus';
export type BrowseSourceType = 'upnp' | 'radio' | 'podcasts';

export interface BrowseNavigationRequest {
	type: BrowseNavigationType;
	query: string;
	timestamp: number;
	source?: BrowseSourceType;
}

/**
 * Store for navigating to the Browser component with a search query.
 * Used by NowPlaying and Queue to trigger artist/album searches.
 */
function createBrowseNavigation() {
	const { subscribe, set } = writable<BrowseNavigationRequest | null>(null);

	return {
		subscribe,
		/**
		 * Navigate to Browser and search for all tracks by an artist
		 */
		searchByArtist: (artist: string) => {
			set({
				type: 'artist',
				query: `artist:"${artist}"`,
				timestamp: Date.now()
			});
		},
		/**
		 * Navigate to Browser and search for all tracks from an album
		 */
		searchByAlbum: (album: string) => {
			set({
				type: 'album',
				query: `album:"${album}"`,
				timestamp: Date.now()
			});
		},
		/**
		 * Focus the search input in Browser (for keyboard shortcut)
		 * @param source - Optional source to switch to (defaults to 'upnp')
		 */
		focusSearch: (source: BrowseSourceType = 'upnp') => {
			set({
				type: 'focus',
				query: '',
				timestamp: Date.now(),
				source
			});
		},
		/**
		 * Clear the navigation request (called after Browser handles it)
		 */
		clear: () => {
			set(null);
		}
	};
}

export const browseNavigation = createBrowseNavigation();
