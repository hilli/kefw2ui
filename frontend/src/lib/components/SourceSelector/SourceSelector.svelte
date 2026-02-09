<script lang="ts">
	import { player, updateSource } from '$lib/stores/player';
	import { api } from '$lib/api/client';
	import { toasts } from '$lib/stores/toast';
	import { cn } from '$lib/utils/cn';
	import {
		Wifi,
		Bluetooth,
		Tv,
		Usb,
		Cable,
		AudioLines,
		ChevronDown,
		Check,
		Power
	} from 'lucide-svelte';

	let isOpen = $state(false);
	let isChanging = $state(false);

	// Available sources on KEF speakers
	const sources = [
		{ id: 'wifi', label: 'WiFi', icon: Wifi, description: 'AirPlay, Chromecast, Roon' },
		{ id: 'bluetooth', label: 'Bluetooth', icon: Bluetooth, description: 'Bluetooth audio' },
		{ id: 'tv', label: 'TV', icon: Tv, description: 'HDMI ARC/eARC' },
		{ id: 'optical', label: 'Optical', icon: AudioLines, description: 'TOSLINK digital' },
		{ id: 'coaxial', label: 'Coaxial', icon: Cable, description: 'Coaxial digital' },
		{ id: 'analog', label: 'Aux', icon: AudioLines, description: '3.5mm auxiliary' },
		{ id: 'usb', label: 'USB', icon: Usb, description: 'USB audio' }
	] as const;

	// Standby and loading states shown in the trigger button
	const standbySource = { id: 'standby', label: 'Standby', icon: Power, description: 'Speaker is asleep' } as const;
	const loadingSource = { id: '', label: '...', icon: Power, description: 'Loading' } as const;

	function getSourceInfo(sourceId: string) {
		if (sourceId === 'standby') return standbySource;
		if (sourceId === '') return loadingSource;
		return sources.find((s) => s.id === sourceId) || sources[0];
	}

	async function handleSelectSource(sourceId: string) {
		if (sourceId === $player.source) {
			isOpen = false;
			return;
		}

		isChanging = true;
		try {
			await api.setSource(sourceId);
			updateSource(sourceId);
			isOpen = false;
		} catch (error) {
			toasts.error('Failed to change source');
		} finally {
			isChanging = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			isOpen = false;
		}
	}

	const currentSource = $derived(getSourceInfo($player.source));
	const CurrentIcon = $derived(currentSource.icon);
	const isStandby = $derived($player.source === 'standby');
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="relative">
	<!-- Trigger Button -->
	<button
		onclick={() => (isOpen = !isOpen)}
		disabled={isChanging}
		class={cn(
			'flex items-center gap-2 rounded-lg px-3 py-2 transition-colors',
			'bg-zinc-800 text-sm text-zinc-300 hover:bg-zinc-700 hover:text-white',
			'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900',
			'disabled:cursor-not-allowed disabled:opacity-50'
		)}
		aria-haspopup="listbox"
		aria-expanded={isOpen}
	>
		<CurrentIcon class="h-4 w-4" />
		<span>{currentSource.label}</span>
		<ChevronDown class={cn('h-4 w-4 transition-transform', isOpen && 'rotate-180')} />
	</button>

	<!-- Dropdown -->
	{#if isOpen}
		<div
			class={cn(
				'absolute right-0 top-full z-50 mt-2 w-56 overflow-hidden rounded-lg',
				'bg-zinc-800 shadow-xl ring-1 ring-zinc-700'
			)}
		>
			<!-- Source List -->
			<div class="py-1">
				{#each sources as source}
					{@const isActive = source.id === $player.source}
					{@const SourceIcon = source.icon}
					<button
						onclick={() => handleSelectSource(source.id)}
						class={cn(
							'flex w-full items-center gap-3 px-4 py-2 text-left transition-colors',
							isActive
								? 'bg-zinc-700 text-white'
								: 'text-zinc-300 hover:bg-zinc-700 hover:text-white'
						)}
					>
						<SourceIcon class="h-4 w-4 flex-shrink-0" />
						<div class="min-w-0 flex-1">
							<div class="font-medium">{source.label}</div>
							<div class="text-xs text-zinc-500">{source.description}</div>
						</div>
						{#if isActive}
							<Check class="h-4 w-4 flex-shrink-0 text-green-500" />
						{/if}
					</button>
				{/each}
			</div>

			<!-- Standby option -->
		<div class="border-t border-zinc-700 py-1">
			<button
					onclick={() => handleSelectSource('standby')}
					class={cn(
						'flex w-full items-center gap-3 px-4 py-2 text-left transition-colors',
						isStandby
							? 'bg-zinc-700 text-white'
							: 'text-zinc-400 hover:bg-zinc-700 hover:text-white'
					)}
				>
					<Power class="h-4 w-4 flex-shrink-0" />
					<div class="min-w-0 flex-1">
						<div class="font-medium">Standby</div>
						<div class="text-xs text-zinc-500">Put speaker to sleep</div>
					</div>
					{#if isStandby}
						<Check class="h-4 w-4 flex-shrink-0 text-green-500" />
					{/if}
				</button>
			</div>
		</div>
	{/if}
</div>

<!-- Click outside to close -->
{#if isOpen}
	<button
		class="fixed inset-0 z-40 cursor-default"
		onclick={() => (isOpen = false)}
		aria-label="Close source menu"
	></button>
{/if}
