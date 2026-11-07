// End-to-end encryption for file-vault. Runs only in the browser (WebCrypto);
// the server stores ciphertext and never sees the key.
//
// Key: 32 random bytes, carried in the URL fragment (#token.key), which
// browsers never send to the server. HKDF-SHA256 derives two AES-256-GCM keys
// from it: one for the file content, one for the metadata.
//
// Ciphertext object (format v1):
//   "FVE1" | record_0 | record_1 | … | record_n-1
//   record_i = AES-GCM(contentKey, nonce = 0x00000000 ‖ uint64be(i),
//                      aad = [final ? 1 : 0], chunk_i)
//   Plaintext chunks are RECORD_SIZE bytes except the last, which may be
//   shorter (or empty for an empty file). The key is unique per file, so a
//   counter nonce never repeats. The index in the nonce stops reordering and
//   the final flag in the AAD stops truncation.
//
// Metadata: base64url(nonce(12, random) ‖ AES-GCM(metaKey, nonce,
//   aad = "file-vault/v1/meta", JSON {name, type, size})).

export const RECORD_SIZE = 1024 * 1024;
const TAG_SIZE = 16;
const MAGIC = new Uint8Array([0x46, 0x56, 0x45, 0x31]); // "FVE1"
const META_AAD = new TextEncoder().encode('file-vault/v1/meta');

export type FileMeta = { name: string; type: string; size: number };
export type Keys = { content: CryptoKey; meta: CryptoKey };
type Bytes = Uint8Array<ArrayBuffer>;

export class DecryptError extends Error {
	constructor(message = 'decryption failed') {
		super(message);
		this.name = 'DecryptError';
	}
}

