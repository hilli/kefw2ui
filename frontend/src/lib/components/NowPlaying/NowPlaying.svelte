<script lang="ts">
	import { player } from '$lib/stores/player';
	import { browseNavigation } from '$lib/stores/browseNavigation';
	import { api } from '$lib/api/client';
	import { cn } from '$lib/utils/cn';
	import { Music } from 'lucide-svelte';

	// State for seeking
	let isSeeking = $state(false);
	let seekPosition = $state(0);
	let progressBarRef = $state<HTMLDivElement | null>(null);

	// Format milliseconds to mm:ss
	function formatTime(ms: number): string {
		if (ms <= 0) return '0:00';
		const seconds = Math.floor(ms / 1000);
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	}

	// Use Svelte 5 $derived for reactive progress calculation
	const progress = $derived(
		isSeeking
			? (seekPosition / $player.duration) * 100
			: $player.duration > 0
				? ($player.position / $player.duration) * 100
				: 0
	);

	// Calculate position from mouse/touch event
	function getPositionFromEvent(event: MouseEvent | TouchEvent): number {
		if (!progressBarRef || $player.duration <= 0) return 0;

		const rect = progressBarRef.getBoundingClientRect();
		const clientX = 'touches' in event ? event.touches[0].clientX : event.clientX;
		const relativeX = Math.max(0, Math.min(clientX - rect.left, rect.width));
		const percentage = relativeX / rect.width;
		return Math.round(percentage * $player.duration);
	}

	// Handle click on progress bar
	async function handleProgressClick(event: MouseEvent) {
		if ($player.duration <= 0) return;

		const positionMs = getPositionFromEvent(event);
		try {
			await api.seek(positionMs);
		} catch (e) {
			console.error('Seek failed:', e);
		}
	}

	// Handle drag start
	function handleDragStart(event: MouseEvent | TouchEvent) {
		if ($player.duration <= 0) return;

		event.preventDefault();
		isSeeking = true;
		seekPosition = getPositionFromEvent(event);

		// Add listeners for drag
		if ('touches' in event) {
			document.addEventListener('touchmove', handleDragMove);
			document.addEventListener('touchend', handleDragEnd);
		} else {
			document.addEventListener('mousemove', handleDragMove);
			document.addEventListener('mouseup', handleDragEnd);
		}
	}

	// Handle drag move
	function handleDragMove(event: MouseEvent | TouchEvent) {
		if (!isSeeking) return;
		seekPosition = getPositionFromEvent(event);
	}

	// Handle drag end
	async function handleDragEnd() {
		if (!isSeeking) return;

		// Remove listeners
		document.removeEventListener('mousemove', handleDragMove);
		document.removeEventListener('mouseup', handleDragEnd);
		document.removeEventListener('touchmove', handleDragMove);
		document.removeEventListener('touchend', handleDragEnd);

		// Perform seek
		try {
			await api.seek(seekPosition);
		} catch (e) {
			console.error('Seek failed:', e);
		}

		isSeeking = false;
	}

	// Display position (current or seeking)
	const displayPosition = $derived(isSeeking ? seekPosition : $player.position);
</script>

<div class="flex flex-col items-center gap-6 p-6">
	<!-- Album Artwork -->
	<div
		class={cn(
			'relative aspect-square w-full max-w-md overflow-hidden rounded-2xl',
			'bg-zinc-800 shadow-2xl'
		)}
	>
		{#if $player.artwork}
			<img
				src={$player.artwork}
				alt="{$player.title} - {$player.album}"
				class="h-full w-full object-cover"
			/>
		{:else}
			<div class="flex h-full w-full items-center justify-center bg-zinc-800">
				<Music class="h-24 w-24 text-zinc-600" />
			</div>
		{/if}

		<!-- Playing indicator overlay -->
		{#if $player.state === 'playing'}
			<div class="absolute bottom-4 right-4">
				<div class="flex items-end gap-1">
					{#each [1, 2, 3, 4] as bar}
						<div
							class="w-1 animate-pulse rounded-full bg-white"
							style="height: {8 + Math.random() * 12}px; animation-delay: {bar * 0.1}s"
						></div>
					{/each}
				</div>
			</div>
		{/if}
	</div>

	<!-- Track Info -->
	<div class="w-full max-w-md text-center">
		<h2 class="truncate text-2xl font-bold text-white">
			{$player.title || 'Not Playing'}
		</h2>
		<p class="truncate text-lg text-zinc-400">
			{#if $player.artist && $player.artist !== 'Unknown Artist'}
				<button
					class="hover:text-zinc-200 hover:underline"
					onclick={() => browseNavigation.searchByArtist($player.artist)}
					title={`Show all tracks by ${$player.artist}`}
				>
					{$player.artist}
				</button>
			{:else}
				{$player.artist || 'Unknown Artist'}
			{/if}
		</p>
		{#if $player.album}
			<p class="truncate text-sm text-zinc-500">
				<button
					class="hover:text-zinc-300 hover:underline"
					onclick={() => browseNavigation.searchByAlbum($player.album)}
					title={`Show all tracks from ${$player.album}`}
				>
					{$player.album}
				</button>
			</p>
		{/if}
	</div>

	<!-- Progress Bar with Seek -->
	<div class="group w-full max-w-md">
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			bind:this={progressBarRef}
			class="relative h-1.5 w-full cursor-pointer overflow-hidden rounded-full bg-zinc-700 transition-all group-hover:h-2"
			class:h-2={isSeeking}
			role="slider"
			aria-valuenow={displayPosition}
			aria-valuemin={0}
			aria-valuemax={$player.duration}
			aria-label="Seek position"
			tabindex={$player.duration > 0 ? 0 : -1}
			onclick={handleProgressClick}
			onmousedown={handleDragStart}
			ontouchstart={handleDragStart}
		>
			<!-- Progress fill -->
			<div
				class="absolute left-0 top-0 h-full rounded-full bg-white transition-all"
				class:duration-150={!isSeeking}
				style="width: {progress}%"
			></div>
			<!-- Thumb indicator -->
			<div
				class="absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-white shadow-md transition-opacity"
				class:opacity-0={!isSeeking}
				class:group-hover:opacity-100={$player.duration > 0}
				style="left: calc({progress}% - 6px)"
			></div>
		</div>
		<div class="mt-2 flex justify-between text-xs text-zinc-500">
			<span>{formatTime(displayPosition)}</span>
			<span>{formatTime($player.duration)}</span>
		</div>
	</div>
</div>
