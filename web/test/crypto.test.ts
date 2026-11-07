// Run with `npm test` (node --test; Node strips the TypeScript types).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
	RECORD_SIZE,
	ciphertextSize,
	decryptMeta,
	deriveKeys,
	encryptFile,
	encryptMeta,
	newKey,
	Decryptor,
	DecryptError,
	b64urlDecode,
	b64urlEncode
} from '../src/lib/crypto.ts';

async function encrypt(keys: Awaited<ReturnType<typeof deriveKeys>>, data: Uint8Array<ArrayBuffer>) {
	const parts: Uint8Array<ArrayBuffer>[] = [];
	await encryptFile(keys, new Blob([data]), async (p) => {
		parts.push(p);
	});
	return new Uint8Array(await new Blob(parts).arrayBuffer());
}

async function decrypt(keys: Awaited<ReturnType<typeof deriveKeys>>, ct: Uint8Array<ArrayBuffer>, slice = 65536) {
	const d = new Decryptor(keys, ct.length);
	const out: Uint8Array<ArrayBuffer>[] = [];
	for (let i = 0; i < ct.length; i += slice) out.push(...(await d.push(ct.subarray(i, i + slice))));
	d.finish();
	return new Uint8Array(await new Blob(out).arrayBuffer());
}

const sizes = [0, 1, 1000, RECORD_SIZE - 1, RECORD_SIZE, RECORD_SIZE + 1, 2 * RECORD_SIZE + 12345];

test('round trip at record boundaries', async () => {
	const keys = await deriveKeys(newKey());
	for (const n of sizes) {
		const data = crypto.getRandomValues(new Uint8Array(n > 65536 ? 65536 : n));
		const full = new Uint8Array(n);
		for (let i = 0; i < n; i += data.length || 1) full.set(data.subarray(0, Math.min(data.length, n - i)), i);
		const ct = await encrypt(keys, full);
		assert.equal(ct.length, ciphertextSize(n), `ciphertext size for ${n}`);
		for (const slice of [1, 4096, 1 << 20, ct.length || 1]) {
			if (slice === 1 && n > 5000) continue;
			assert.deepEqual(await decrypt(keys, ct, slice), full, `n=${n} slice=${slice}`);
		}
	}
});

test('wrong key, truncation, reordering and tampering are rejected', async () => {
	const keys = await deriveKeys(newKey());
	const data = crypto.getRandomValues(new Uint8Array(65536));
	const big = new Uint8Array(2 * RECORD_SIZE + 100);
	big.set(data);
	const ct = await encrypt(keys, big);
	const rec = RECORD_SIZE + 16;

	await assert.rejects(decrypt(await deriveKeys(newKey()), ct), DecryptError);

	// Truncate at a record boundary: the shortened stream's last record is not marked final.
	await assert.rejects(decrypt(keys, ct.slice(0, 4 + rec)), DecryptError);
	await assert.rejects(decrypt(keys, ct.slice(0, ct.length - 1)), DecryptError);

	// Swap the first two records.
	const swapped = new Uint8Array(ct);
	swapped.set(ct.subarray(4 + rec, 4 + 2 * rec), 4);
	swapped.set(ct.subarray(4, 4 + rec), 4 + rec);
	await assert.rejects(decrypt(keys, swapped), DecryptError);

	const flipped = new Uint8Array(ct);
	flipped[100] ^= 1;
	await assert.rejects(decrypt(keys, flipped), DecryptError);
});

test('metadata round trip and wrong key', async () => {
	const keys = await deriveKeys(newKey());
	const meta = { name: 'rapport été.pdf', type: 'application/pdf', size: 42 };
	const enc = await encryptMeta(keys, meta);
	assert.match(enc, /^[A-Za-z0-9_-]+$/);
	assert.deepEqual(await decryptMeta(keys, enc), meta);
	await assert.rejects(decryptMeta(await deriveKeys(newKey()), enc), DecryptError);
});

test('base64url', () => {
	const b = crypto.getRandomValues(new Uint8Array(32));
	const s = b64urlEncode(b);
	assert.equal(s.length, 43);
	assert.deepEqual(b64urlDecode(s), b);
	assert.throws(() => b64urlDecode('a+b'), DecryptError);
});
