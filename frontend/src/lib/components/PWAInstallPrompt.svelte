<script lang="ts">
	import { onMount } from 'svelte';
	import { X, Share, Plus, Download } from 'lucide-svelte';

	let showPrompt = $state(false);
	let isIOS = $state(false);
	let deferredPrompt: any = $state(null);

	const DISMISS_KEY = 'pwa-install-dismissed';
	const DISMISS_DAYS = 30;

	onMount(() => {
		// Check if already running as installed PWA
		const isStandalone =
			window.matchMedia('(display-mode: standalone)').matches ||
			(navigator as any).standalone === true; // iOS Safari

		if (isStandalone) {
			// Already installed, don't show prompt
			return;
		}

		// Check if user dismissed recently
		const dismissed = localStorage.getItem(DISMISS_KEY);
		if (dismissed) {
			const dismissedDate = new Date(dismissed);
			const daysSince = (Date.now() - dismissedDate.getTime()) / (1000 * 60 * 60 * 24);
			if (daysSince < DISMISS_DAYS) {
				// Dismissed within 30 days, don't show
				return;
			}
			// Dismissed > 30 days ago, clear and allow re-prompting
			localStorage.removeItem(DISMISS_KEY);
		}

		// Detect iOS
		isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent) && !(window as any).MSStream;

		if (isIOS) {
			// iOS doesn't have beforeinstallprompt, show manual instructions
			showPrompt = true;
			return;
		}

		// Listen for the beforeinstallprompt event (Chrome, Edge, Android)
		const handleBeforeInstall = (e: Event) => {
			// Prevent the mini-infobar from appearing on mobile
			e.preventDefault();
			// Store the event for later use
			deferredPrompt = e;
			// Show our custom prompt
			showPrompt = true;
		};

		window.addEventListener('beforeinstallprompt', handleBeforeInstall);

		// Listen for successful installation
		const handleAppInstalled = () => {
			// Hide the prompt permanently
			showPrompt = false;
			deferredPrompt = null;
			// Store a far-future date to prevent re-prompting
			localStorage.setItem(DISMISS_KEY, new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString());
		};

		window.addEventListener('appinstalled', handleAppInstalled);

		return () => {
			window.removeEventListener('beforeinstallprompt', handleBeforeInstall);
			window.removeEventListener('appinstalled', handleAppInstalled);
		};
	});

	async function install() {
		if (!deferredPrompt) return;

		// Show the native install prompt
		deferredPrompt.prompt();

		// Wait for the user's response
		const { outcome } = await deferredPrompt.userChoice;

		if (outcome === 'accepted') {
			showPrompt = false;
		}

		// Clear the stored prompt - it can only be used once
		deferredPrompt = null;
	}

	function dismiss() {
		// Store dismissal timestamp
		localStorage.setItem(DISMISS_KEY, new Date().toISOString());
		showPrompt = false;
	}
</script>

{#if showPrompt}
	<div
		class="fixed bottom-0 left-0 right-0 z-40 animate-slide-up border-t border-zinc-700 bg-zinc-800/95 px-4 py-3 shadow-lg backdrop-blur-sm"
	>
		<div class="mx-auto flex max-w-2xl items-center justify-between gap-3">
			{#if isIOS}
				<!-- iOS: Manual instructions -->
				<div class="flex flex-1 items-center gap-3 text-sm text-zinc-300">
					<Download class="h-5 w-5 flex-shrink-0 text-amber-400" />
					<p>
						<span class="hidden sm:inline">Install this app: tap </span>
						<span class="sm:hidden">Tap </span>
						<Share class="inline h-4 w-4 text-zinc-400" />
						<span class="hidden sm:inline"> Share</span>
						<span> then </span>
						<span class="font-medium text-zinc-100">"Add to Home Screen"</span>
					</p>
				</div>
				<button
					onclick={dismiss}
					class="flex-shrink-0 rounded-lg bg-zinc-700 px-3 py-1.5 text-sm font-medium text-zinc-200 transition-colors hover:bg-zinc-600"
				>
					Got it
				</button>
			{:else}
				<!-- Chrome/Edge/Android: Install button -->
				<div class="flex flex-1 items-center gap-3 text-sm text-zinc-300">
					<Download class="h-5 w-5 flex-shrink-0 text-amber-400" />
					<p>
						<span class="hidden sm:inline">Add KEF Controller to your home screen for quick access</span>
						<span class="sm:hidden">Install for quick access</span>
					</p>
				</div>
				<div class="flex flex-shrink-0 items-center gap-2">
					<button
						onclick={install}
						class="rounded-lg bg-amber-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-amber-500"
					>
						Install
					</button>
					<button
						onclick={dismiss}
						class="rounded p-1.5 text-zinc-400 transition-colors hover:bg-zinc-700 hover:text-zinc-200"
						title="Dismiss"
					>
						<X class="h-4 w-4" />
					</button>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	@keyframes slide-up {
		from {
			transform: translateY(100%);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}

	.animate-slide-up {
		animation: slide-up 0.3s ease-out;
	}
</style>
