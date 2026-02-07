<script lang="ts">
	import { player, connectionStatus } from '$lib/stores/player';
	import { api } from '$lib/api/client';
	import { cn } from '$lib/utils/cn';
	import { toasts } from '$lib/stores/toast';
	import {
		Play,
		Pause,
		SkipBack,
		SkipForward,
		Volume2,
		VolumeX,
		Volume1,
		Power,
		Shuffle,
		Repeat,
		Repeat1
	} from 'lucide-svelte';

	// Derived: are controls disabled due to disconnection?
	const isDisconnected = $derived($connectionStatus !== 'connected');

	// Local state for volume control
	let volumeChanging = $state(false);
	let volumeValue = $state($player.volume);
	let isPowerChanging = $state(false);

	// Play mode state
	let shuffle = $state(false);
	let repeat = $state<'off' | 'one' | 'all'>('off');
	let modeLoading = $state(false);

	// Sync local volume with store when not dragging
	$effect(() => {
		if (!volumeChanging) {
			volumeValue = $player.volume;
		}
	});

	// Load play mode on mount
	$effect(() => {
		loadPlayMode();
	});

	async function loadPlayMode() {
		try {
			const mode = await api.getPlayMode();
			shuffle = mode.shuffle;
			repeat = mode.repeat;
		} catch (e) {
			console.error('Failed to load play mode:', e);
		}
	}

	async function handlePlayPause() {
		try {
			await api.playPause();
		} catch (error) {
			toasts.error('Play/pause failed');
		}
	}

	async function handlePrevious() {
		try {
			await api.previousTrack();
		} catch (error) {
			toasts.error('Previous track failed');
		}
	}

	async function handleNext() {
		try {
			await api.nextTrack();
		} catch (error) {
			toasts.error('Next track failed');
		}
	}

	function handleVolumeChange(event: Event) {
		const target = event.target as HTMLInputElement;
		volumeValue = parseInt(target.value, 10);
	}

	async function handleVolumeCommit() {
		volumeChanging = false;
		try {
			await api.setVolume(volumeValue);
		} catch (error) {
			toasts.error('Volume change failed');
		}
	}

	async function handleMuteToggle() {
		try {
			await api.toggleMute();
		} catch (error) {
			toasts.error('Mute toggle failed');
		}
	}

	async function handlePowerToggle() {
		if (isPowerChanging) return;
		isPowerChanging = true;
		try {
			await api.togglePower();
		} catch (error) {
			toasts.error('Power toggle failed');
		} finally {
			isPowerChanging = false;
		}
	}

	async function handleShuffleToggle() {
		if (modeLoading) return;
		modeLoading = true;
		try {
			const result = await api.toggleShuffle();
			shuffle = result.shuffle;
			repeat = result.repeat;
		} catch (error) {
			toasts.error('Shuffle toggle failed');
		} finally {
			modeLoading = false;
		}
	}

	async function handleRepeatCycle() {
		if (modeLoading) return;
		modeLoading = true;
		try {
			const result = await api.cycleRepeat();
			shuffle = result.shuffle;
			repeat = result.repeat;
		} catch (error) {
			toasts.error('Repeat cycle failed');
		} finally {
			modeLoading = false;
		}
	}

	// Derived values
	const volumePercent = $derived(volumeValue);
</script>

