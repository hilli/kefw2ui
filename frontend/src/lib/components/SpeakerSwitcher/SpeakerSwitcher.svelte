<script lang="ts">
	import { speakers, activeSpeaker, setActiveSpeaker } from '$lib/stores/speakers';
	import { api } from '$lib/api/client';
	import { cn } from '$lib/utils/cn';
	import { Speaker, ChevronDown, RefreshCw, Check } from 'lucide-svelte';
	import { onMount } from 'svelte';

	let isOpen = false;
	let isDiscovering = false;

	onMount(async () => {
		await loadSpeakers();
	});

	async function loadSpeakers() {
		try {
			const response = await api.getSpeakers();
			speakers.set(response.speakers);
			const active = response.speakers.find((s) => s.active);
			if (active) {
				activeSpeaker.set(active);
			}
		} catch (error) {
			console.error('Failed to load speakers:', error);
		}
	}

	async function handleDiscover() {
		isDiscovering = true;
		try {
			await api.discoverSpeakers();
			await loadSpeakers();
		} catch (error) {
			console.error('Discovery failed:', error);
		} finally {
			isDiscovering = false;
		}
	}

	async function handleSelectSpeaker(speaker: (typeof $speakers)[0]) {
		if (speaker.active) {
			isOpen = false;
			return;
		}

		try {
			await api.setActiveSpeaker(speaker.ip);
			setActiveSpeaker(speaker);
			isOpen = false;
		} catch (error) {
			console.error('Failed to switch speaker:', error);
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			isOpen = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="relative">
	<!-- Trigger Button -->
	<button
		onclick={() => (isOpen = !isOpen)}
		class={cn(
			'flex items-center gap-2 rounded-lg px-3 py-2 transition-colors',
			'bg-zinc-800 text-sm text-zinc-300 hover:bg-zinc-700 hover:text-white',
			'focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-zinc-900'
		)}
		aria-haspopup="listbox"
		aria-expanded={isOpen}
	>
		<Speaker class="h-4 w-4" />
		<span class="max-w-32 truncate">
			{$activeSpeaker?.name || 'No Speaker'}
		</span>
		<ChevronDown class={cn('h-4 w-4 transition-transform', isOpen && 'rotate-180')} />
	</button>

	<!-- Dropdown -->
	{#if isOpen}
		<div
			class={cn(
				'absolute left-0 top-full z-50 mt-2 w-64 overflow-hidden rounded-lg',
				'bg-zinc-800 shadow-xl ring-1 ring-zinc-700'
			)}
		>
			<!-- Speaker List -->
			<div class="max-h-64 overflow-y-auto py-1">
				{#if $speakers.length === 0}
					<div class="px-4 py-3 text-center text-sm text-zinc-500">
						No speakers found
					</div>
				{:else}
					{#each $speakers as speaker}
						<button
							onclick={() => handleSelectSpeaker(speaker)}
							class={cn(
								'flex w-full items-center gap-3 px-4 py-2 text-left transition-colors',
								speaker.active
									? 'bg-zinc-700 text-white'
									: 'text-zinc-300 hover:bg-zinc-700 hover:text-white'
							)}
						>
							<Speaker class="h-4 w-4 flex-shrink-0" />
							<div class="min-w-0 flex-1">
								<div class="truncate font-medium">{speaker.name}</div>
								<div class="truncate text-xs text-zinc-500">{speaker.model}</div>
							</div>
							{#if speaker.active}
								<Check class="h-4 w-4 flex-shrink-0 text-green-500" />
							{/if}
						</button>
					{/each}
				{/if}
			</div>

			<!-- Discover Button -->
			<div class="border-t border-zinc-700 p-2">
				<button
					onclick={handleDiscover}
					disabled={isDiscovering}
					class={cn(
						'flex w-full items-center justify-center gap-2 rounded-md px-3 py-2',
						'text-sm text-zinc-400 transition-colors',
						'hover:bg-zinc-700 hover:text-white',
						'disabled:cursor-not-allowed disabled:opacity-50'
					)}
				>
					<RefreshCw class={cn('h-4 w-4', isDiscovering && 'animate-spin')} />
					<span>{isDiscovering ? 'Discovering...' : 'Discover Speakers'}</span>
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
		aria-label="Close speaker menu"
	></button>
{/if}
