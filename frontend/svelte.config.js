import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		adapter: adapter({
			pages: 'build',
			assets: 'build',
			fallback: 'index.html',
			precompress: false,
			strict: true
		}),
		alias: {
			$components: 'src/lib/components',
			$stores: 'src/lib/stores',
			$api: 'src/lib/api'
		},
		prerender: {
			handleHttpError: ({ path, message }) => {
				// Ignore missing static assets during prerendering
				if (path.endsWith('.png') || path.endsWith('.ico')) {
					console.warn(`Warning: ${message}`);
					return;
				}
				throw new Error(message);
			}
		}
	}
};

export default config;
