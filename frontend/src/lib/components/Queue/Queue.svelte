<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { player } from '$lib/stores/player';
	import { queueRefresh } from '$lib/stores/queue';
	import { Music, Play, Loader2, X, Trash2, Shuffle, Repeat, Repeat1, GripVertical, Save, CheckSquare, Square, MinusSquare } from 'lucide-svelte';
	import { browseNavigation } from '$lib/stores/browseNavigation';

	interface Props {
		fullHeight?: boolean;
	}

	let { fullHeight = false }: Props = $props();

	interface Track {
		index: number;
		title: string;
		artist?: string;
		album?: string;
		id: string;
		path: string;
		icon?: string;
		type: string;
		duration: number;
	}

	let tracks = $state<Track[]>([]);
	let currentIndex = $state(-1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let collapsed = $state(false);
	let shuffle = $state(false);
	let repeat = $state<'off' | 'one' | 'all'>('off');
	let actionLoading = $state<number | string | null>(null);
	
	// Drag and drop state
	let draggedIndex = $state<number | null>(null);
	let dropTargetIndex = $state<number | null>(null);
	
	// Multi-select state
	let selectedIndices = $state<Set<number>>(new Set());
	let selectMode = $state(false);
	
	// Derived state for select all
	let allSelected = $derived(tracks.length > 0 && selectedIndices.size === tracks.length);
	let someSelected = $derived(selectedIndices.size > 0 && selectedIndices.size < tracks.length);

	async function loadQueue() {
		try {
			loading = true;
			error = null;
			const [queueResponse, modeResponse] = await Promise.all([
				api.getQueue(),
				api.getPlayMode().catch(() => ({ mode: 'normal', shuffle: false, repeat: 'off' as const }))
			]);
			tracks = queueResponse.tracks || [];
			currentIndex = queueResponse.currentIndex ?? -1;
			shuffle = modeResponse.shuffle;
			repeat = modeResponse.repeat;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load queue';
			tracks = [];
		} finally {
			loading = false;
		}
	}

	function formatDuration(ms: number): string {
		if (!ms || ms <= 0) return '';
		const totalSeconds = Math.floor(ms / 1000);
		const minutes = Math.floor(totalSeconds / 60);
		const seconds = totalSeconds % 60;
		return `${minutes}:${seconds.toString().padStart(2, '0')}`;
	}

	async function playTrack(index: number) {
		try {
			actionLoading = index;
			await api.playQueueTrack(index);
			currentIndex = index;
		} catch (e) {
			console.error('Failed to play track:', e);
		} finally {
			actionLoading = null;
		}
	}

	async function removeTrack(index: number, event: MouseEvent) {
		event.stopPropagation();
		try {
			actionLoading = `remove-${index}`;
			await api.removeFromQueue([index]);
			// Reload queue to get updated state
			await loadQueue();
		} catch (e) {
			console.error('Failed to remove track:', e);
		} finally {
			actionLoading = null;
		}
	}

	async function clearQueue() {
		if (!confirm('Clear all tracks from the queue?')) return;
		try {
			actionLoading = 'clear';
			await api.clearQueue();
			tracks = [];
			currentIndex = -1;
		} catch (e) {
			console.error('Failed to clear queue:', e);
		} finally {
			actionLoading = null;
		}
	}

	async function toggleShuffle() {
		try {
			actionLoading = 'shuffle';
			const result = await api.toggleShuffle();
			shuffle = result.shuffle;
			repeat = result.repeat;
		} catch (e) {
			console.error('Failed to toggle shuffle:', e);
		} finally {
			actionLoading = null;
		}
	}

	async function cycleRepeat() {
		try {
			actionLoading = 'repeat';
			const result = await api.cycleRepeat();
			shuffle = result.shuffle;
			repeat = result.repeat;
		} catch (e) {
			console.error('Failed to cycle repeat:', e);
		} finally {
			actionLoading = null;
		}
	}

	async function saveAsPlaylist() {
		const name = prompt('Enter playlist name:', `Queue - ${new Date().toLocaleDateString()}`);
		if (!name) return;
		
		try {
			actionLoading = 'save';
			await api.saveQueueAsPlaylist(name);
			// Could show a success toast here
		} catch (e) {
			console.error('Failed to save playlist:', e);
			alert('Failed to save playlist: ' + (e instanceof Error ? e.message : 'Unknown error'));
		} finally {
			actionLoading = null;
		}
	}

	// Multi-select functions
	function toggleSelectMode() {
		selectMode = !selectMode;
		if (!selectMode) {
			selectedIndices = new Set();
		}
	}

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
		if (allSelected) {
			// Deselect all
			selectedIndices = new Set();
			selectMode = false;
		} else {
			// Select all
			selectedIndices = new Set(tracks.map((_, i) => i));
			selectMode = true;
		}
	}

	async function removeSelectedTracks() {
		if (selectedIndices.size === 0) return;
		
		const count = selectedIndices.size;
		if (!confirm(`Remove ${count} track${count > 1 ? 's' : ''} from the queue?`)) return;
		
		try {
			actionLoading = 'removeSelected';
			// Sort indices in descending order to remove from end first
			const indices = Array.from(selectedIndices).sort((a, b) => b - a);
			await api.removeFromQueue(indices);
			selectedIndices = new Set();
			selectMode = false;
			await loadQueue();
		} catch (e) {
			console.error('Failed to remove tracks:', e);
		} finally {
			actionLoading = null;
		}
	}

	// Drag and drop handlers
	function handleDragStart(event: DragEvent, index: number) {
		if (!event.dataTransfer) return;
		draggedIndex = index;
		event.dataTransfer.effectAllowed = 'move';
		event.dataTransfer.setData('text/plain', index.toString());
		
		// Add a slight delay to allow the drag image to be set
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
	}

	function handleDragOver(event: DragEvent, index: number) {
		event.preventDefault();
		if (!event.dataTransfer) return;
		event.dataTransfer.dropEffect = 'move';
		
		if (draggedIndex !== null && draggedIndex !== index) {
			dropTargetIndex = index;
		}
	}

	function handleDragLeave() {
		dropTargetIndex = null;
	}

	async function handleDrop(event: DragEvent, toIndex: number) {
		event.preventDefault();
		dropTargetIndex = null;
		
		if (draggedIndex === null || draggedIndex === toIndex) {
			draggedIndex = null;
			return;
		}

		const fromIndex = draggedIndex;
		draggedIndex = null;

		try {
			actionLoading = 'move';
			
			// Optimistic UI update
			const newTracks = [...tracks];
			const [movedTrack] = newTracks.splice(fromIndex, 1);
			newTracks.splice(toIndex, 0, movedTrack);
			tracks = newTracks;
			
			// Update current index if needed
			if (currentIndex === fromIndex) {
				currentIndex = toIndex;
			} else if (fromIndex < currentIndex && toIndex >= currentIndex) {
				currentIndex--;
			} else if (fromIndex > currentIndex && toIndex <= currentIndex) {
				currentIndex++;
			}
			
			// Call API
			await api.moveQueueItem(fromIndex, toIndex);
			
			// Reload to ensure sync
			await loadQueue();
		} catch (e) {
			console.error('Failed to move track:', e);
			// Reload queue on error to restore correct state
			await loadQueue();
		} finally {
			actionLoading = null;
		}
	}

	onMount(() => {
		loadQueue();
		
		// Refresh queue periodically
		const interval = setInterval(loadQueue, 30000);
		
		// Subscribe to player changes
		const unsubscribePlayer = player.subscribe(() => {
			// Queue updates come via SSE, just update currentIndex based on title match
		});
		
		// Subscribe to manual refresh triggers (e.g., when Browser adds to queue)
		const unsubscribeRefresh = queueRefresh.subscribe(() => {
			// Skip initial subscription call, only refresh on subsequent triggers
			loadQueue();
		});

		return () => {
			clearInterval(interval);
			unsubscribePlayer();
			unsubscribeRefresh();
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
			<Music class="h-4 w-4 text-zinc-400" />
			<span class="text-sm font-medium text-zinc-200">Queue</span>
			{#if tracks.length > 0}
				<span class="text-xs text-zinc-500">({tracks.length} tracks)</span>
			{/if}
			{#if loading}
				<Loader2 class="h-4 w-4 animate-spin text-zinc-500" />
			{/if}
		</button>
		
		<!-- Queue controls -->
		{#if !collapsed && tracks.length > 0}
			<div class="flex items-center gap-1 px-2">
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
						
						<!-- Remove selected -->
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
						
						<!-- Cancel selection -->
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
					<!-- Save as playlist -->
					<button
						class="rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-zinc-200"
						onclick={saveAsPlaylist}
						disabled={actionLoading === 'save'}
						title="Save queue as playlist"
					>
						{#if actionLoading === 'save'}
							<Loader2 class="h-4 w-4 animate-spin" />
						{:else}
							<Save class="h-4 w-4" />
						{/if}
					</button>
					
					<!-- Shuffle -->
					<button
						class="rounded p-1.5 transition-colors hover:bg-zinc-700"
						class:text-green-400={shuffle}
						class:text-zinc-400={!shuffle}
						onclick={toggleShuffle}
						disabled={actionLoading === 'shuffle'}
						title={shuffle ? 'Shuffle on' : 'Shuffle off'}
					>
						{#if actionLoading === 'shuffle'}
							<Loader2 class="h-4 w-4 animate-spin" />
						{:else}
							<Shuffle class="h-4 w-4" />
						{/if}
					</button>
					
					<!-- Repeat -->
					<button
						class="rounded p-1.5 transition-colors hover:bg-zinc-700"
						class:text-green-400={repeat !== 'off'}
						class:text-zinc-400={repeat === 'off'}
						onclick={cycleRepeat}
						disabled={actionLoading === 'repeat'}
						title={repeat === 'off' ? 'Repeat off' : repeat === 'one' ? 'Repeat one' : 'Repeat all'}
					>
						{#if actionLoading === 'repeat'}
							<Loader2 class="h-4 w-4 animate-spin" />
						{:else if repeat === 'one'}
							<Repeat1 class="h-4 w-4" />
						{:else}
							<Repeat class="h-4 w-4" />
						{/if}
					</button>
					
					<!-- Clear queue -->
					<button
						class="rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-red-400"
						onclick={clearQueue}
						disabled={actionLoading === 'clear'}
						title="Clear queue"
					>
						{#if actionLoading === 'clear'}
							<Loader2 class="h-4 w-4 animate-spin" />
						{:else}
							<Trash2 class="h-4 w-4" />
						{/if}
					</button>
				{/if}
			</div>
		{/if}
		
		<button
			class="px-3 py-3 text-zinc-500 transition-transform hover:bg-zinc-800/50"
			class:rotate-180={!collapsed}
			onclick={() => (collapsed = !collapsed)}
		>
			▼
		</button>
	</div>

	<!-- Queue List -->
	{#if !collapsed}
		<div class="min-h-0 flex-1 overflow-hidden">
			{#if error}
				<div class="px-4 py-6 text-center text-sm text-red-400">
					{error}
				</div>
			{:else if tracks.length === 0}
				<div class="px-4 py-6 text-center text-sm text-zinc-500">
					{#if loading}
						Loading queue...
					{:else}
						No tracks in queue
					{/if}
				</div>
			{:else}
				<div class="h-full overflow-y-auto" class:max-h-64={!fullHeight}>
					{#each tracks as track, i (track.id || i)}
						<div
							class="group flex items-center gap-2 px-2 py-2 transition-colors hover:bg-zinc-800/50"
							class:bg-zinc-800={i === currentIndex || selectedIndices.has(i)}
							class:border-t-2={dropTargetIndex === i && draggedIndex !== null && draggedIndex > i}
							class:border-b-2={dropTargetIndex === i && draggedIndex !== null && draggedIndex < i}
							class:border-green-500={dropTargetIndex === i}
							class:ring-1={selectedIndices.has(i)}
							class:ring-green-500={selectedIndices.has(i)}
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
							
							<!-- Track number or playing indicator (clickable to play) -->
							<button
								class="flex w-6 flex-shrink-0 items-center justify-center"
								onclick={() => playTrack(i)}
								disabled={actionLoading === i}
								title="Play this track"
							>
								{#if actionLoading === i}
									<Loader2 class="h-4 w-4 animate-spin text-zinc-400" />
								{:else if i === currentIndex}
									<Play class="h-4 w-4 text-green-400" />
								{:else}
									<span class="text-xs text-zinc-500 group-hover:hidden">{i + 1}</span>
									<Play class="hidden h-4 w-4 text-zinc-400 group-hover:block" />
								{/if}
							</button>

							<!-- Track icon/artwork -->
							{#if track.icon}
								<img
									src={track.icon}
									alt=""
									class="h-8 w-8 flex-shrink-0 rounded bg-zinc-700 object-cover"
									onerror={(e) => (e.currentTarget as HTMLImageElement).style.display = 'none'}
								/>
							{:else}
								<div class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded bg-zinc-700">
									<Music class="h-4 w-4 text-zinc-500" />
								</div>
							{/if}

							<!-- Track info -->
							<div class="min-w-0 flex-1">
								<p
									class="truncate text-sm"
									class:text-green-400={i === currentIndex}
									class:text-zinc-200={i !== currentIndex}
								>
									{track.title || 'Unknown Track'}
								</p>
								{#if track.artist || track.album}
									<p class="truncate text-xs text-zinc-500">
										{#if track.artist}
											<button
												class="hover:text-zinc-300 hover:underline"
												onclick={(e) => {
													e.stopPropagation();
													browseNavigation.searchByArtist(track.artist!);
												}}
												title={`Show all tracks by ${track.artist}`}
											>
												{track.artist}
											</button>
										{/if}
										{#if track.artist && track.album}
											<span class="mx-0.5">·</span>
										{/if}
										{#if track.album}
											<button
												class="hover:text-zinc-300 hover:underline"
												onclick={(e) => {
													e.stopPropagation();
													browseNavigation.searchByAlbum(track.album!);
												}}
												title={`Show all tracks from ${track.album}`}
											>
												{track.album}
											</button>
										{/if}
									</p>
								{/if}
							</div>

							<!-- Duration -->
							{#if track.duration > 0}
								<span class="flex-shrink-0 text-xs text-zinc-500 group-hover:hidden">
									{formatDuration(track.duration)}
								</span>
							{/if}
							
							<!-- Remove button (shown on hover) -->
							<button
								class="hidden flex-shrink-0 rounded p-1 text-zinc-500 transition-colors hover:bg-zinc-700 hover:text-red-400 group-hover:block"
								onclick={(e) => removeTrack(i, e)}
								disabled={actionLoading === `remove-${i}`}
								title="Remove from queue"
							>
								{#if actionLoading === `remove-${i}`}
									<Loader2 class="h-4 w-4 animate-spin" />
								{:else}
									<X class="h-4 w-4" />
								{/if}
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
