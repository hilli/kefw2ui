<script lang="ts">
	import { toasts, type ToastType } from '$lib/stores/toast';
	import { cn } from '$lib/utils/cn';
	import { AlertCircle, AlertTriangle, CheckCircle, Info, X } from 'lucide-svelte';
	import { fly, fade } from 'svelte/transition';

	const iconMap: Record<ToastType, typeof AlertCircle> = {
		error: AlertCircle,
		warning: AlertTriangle,
		success: CheckCircle,
		info: Info
	};

	const colorMap: Record<ToastType, string> = {
		error: 'border-red-500/40 bg-red-950/90 text-red-200',
		warning: 'border-yellow-500/40 bg-yellow-950/90 text-yellow-200',
		success: 'border-green-500/40 bg-green-950/90 text-green-200',
		info: 'border-blue-500/40 bg-blue-950/90 text-blue-200'
	};

	const iconColorMap: Record<ToastType, string> = {
		error: 'text-red-400',
		warning: 'text-yellow-400',
		success: 'text-green-400',
		info: 'text-blue-400'
	};
</script>

{#if $toasts.length > 0}
	<div class="pointer-events-none fixed bottom-4 right-4 z-50 flex flex-col gap-2">
		{#each $toasts as toast (toast.id)}
			<div
				class={cn(
					'pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 shadow-lg backdrop-blur-sm',
					'max-w-sm text-sm',
					colorMap[toast.type]
				)}
				role="alert"
				in:fly={{ x: 100, duration: 200 }}
				out:fade={{ duration: 150 }}
			>
				<svelte:component
					this={iconMap[toast.type]}
					class={cn('mt-0.5 h-4 w-4 flex-shrink-0', iconColorMap[toast.type])}
				/>
				<span class="flex-1">{toast.message}</span>
				<button
					class="flex-shrink-0 rounded p-0.5 opacity-60 transition-opacity hover:opacity-100"
					onclick={() => toasts.removeToast(toast.id)}
					aria-label="Dismiss"
				>
					<X class="h-3.5 w-3.5" />
				</button>
			</div>
		{/each}
	</div>
{/if}
