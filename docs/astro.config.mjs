// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Published to GitHub Pages as a project site: https://<owner>.github.io/<repo>/.
// The Pages workflow sets SITE and BASE from the repository, so forks publish
// under their own owner and name without editing this file.
export default defineConfig({
	site: process.env.SITE ?? 'http://localhost:4321',
	base: process.env.BASE ?? '/',
	integrations: [
		starlight({
			title: 'file-vault',
			description: 'End-to-end encrypted, self-destructing file sharing you host on Google Cloud.',
			logo: { src: './src/assets/logo.svg' },
			favicon: '/favicon.svg',
			customCss: ['./src/styles/theme.css'],
			social: process.env.REPO_URL
				? [{ icon: 'github', label: 'GitHub', href: process.env.REPO_URL }]
				: [],
			editLink: process.env.REPO_URL
				? { baseUrl: `${process.env.REPO_URL}/edit/main/docs/` }
				: undefined,
			sidebar: [
				{
					label: 'Guides',
					items: [
						{ label: 'How it works', slug: 'guides/how-it-works' },
						{ label: 'Security model', slug: 'guides/security' }
					]
				},
				{
					label: 'Self-hosting',
					items: [
						{ label: 'Deploy on Google Cloud', slug: 'self-hosting/gcp' },
						{ label: 'Load balancer and DNS', slug: 'self-hosting/load-balancer' },
						{ label: 'Configuration', slug: 'self-hosting/configuration' }
					]
				},
				{
					label: 'Development',
					items: [{ label: 'Local development', slug: 'development/local' }]
				},
				{
					label: 'Reference',
					items: [
						{ label: 'HTTP API', slug: 'reference/api' },
						{ label: 'Encryption format', slug: 'reference/encryption-format' }
					]
				}
			]
		})
	]
});
