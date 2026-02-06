<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type BrowseItem } from '$lib/api/client';
	import { queueRefresh } from '$lib/stores/queue';
	import { browseNavigation } from '$lib/stores/browseNavigation';
	import {
		Radio,
		Podcast,
		Server,
		Folder,
		Music,
		Play,
		ListPlus,
		ChevronLeft,
		ChevronRight,
		Loader2,
		Search,
		X,
		Heart
	} from 'lucide-svelte';

	interface Props {
		fullHeight?: boolean;
	}

	let { fullHeight = false }: Props = $props();

	type SourceType = 'upnp' | 'radio' | 'podcasts';

	interface BreadcrumbItem {
		title: string;
		path: string;
		endpoint?: string;
	}

	let activeSource = $state<SourceType>('radio');
	let items = $state<BrowseItem[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);
	let collapsed = $state(false);
	let playingPath = $state<string | null>(null);
	let queueingPath = $state<string | null>(null);
	let favoritingPath = $state<string | null>(null);

	// Navigation breadcrumbs
	let breadcrumbs = $state<BreadcrumbItem[]>([]);

	// Search
	let searchQuery = $state('');
	let searchActive = $state(false);
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;
	let searchInputElement: HTMLInputElement | undefined = $state();

	// Source configurations
	const sources: { id: SourceType; label: string; icon: typeof Radio }[] = [
		{ id: 'radio', label: 'Radio', icon: Radio },
		{ id: 'podcasts', label: 'Podcasts', icon: Podcast },
		{ id: 'upnp', label: 'Media', icon: Server }
	];

	async function loadContent(source: SourceType, endpoint?: string, path?: string) {
		loading = true;
		error = null;

		try {
			let result;
			switch (source) {
				case 'upnp':
					result = await api.browseUPnP(path);
					break;
				case 'radio':
					result = await api.browseRadio(endpoint, { path });
					break;
				case 'podcasts':
					result = await api.browsePodcasts(endpoint, { path });
					break;
			}
			items = result.items || [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load content';
			items = [];
		} finally {
			loading = false;
		}
	}

	async function searchContent(query: string) {
		if (!query.trim()) {
			searchActive = false;
			loadRoot();
			return;
		}

		loading = true;
		error = null;
		searchActive = true;

		try {
			let result;
			switch (activeSource) {
				case 'radio':
					result = await api.browseRadio('search', { query });
					break;
				case 'podcasts':
					result = await api.browsePodcasts('search', { query });
					break;
				case 'upnp':
					// Search cached UPnP media content
					result = await api.browseUPnP(undefined, { query });
					break;
			}
			items = result.items || [];
			breadcrumbs = [{ title: `Search: "${query}"`, path: '' }];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Search failed';
			items = [];
		} finally {
			loading = false;
		}
	}

	function handleSearchInput(e: Event) {
		const value = (e.target as HTMLInputElement).value;
		searchQuery = value;

		// Debounce search
		if (searchTimeout) clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => {
			if (value.trim()) {
				searchContent(value);
			} else {
				searchActive = false;
				loadRoot();
			}
		}, 500);
	}

	function clearSearch() {
		searchQuery = '';
		searchActive = false;
		loadRoot();
	}

	function loadRoot() {
		breadcrumbs = [];
		loadContent(activeSource);
	}

	function switchSource(source: SourceType) {
		if (source === activeSource) return;
		activeSource = source;
		searchQuery = '';
		searchActive = false;
		loadRoot();
	}

	async function navigateToItem(item: BrowseItem) {
		if (item.type === 'container' || (item.type !== 'audio' && !item.playable)) {
			// Navigate into container
			breadcrumbs = [...breadcrumbs, { title: item.title, path: item.path }];
			await loadContent(activeSource, undefined, item.path);
		} else {
			// Play the item
			await playItem(item);
		}
	}

	async function navigateToBreadcrumb(index: number) {
		if (index < 0) {
			loadRoot();
			return;
		}
		const crumb = breadcrumbs[index];
		breadcrumbs = breadcrumbs.slice(0, index + 1);
		await loadContent(activeSource, crumb.endpoint, crumb.path);
	}

	async function playItem(item: BrowseItem) {
		try {
			playingPath = item.path;
			await api.playBrowseItem({
				path: item.path,
				source: activeSource,
				type: item.type,
				audioType: item.audioType,
				title: item.title,
				icon: item.icon,
				id: item.id,
				containerPath: item.containerPath // For podcast episodes
			});
		} catch (e) {
			console.error('Failed to play:', e);
		} finally {
			setTimeout(() => {
				playingPath = null;
			}, 1000);
		}
	}

	async function addToQueue(item: BrowseItem) {
		try {
			queueingPath = item.path;
			await api.addBrowseItemToQueue({
				path: item.path,
				source: activeSource,
				type: item.type,
				title: item.title,
				icon: item.icon,
				artist: item.artist,
				audioType: item.audioType,
				mediaData: item.mediaData // Include full media data for queue playback
			});
			// Trigger queue refresh in the Queue component
			queueRefresh.refresh();
		} catch (e) {
			console.error('Failed to add to queue:', e);
		} finally {
			setTimeout(() => {
				queueingPath = null;
			}, 1000);
		}
	}

	async function addToFavorites(item: BrowseItem) {
		try {
			favoritingPath = item.path;
			await api.toggleFavorite({
				path: item.path,
				source: activeSource,
				id: item.id,
				title: item.title,
				add: true
			});
		} catch (e) {
			console.error('Failed to add to favorites:', e);
		} finally {
			setTimeout(() => {
				favoritingPath = null;
			}, 1000);
		}
	}

	function formatDuration(ms?: number): string {
		if (!ms || ms <= 0) return '';
		const totalSeconds = Math.floor(ms / 1000);
		const hours = Math.floor(totalSeconds / 3600);
		const minutes = Math.floor((totalSeconds % 3600) / 60);
		const seconds = totalSeconds % 60;

		if (hours > 0) {
			return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
		}
		return `${minutes}:${seconds.toString().padStart(2, '0')}`;
	}

	// Search by artist - shows all tracks by this artist
	function searchByArtist(artist: string) {
		const query = `artist:"${artist}"`;
		searchQuery = query;
		searchContent(query);
	}

	// Search by album - shows all tracks from this album
	function searchByAlbum(album: string) {
		const query = `album:"${album}"`;
		searchQuery = query;
		searchContent(query);
	}

	function getItemIcon(item: BrowseItem) {
		if (item.type === 'container') return Folder;
		if (item.audioType === 'audioBroadcast') return Radio;
		return Music;
	}

	onMount(() => {
		loadRoot();

		// Subscribe to browse navigation requests from other components (NowPlaying, Queue, keyboard shortcuts)
		const unsubscribe = browseNavigation.subscribe((request) => {
			if (request) {
				collapsed = false; // Ensure browser is not collapsed
				
				if (request.type === 'focus') {
					// Switch to requested source (or default to upnp) and focus search
					const targetSource = request.source || 'upnp';
					if (activeSource !== targetSource) {
						activeSource = targetSource;
						loadRoot(); // Load root content for new source
					}
					// Focus search input after a tick to ensure DOM is ready
					setTimeout(() => searchInputElement?.focus(), 10);
				} else {
					// Switch to UPnP source and search
					if (activeSource !== 'upnp') {
						activeSource = 'upnp';
					}
					searchQuery = request.query;
					searchContent(request.query);
				}
				browseNavigation.clear(); // Clear after handling
			}
		});

		return () => {
			unsubscribe();
		};
	});
