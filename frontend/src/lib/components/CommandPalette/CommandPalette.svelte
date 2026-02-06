<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { api } from '$lib/api/client';
	import { player, updateSource } from '$lib/stores/player';
	import { speakers, activeSpeaker, setActiveSpeaker } from '$lib/stores/speakers';
	import { browseNavigation } from '$lib/stores/browseNavigation';
	import {
		Search,
		Play,
		Pause,
		SkipBack,
		SkipForward,
		Volume2,
		VolumeX,
		Wifi,
		Bluetooth,
		Tv,
		Speaker,
		Power,
		Command
	} from 'lucide-svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open = $bindable(), onClose }: Props = $props();

	let searchQuery = $state('');
	let selectedIndex = $state(0);
	let inputElement: HTMLInputElement | undefined = $state();

	// Command definitions
	const commands = [
		// Navigation / Search
		{ id: 'search-media', label: 'Search Media', keywords: 'search media upnp browse find music', icon: Search, action: () => browseNavigation.focusSearch('upnp') },
		{ id: 'search-radio', label: 'Search Radio', keywords: 'search radio stations fm am', icon: Search, action: () => browseNavigation.focusSearch('radio') },
		{ id: 'search-podcasts', label: 'Search Podcasts', keywords: 'search podcasts shows episodes', icon: Search, action: () => browseNavigation.focusSearch('podcasts') },
		
		// Playback
		{ id: 'play', label: 'Play / Pause', keywords: 'play pause toggle', icon: Play, action: () => api.playPause() },
		{ id: 'next', label: 'Next Track', keywords: 'next skip forward', icon: SkipForward, action: () => api.nextTrack() },
		{ id: 'prev', label: 'Previous Track', keywords: 'previous back', icon: SkipBack, action: () => api.previousTrack() },
		{ id: 'mute', label: 'Toggle Mute', keywords: 'mute unmute silence', icon: VolumeX, action: () => api.toggleMute() },
		
		// Volume shortcuts
		{ id: 'vol-50', label: 'Set Volume to 50%', keywords: 'volume half', icon: Volume2, action: () => api.setVolume(50) },
		{ id: 'vol-25', label: 'Set Volume to 25%', keywords: 'volume low quiet', icon: Volume2, action: () => api.setVolume(25) },
		{ id: 'vol-75', label: 'Set Volume to 75%', keywords: 'volume high loud', icon: Volume2, action: () => api.setVolume(75) },
		{ id: 'vol-100', label: 'Set Volume to 100%', keywords: 'volume max maximum', icon: Volume2, action: () => api.setVolume(100) },
		
		// Sources
		{ id: 'src-wifi', label: 'Switch to WiFi', keywords: 'source wifi airplay chromecast roon stream', icon: Wifi, action: async () => { await api.setSource('wifi'); updateSource('wifi'); } },
		{ id: 'src-bluetooth', label: 'Switch to Bluetooth', keywords: 'source bluetooth bt', icon: Bluetooth, action: async () => { await api.setSource('bluetooth'); updateSource('bluetooth'); } },
		{ id: 'src-tv', label: 'Switch to TV', keywords: 'source tv hdmi arc', icon: Tv, action: async () => { await api.setSource('tv'); updateSource('tv'); } },
		{ id: 'src-optical', label: 'Switch to Optical', keywords: 'source optical toslink digital', icon: Speaker, action: async () => { await api.setSource('optical'); updateSource('optical'); } },
		{ id: 'src-usb', label: 'Switch to USB', keywords: 'source usb', icon: Speaker, action: async () => { await api.setSource('usb'); updateSource('usb'); } },
		{ id: 'standby', label: 'Put Speaker to Standby', keywords: 'standby sleep off power', icon: Power, action: async () => { await api.setSource('standby'); updateSource('standby'); } },
	];

	// Generate speaker switch commands dynamically
	const speakerCommands = $derived(
		$speakers
			.filter((s) => !s.active)
			.map((s) => ({
				id: `speaker-${s.ip}`,
				label: `Switch to ${s.name}`,
				keywords: `speaker switch ${s.name.toLowerCase()} ${s.model.toLowerCase()}`,
				icon: Speaker,
				action: async () => {
					await api.setActiveSpeaker(s.ip);
					setActiveSpeaker({ ...s, active: true });
				}
			}))
	);

	// Filter commands based on search
	const filteredCommands = $derived(() => {
		const allCommands = [...commands, ...speakerCommands];
		if (!searchQuery.trim()) {
			return allCommands;
		}
		const query = searchQuery.toLowerCase();
		return allCommands.filter(
			(cmd) =>
				cmd.label.toLowerCase().includes(query) ||
				cmd.keywords.toLowerCase().includes(query)
		);
	});

	// Reset state when opened
	$effect(() => {
		if (open) {
			searchQuery = '';
			selectedIndex = 0;
			// Focus input after a tick
			setTimeout(() => inputElement?.focus(), 10);
		}
	});

	// Keep selected index in bounds
	$effect(() => {
		const cmds = filteredCommands();
		if (selectedIndex >= cmds.length) {
			selectedIndex = Math.max(0, cmds.length - 1);
		}
	});

	async function executeCommand(cmd: typeof commands[0]) {
		try {
			await cmd.action();
		} catch (error) {
			console.error(`Command failed: ${cmd.label}`, error);
		}
		onClose();
	}

	function handleKeydown(event: KeyboardEvent) {
		const cmds = filteredCommands();
		
		switch (event.key) {
			case 'ArrowDown':
				event.preventDefault();
				selectedIndex = (selectedIndex + 1) % cmds.length;
				break;
			case 'ArrowUp':
				event.preventDefault();
				selectedIndex = (selectedIndex - 1 + cmds.length) % cmds.length;
				break;
			case 'Enter':
				event.preventDefault();
				if (cmds[selectedIndex]) {
					executeCommand(cmds[selectedIndex]);
				}
				break;
			case 'Escape':
				event.preventDefault();
				onClose();
				break;
		}
	}
