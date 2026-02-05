<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { player } from '$lib/stores/player';
	import { Music, Play, Loader2 } from 'lucide-svelte';

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

	async function loadQueue() {
		try {
			loading = true;
			error = null;
			const response = await api.getQueue();
			tracks = response.tracks || [];
			currentIndex = response.currentIndex ?? -1;
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

	onMount(() => {
		loadQueue();
		
		// Refresh queue periodically or when player state changes
		const interval = setInterval(loadQueue, 30000); // Every 30 seconds
		
		// Subscribe to player changes to refresh queue
		const unsubscribe = player.subscribe(() => {
			// Debounce queue refresh on player changes
			// loadQueue(); // Can be too aggressive, disabled for now
		});

		return () => {
			clearInterval(interval);
			unsubscribe();
		};
	});
</script>

<div class="rounded-lg border border-zinc-800 bg-zinc-900/50">
	<!-- Header -->
	<button
		class="flex w-full items-center justify-between px-4 py-3 text-left hover:bg-zinc-800/50"
		onclick={() => (collapsed = !collapsed)}
	>
		<div class="flex items-center gap-2">
			<Music class="h-4 w-4 text-zinc-400" />
			<span class="text-sm font-medium text-zinc-200">Queue</span>
			{#if tracks.length > 0}
				<span class="text-xs text-zinc-500">({tracks.length} tracks)</span>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			{#if loading}
				<Loader2 class="h-4 w-4 animate-spin text-zinc-500" />
			{/if}
			<span class="text-zinc-500 transition-transform" class:rotate-180={!collapsed}>
				▼
			</span>
		</div>
	</button>

	<!-- Queue List -->
	{#if !collapsed}
		<div class="border-t border-zinc-800">
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
				<div class="max-h-64 overflow-y-auto">
					{#each tracks as track, i (track.id || i)}
						<div
							class="flex items-center gap-3 px-4 py-2 transition-colors hover:bg-zinc-800/50"
							class:bg-zinc-800={i === currentIndex}
						>
							<!-- Track number or playing indicator -->
							<div class="w-6 flex-shrink-0 text-center">
								{#if i === currentIndex}
									<Play class="h-4 w-4 text-green-400" />
								{:else}
									<span class="text-xs text-zinc-500">{i + 1}</span>
								{/if}
							</div>

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
								{#if track.artist}
									<p class="truncate text-xs text-zinc-500">
										{track.artist}
										{#if track.album}
											· {track.album}
										{/if}
									</p>
								{/if}
							</div>

							<!-- Duration -->
							{#if track.duration > 0}
								<span class="flex-shrink-0 text-xs text-zinc-500">
									{formatDuration(track.duration)}
								</span>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
