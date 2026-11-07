// Static SPA: every page is prerendered as an empty shell and rendered in the
// browser, because all state comes from the URL fragment, which only the
// browser can read.
export const prerender = true;
export const ssr = false;