</script>

{#if open}
	<!-- Backdrop -->
	<button
		class="fixed inset-0 z-50 cursor-default bg-black/60 backdrop-blur-sm"
		onclick={onClose}
		aria-label="Close command palette"
	></button>

	<!-- Palette -->
	<div
		class={cn(
			'fixed left-1/2 top-1/4 z-50 w-full max-w-lg -translate-x-1/2',
			'rounded-xl bg-zinc-800 shadow-2xl ring-1 ring-zinc-700'
		)}
		role="dialog"
		aria-modal="true"
		aria-label="Command palette"
	>
		<!-- Search Input -->
		<div class="flex items-center gap-3 border-b border-zinc-700 px-4 py-3">
			<Search class="h-5 w-5 text-zinc-400" />
			<input
				bind:this={inputElement}
				bind:value={searchQuery}
				onkeydown={handleKeydown}
				type="text"
				placeholder="Type a command or search..."
				class="flex-1 bg-transparent text-white placeholder-zinc-500 outline-none"
				autocomplete="off"
				autocorrect="off"
				spellcheck="false"
			/>
			<kbd class="rounded bg-zinc-700 px-2 py-0.5 text-xs text-zinc-400">ESC</kbd>
		</div>

		<!-- Command List -->
		<div class="max-h-80 overflow-y-auto p-2">
			{#each filteredCommands() as cmd, index}
				{@const Icon = cmd.icon}
				<button
					onclick={() => executeCommand(cmd)}
					class={cn(
						'flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors',
						index === selectedIndex
							? 'bg-zinc-700 text-white'
							: 'text-zinc-300 hover:bg-zinc-700/50 hover:text-white'
					)}
				>
					<Icon class="h-4 w-4 flex-shrink-0 text-zinc-400" />
					<span class="flex-1">{cmd.label}</span>
					{#if index === selectedIndex}
						<kbd class="rounded bg-zinc-600 px-1.5 py-0.5 text-xs text-zinc-300">↵</kbd>
					{/if}
				</button>
			{:else}
				<div class="px-3 py-8 text-center text-zinc-500">
					No commands found
				</div>
			{/each}
		</div>

		<!-- Footer -->
		<div class="flex items-center justify-between border-t border-zinc-700 px-4 py-2 text-xs text-zinc-500">
			<div class="flex items-center gap-4">
				<span class="flex items-center gap-1">
					<kbd class="rounded bg-zinc-700 px-1 py-0.5">↑</kbd>
					<kbd class="rounded bg-zinc-700 px-1 py-0.5">↓</kbd>
					navigate
				</span>
				<span class="flex items-center gap-1">
					<kbd class="rounded bg-zinc-700 px-1 py-0.5">↵</kbd>
					select
				</span>
			</div>
			<div class="flex items-center gap-1">
				<Command class="h-3 w-3" />
				<span>K to open</span>
			</div>
		</div>
	</div>
{/if}
