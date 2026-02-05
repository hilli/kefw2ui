<script lang="ts">
	import { player, connectionStatus } from '$lib/stores/player';
</script>

<svelte:head>
	<title>KEF Controller</title>
</svelte:head>

<div class="flex h-screen flex-col">
	<!-- Header -->
	<header class="flex items-center justify-between border-b border-border px-4 py-2">
		<div class="flex items-center gap-4">
			<h1 class="text-lg font-semibold">KEF Controller</h1>
			<button class="rounded-md bg-secondary px-3 py-1 text-sm hover:bg-secondary/80">
				Living Room
				<span class="ml-1 text-muted-foreground">▼</span>
			</button>
		</div>
		<div class="flex items-center gap-4">
			<button class="rounded-md bg-secondary px-3 py-1 text-sm hover:bg-secondary/80">
				WiFi
				<span class="ml-1 text-muted-foreground">▼</span>
			</button>
			<div class="flex items-center gap-2">
				<span
					class="h-2 w-2 rounded-full"
					class:bg-green-500={$connectionStatus === 'connected'}
					class:bg-yellow-500={$connectionStatus === 'connecting'}
					class:bg-red-500={$connectionStatus === 'disconnected'}
				></span>
				<span class="text-xs text-muted-foreground capitalize">{$connectionStatus}</span>
			</div>
		</div>
	</header>

	<!-- Main content -->
	<main class="flex flex-1 overflow-hidden">
		<!-- Now Playing (left side) -->
		<div class="flex flex-1 flex-col items-center justify-center p-8">
			<!-- Album Art -->
			<div
				class="mb-8 aspect-square w-full max-w-md overflow-hidden rounded-lg bg-card shadow-2xl"
			>
				{#if $player.artwork}
					<img src={$player.artwork} alt="Album art" class="h-full w-full object-cover" />
				{:else}
					<div class="flex h-full w-full items-center justify-center text-muted-foreground">
						<svg
							class="h-24 w-24"
							fill="none"
							stroke="currentColor"
							viewBox="0 0 24 24"
							xmlns="http://www.w3.org/2000/svg"
						>
							<path
								stroke-linecap="round"
								stroke-linejoin="round"
								stroke-width="1.5"
								d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"
							/>
						</svg>
					</div>
				{/if}
			</div>

			<!-- Track Info -->
			<div class="mb-6 text-center">
				<h2 class="text-2xl font-bold">{$player.title || 'No track playing'}</h2>
				<p class="text-lg text-muted-foreground">
					{$player.artist || 'Unknown artist'}
					{#if $player.album}
						<span class="mx-2">•</span>
						{$player.album}
					{/if}
				</p>
			</div>

			<!-- Progress Bar -->
			<div class="mb-6 w-full max-w-md">
				<div class="mb-1 h-1 w-full overflow-hidden rounded-full bg-secondary">
					<div
						class="h-full rounded-full bg-primary transition-all"
						style="width: {$player.duration > 0
							? ($player.position / $player.duration) * 100
							: 0}%"
					></div>
				</div>
				<div class="flex justify-between text-xs text-muted-foreground">
					<span>{formatTime($player.position)}</span>
					<span>{formatTime($player.duration)}</span>
				</div>
			</div>

			<!-- Playback Controls -->
			<div class="mb-8 flex items-center gap-6">
				<button class="rounded-full p-2 hover:bg-secondary">
					<svg
						class="h-8 w-8"
						fill="currentColor"
						viewBox="0 0 24 24"
						xmlns="http://www.w3.org/2000/svg"
					>
						<path d="M6 6h2v12H6zm3.5 6l8.5 6V6z" />
					</svg>
				</button>
				<button
					class="rounded-full bg-primary p-4 text-primary-foreground hover:bg-primary/90"
				>
					{#if $player.state === 'playing'}
						<svg
							class="h-8 w-8"
							fill="currentColor"
							viewBox="0 0 24 24"
							xmlns="http://www.w3.org/2000/svg"
						>
							<path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
						</svg>
					{:else}
						<svg
							class="h-8 w-8"
							fill="currentColor"
							viewBox="0 0 24 24"
							xmlns="http://www.w3.org/2000/svg"
						>
							<path d="M8 5v14l11-7z" />
						</svg>
					{/if}
				</button>
				<button class="rounded-full p-2 hover:bg-secondary">
					<svg
						class="h-8 w-8"
						fill="currentColor"
						viewBox="0 0 24 24"
						xmlns="http://www.w3.org/2000/svg"
					>
						<path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z" />
					</svg>
				</button>
			</div>

			<!-- Volume -->
			<div class="flex w-full max-w-md items-center gap-4">
				<button class="rounded-full p-1 hover:bg-secondary">
					<svg
						class="h-5 w-5"
						fill="currentColor"
						viewBox="0 0 24 24"
						xmlns="http://www.w3.org/2000/svg"
					>
						{#if $player.muted || $player.volume === 0}
							<path
								d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"
							/>
						{:else}
							<path
								d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"
							/>
						{/if}
					</svg>
				</button>
				<input
					type="range"
					min="0"
					max="100"
					value={$player.volume}
					class="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-secondary"
				/>
				<span class="w-8 text-right text-sm text-muted-foreground">{$player.volume}</span>
			</div>
		</div>

		<!-- Right sidebar (Queue + Browse) -->
		<aside class="flex w-96 flex-col border-l border-border">
			<!-- Queue -->
			<div class="flex-1 overflow-y-auto border-b border-border p-4">
				<div class="mb-4 flex items-center justify-between">
					<h3 class="font-semibold">Queue</h3>
					<span class="text-sm text-muted-foreground">0 tracks</span>
				</div>
				<div class="text-center text-muted-foreground">
					<p>Queue is empty</p>
					<p class="text-sm">Add tracks from the browser below</p>
				</div>
			</div>

			<!-- Browse -->
			<div class="flex-1 overflow-y-auto p-4">
				<div class="mb-4">
					<div class="flex gap-2">
						<button class="rounded-md bg-primary px-3 py-1 text-sm text-primary-foreground">
							UPnP
						</button>
						<button class="rounded-md bg-secondary px-3 py-1 text-sm hover:bg-secondary/80">
							Radio
						</button>
						<button class="rounded-md bg-secondary px-3 py-1 text-sm hover:bg-secondary/80">
							Podcasts
						</button>
					</div>
				</div>
				<div class="text-center text-muted-foreground">
					<p>Select a speaker to browse content</p>
				</div>
			</div>
		</aside>
	</main>
</div>

<script lang="ts" context="module">
	function formatTime(seconds: number): string {
		if (!seconds || seconds < 0) return '0:00';
		const mins = Math.floor(seconds / 60);
		const secs = Math.floor(seconds % 60);
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}
</script>
