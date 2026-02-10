<script lang="ts">
	import { onMount } from 'svelte';
	import { api, type MediaData } from '$lib/api/client';
	import { toasts } from '$lib/stores/toast';
	import { queueRefresh, playlistsRefresh } from '$lib/stores/queue';
	import {
		ListMusic,
		Play,
		Trash2,
		Loader2,
		ChevronRight,
		ChevronLeft,
		Music,
		GripVertical,
		ListPlus,
		CheckSquare,
		Square,
		MinusSquare,
		X
	} from 'lucide-svelte';

	interface Props {
		fullHeight?: boolean;
	}

	let { fullHeight = false }: Props = $props();

	interface PlaylistSummary {
		id: string;
		name: string;
		description?: string;
		trackCount: number;
		createdAt: string;
		updatedAt: string;
	}

	interface Track {
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
	}

	// Custom MIME type for cross-panel drag-and-drop
	const DRAG_MIME = 'application/x-kefw2-browse-item';

	function serviceIdToSource(serviceId?: string): string {
		if (!serviceId || serviceId === 'UPnP') return 'upnp';
		if (serviceId === 'airableRadios') return 'radio';
		return 'podcasts';
	}

	function trackToMediaData(track: Track): MediaData | undefined {
		if (!track.uri) return undefined;
		return {
			metaData: {
				artist: track.artist,
				album: track.album,
				serviceID: track.serviceId
			},
			resources: [
				{
					uri: track.uri,
					mimeType: track.mimeType || '',
					duration: track.duration
				}
			]
		};
	}

	interface PlaylistDetail {
		id: string;
		name: string;
		description?: string;
		tracks: Track[];
		createdAt: string;
		updatedAt: string;
	}

	let playlists = $state<PlaylistSummary[]>([]);
	let selectedPlaylist = $state<PlaylistDetail | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let collapsed = $state(false);
	let actionLoading = $state<string | null>(null);

	// Drag and drop state for playlist detail view
	let draggedIndex = $state<number | null>(null);
	let dropTargetIndex = $state<number | null>(null);

	// External drag-and-drop state (drops from Browser/Queue)
	let externalDragOver = $state(false);

	// Track selection state (for multi-select removal)
	let selectedIndices = $state<Set<number>>(new Set());
	let selectMode = $state(false);
	let allSelected = $derived(
		selectedPlaylist !== null && selectedPlaylist.tracks.length > 0 && selectedIndices.size === selectedPlaylist.tracks.length
	);
	let someSelected = $derived(selectedIndices.size > 0 && !allSelected);

	async function loadPlaylists() {
		try {
			loading = true;
			error = null;
			const response = await api.getPlaylists();
			playlists = response.playlists || [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load playlists';
			playlists = [];
		} finally {
			loading = false;
		}
	}

	// Silent refresh: re-fetches the playlist list without setting the loading
	// flag, so the UI doesn't flicker. Used when an SSE event tells us another
	// client (or MCP) changed playlists.
	async function refreshPlaylistsSilently() {
		try {
			const response = await api.getPlaylists();
			playlists = response.playlists || [];
		} catch {
			// Silently ignore — the user's current view stays intact
		}
	}

	// Subscribe to SSE-driven playlist refresh events
	const unsubPlaylistRefresh = playlistsRefresh.subscribe((n: number) => {
		// Skip the initial value (0) emitted on subscribe
		if (n > 0) {
			refreshPlaylistsSilently();
		}
	});

	async function selectPlaylist(id: string) {
		try {
			actionLoading = `select-${id}`;
			const response = await api.getPlaylist(id);
			selectedPlaylist = response.playlist;
		} catch (e) {
			toasts.error('Failed to load playlist');
		} finally {
			actionLoading = null;
		}
	}

	async function loadPlaylistToQueue(id: string, append = false) {
		try {
			actionLoading = append ? `append-${id}` : `load-${id}`;
			const result = await api.loadPlaylist(id, append);
			const count = result.trackCount ?? 0;
			queueRefresh.refresh();
			if (append) {
				toasts.success(`Appended ${count} track${count !== 1 ? 's' : ''} to queue`);
			} else {
				toasts.success(`Loaded ${count} track${count !== 1 ? 's' : ''} to queue`);
			}
		} catch (e) {
			toasts.error('Failed to load playlist');
		} finally {
			actionLoading = null;
		}
	}

	async function deletePlaylist(id: string, event: MouseEvent) {
		event.stopPropagation();
		if (!confirm('Delete this playlist?')) return;

		try {
			actionLoading = `delete-${id}`;
			await api.deletePlaylist(id);
			if (selectedPlaylist?.id === id) {
				selectedPlaylist = null;
			}
			await loadPlaylists();
		} catch (e) {
			toasts.error('Failed to delete playlist');
		} finally {
			actionLoading = null;
		}
	}

	// Drag and drop handlers for playlist track reordering
	function handleDragStart(event: DragEvent, index: number) {
		if (!event.dataTransfer || !selectedPlaylist) return;
		draggedIndex = index;
		event.dataTransfer.effectAllowed = 'copyMove';
		event.dataTransfer.setData('text/plain', index.toString());

		// Also set the cross-panel MIME type so this track can be dropped on Queue
		const track = selectedPlaylist.tracks[index];
		const source = serviceIdToSource(track.serviceId);
		const dragData = JSON.stringify({
			path: track.path || '',
			source,
			type: track.type || 'audio',
			title: track.title,
			icon: track.icon,
			artist: track.artist,
			album: track.album,
			mediaData: trackToMediaData(track)
		});
		event.dataTransfer.setData(DRAG_MIME, dragData);

		requestAnimationFrame(() => {
			const target = event.target as HTMLElement;
			target.classList.add('opacity-50');
		});
	}

	function handleDragEnd(event: DragEvent) {
		const target = event.target as HTMLElement;
		target.classList.remove('opacity-50');
		draggedIndex = null;
		dropTargetIndex = null;
		externalDragOver = false;
	}

	function handleDragOver(event: DragEvent, index: number) {
		event.preventDefault();
		if (!event.dataTransfer) return;

		// Check if this is an external drop (from Browser or Queue)
		if (event.dataTransfer.types.includes(DRAG_MIME) && draggedIndex === null) {
			event.dataTransfer.dropEffect = 'copy';
			externalDragOver = true;
			dropTargetIndex = index;
			return;
		}

		// Internal reorder
		event.dataTransfer.dropEffect = 'move';
		if (draggedIndex !== null && draggedIndex !== index) {
			dropTargetIndex = index;
		}
	}

	function handleDragLeave(event: DragEvent) {
		const relatedTarget = event.relatedTarget as HTMLElement | null;
		const currentTarget = event.currentTarget as HTMLElement;
		if (relatedTarget && currentTarget.contains(relatedTarget)) return;
		dropTargetIndex = null;
	}

	async function handleDrop(event: DragEvent, toIndex: number) {
		event.preventDefault();
		dropTargetIndex = null;
		externalDragOver = false;

		// Check for external drop first (from Browser or Queue)
		const mimeData = event.dataTransfer?.getData(DRAG_MIME);
		if (mimeData && draggedIndex === null) {
			// Stop propagation so panel-level handlePanelDrop doesn't also fire
			event.stopPropagation();
			try {
				const data = JSON.parse(mimeData);
				await addExternalTrackToPlaylist(data, toIndex);
			} catch (e) {
				toasts.error('Failed to add track to playlist');
			}
			return;
		}

		// Internal reorder
		if (!selectedPlaylist || draggedIndex === null || draggedIndex === toIndex) {
			draggedIndex = null;
			return;
		}

		event.stopPropagation();
		const fromIndex = draggedIndex;
		draggedIndex = null;

		try {
			actionLoading = 'reorder';

			// Optimistic UI update
			const newTracks = [...selectedPlaylist.tracks];
			const [movedTrack] = newTracks.splice(fromIndex, 1);
			newTracks.splice(toIndex, 0, movedTrack);
			selectedPlaylist = { ...selectedPlaylist, tracks: newTracks };

			// Persist to backend
			await api.updatePlaylist(selectedPlaylist.id, { tracks: newTracks });
		} catch (e) {
			toasts.error('Failed to reorder tracks');
			// Re-fetch to restore correct state
			if (selectedPlaylist) {
				await selectPlaylist(selectedPlaylist.id);
			}
		} finally {
			actionLoading = null;
		}
	}

	// Convert external drag data to a playlist Track
	function dragDataToTrack(data: Record<string, unknown>): Track {
		return {
			title: (data.title as string) || 'Unknown Track',
			artist: data.artist as string | undefined,
			album: data.album as string | undefined,
			icon: data.icon as string | undefined,
			path: data.path as string | undefined,
			type: data.type as string | undefined,
			uri: (data.mediaData as { resources?: Array<{ uri?: string }> })?.resources?.[0]?.uri,
			mimeType: (data.mediaData as { resources?: Array<{ mimeType?: string }> })?.resources?.[0]?.mimeType,
			serviceId: (data.mediaData as { metaData?: { serviceID?: string } })?.metaData?.serviceID,
			duration: (data.mediaData as { resources?: Array<{ duration?: number }> })?.resources?.[0]?.duration
		};
	}

	// Add a track from external drag data to the current playlist at a specific position
	async function addExternalTrackToPlaylist(data: Record<string, unknown>, atIndex?: number) {
		if (!selectedPlaylist) {
			toasts.error('Open a playlist first to add tracks');
			return;
		}

		// Reject containers (albums, folders) — they can't be played as individual tracks
		if (data.type === 'container') {
			toasts.error('Cannot add albums/folders to playlists — add individual tracks instead');
			return;
		}

		try {
			actionLoading = 'addExternal';
			const newTrack = dragDataToTrack(data);
			const newTracks = [...selectedPlaylist.tracks];
			if (atIndex !== undefined && atIndex < newTracks.length) {
				newTracks.splice(atIndex, 0, newTrack);
			} else {
				newTracks.push(newTrack);
			}

			// Optimistic update
			selectedPlaylist = { ...selectedPlaylist, tracks: newTracks };

			// Persist
			await api.updatePlaylist(selectedPlaylist.id, { tracks: newTracks });
			toasts.success(`Added "${newTrack.title}" to ${selectedPlaylist.name}`);
		} catch (e) {
			toasts.error('Failed to add track to playlist');
			// Re-fetch to restore correct state
			if (selectedPlaylist) {
				await selectPlaylist(selectedPlaylist.id);
			}
		} finally {
			actionLoading = null;
		}
	}

	// Container-level handlers for external drag over the whole playlists panel
	function handlePanelDragOver(event: DragEvent) {
		if (!event.dataTransfer?.types.includes(DRAG_MIME)) return;
		event.preventDefault();
		event.dataTransfer.dropEffect = 'copy';
		externalDragOver = true;
	}

	function handlePanelDragLeave(event: DragEvent) {
		const relatedTarget = event.relatedTarget as HTMLElement | null;
		const currentTarget = event.currentTarget as HTMLElement;
		if (relatedTarget && currentTarget.contains(relatedTarget)) return;
		externalDragOver = false;
		dropTargetIndex = null;
	}

	async function handlePanelDrop(event: DragEvent) {
		// Ignore internal reorders — only handle external drops
		if (draggedIndex !== null) return;
		const mimeData = event.dataTransfer?.getData(DRAG_MIME);
		if (!mimeData) return;
		event.preventDefault();
		externalDragOver = false;
		dropTargetIndex = null;

		if (!selectedPlaylist) {
			toasts.error('Open a playlist first to add tracks');
			return;
		}

		try {
			const data = JSON.parse(mimeData);
			await addExternalTrackToPlaylist(data);
		} catch (e) {
			toasts.error('Failed to add track to playlist');
		}
	}

	// Track selection functions for multi-select removal
	function toggleTrackSelection(index: number) {
		const newSet = new Set(selectedIndices);
		if (newSet.has(index)) {
			newSet.delete(index);
		} else {
			newSet.add(index);
		}
		selectedIndices = newSet;

		// Auto-exit select mode if nothing selected
		if (newSet.size === 0) {
			selectMode = false;
		}
	}

	function toggleSelectAll() {
		if (!selectedPlaylist) return;
		if (allSelected) {
			selectedIndices = new Set();
			selectMode = false;
		} else {
			selectedIndices = new Set(selectedPlaylist.tracks.map((_, i) => i));
			selectMode = true;
		}
	}

	async function removeSelectedTracks() {
		if (!selectedPlaylist || selectedIndices.size === 0) return;

		const count = selectedIndices.size;
		if (!confirm(`Remove ${count} track${count > 1 ? 's' : ''} from "${selectedPlaylist.name}"?`)) return;

		try {
			actionLoading = 'removeSelected';
			// Remove selected indices from tracks (sort descending to avoid index shift)
			const indices = Array.from(selectedIndices).sort((a, b) => b - a);
			const newTracks = [...selectedPlaylist.tracks];
			for (const idx of indices) {
				newTracks.splice(idx, 1);
			}

			// Persist to backend
			await api.updatePlaylist(selectedPlaylist.id, { tracks: newTracks });
			selectedPlaylist = { ...selectedPlaylist, tracks: newTracks };
			selectedIndices = new Set();
			selectMode = false;

			// Update the playlist count in the list
			await loadPlaylists();
		} catch (e) {
			toasts.error('Failed to remove tracks');
			// Re-fetch to restore correct state
			if (selectedPlaylist) {
				await selectPlaylist(selectedPlaylist.id);
			}
		} finally {
			actionLoading = null;
		}
	}

	async function removeTrack(index: number, event: MouseEvent) {
		event.stopPropagation();
		if (!selectedPlaylist) return;

		try {
			actionLoading = `remove-${index}`;
			const newTracks = [...selectedPlaylist.tracks];
			newTracks.splice(index, 1);

			await api.updatePlaylist(selectedPlaylist.id, { tracks: newTracks });
			selectedPlaylist = { ...selectedPlaylist, tracks: newTracks };

			// Update the playlist count in the list
			await loadPlaylists();
		} catch (e) {
			toasts.error('Failed to remove track');
			if (selectedPlaylist) {
				await selectPlaylist(selectedPlaylist.id);
			}
		} finally {
			actionLoading = null;
		}
	}

	function clearSelectionOnBack() {
		selectedIndices = new Set();
		selectMode = false;
		selectedPlaylist = null;
	}

	function formatDate(dateStr: string): string {
		try {
			const date = new Date(dateStr);
			return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
		} catch {
			return '';
		}
	}

	function formatDuration(ms?: number): string {
		if (!ms || ms <= 0) return '';
		const totalSeconds = Math.floor(ms / 1000);
		const minutes = Math.floor(totalSeconds / 60);
		const seconds = totalSeconds % 60;
		return `${minutes}:${seconds.toString().padStart(2, '0')}`;
	}

	onMount(() => {
		loadPlaylists();
		return () => {
			unsubPlaylistRefresh();
		};
	});
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="flex flex-col rounded-lg border bg-zinc-900/50 transition-all {externalDragOver ? 'border-green-500 ring-2 ring-green-500/50' : 'border-zinc-800'}"
	class:h-full={fullHeight}
	ondragover={handlePanelDragOver}
	ondragleave={handlePanelDragLeave}
	ondrop={handlePanelDrop}
>
	<!-- Header -->
	<div class="flex flex-shrink-0 items-center justify-between border-b border-zinc-800">
		<button
			class="flex flex-1 items-center gap-2 px-4 py-3 text-left hover:bg-zinc-800/50"
			onclick={() => (collapsed = !collapsed)}
		>
			<ListMusic class="h-4 w-4 text-zinc-400" />
			<span class="text-sm font-medium text-zinc-200">Playlists</span>
			{#if playlists.length > 0}
				<span class="text-xs text-zinc-500">({playlists.length})</span>
			{/if}
			{#if loading}
				<Loader2 class="h-4 w-4 animate-spin text-zinc-500" />
			{/if}
		</button>

		<button
			class="px-3 py-3 text-zinc-500 transition-transform hover:bg-zinc-800/50"
			class:rotate-180={!collapsed}
			onclick={() => (collapsed = !collapsed)}
		>
			▼
		</button>
	</div>

	<!-- Content -->
	{#if !collapsed}
		<div class="flex min-h-0 flex-1 overflow-hidden">
			<!-- Playlist List -->
			<div class="flex min-h-0 flex-1 flex-col border-r border-zinc-800 overflow-hidden" class:hidden={selectedPlaylist}>
				{#if externalDragOver && !selectedPlaylist}
					<div class="px-4 py-6 text-center text-sm text-green-400">
						Open a playlist to drop tracks into it
					</div>
				{:else if error}
					<div class="px-4 py-6 text-center text-sm text-red-400">
						{error}
					</div>
				{:else if playlists.length === 0}
					<div class="px-4 py-6 text-center text-sm text-zinc-500">
						{#if loading}
							Loading playlists...
						{:else}
							No playlists saved yet
						{/if}
					</div>
				{:else}
					<div class="h-full overflow-y-auto" class:max-h-64={!fullHeight}>
						{#each playlists as pl (pl.id)}
							<div
								class="group flex items-center gap-3 px-4 py-2 transition-colors hover:bg-zinc-800/50"
							>
								<!-- Playlist info (clickable to view) -->
								<button
									class="flex flex-1 items-center gap-3 text-left"
									onclick={() => selectPlaylist(pl.id)}
								>
									<div class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded bg-zinc-700">
										<ListMusic class="h-4 w-4 text-zinc-400" />
									</div>
									<div class="min-w-0 flex-1">
										<p class="truncate text-sm text-zinc-200">{pl.name}</p>
										<p class="text-xs text-zinc-500">
											{pl.trackCount} tracks · {formatDate(pl.updatedAt)}
										</p>
									</div>
								</button>

								<!-- Actions -->
								<div class="flex items-center gap-1">
									{#if actionLoading === `load-${pl.id}` || actionLoading === `append-${pl.id}`}
										<Loader2 class="h-4 w-4 animate-spin text-zinc-400" />
									{:else}
									<button
										class="can-hover:hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-green-400 can-hover:group-hover:block"
										onclick={() => loadPlaylistToQueue(pl.id)}
										title="Play playlist (replace queue)"
									>
										<Play class="h-4 w-4" />
									</button>
									<button
										class="can-hover:hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-blue-400 can-hover:group-hover:block"
										onclick={() => loadPlaylistToQueue(pl.id, true)}
										title="Append to queue"
									>
										<ListPlus class="h-4 w-4" />
									</button>
									{/if}
								<button
									class="can-hover:hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-red-400 can-hover:group-hover:block"
									onclick={(e) => deletePlaylist(pl.id, e)}
										disabled={actionLoading === `delete-${pl.id}`}
										title="Delete playlist"
									>
										{#if actionLoading === `delete-${pl.id}`}
											<Loader2 class="h-4 w-4 animate-spin" />
										{:else}
											<Trash2 class="h-4 w-4" />
										{/if}
									</button>
									<ChevronRight class="h-4 w-4 text-zinc-600" />
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>

			<!-- Playlist Detail View -->
			{#if selectedPlaylist}
				<div class="flex min-h-0 flex-1 flex-col overflow-hidden">
					<!-- Detail header -->
					<div class="flex flex-shrink-0 items-center gap-2 border-b border-zinc-800 px-4 py-2">
						<button
							class="rounded p-1 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-zinc-200"
							onclick={clearSelectionOnBack}
							title="Back to list"
						>
							<ChevronLeft class="h-4 w-4" />
						</button>
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-medium text-zinc-200">{selectedPlaylist.name}</p>
							<p class="text-xs text-zinc-500">{selectedPlaylist.tracks.length} tracks</p>
						</div>
						{#if selectMode || selectedIndices.size > 0}
							<!-- Select mode controls -->
							<button
								class="rounded p-1.5 transition-colors hover:bg-zinc-700"
								class:text-green-400={allSelected}
								class:text-yellow-400={someSelected}
								class:text-zinc-400={selectedIndices.size === 0}
								onclick={toggleSelectAll}
								title={allSelected ? 'Deselect all' : 'Select all'}
							>
								{#if allSelected}
									<CheckSquare class="h-4 w-4" />
								{:else if someSelected}
									<MinusSquare class="h-4 w-4" />
								{:else}
									<Square class="h-4 w-4" />
								{/if}
							</button>
							{#if selectedIndices.size > 0}
								<span class="text-xs text-zinc-400">{selectedIndices.size} selected</span>
								<button
									class="rounded p-1.5 text-red-400 transition-colors hover:bg-zinc-700"
									onclick={removeSelectedTracks}
									disabled={actionLoading === 'removeSelected'}
									title="Remove selected tracks"
								>
									{#if actionLoading === 'removeSelected'}
										<Loader2 class="h-4 w-4 animate-spin" />
									{:else}
										<Trash2 class="h-4 w-4" />
									{/if}
								</button>
								<button
									class="rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700"
									onclick={() => { selectedIndices = new Set(); selectMode = false; }}
									title="Cancel selection"
								>
									<X class="h-4 w-4" />
								</button>
							{/if}
						{:else}
							<!-- Normal controls -->
							<button
								class="flex items-center gap-1 rounded bg-green-600 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-green-500"
								onclick={() => loadPlaylistToQueue(selectedPlaylist!.id)}
								disabled={actionLoading === `load-${selectedPlaylist.id}` || actionLoading === `append-${selectedPlaylist.id}`}
							>
								{#if actionLoading === `load-${selectedPlaylist.id}`}
									<Loader2 class="h-3.5 w-3.5 animate-spin" />
								{:else}
									<Play class="h-3.5 w-3.5" />
								{/if}
								Play
							</button>
							<button
								class="flex items-center gap-1 rounded border border-zinc-600 px-3 py-1 text-xs font-medium text-zinc-300 transition-colors hover:bg-zinc-700 hover:text-white"
								onclick={() => loadPlaylistToQueue(selectedPlaylist!.id, true)}
								disabled={actionLoading === `load-${selectedPlaylist.id}` || actionLoading === `append-${selectedPlaylist.id}`}
								title="Append to queue"
							>
								{#if actionLoading === `append-${selectedPlaylist.id}`}
									<Loader2 class="h-3.5 w-3.5 animate-spin" />
								{:else}
									<ListPlus class="h-3.5 w-3.5" />
								{/if}
								Append
							</button>
						{/if}
					</div>

					<!-- Track list -->
					<div class="min-h-0 flex-1 overflow-hidden">
						<div class="h-full overflow-y-auto" class:max-h-52={!fullHeight}>
						{#if selectedPlaylist.tracks.length === 0}
							<div class="px-4 py-6 text-center text-sm text-zinc-500">
								{#if externalDragOver}
									<span class="text-green-400">Drop here to add to playlist</span>
								{:else}
									No tracks in playlist
								{/if}
							</div>
						{:else}
						{#each selectedPlaylist.tracks as track, i (i)}
							<div
								class="group flex items-center gap-2 px-2 py-1.5 transition-colors hover:bg-zinc-800/50"
								class:bg-zinc-800={selectedIndices.has(i)}
								class:ring-1={selectedIndices.has(i)}
								class:ring-green-500={selectedIndices.has(i)}
								class:border-t-2={dropTargetIndex === i && (externalDragOver || (draggedIndex !== null && draggedIndex > i))}
								class:border-b-2={dropTargetIndex === i && draggedIndex !== null && draggedIndex < i}
								class:border-green-500={dropTargetIndex === i}
								role="listitem"
								draggable={!selectMode}
								ondragstart={(e) => !selectMode && handleDragStart(e, i)}
								ondragend={handleDragEnd}
								ondragover={(e) => handleDragOver(e, i)}
								ondragleave={handleDragLeave}
								ondrop={(e) => handleDrop(e, i)}
							>
								<!-- Checkbox (in select mode) or Drag handle (normal mode) -->
								{#if selectMode || selectedIndices.size > 0}
									<button
										class="flex-shrink-0 p-0.5 transition-colors"
										class:text-green-400={selectedIndices.has(i)}
										class:text-zinc-500={!selectedIndices.has(i)}
										onclick={() => toggleTrackSelection(i)}
										title={selectedIndices.has(i) ? 'Deselect' : 'Select'}
									>
										{#if selectedIndices.has(i)}
											<CheckSquare class="h-4 w-4" />
										{:else}
											<Square class="h-4 w-4" />
										{/if}
									</button>
								{:else}
									<div class="flex-shrink-0 cursor-grab text-zinc-600 hover:text-zinc-400 active:cursor-grabbing">
										<GripVertical class="h-4 w-4" />
									</div>
								{/if}
								<span class="w-5 text-center text-xs text-zinc-500">{i + 1}</span>
								{#if track.icon}
									<img
										src={track.icon}
										alt=""
										class="h-6 w-6 flex-shrink-0 rounded bg-zinc-700 object-cover"
										onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = 'none')}
									/>
								{:else}
									<div class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded bg-zinc-700">
										<Music class="h-3 w-3 text-zinc-500" />
									</div>
								{/if}
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm text-zinc-300">{track.title}</p>
									{#if track.artist}
										<p class="truncate text-xs text-zinc-500">{track.artist}</p>
									{/if}
								</div>
								{#if track.duration}
									<span class="text-xs text-zinc-500 can-hover:group-hover:hidden">{formatDuration(track.duration)}</span>
								{/if}
								<!-- Remove button (shown on hover, hidden in select mode) -->
								{#if !(selectMode || selectedIndices.size > 0)}
									<button
										class="can-hover:hidden flex-shrink-0 rounded p-1 text-zinc-500 transition-colors hover:bg-zinc-700 hover:text-red-400 can-hover:group-hover:block"
										onclick={(e) => removeTrack(i, e)}
										disabled={actionLoading === `remove-${i}`}
										title="Remove from playlist"
									>
										{#if actionLoading === `remove-${i}`}
											<Loader2 class="h-4 w-4 animate-spin" />
										{:else}
											<X class="h-4 w-4" />
										{/if}
									</button>
								{/if}
							</div>
						{/each}
						{/if}
						</div>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>
