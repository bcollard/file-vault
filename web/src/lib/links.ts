// Capability links carry both the token and the decryption key in the URL
// fragment: /d#<token>.<key> and /a#<admin-token>.<key>. Browsers never send
// the fragment to the server, so neither appears in any request log.

const PART = /^[A-Za-z0-9_-]{43}$/;

export type Capability = { token: string; key: string };

export function parseFragment(hash: string): Capability | null {
	const [token, key, ...rest] = hash.replace(/^#/, '').split('.');
	if (rest.length || !PART.test(token ?? '') || !PART.test(key ?? '')) return null;
	return { token, key };
}

export function buildLink(kind: 'd' | 'a', token: string, key: string): string {
	return `${location.origin}/${kind}#${token}.${key}`;
}