</script>

<div class="flex flex-col rounded-lg border border-zinc-800 bg-zinc-900/50" class:h-full={fullHeight}>
	<!-- Header -->
	<div class="flex flex-shrink-0 items-center justify-between border-b border-zinc-800">
		<button
			class="flex flex-1 items-center gap-2 px-4 py-3 text-left hover:bg-zinc-800/50"
			onclick={() => (collapsed = !collapsed)}
		>
			<Radio class="h-4 w-4 text-zinc-400" />
			<span class="text-sm font-medium text-zinc-200">Browse</span>
			{#if loading}
				<Loader2 class="h-4 w-4 animate-spin text-zinc-500" />
			{/if}
		</button>
		<button
			class="px-3 py-3 text-zinc-500 transition-transform hover:bg-zinc-800/50"
			class:rotate-180={!collapsed}
			onclick={() => (collapsed = !collapsed)}
		>
			&#9660;
		</button>
	</div>

	{#if !collapsed}
		<!-- Source Tabs -->
		<div class="flex border-b border-zinc-800">
			{#each sources as source}
				{@const Icon = source.icon}
				<button
					class="flex flex-1 items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium transition-colors"
					class:bg-zinc-800={activeSource === source.id}
					class:text-zinc-200={activeSource === source.id}
					class:text-zinc-500={activeSource !== source.id}
					class:hover:text-zinc-300={activeSource !== source.id}
					onclick={() => switchSource(source.id)}
				>
					<Icon class="h-3.5 w-3.5" />
					{source.label}
				</button>
			{/each}
		</div>

		<!-- Search Bar -->
		<div class="border-b border-zinc-800 px-3 py-2">
			<div class="relative">
				<Search class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-500" />
				<input
					bind:this={searchInputElement}
					type="text"
					class="w-full rounded bg-zinc-800 py-1.5 pl-8 pr-8 text-sm text-zinc-200 placeholder-zinc-500 focus:outline-none focus:ring-1 focus:ring-zinc-600"
					placeholder={activeSource === 'radio' ? 'Search radio stations...' : activeSource === 'podcasts' ? 'Search podcasts...' : 'Search cached media...'}
					value={searchQuery}
					oninput={handleSearchInput}
				/>
				{#if searchQuery}
					<button
						class="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300"
						onclick={clearSearch}
					>
						<X class="h-3.5 w-3.5" />
					</button>
				{/if}
			</div>
		</div>

		<!-- Breadcrumbs -->
		{#if breadcrumbs.length > 0}
			<div class="flex items-center gap-1 border-b border-zinc-800 px-3 py-1.5 text-xs">
				<button
					class="text-zinc-400 hover:text-zinc-200"
					onclick={() => navigateToBreadcrumb(-1)}
				>
					Home
				</button>
				{#each breadcrumbs as crumb, i}
					<ChevronRight class="h-3 w-3 text-zinc-600" />
					<button
						class="max-w-24 truncate text-zinc-400 hover:text-zinc-200"
						class:text-zinc-200={i === breadcrumbs.length - 1}
						onclick={() => navigateToBreadcrumb(i)}
					>
						{crumb.title}
					</button>
				{/each}
			</div>
		{/if}

		<!-- Content List -->
		<div class="min-h-0 flex-1 overflow-hidden">
			<div class="h-full overflow-y-auto" class:max-h-80={!fullHeight}>
				{#if error}
				<div class="px-4 py-6 text-center text-sm text-red-400">
					{error}
				</div>
			{:else if loading && items.length === 0}
				<div class="flex items-center justify-center py-8">
					<Loader2 class="h-6 w-6 animate-spin text-zinc-500" />
				</div>
			{:else if items.length === 0}
				<div class="px-4 py-6 text-center text-sm text-zinc-500">
					{#if searchActive}
						No results found
					{:else}
						No content available
					{/if}
				</div>
			{:else}
				{#each items as item (item.path)}
					{@const ItemIcon = getItemIcon(item)}
					<div
						class="group flex items-center gap-3 px-3 py-2 transition-colors hover:bg-zinc-800/50"
					>
						<!-- Click to navigate or play -->
						<!-- Using div instead of button to allow nested interactive elements for artist/album links -->
						<div
							class="flex flex-1 cursor-pointer items-center gap-3 text-left"
							role="button"
							tabindex="0"
							onclick={() => navigateToItem(item)}
							onkeydown={(e) => {
								if (e.key === 'Enter' || e.key === ' ') {
									e.preventDefault();
									navigateToItem(item);
								}
							}}
						>
							<!-- Icon/Thumbnail -->
							{#if item.icon}
								<img
									src={item.icon}
									alt=""
									class="h-10 w-10 flex-shrink-0 rounded bg-zinc-700 object-cover"
									onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = 'none')}
								/>
							{:else}
								<div
									class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded bg-zinc-700"
								>
									<ItemIcon class="h-5 w-5 text-zinc-400" />
								</div>
							{/if}

							<!-- Content info -->
							<div class="min-w-0 flex-1">
								<p class="truncate text-sm text-zinc-200">{item.title}</p>
								{#if activeSource === 'upnp' && (item.artist || item.album)}
									<!-- UPnP: Clickable artist and album links -->
									<p class="truncate text-xs text-zinc-500">
										{#if item.artist}
											<button
												class="hover:text-zinc-300 hover:underline"
												onclick={(e) => {
													e.stopPropagation();
													searchByArtist(item.artist!);
												}}
												onkeydown={(e) => e.stopPropagation()}
												title={`Show all tracks by ${item.artist}`}
											>
												{item.artist}
											</button>
										{/if}
										{#if item.artist && item.album}
											<span class="mx-1">•</span>
										{/if}
										{#if item.album}
											<button
												class="hover:text-zinc-300 hover:underline"
												onclick={(e) => {
													e.stopPropagation();
													searchByAlbum(item.album!);
												}}
												onkeydown={(e) => e.stopPropagation()}
												title={`Show all tracks from ${item.album}`}
											>
												{item.album}
											</button>
										{/if}
									</p>
								{:else if item.artist || item.description}
									<p class="truncate text-xs text-zinc-500">
										{item.artist || item.description}
									</p>
								{/if}
							</div>

							<!-- Duration -->
							{#if item.duration}
								<span class="text-xs text-zinc-500">{formatDuration(item.duration)}</span>
							{/if}
						</div>

						<!-- Actions -->
						<div class="flex items-center gap-1">
							<!-- Add to Favorites button - for podcast shows and radio stations -->
							{#if (activeSource === 'podcasts' && item.type === 'container') || (activeSource === 'radio' && item.audioType === 'audioBroadcast')}
								<button
									class="rounded p-1.5 text-zinc-400 opacity-0 transition-colors hover:bg-zinc-700 hover:text-red-400 group-hover:opacity-100"
									onclick={(e) => {
										e.stopPropagation();
										addToFavorites(item);
									}}
									disabled={favoritingPath === item.path}
									title="Add to Favorites"
								>
									{#if favoritingPath === item.path}
										<Loader2 class="h-4 w-4 animate-spin" />
									{:else}
										<Heart class="h-4 w-4" />
									{/if}
								</button>
							{/if}
							{#if item.playable || item.type === 'audio'}
								<!-- Add to Queue button - not for radio streams (they don't end) -->
								{#if item.audioType !== 'audioBroadcast'}
									<button
										class="rounded p-1.5 text-zinc-400 opacity-0 transition-colors hover:bg-zinc-700 hover:text-blue-400 group-hover:opacity-100"
										onclick={(e) => {
											e.stopPropagation();
											addToQueue(item);
										}}
										disabled={queueingPath === item.path}
										title="Add to Queue"
									>
										{#if queueingPath === item.path}
											<Loader2 class="h-4 w-4 animate-spin" />
										{:else}
											<ListPlus class="h-4 w-4" />
										{/if}
									</button>
								{/if}
								<!-- Play button -->
								<button
									class="rounded p-1.5 text-zinc-400 opacity-0 transition-colors hover:bg-zinc-700 hover:text-green-400 group-hover:opacity-100"
									onclick={(e) => {
										e.stopPropagation();
										playItem(item);
									}}
									disabled={playingPath === item.path}
									title="Play Now"
								>
									{#if playingPath === item.path}
										<Loader2 class="h-4 w-4 animate-spin" />
									{:else}
										<Play class="h-4 w-4" />
									{/if}
								</button>
							{/if}
							{#if item.type === 'container'}
								<ChevronRight class="h-4 w-4 text-zinc-600" />
							{/if}
						</div>
					</div>
				{/each}
			{/if}
			</div>
		</div>
	{/if}
</div>
