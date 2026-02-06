/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

// Create a unique cache name for this deployment
const CACHE_NAME = `kefw2ui-cache-${version}`;

// Assets to cache immediately on install
const PRECACHE_ASSETS = [
	...build, // the app itself (JS/CSS bundles)
	...files  // everything in `static` (icons, manifest, etc.)
];

// Install: cache all static assets
sw.addEventListener('install', (event) => {
	event.waitUntil(
		caches
			.open(CACHE_NAME)
			.then((cache) => cache.addAll(PRECACHE_ASSETS))
			.then(() => {
				// Skip waiting to activate immediately
				sw.skipWaiting();
			})
	);
});

// Activate: clean up old caches
sw.addEventListener('activate', (event) => {
	event.waitUntil(
		caches.keys().then(async (keys) => {
			// Delete old caches
			for (const key of keys) {
				if (key !== CACHE_NAME) {
					await caches.delete(key);
				}
			}
			// Take control of all clients immediately
			sw.clients.claim();
		})
	);
});

// Fetch: serve from cache, fallback to network
sw.addEventListener('fetch', (event) => {
	const url = new URL(event.request.url);

	// Skip non-GET requests
	if (event.request.method !== 'GET') {
		return;
	}

	// Skip API requests and SSE - these must always go to network
	if (url.pathname.startsWith('/api/') || url.pathname === '/events') {
		return;
	}

	// Skip cross-origin requests (external images, etc.)
	if (url.origin !== location.origin) {
		return;
	}

	event.respondWith(
		caches.match(event.request).then((cachedResponse) => {
			if (cachedResponse) {
				// Return cached response immediately
				// Also fetch fresh version in background (stale-while-revalidate for static assets)
				if (isStaticAsset(url.pathname)) {
					fetchAndCache(event.request);
				}
				return cachedResponse;
			}

			// Not in cache, fetch from network
			return fetch(event.request)
				.then((response) => {
					// Cache successful responses for static assets
					if (response.ok && isStaticAsset(url.pathname)) {
						const responseClone = response.clone();
						caches.open(CACHE_NAME).then((cache) => {
							cache.put(event.request, responseClone);
						});
					}
					return response;
				})
				.catch(() => {
					// Network failed - return offline fallback for navigation requests
					if (event.request.mode === 'navigate') {
						return caches.match('/') || new Response('Offline', { status: 503 });
					}
					return new Response('Offline', { status: 503 });
				});
		})
	);
});

// Helper: check if this is a static asset that should be cached
function isStaticAsset(pathname: string): boolean {
	return (
		pathname.startsWith('/_app/') || // SvelteKit app bundles
		pathname.endsWith('.js') ||
		pathname.endsWith('.css') ||
		pathname.endsWith('.png') ||
		pathname.endsWith('.jpg') ||
		pathname.endsWith('.svg') ||
		pathname.endsWith('.ico') ||
		pathname.endsWith('.woff') ||
		pathname.endsWith('.woff2') ||
		pathname.endsWith('.json')
	);
}

// Helper: fetch and update cache in background
async function fetchAndCache(request: Request): Promise<void> {
	try {
		const response = await fetch(request);
		if (response.ok) {
			const cache = await caches.open(CACHE_NAME);
			await cache.put(request, response);
		}
	} catch {
		// Ignore network errors during background refresh
	}
}

// Handle messages from the app
sw.addEventListener('message', (event) => {
	if (event.data?.type === 'SKIP_WAITING') {
		sw.skipWaiting();
	}
});
