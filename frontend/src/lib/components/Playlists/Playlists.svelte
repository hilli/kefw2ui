<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { toasts } from '$lib/stores/toast';
	import {
		ListMusic,
		Play,
		Trash2,
		Loader2,
		ChevronRight,
		ChevronLeft,
		Music,
		GripVertical,
		ListPlus
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
		if (!event.dataTransfer) return;
		draggedIndex = index;
		event.dataTransfer.effectAllowed = 'move';
		event.dataTransfer.setData('text/plain', index.toString());

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

		if (!selectedPlaylist || draggedIndex === null || draggedIndex === toIndex) {
			draggedIndex = null;
			return;
		}

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
	});
</script>

<div class="flex flex-col rounded-lg border border-zinc-800 bg-zinc-900/50" class:h-full={fullHeight}>
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
				{#if error}
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
											class="hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-green-400 group-hover:block"
											onclick={() => loadPlaylistToQueue(pl.id)}
											title="Play playlist (replace queue)"
										>
											<Play class="h-4 w-4" />
										</button>
										<button
											class="hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-blue-400 group-hover:block"
											onclick={() => loadPlaylistToQueue(pl.id, true)}
											title="Append to queue"
										>
											<ListPlus class="h-4 w-4" />
										</button>
									{/if}
									<button
										class="hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-red-400 group-hover:block"
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
							onclick={() => (selectedPlaylist = null)}
							title="Back to list"
						>
							<ChevronLeft class="h-4 w-4" />
						</button>
						<div class="min-w-0 flex-1">
							<p class="truncate text-sm font-medium text-zinc-200">{selectedPlaylist.name}</p>
							<p class="text-xs text-zinc-500">{selectedPlaylist.tracks.length} tracks</p>
						</div>
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
					</div>

					<!-- Track list -->
					<div class="min-h-0 flex-1 overflow-hidden">
						<div class="h-full overflow-y-auto" class:max-h-52={!fullHeight}>
						{#each selectedPlaylist.tracks as track, i (i)}
							<div
								class="group flex items-center gap-2 px-2 py-1.5 transition-colors hover:bg-zinc-800/50"
								class:border-t-2={dropTargetIndex === i && draggedIndex !== null && draggedIndex > i}
								class:border-b-2={dropTargetIndex === i && draggedIndex !== null && draggedIndex < i}
								class:border-green-500={dropTargetIndex === i}
								role="listitem"
								draggable={true}
								ondragstart={(e) => handleDragStart(e, i)}
								ondragend={handleDragEnd}
								ondragover={(e) => handleDragOver(e, i)}
								ondragleave={handleDragLeave}
								ondrop={(e) => handleDrop(e, i)}
							>
								<div class="flex-shrink-0 cursor-grab text-zinc-600 hover:text-zinc-400 active:cursor-grabbing">
									<GripVertical class="h-4 w-4" />
								</div>
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
									<span class="text-xs text-zinc-500">{formatDuration(track.duration)}</span>
								{/if}
							</div>
						{/each}
						</div>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</div>
