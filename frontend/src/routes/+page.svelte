<script lang="ts">
	import NowPlaying from '$lib/components/NowPlaying/NowPlaying.svelte';
	import Controls from '$lib/components/Controls/Controls.svelte';
	import SpeakerSwitcher from '$lib/components/SpeakerSwitcher/SpeakerSwitcher.svelte';
	import ConnectionStatus from '$lib/components/ConnectionStatus.svelte';
</script>

<svelte:head>
	<title>KEF Controller</title>
</svelte:head>

<div class="flex min-h-screen flex-col bg-zinc-900">
	<!-- Header -->
	<header class="flex items-center justify-between border-b border-zinc-800 px-4 py-3">
		<div class="flex items-center gap-4">
			<SpeakerSwitcher />
		</div>
		<ConnectionStatus />
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
