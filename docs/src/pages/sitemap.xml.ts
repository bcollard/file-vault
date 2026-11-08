// /sitemap.xml: the conventional sitemap URL. Astro's sitemap integration
// (enabled by Starlight) writes sitemap-index.xml + sitemap-0.xml; this
// endpoint lists the same pages under the name most tools look for first.
import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';

export const GET: APIRoute = async ({ site }) => {
	const base = import.meta.env.BASE_URL.replace(/\/?$/, '/');
	const origin = site ?? new URL('http://localhost:4321');
	const pages = await getCollection('docs');
	const urls = pages
		.map((p) => (p.id === 'index' ? '' : `${p.id.replace(/\/index$/, '')}/`))
		.sort()
		.map((path) => new URL(base + path, origin).href);

	const body = [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		...urls.map((u) => `  <url><loc>${u}</loc></url>`),
		'</urlset>',
		''
	].join('\n');
	return new Response(body, { headers: { 'Content-Type': 'application/xml' } });
};