export function b64urlEncode(bytes: Uint8Array): string {
	let bin = '';
	for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
	return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function b64urlDecode(s: string): Bytes {
	if (!/^[A-Za-z0-9_-]*$/.test(s)) throw new DecryptError('invalid base64url');
	const pad = s.length % 4 === 0 ? '' : '='.repeat(4 - (s.length % 4));
	const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/') + pad);
	const out = new Uint8Array(bin.length);
	for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
	return out;
}

export function newKey(): Bytes {
	return crypto.getRandomValues(new Uint8Array(32));
}

export async function deriveKeys(raw: Uint8Array): Promise<Keys> {
	if (raw.length !== 32) throw new DecryptError('invalid key length');
	const base = await crypto.subtle.importKey('raw', new Uint8Array(raw), 'HKDF', false, [
		'deriveKey'
	]);
	const derive = (info: string) =>
		crypto.subtle.deriveKey(
			{
				name: 'HKDF',
				hash: 'SHA-256',
				salt: new Uint8Array(32),
				info: new TextEncoder().encode(info)
			},
			base,
			{ name: 'AES-GCM', length: 256 },
			false,
			['encrypt', 'decrypt']
		);
	const [content, meta] = await Promise.all([
		derive('file-vault/v1/content'),
		derive('file-vault/v1/meta')
	]);
	return { content, meta };
}

export async function encryptMeta(keys: Keys, meta: FileMeta): Promise<string> {
	const iv = crypto.getRandomValues(new Uint8Array(12));
	const pt = new TextEncoder().encode(JSON.stringify(meta));
	const ct = new Uint8Array(
		await crypto.subtle.encrypt({ name: 'AES-GCM', iv, additionalData: META_AAD }, keys.meta, pt)
	);
	const out = new Uint8Array(iv.length + ct.length);
	out.set(iv);
	out.set(ct, iv.length);
	return b64urlEncode(out);
}

export async function decryptMeta(keys: Keys, encoded: string): Promise<FileMeta> {
	const raw = b64urlDecode(encoded);
	if (raw.length < 12 + TAG_SIZE) throw new DecryptError();
	let pt: ArrayBuffer;
	try {
		pt = await crypto.subtle.decrypt(
			{ name: 'AES-GCM', iv: raw.slice(0, 12), additionalData: META_AAD },
			keys.meta,
			raw.slice(12)
		);
	} catch {
		throw new DecryptError();
	}
	const m = JSON.parse(new TextDecoder().decode(pt));
	if (typeof m?.name !== 'string' || typeof m?.size !== 'number') throw new DecryptError();
	return { name: m.name, type: typeof m.type === 'string' ? m.type : '', size: m.size };
}

function recordCount(plainSize: number): number {
	return Math.max(1, Math.ceil(plainSize / RECORD_SIZE));
}

export function ciphertextSize(plainSize: number): number {
	return MAGIC.length + plainSize + recordCount(plainSize) * TAG_SIZE;
}

function nonce(i: number): Bytes {
	const n = new Uint8Array(12);
	new DataView(n.buffer).setBigUint64(4, BigInt(i));
	return n;
}

const AAD_MID = new Uint8Array([0]);
const AAD_FINAL = new Uint8Array([1]);

/**
 * Encrypts `file` record by record and hands each ciphertext piece to `emit`
 * in order (the header first). Only one record is held in memory at a time.
 */
export async function encryptFile(
	keys: Keys,
	file: Blob,
	emit: (piece: Bytes) => Promise<void>
): Promise<void> {
	await emit(new Uint8Array(MAGIC));
	const n = recordCount(file.size);
	for (let i = 0; i < n; i++) {
		const start = i * RECORD_SIZE;
		const chunk = new Uint8Array(
			await file.slice(start, Math.min(start + RECORD_SIZE, file.size)).arrayBuffer()
		);
		const ct = await crypto.subtle.encrypt(
			{ name: 'AES-GCM', iv: nonce(i), additionalData: i === n - 1 ? AAD_FINAL : AAD_MID },
			keys.content,
			chunk
		);
		await emit(new Uint8Array(ct));
	}
}

/**
 * Streaming decryptor. Feed ciphertext with push() in any slice sizes; it
 * returns plaintext records as they complete. finish() throws if the stream
 * was truncated. `total` is the ciphertext length the server reported.
 */
export class Decryptor {
	private readonly keys: Keys;
	private readonly total: number;
	private pending: Uint8Array[] = [];
	private pendingLen = 0;
	private consumed = 0;
	private index = 0;
	private headerDone = false;
	private done = false;

	constructor(keys: Keys, total: number) {
		this.keys = keys;
		this.total = total;
	}

	/** Removes and returns exactly n buffered bytes (caller checks pendingLen). */
	private take(n: number): Bytes {
		const out = new Uint8Array(n);
		let off = 0;
		while (off < n) {
			const head = this.pending[0];
			const k = Math.min(head.length, n - off);
			out.set(head.subarray(0, k), off);
			off += k;
			if (k === head.length) this.pending.shift();
			else this.pending[0] = head.subarray(k);
		}
		this.pendingLen -= n;
		return out;
	}

	async push(data: Uint8Array): Promise<Bytes[]> {
		if (data.length === 0) return [];
		if (this.done) throw new DecryptError('data after final record');
		this.pending.push(data);
		this.pendingLen += data.length;

		const out: Bytes[] = [];
		if (!this.headerDone) {
			if (this.pendingLen < MAGIC.length) return out;
			const head = this.take(MAGIC.length);
			if (!MAGIC.every((b, i) => head[i] === b)) throw new DecryptError('not a file-vault object');
			this.consumed = MAGIC.length;
			this.headerDone = true;
		}
		while (!this.done) {
			const recLen = Math.min(RECORD_SIZE + TAG_SIZE, this.total - this.consumed);
			if (recLen < TAG_SIZE) throw new DecryptError('truncated');
			if (this.pendingLen < recLen) break;
			const final = this.consumed + recLen === this.total;
			let pt: ArrayBuffer;
			try {
				pt = await crypto.subtle.decrypt(
					{
						name: 'AES-GCM',
						iv: nonce(this.index),
						additionalData: final ? AAD_FINAL : AAD_MID
					},
					this.keys.content,
					this.take(recLen)
				);
			} catch {
				throw new DecryptError();
			}
			out.push(new Uint8Array(pt));
			this.consumed += recLen;
			this.index++;
			this.done = final;
		}
		if (this.done && this.pendingLen > 0) throw new DecryptError('data after final record');
		return out;
	}

	finish(): void {
		if (!this.done || this.pendingLen > 0) throw new DecryptError('truncated');
	}
}
