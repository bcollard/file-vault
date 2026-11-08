// Click a diagram or screenshot to view it full size; click again or press
// Escape to close. Plain script, no dependencies.
(() => {
	const selector = '.sl-markdown-content img[src$=".svg"], .sl-markdown-content img.screenshot';
	let overlay;
	const close = () => {
		overlay?.remove();
		overlay = undefined;
	};
	document.addEventListener('click', (e) => {
		const img = e.target instanceof Element ? e.target.closest(selector) : null;
		if (overlay && (e.target === overlay || overlay.contains(e.target))) return close();
		if (!img) return;
		overlay = document.createElement('div');
		overlay.className = 'zoom-overlay';
		overlay.setAttribute('role', 'dialog');
		overlay.setAttribute('aria-label', img.alt || 'Enlarged image');
		const big = img.cloneNode();
		big.removeAttribute('width');
		big.removeAttribute('height');
		big.removeAttribute('class');
		overlay.append(big);
		document.body.append(overlay);
	});
	document.addEventListener('keydown', (e) => e.key === 'Escape' && close());
})();
