// Applies the saved or system theme before first paint. A static file rather
// than an inline script, so the CSP needs no extra hash.
try {
	const t = localStorage.getItem('theme');
	if (t === 'dark' || (!t && matchMedia('(prefers-color-scheme: dark)').matches)) {
		document.documentElement.classList.add('dark');
	}
} catch {
	// storage blocked: fall back to light
}
