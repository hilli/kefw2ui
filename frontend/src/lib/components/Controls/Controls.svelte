<script lang="ts">
	import { player } from '$lib/stores/player';
	import { api } from '$lib/api/client';
	import { cn } from '$lib/utils/cn';
	import {
		Play,
		Pause,
		SkipBack,
		SkipForward,
		Volume2,
		VolumeX,
		Volume1
	} from 'lucide-svelte';

	let volumeChanging = false;
	let volumeValue = $player.volume;

	// Sync local volume with store when not dragging
	$: if (!volumeChanging) {
		volumeValue = $player.volume;
	}

	async function handlePlayPause() {
		try {
			await api.playPause();
		} catch (error) {
			console.error('Play/pause failed:', error);
		}
	}

	async function handlePrevious() {
		try {
			await api.previousTrack();
		} catch (error) {
			console.error('Previous track failed:', error);
		}
	}

	async function handleNext() {
		try {
			await api.nextTrack();
		} catch (error) {
			console.error('Next track failed:', error);
		}
	}

	async function handleVolumeChange(event: Event) {
		const target = event.target as HTMLInputElement;
		volumeValue = parseInt(target.value, 10);
	}

	async function handleVolumeCommit() {
		volumeChanging = false;
		try {
			await api.setVolume(volumeValue);
		} catch (error) {
			console.error('Volume change failed:', error);
		}
	}

	async function handleMuteToggle() {
		try {
			await api.toggleMute();
		} catch (error) {
			console.error('Mute toggle failed:', error);
		}
	}

	function getVolumeIcon(volume: number, muted: boolean) {
		if (muted || volume === 0) return VolumeX;
		if (volume < 50) return Volume1;
		return Volume2;
	}

	$: VolumeIcon = getVolumeIcon($player.volume, $player.muted);
</script>

<div class="flex flex-col items-center gap-6 p-6">
	<!-- Playback Controls -->
	<div class="flex items-center justify-center gap-6">
		<button
			onclick={handlePrevious}
			class={cn(
				'rounded-full p-3 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900'
			)}
			aria-label="Previous track"
		>
			<SkipBack class="h-6 w-6" />
		</button>

		<button
			onclick={handlePlayPause}
			class={cn(
				'rounded-full p-4 transition-colors',
				'bg-white text-black hover:bg-zinc-200',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900'
			)}
			aria-label={$player.state === 'playing' ? 'Pause' : 'Play'}
		>
			{#if $player.state === 'playing'}
				<Pause class="h-8 w-8" />
			{:else}
				<Play class="h-8 w-8 ml-1" />
			{/if}
		</button>

		<button
			onclick={handleNext}
			class={cn(
				'rounded-full p-3 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900'
			)}
			aria-label="Next track"
		>
			<SkipForward class="h-6 w-6" />
		</button>
	</div>

	<!-- Volume Control -->
	<div class="flex w-full max-w-md items-center gap-4">
		<button
			onclick={handleMuteToggle}
			class={cn(
				'rounded-full p-2 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900'
			)}
			aria-label={$player.muted ? 'Unmute' : 'Mute'}
		>
			<svelte:component this={VolumeIcon} class="h-5 w-5" />
		</button>

		<input
			type="range"
			min="0"
			max="100"
			bind:value={volumeValue}
			onmousedown={() => (volumeChanging = true)}
			ontouchstart={() => (volumeChanging = true)}
			oninput={handleVolumeChange}
			onmouseup={handleVolumeCommit}
			ontouchend={handleVolumeCommit}
			class={cn(
				'h-1 w-full cursor-pointer appearance-none rounded-full bg-zinc-700',
				'[&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4',
				'[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full',
				'[&::-webkit-slider-thumb]:bg-white',
				'[&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4',
				'[&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-white',
				'[&::-moz-range-thumb]:border-0'
			)}
			aria-label="Volume"
		/>

		<span class="w-10 text-right text-sm text-zinc-400">
			{volumeValue}%
		</span>
	</div>

	<!-- Source Indicator -->
	<div class="flex items-center gap-2 text-sm text-zinc-500">
		<span class="capitalize">{$player.source}</span>
	</div>
</div>
