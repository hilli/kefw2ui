<script lang="ts">
	import '../app.css';
	import { onMount, type Snippet } from 'svelte';
	import { connectSSE } from '$lib/api/sse';
	import { api } from '$lib/api/client';
	import { player } from '$lib/stores/player';
	import { speakers, setActiveSpeaker } from '$lib/stores/speakers';
	import { get } from 'svelte/store';
	import CommandPalette from '$lib/components/CommandPalette/CommandPalette.svelte';
	import { initMediaSession, updateMediaSessionMetadata, cleanupMediaSession } from '$lib/api/mediaSession';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
	let commandPaletteOpen = $state(false);
	let showShortcuts = $state(false);

	onMount(() => {
		// Connect to SSE when the app mounts
		const cleanup = connectSSE();

		// Initialize Media Session API for OS-level media controls
		initMediaSession();

		// Subscribe to player state changes and update Media Session
		const unsubscribe = player.subscribe((state) => {
			updateMediaSessionMetadata(state);
		});

		return () => {
			cleanup();
			unsubscribe();
			cleanupMediaSession();
		};
	});

	// Quick switch to speaker by index (1-9)
	async function switchToSpeakerByIndex(index: number) {
		const speakerList = get(speakers);
		if (index >= 0 && index < speakerList.length) {
			const speaker = speakerList[index];
			try {
				await api.setActiveSpeaker(speaker.ip);
				setActiveSpeaker(speaker);
			} catch (error) {
				console.error(`Failed to switch to speaker ${index + 1}:`, error);
			}
		}
	}

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

		// Escape closes any open modal/panel
		if (event.key === 'Escape') {
			if (commandPaletteOpen) {
				commandPaletteOpen = false;
			}
			if (showShortcuts) {
				showShortcuts = false;
			}
			return;
		}

		// Skip other shortcuts if in input or modal is open
		if (isInput || commandPaletteOpen || showShortcuts) {
			return;
		}

		// Number keys 1-9 for quick speaker switching
		if (event.key >= '1' && event.key <= '9') {
			event.preventDefault();
			await switchToSpeakerByIndex(parseInt(event.key) - 1);
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

			case '?':
			case '/':
				event.preventDefault();
				showShortcuts = true;
				break;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="min-h-screen bg-zinc-900 text-white">
	{@render children()}
</div>

<CommandPalette bind:open={commandPaletteOpen} onClose={() => (commandPaletteOpen = false)} />

<!-- Keyboard Shortcuts Help Modal -->
{#if showShortcuts}
	<button
		class="fixed inset-0 z-50 cursor-default bg-black/60 backdrop-blur-sm"
		onclick={() => (showShortcuts = false)}
		aria-label="Close shortcuts help"
	></button>
	<div class="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg border border-zinc-700 bg-zinc-800 p-6 shadow-xl">
		<h2 class="mb-4 text-lg font-semibold text-white">Keyboard Shortcuts</h2>
		<div class="space-y-3 text-sm">
			<div class="border-b border-zinc-700 pb-2">
				<h3 class="mb-2 text-xs font-medium uppercase text-zinc-400">Playback</h3>
				<div class="grid grid-cols-2 gap-2">
					<div class="flex justify-between"><span class="text-zinc-400">Play/Pause</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">Space</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Previous</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">←</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Next</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">→</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Mute</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">M</kbd></div>
				</div>
			</div>
			<div class="border-b border-zinc-700 pb-2">
				<h3 class="mb-2 text-xs font-medium uppercase text-zinc-400">Volume</h3>
				<div class="grid grid-cols-2 gap-2">
					<div class="flex justify-between"><span class="text-zinc-400">Volume Up</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">↑</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Volume Down</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">↓</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Fine +1%</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">Shift+↑</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Fine -1%</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">Shift+↓</kbd></div>
				</div>
			</div>
			<div>
				<h3 class="mb-2 text-xs font-medium uppercase text-zinc-400">Navigation</h3>
				<div class="grid grid-cols-2 gap-2">
					<div class="flex justify-between"><span class="text-zinc-400">Command Palette</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">⌘K</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Speaker 1-9</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">1-9</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">This Help</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">?</kbd></div>
					<div class="flex justify-between"><span class="text-zinc-400">Close</span><kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs">Esc</kbd></div>
				</div>
			</div>
		</div>
		<button
			class="mt-4 w-full rounded bg-zinc-700 px-4 py-2 text-sm text-white transition-colors hover:bg-zinc-600"
			onclick={() => (showShortcuts = false)}
		>
			Close
		</button>
	</div>
{/if}
