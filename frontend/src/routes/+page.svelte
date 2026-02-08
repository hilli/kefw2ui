<script lang="ts">
	import { onMount } from 'svelte';
	import { PaneGroup, Pane, PaneResizer } from 'paneforge';
	import NowPlaying from '$lib/components/NowPlaying/NowPlaying.svelte';
	import Controls from '$lib/components/Controls/Controls.svelte';
	import SpeakerSwitcher from '$lib/components/SpeakerSwitcher/SpeakerSwitcher.svelte';
	import SourceSelector from '$lib/components/SourceSelector/SourceSelector.svelte';
	import ConnectionStatus from '$lib/components/ConnectionStatus.svelte';
	import Queue from '$lib/components/Queue/Queue.svelte';
	import Playlists from '$lib/components/Playlists/Playlists.svelte';
	import Browser from '$lib/components/Browser/Browser.svelte';
	import Settings from '$lib/components/Settings/Settings.svelte';
	import PWAInstallPrompt from '$lib/components/PWAInstallPrompt.svelte';
	import { activeSpeaker } from '$lib/stores/speakers';
	import { connectionStatus, speakerConnected } from '$lib/stores/player';
	import { GripVertical, GripHorizontal, WifiOff, Loader2, Volume2 } from 'lucide-svelte';

	// Track mobile/desktop for responsive layout
	let isMobile = $state(false);

	onMount(() => {
		const mediaQuery = window.matchMedia('(max-width: 1024px)');
		isMobile = mediaQuery.matches;
		const handler = (e: MediaQueryListEvent) => (isMobile = e.matches);
		mediaQuery.addEventListener('change', handler);
		return () => mediaQuery.removeEventListener('change', handler);
	});
</script>

<svelte:head>
	<title>KEF Controller</title>
</svelte:head>

<div class="flex h-screen flex-col bg-zinc-900">
	<!-- Header -->
	<header class="flex flex-shrink-0 items-center justify-between border-b border-zinc-800 px-4 py-3">
		<SpeakerSwitcher />
		<!-- KEF Logo - proxied from speaker -->
		{#if $activeSpeaker}
			<img
				src="/api/speaker/logo"
				alt="KEF"
				class="h-6 w-auto opacity-80 invert"
				onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = 'none')}
			/>
		{/if}
		<div class="flex items-center gap-3">
			<SourceSelector />
			<ConnectionStatus />
			<Settings />
		</div>
	</header>

	<!-- Disconnection Banner -->
	{#if $connectionStatus === 'connecting'}
		<div class="flex items-center justify-center gap-2 border-b border-yellow-500/30 bg-yellow-900/50 px-4 py-2 text-sm text-yellow-200">
			<Loader2 class="h-4 w-4 animate-spin" />
			<span>Reconnecting to server...</span>
		</div>
	{:else if $connectionStatus === 'disconnected'}
		<div class="flex items-center justify-center gap-2 border-b border-red-500/30 bg-red-900/50 px-4 py-2 text-sm text-red-200">
			<WifiOff class="h-4 w-4" />
			<span>Connection lost — attempting to reconnect...</span>
		</div>
	{/if}

	<!-- Speaker Unreachable Banner -->
	{#if $connectionStatus === 'connected' && !$speakerConnected}
		<div class="flex items-center justify-center gap-2 border-b border-amber-500/30 bg-amber-900/50 px-4 py-2 text-sm text-amber-200">
			<Volume2 class="h-4 w-4" />
			<span>Speaker unreachable — reconnecting...</span>
		</div>
	{/if}

	<!-- Main Content -->
	{#if isMobile}
		<!-- Mobile: Stacked layout without resizers -->
		<main class="flex flex-1 flex-col overflow-y-auto">
			<!-- Now Playing -->
			<div class="flex flex-col items-center justify-center p-4">
				<div class="w-full max-w-lg">
					<NowPlaying />
					<Controls />
				</div>
			</div>

			<!-- Sidebar panels stacked -->
			<aside class="w-full space-y-4 border-t border-zinc-800 p-4">
				<Queue />
				<Browser />
				<Playlists />
			</aside>
		</main>
	{:else}
		<!-- Desktop: Resizable panels -->
		<main class="flex-1 overflow-hidden">
			<PaneGroup direction="horizontal" autoSaveId="kefw2ui-main-layout" class="h-full">
				<!-- Main Area (NowPlaying + Controls) -->
				<Pane defaultSize={65} minSize={40} class="h-full">
				<div class="flex h-full flex-col overflow-y-auto p-4">
					<div class="m-auto w-full max-w-lg">
							<NowPlaying />
							<Controls />
						</div>
					</div>
				</Pane>

				<!-- Horizontal Resizer -->
				<PaneResizer
					class="group flex w-1.5 items-center justify-center bg-zinc-800 transition-colors hover:bg-zinc-600 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
				>
					<div
						class="flex h-8 w-3 items-center justify-center rounded-sm opacity-0 transition-opacity group-hover:opacity-100"
					>
						<GripVertical class="h-4 w-4 text-zinc-400" />
					</div>
				</PaneResizer>

				<!-- Sidebar with nested vertical panes -->
				<Pane defaultSize={35} minSize={15} collapsible collapsedSize={0} class="h-full">
					<PaneGroup direction="vertical" autoSaveId="kefw2ui-sidebar-layout" class="h-full">
						<!-- Queue -->
						<Pane defaultSize={40} minSize={10} collapsible collapsedSize={4} class="h-full">
							<div class="h-full overflow-hidden">
								<Queue fullHeight />
							</div>
						</Pane>

						<!-- Vertical Resizer -->
						<PaneResizer
							class="group flex h-1.5 items-center justify-center bg-zinc-800 transition-colors hover:bg-zinc-600 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
						>
							<div
								class="flex h-3 w-8 items-center justify-center rounded-sm opacity-0 transition-opacity group-hover:opacity-100"
							>
								<GripHorizontal class="h-4 w-4 text-zinc-400" />
							</div>
						</PaneResizer>

						<!-- Browser -->
						<Pane defaultSize={40} minSize={10} collapsible collapsedSize={4} class="h-full">
							<div class="h-full overflow-hidden">
								<Browser fullHeight />
							</div>
						</Pane>

						<!-- Vertical Resizer -->
						<PaneResizer
							class="group flex h-1.5 items-center justify-center bg-zinc-800 transition-colors hover:bg-zinc-600 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-zinc-400"
						>
							<div
								class="flex h-3 w-8 items-center justify-center rounded-sm opacity-0 transition-opacity group-hover:opacity-100"
							>
								<GripHorizontal class="h-4 w-4 text-zinc-400" />
							</div>
						</PaneResizer>

						<!-- Playlists -->
						<Pane defaultSize={20} minSize={8} collapsible collapsedSize={4} class="h-full">
							<div class="h-full overflow-hidden">
								<Playlists fullHeight />
							</div>
						</Pane>
					</PaneGroup>
				</Pane>
			</PaneGroup>
		</main>
	{/if}

	<!-- Footer -->
	<footer
		class="flex-shrink-0 border-t border-zinc-800 px-4 py-2 text-center text-xs text-zinc-600"
	>
		kefw2ui &middot; Control your KEF speakers
	</footer>
</div>

<!-- PWA Install Prompt (fixed at bottom, above footer) -->
<PWAInstallPrompt />
