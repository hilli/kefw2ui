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

<div class="flex items-center gap-2 text-xs text-zinc-500">
	<div class={cn('h-2 w-2 rounded-full', statusColors[$connectionStatus])}></div>
	<span>{statusLabels[$connectionStatus]}</span>
</div>
