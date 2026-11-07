// End-to-end round trip against a running server: `make e2e` starts one in
// LOCAL_MODE. Uses the same modules the browser runs (api, crypto, transfer).
// Skipped unless E2E_BASE_URL is set.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { api, ApiError } from '../src/lib/api.ts';
import { b64urlDecode, b64urlEncode, ciphertextSize, decryptMeta, deriveKeys, encryptMeta, newKey } from '../src/lib/crypto.ts';
import { downloadDecrypted, uploadEncrypted, UPLOAD_CHUNK } from '../src/lib/transfer.ts';

const base = process.env.E2E_BASE_URL;

// The browser resolves relative URLs against the page origin; Node needs help.
if (base) {
	const real = globalThis.fetch;
	globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit) =>
		real(typeof input === 'string' && input.startsWith('/') ? base + input : input, init);
}

async function sha256(b: Blob) {
	return b64urlEncode(new Uint8Array(await crypto.subtle.digest('SHA-256', await b.arrayBuffer())));
}

test('upload, peek, burn, decrypt', { skip: !base }, async () => {
	// Two full upload chunks plus a remainder, with a record boundary inside.
	const size = 2 * UPLOAD_CHUNK + 777_777;
	const plain = new Uint8Array(size);
	for (let i = 0; i < size; i += 65536) crypto.getRandomValues(plain.subarray(i, i + 65536));
	const file = new Blob([plain], { type: 'application/pdf' });

	const raw = newKey();
	const keys = await deriveKeys(raw);
	const total = ciphertextSize(size);
	const meta = await encryptMeta(keys, { name: 'report.pdf', type: 'application/pdf', size });

	const created = await api.createShare({ size: total, meta, ttl_hours: 24, max_uses: 2 });
	let lastProgress = 0;
	await uploadEncrypted(created.upload_url, keys, file, total, (d, t) => {
		assert.ok(d >= lastProgress && d <= t);
		lastProgress = d;
	});
	assert.equal(lastProgress, total);
	const fin = await api.finalize(created.admin_token);

	// The recipient side starts from nothing but the fragment.
	const fragment = `${fin.download_token}.${b64urlEncode(raw)}`;
	const [token, key] = fragment.split('.');
	const rkeys = await deriveKeys(b64urlDecode(key));
	const peek = await api.peek(token);
	assert.equal(peek.uses_remaining, 2);
	const rmeta = await decryptMeta(rkeys, peek.meta);
	assert.deepEqual(rmeta, { name: 'report.pdf', type: 'application/pdf', size });

	const burn = await api.burn(token);
	// What storage holds is ciphertext with our header, not the plaintext.
	const stored = new Uint8Array(await (await fetch(burn.url)).arrayBuffer());
	assert.equal(stored.length, total);
	assert.equal(new TextDecoder().decode(stored.subarray(0, 4)), 'FVE1');
	assert.notDeepEqual(stored.subarray(4, 68), plain.subarray(0, 64));

	const out = await downloadDecrypted(burn.url, rkeys, burn.size, rmeta, () => {});
	assert.equal(out.size, size);
	assert.equal(await sha256(out), await sha256(file));

	await api.burn(token);
	await assert.rejects(api.burn(token), (e) => e instanceof ApiError && e.notFound);

	const admin = await api.admin(created.admin_token);
	assert.equal(admin.status, 'ready');
	assert.equal(admin.tokens[0].status, 'exhausted');

	await api.remove(created.admin_token);
	await assert.rejects(api.peek(token), (e) => e instanceof ApiError && e.notFound);
});

test('empty file', { skip: !base }, async () => {
	const raw = newKey();
	const keys = await deriveKeys(raw);
	const total = ciphertextSize(0);
	const created = await api.createShare({
		size: total,
		meta: await encryptMeta(keys, { name: 'empty.txt', type: '', size: 0 }),
		ttl_hours: 1,
		max_uses: 1
	});
	await uploadEncrypted(created.upload_url, keys, new Blob([]), total, () => {});
	const fin = await api.finalize(created.admin_token);
	const burn = await api.burn(fin.download_token);
	const out = await downloadDecrypted(burn.url, keys, burn.size, { name: 'empty.txt', type: '', size: 0 }, () => {});
	assert.equal(out.size, 0);
});
