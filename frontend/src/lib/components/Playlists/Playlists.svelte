<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { toasts } from '$lib/stores/toast';
	import {
		ListMusic,
		Play,
		Plus,
		Trash2,
		Loader2,
		ChevronRight,
		ChevronLeft,
		Save,
		Music
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
	let showSaveDialog = $state(false);
	let newPlaylistName = $state('');
	let newPlaylistDescription = $state('');

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
			actionLoading = `load-${id}`;
			await api.loadPlaylist(id, append);
			// Could show success message
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

	async function saveQueueAsPlaylist() {
		if (!newPlaylistName.trim()) return;

		try {
			actionLoading = 'save';
			await api.saveQueueAsPlaylist(newPlaylistName.trim(), newPlaylistDescription.trim());
			showSaveDialog = false;
			newPlaylistName = '';
			newPlaylistDescription = '';
			await loadPlaylists();
		} catch (e) {
			toasts.error('Failed to save playlist');
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

		<!-- Save queue button -->
		{#if !collapsed}
			<button
				class="mr-2 flex items-center gap-1 rounded px-2 py-1 text-xs text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-zinc-200"
				onclick={() => (showSaveDialog = true)}
				title="Save current queue as playlist"
			>
				<Save class="h-3.5 w-3.5" />
				<span>Save Queue</span>
			</button>
		{/if}

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
									{#if actionLoading === `load-${pl.id}`}
										<Loader2 class="h-4 w-4 animate-spin text-zinc-400" />
									{:else}
										<button
											class="hidden rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-green-400 group-hover:block"
											onclick={() => loadPlaylistToQueue(pl.id)}
											title="Play playlist"
										>
											<Play class="h-4 w-4" />
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
							disabled={actionLoading === `load-${selectedPlaylist.id}`}
						>
							{#if actionLoading === `load-${selectedPlaylist.id}`}
								<Loader2 class="h-3.5 w-3.5 animate-spin" />
							{:else}
								<Play class="h-3.5 w-3.5" />
							{/if}
							Play
						</button>
					</div>

					<!-- Track list -->
					<div class="min-h-0 flex-1 overflow-hidden">
						<div class="h-full overflow-y-auto" class:max-h-52={!fullHeight}>
						{#each selectedPlaylist.tracks as track, i (i)}
							<div class="flex items-center gap-3 px-4 py-1.5">
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

<!-- Save Queue Dialog -->
{#if showSaveDialog}
	<button
		class="fixed inset-0 z-50 cursor-default bg-black/60 backdrop-blur-sm"
		onclick={() => (showSaveDialog = false)}
		aria-label="Close dialog"
	></button>
	<div class="fixed left-1/2 top-1/2 z-50 w-full max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-lg border border-zinc-700 bg-zinc-800 p-5 shadow-xl">
		<h3 class="mb-4 text-lg font-semibold text-white">Save Queue as Playlist</h3>

		<div class="space-y-3">
			<div>
				<label for="playlist-name" class="mb-1 block text-sm text-zinc-400">Name</label>
				<input
					id="playlist-name"
					type="text"
					class="w-full rounded border border-zinc-600 bg-zinc-700 px-3 py-2 text-sm text-white placeholder-zinc-500 focus:border-zinc-500 focus:outline-none"
					placeholder="My Playlist"
					bind:value={newPlaylistName}
					onkeydown={(e) => e.key === 'Enter' && saveQueueAsPlaylist()}
				/>
			</div>
			<div>
				<label for="playlist-desc" class="mb-1 block text-sm text-zinc-400">Description (optional)</label>
				<input
					id="playlist-desc"
					type="text"
					class="w-full rounded border border-zinc-600 bg-zinc-700 px-3 py-2 text-sm text-white placeholder-zinc-500 focus:border-zinc-500 focus:outline-none"
					placeholder="A great mix..."
					bind:value={newPlaylistDescription}
				/>
			</div>
		</div>

		<div class="mt-4 flex justify-end gap-2">
			<button
				class="rounded px-4 py-2 text-sm text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-white"
				onclick={() => (showSaveDialog = false)}
			>
				Cancel
			</button>
			<button
				class="flex items-center gap-2 rounded bg-green-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-green-500 disabled:opacity-50"
				onclick={saveQueueAsPlaylist}
				disabled={!newPlaylistName.trim() || actionLoading === 'save'}
			>
				{#if actionLoading === 'save'}
					<Loader2 class="h-4 w-4 animate-spin" />
				{/if}
				Save
			</button>
		</div>
	</div>
{/if}
