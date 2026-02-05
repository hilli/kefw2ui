<script lang="ts">
	import NowPlaying from '$lib/components/NowPlaying/NowPlaying.svelte';
	import Controls from '$lib/components/Controls/Controls.svelte';
	import SpeakerSwitcher from '$lib/components/SpeakerSwitcher/SpeakerSwitcher.svelte';
	import SourceSelector from '$lib/components/SourceSelector/SourceSelector.svelte';
	import ConnectionStatus from '$lib/components/ConnectionStatus.svelte';
	import { activeSpeaker } from '$lib/stores/speakers';
</script>

<svelte:head>
	<title>KEF Controller</title>
</svelte:head>

<div class="flex min-h-screen flex-col bg-zinc-900">
	<!-- Header -->
	<header class="flex items-center justify-between border-b border-zinc-800 px-4 py-3">
		<div class="flex items-center gap-4">
			<!-- KEF Logo - proxied from speaker -->
			{#if $activeSpeaker}
				<img 
					src="/api/speaker/logo" 
					alt="KEF" 
					class="h-6 w-auto opacity-80 invert"
					onerror={(e) => (e.currentTarget as HTMLImageElement).style.display = 'none'}
				/>
			{/if}
			<SpeakerSwitcher />
		</div>
		<div class="flex items-center gap-3">
			<SourceSelector />
			<ConnectionStatus />
		</div>
	</header>

	<!-- Main Content -->
	<main class="flex flex-1 flex-col items-center justify-center p-4">
		<div class="w-full max-w-lg">
			<NowPlaying />
			<Controls />
		</div>
	</main>

	<!-- Footer -->
	<footer class="border-t border-zinc-800 px-4 py-3 text-center text-xs text-zinc-600">
		kefw2ui &middot; Control your KEF speakers
	</footer>
</div>
