// Post-build: Astro lazy-loads every Markdown image. The first image inside a
// page's content is usually above the fold and is the Largest Contentful
// Paint element, so make it load eagerly with high priority. Header images
// (the logo) come before the content marker and are left alone.
import { readdirSync, readFileSync, writeFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const MARKER = 'sl-markdown-content';
let changed = 0;
const walk = (dir) =>
	readdirSync(dir).flatMap((f) => {
		const p = join(dir, f);
		return statSync(p).isDirectory() ? walk(p) : p.endsWith('.html') ? [p] : [];
	});

for (const file of walk('dist')) {
	const html = readFileSync(file, 'utf8');
	const at = html.indexOf(MARKER);
	if (at < 0) continue;
	const rest = html.slice(at).replace(/<img\b([^>]*?)\sloading="lazy"([^>]*)>/, (m, a, b) => {
		changed++;
		return `<img${a} loading="eager" fetchpriority="high"${b}>`;
	});
	writeFileSync(file, html.slice(0, at) + rest);
}
console.log(`eager-first-image: ${changed} page(s) updated`);