<div class={cn('flex flex-col items-center gap-6 p-6', isDisconnected && 'opacity-50 pointer-events-none')}>
	<!-- Playback Controls -->
	<div class="flex items-center justify-center gap-4">
		<!-- Shuffle Button -->
		<button
			onclick={handleShuffleToggle}
			disabled={isDisconnected || modeLoading}
			class={cn(
				'rounded-full p-2 transition-colors',
				shuffle
					? 'text-green-500 hover:bg-zinc-800 hover:text-green-400'
					: 'text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label={shuffle ? 'Disable shuffle' : 'Enable shuffle'}
			title={shuffle ? 'Shuffle on' : 'Shuffle off'}
		>
			<Shuffle class="h-4 w-4" />
		</button>

		<button
			onclick={handlePrevious}
			disabled={isDisconnected}
			class={cn(
				'rounded-full p-3 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label="Previous track"
		>
			<SkipBack class="h-6 w-6" />
		</button>

		<button
			onclick={handlePlayPause}
			disabled={isDisconnected}
			class={cn(
				'rounded-full p-4 transition-colors',
				'bg-white text-black hover:bg-zinc-200',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label={$player.state === 'playing' ? 'Pause' : 'Play'}
		>
			{#if $player.state === 'playing'}
				<Pause class="h-8 w-8" />
			{:else}
				<Play class="ml-1 h-8 w-8" />
			{/if}
		</button>

		<button
			onclick={handleNext}
			disabled={isDisconnected}
			class={cn(
				'rounded-full p-3 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label="Next track"
		>
			<SkipForward class="h-6 w-6" />
		</button>

		<!-- Repeat Button -->
		<button
			onclick={handleRepeatCycle}
			disabled={isDisconnected || modeLoading}
			class={cn(
				'rounded-full p-2 transition-colors',
				repeat !== 'off'
					? 'text-green-500 hover:bg-zinc-800 hover:text-green-400'
					: 'text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label={repeat === 'off' ? 'Enable repeat all' : repeat === 'all' ? 'Enable repeat one' : 'Disable repeat'}
			title={repeat === 'off' ? 'Repeat off' : repeat === 'all' ? 'Repeat all' : 'Repeat one'}
		>
			{#if repeat === 'one'}
				<Repeat1 class="h-4 w-4" />
			{:else}
				<Repeat class="h-4 w-4" />
			{/if}
		</button>
	</div>

	<!-- Volume Control -->
	<div class="flex w-full max-w-md items-center gap-4">
		<button
			onclick={handleMuteToggle}
			disabled={isDisconnected}
			class={cn(
				'rounded-full p-2 transition-colors',
				'text-zinc-400 hover:bg-zinc-800 hover:text-white',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label={$player.muted ? 'Unmute' : 'Mute'}
		>
			{#if $player.muted || $player.volume === 0}
				<VolumeX class="h-5 w-5" />
			{:else if $player.volume < 50}
				<Volume1 class="h-5 w-5" />
			{:else}
				<Volume2 class="h-5 w-5" />
			{/if}
		</button>

		<!-- Custom volume slider with visual fill -->
		<div class="relative flex-1">
			<div class="relative h-1 w-full rounded-full bg-zinc-700">
				<!-- Fill indicator -->
				<div
					class="absolute left-0 top-0 h-full rounded-full bg-white transition-all duration-75"
					style="width: {volumePercent}%"
				></div>
			</div>
			<input
				type="range"
				min="0"
				max="100"
				bind:value={volumeValue}
				disabled={isDisconnected}
				onmousedown={() => (volumeChanging = true)}
				ontouchstart={() => (volumeChanging = true)}
				oninput={handleVolumeChange}
				onmouseup={handleVolumeCommit}
				ontouchend={handleVolumeCommit}
				class={cn(
					'absolute inset-0 h-1 w-full cursor-pointer appearance-none bg-transparent',
					'[&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4',
					'[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full',
					'[&::-webkit-slider-thumb]:bg-white [&::-webkit-slider-thumb]:shadow-md',
					'[&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:w-4',
					'[&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:bg-white',
					'[&::-moz-range-thumb]:border-0 [&::-moz-range-thumb]:shadow-md'
				)}
				aria-label="Volume"
			/>
		</div>

		<span class="w-10 text-right text-sm text-zinc-400">
			{volumeValue}%
		</span>
	</div>

	<!-- Source Indicator and Power Button -->
	<div class="flex items-center justify-center gap-4 text-sm text-zinc-500">
		<span class="capitalize">{$player.source === 'standby' ? 'Standby' : $player.source}</span>
		<button
			onclick={handlePowerToggle}
			disabled={isDisconnected || isPowerChanging}
			class={cn(
				'rounded-full p-2 transition-colors',
				$player.poweredOn
					? 'text-green-500 hover:bg-zinc-800 hover:text-green-400'
					: 'text-zinc-500 hover:bg-zinc-800 hover:text-zinc-300',
				'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
				'disabled:cursor-not-allowed disabled:opacity-50'
			)}
			aria-label={$player.poweredOn ? 'Turn off speaker (standby)' : 'Turn on speaker'}
			title={$player.poweredOn ? 'Turn off speaker (standby)' : 'Turn on speaker'}
		>
			<Power class={cn('h-5 w-5', isPowerChanging && 'animate-pulse')} />
		</button>
	</div>
</div>
