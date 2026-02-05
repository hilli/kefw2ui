<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { connectSSE } from '$lib/api/sse';

	onMount(() => {
		// Connect to SSE when the app mounts
		const cleanup = connectSSE();
		return cleanup;
	});

	// Global keyboard handler
	function handleKeydown(event: KeyboardEvent) {
		// Ignore if typing in an input
		if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) {
			return;
		}

		// Command palette: Cmd/Ctrl + K
		if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
			event.preventDefault();
			// TODO: Open command palette
			console.log('Open command palette');
			return;
		}

		switch (event.key) {
			case ' ':
				event.preventDefault();
				// TODO: Play/Pause
				console.log('Play/Pause');
				break;
			case 'ArrowUp':
				event.preventDefault();
				// TODO: Volume up
				console.log('Volume up');
				break;
			case 'ArrowDown':
				event.preventDefault();
				// TODO: Volume down
				console.log('Volume down');
				break;
			case 'ArrowLeft':
				event.preventDefault();
				// TODO: Previous track
				console.log('Previous track');
				break;
			case 'ArrowRight':
				event.preventDefault();
				// TODO: Next track
				console.log('Next track');
				break;
			case 'm':
			case 'M':
				// TODO: Toggle mute
				console.log('Toggle mute');
				break;
		}
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="min-h-screen bg-background text-foreground">
	<slot />
</div>
