<script lang="ts">
	import { player } from '$lib/stores/player';
	import { cn } from '$lib/utils/cn';
	import { Music } from 'lucide-svelte';

	// Format milliseconds to mm:ss
	function formatTime(ms: number): string {
		if (ms <= 0) return '0:00';
		const seconds = Math.floor(ms / 1000);
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	}

	// Use Svelte 5 $derived for reactive progress calculation
	const progress = $derived($player.duration > 0 ? ($player.position / $player.duration) * 100 : 0);
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
			{$player.artist || 'Unknown Artist'}
		</p>
		{#if $player.album}
			<p class="truncate text-sm text-zinc-500">
				{$player.album}
			</p>
		{/if}
	</div>

	<!-- Progress Bar -->
	<div class="group w-full max-w-md">
		<div 
			class="relative h-1.5 w-full cursor-pointer overflow-hidden rounded-full bg-zinc-700 transition-all group-hover:h-2"
			role="progressbar"
			aria-valuenow={progress}
			aria-valuemin={0}
			aria-valuemax={100}
			aria-label="Playback progress"
		>
			<div
				class="absolute left-0 top-0 h-full rounded-full bg-white transition-all duration-150"
				style="width: {progress}%"
			></div>
			<!-- Thumb indicator on hover -->
			<div 
				class="absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md transition-opacity group-hover:opacity-100"
				style="left: calc({progress}% - 6px)"
			></div>
		</div>
		<div class="mt-2 flex justify-between text-xs text-zinc-500">
			<span>{formatTime($player.position)}</span>
			<span>{formatTime($player.duration)}</span>
		</div>
	</div>
</div>
