<script lang="ts">
	import { connectionStatus } from '$lib/stores/player';
	import { cn } from '$lib/utils/cn';

	const statusColors = {
		connected: 'bg-green-500',
		connecting: 'bg-yellow-500 animate-pulse',
		disconnected: 'bg-red-500'
	};

	const statusLabels = {
		connected: 'Connected',
		connecting: 'Connecting...',
		disconnected: 'Disconnected'
	};
</script>

<div class="group relative flex items-center gap-2 text-xs text-zinc-500" title={statusLabels[$connectionStatus]}>
	<div class={cn('h-2 w-2 rounded-full', statusColors[$connectionStatus])}></div>
	<!-- Hide text on mobile, show on hover/focus or on desktop -->
	<span class="hidden md:inline">{statusLabels[$connectionStatus]}</span>
	<!-- Mobile: tooltip on hover/tap -->
	<span class="pointer-events-none absolute left-1/2 top-full z-10 mt-1 -translate-x-1/2 whitespace-nowrap rounded bg-zinc-800 px-2 py-1 text-xs text-zinc-300 opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 md:hidden">
		{statusLabels[$connectionStatus]}
	</span>
</div>
