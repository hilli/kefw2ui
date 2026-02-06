<script lang="ts">
	import { speakers, activeSpeaker, setActiveSpeaker, defaultSpeakerIP, setDefaultSpeakerIP } from '$lib/stores/speakers';
	import { api } from '$lib/api/client';
	import { cn } from '$lib/utils/cn';
	import { Speaker, ChevronDown, RefreshCw, Check, Plus, Star } from 'lucide-svelte';
	import { onMount } from 'svelte';

	let isOpen = false;
	let isDiscovering = false;
	let showAddForm = false;
	let manualIP = '';
	let isAdding = false;
	let addError = '';

	onMount(async () => {
		await loadSpeakers();
	});

	async function loadSpeakers() {
		try {
			const response = await api.getSpeakers();
			speakers.set(response.speakers);
			defaultSpeakerIP.set(response.defaultSpeaker || '');
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

	async function handleSetDefault(event: Event, speaker: (typeof $speakers)[0]) {
		event.stopPropagation();
		try {
			await api.setDefaultSpeaker(speaker.ip);
			setDefaultSpeakerIP(speaker.ip);
		} catch (error) {
			console.error('Failed to set default speaker:', error);
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			isOpen = false;
			showAddForm = false;
			addError = '';
		}
	}

	async function handleAddSpeaker(event: Event) {
		event.preventDefault();
		if (!manualIP.trim()) return;

		isAdding = true;
		addError = '';

		try {
			const result = await api.addSpeaker(manualIP.trim());
			// Reload speakers list
			await loadSpeakers();
			// Automatically switch to the new speaker
			await api.setActiveSpeaker(result.speaker.ip);
			await loadSpeakers();
			// Reset form
			manualIP = '';
			showAddForm = false;
			isOpen = false;
		} catch (error) {
			addError = error instanceof Error ? error.message : 'Failed to add speaker';
		} finally {
			isAdding = false;
		}
	}

	function toggleAddForm() {
		showAddForm = !showAddForm;
		addError = '';
		if (showAddForm) {
			// Focus the input after a tick
			setTimeout(() => {
				const input = document.getElementById('manual-speaker-ip');
				input?.focus();
			}, 50);
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
				'absolute left-0 top-full z-50 mt-2 w-72 overflow-hidden rounded-lg',
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
						{@const isDefault = speaker.ip === $defaultSpeakerIP}
						<div
							class={cn(
								'flex w-full items-center gap-2 px-4 py-2 transition-colors',
								speaker.active
									? 'bg-zinc-700 text-white'
									: 'text-zinc-300 hover:bg-zinc-700 hover:text-white'
							)}
						>
							<button
								onclick={() => handleSelectSpeaker(speaker)}
								class="flex flex-1 items-center gap-3 text-left"
							>
								<Speaker class="h-4 w-4 flex-shrink-0" />
								<div class="min-w-0 flex-1">
									<div class="flex items-center gap-1.5 truncate font-medium">
										{speaker.name}
										{#if isDefault}
											<span class="text-xs text-yellow-500" title="Default speaker">(default)</span>
										{/if}
									</div>
									<div class="truncate text-xs text-zinc-500">{speaker.model}</div>
								</div>
								{#if speaker.active}
									<Check class="h-4 w-4 flex-shrink-0 text-green-500" />
								{/if}
							</button>
							<!-- Set as Default button -->
							<button
								onclick={(e) => handleSetDefault(e, speaker)}
								class={cn(
									'rounded p-1 transition-colors',
									isDefault
										? 'text-yellow-500'
										: 'text-zinc-500 hover:text-yellow-500 hover:bg-zinc-600'
								)}
								title={isDefault ? 'Default speaker' : 'Set as default'}
								disabled={isDefault}
							>
								<Star class={cn('h-4 w-4', isDefault && 'fill-current')} />
							</button>
						</div>
					{/each}
				{/if}
			</div>

			<!-- Action Buttons -->
			<div class="border-t border-zinc-700 p-2 space-y-1">
				<!-- Discover Button -->
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

				<!-- Add Manual Button / Form -->
				{#if showAddForm}
					<form onsubmit={handleAddSpeaker} class="space-y-2">
						<div class="flex gap-2">
							<input
								id="manual-speaker-ip"
								type="text"
								bind:value={manualIP}
								placeholder="IP address (e.g. 192.168.1.100)"
								disabled={isAdding}
								class={cn(
									'flex-1 rounded-md bg-zinc-700 px-3 py-2 text-sm text-white',
									'placeholder:text-zinc-500',
									'focus:outline-none focus:ring-2 focus:ring-white',
									'disabled:cursor-not-allowed disabled:opacity-50'
								)}
							/>
							<button
								type="submit"
								disabled={isAdding || !manualIP.trim()}
								class={cn(
									'rounded-md bg-zinc-600 px-3 py-2 text-sm text-white',
									'hover:bg-zinc-500 transition-colors',
									'disabled:cursor-not-allowed disabled:opacity-50'
								)}
							>
								{isAdding ? '...' : 'Add'}
							</button>
						</div>
						{#if addError}
							<p class="text-xs text-red-400 px-1">{addError}</p>
						{/if}
						<button
							type="button"
							onclick={toggleAddForm}
							class="w-full text-xs text-zinc-500 hover:text-zinc-400"
						>
							Cancel
						</button>
					</form>
				{:else}
					<button
						onclick={toggleAddForm}
						class={cn(
							'flex w-full items-center justify-center gap-2 rounded-md px-3 py-2',
							'text-sm text-zinc-400 transition-colors',
							'hover:bg-zinc-700 hover:text-white'
						)}
					>
						<Plus class="h-4 w-4" />
						<span>Add Speaker by IP</span>
					</button>
				{/if}
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
