<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { connectSSE } from '$lib/api/sse';
	import { api } from '$lib/api/client';
	import { player } from '$lib/stores/player';
	import { get } from 'svelte/store';
	import CommandPalette from '$lib/components/CommandPalette/CommandPalette.svelte';

	let commandPaletteOpen = $state(false);

	onMount(() => {
		// Connect to SSE when the app mounts
		const cleanup = connectSSE();
		return cleanup;
	});

	// Global keyboard handler
	async function handleKeydown(event: KeyboardEvent) {
		// Ignore if typing in an input (except for command palette shortcut)
		const isInput = event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement;
		
		// Command palette: Cmd/Ctrl + K (always works)
		if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
			event.preventDefault();
			commandPaletteOpen = !commandPaletteOpen;
			return;
		}

		// Skip other shortcuts if in input or command palette is open
		if (isInput || commandPaletteOpen) {
			return;
		}

		switch (event.key) {
			case ' ':
				event.preventDefault();
				try {
					await api.playPause();
				} catch (error) {
					console.error('Play/pause failed:', error);
				}
				break;

			case 'ArrowUp':
				event.preventDefault();
				try {
					const currentPlayer = get(player);
					const step = event.shiftKey ? 1 : 5;
					const newVolume = Math.min(100, currentPlayer.volume + step);
					await api.setVolume(newVolume);
				} catch (error) {
					console.error('Volume up failed:', error);
				}
				break;

			case 'ArrowDown':
				event.preventDefault();
				try {
					const currentPlayer = get(player);
					const step = event.shiftKey ? 1 : 5;
					const newVolume = Math.max(0, currentPlayer.volume - step);
					await api.setVolume(newVolume);
				} catch (error) {
					console.error('Volume down failed:', error);
				}
				break;

			case 'ArrowLeft':
				event.preventDefault();
				try {
					await api.previousTrack();
				} catch (error) {
					console.error('Previous track failed:', error);
				}
				break;

			case 'ArrowRight':
				event.preventDefault();
				try {
					await api.nextTrack();
				} catch (error) {
					console.error('Next track failed:', error);
				}
				break;

			case 'm':
			case 'M':
				try {
					await api.toggleMute();
				} catch (error) {
					console.error('Mute toggle failed:', error);
				}
				break;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="min-h-screen bg-zinc-900 text-white">
	<slot />
</div>

<CommandPalette bind:open={commandPaletteOpen} onClose={() => (commandPaletteOpen = false)} />
