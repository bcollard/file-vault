import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// The Go server (`make dev`, LOCAL_MODE) answers /api and /_local; Vite
// serves the UI with hot reload and proxies the rest.
const GO = process.env.GO_SERVER ?? 'http://localhost:8080';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			preprocess: vitePreprocess(),
			// Static build, embedded into the Go binary. Every route is
			// prerendered; 200.html is the fallback for unknown paths.
			adapter: adapter({ fallback: '200.html' }),
			// Emitted as a <meta> tag with hashes of SvelteKit's inline
			// bootstrap script. frame-ancestors is sent as a header by Go.
			csp: {
				mode: 'hash',
				directives: {
					'default-src': ['self'],
					'script-src': ['self'],
					'style-src': ['self', 'unsafe-inline'],
					'img-src': ['self', 'data:', 'blob:'],
					'connect-src': ['self', 'https://storage.googleapis.com'],
					'font-src': ['self'],
					'object-src': ['none'],
					'base-uri': ['none'],
					'form-action': ['self']
				}
			}
		})
	],
	server: {
		port: 5173,
		proxy: {
			'/api': GO,
			'/_local': GO
		}
	}
});
