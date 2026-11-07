// Moves ciphertext between the browser and object storage. Uploads go
// straight to the resumable session URL the server opened (GCS, or the Go
// server's /_local/ endpoints in LOCAL_MODE); downloads read a short-lived
// signed URL. The Go service never proxies file bytes.

import { Decryptor, encryptFile, type FileMeta, type Keys } from './crypto.ts';

// GCS requires every non-final chunk to be a multiple of 256 KiB.
export const UPLOAD_CHUNK = 32 * 256 * 1024; // 8 MiB

type Progress = (done: number, total: number) => void;

async function putChunk(url: string, data: Uint8Array<ArrayBuffer>, start: number, total: number) {
	const last = start + data.length === total;
	let lastErr: unknown;
	for (let attempt = 0; attempt < 5; attempt++) {
		if (attempt > 0) await new Promise((r) => setTimeout(r, 500 * 2 ** attempt));
		try {
			const resp = await fetch(url, {
				method: 'PUT',
				headers: { 'Content-Range': `bytes ${start}-${start + data.length - 1}/${total}` },
				body: data
			});
			// 308 = chunk stored, more expected. 200/201 = object complete.
			if ((!last && resp.status === 308) || (last && (resp.status === 200 || resp.status === 201))) {
				return;
			}
			lastErr = new Error(`upload chunk rejected: HTTP ${resp.status}`);
			if (resp.status >= 400 && resp.status < 500 && resp.status !== 408 && resp.status !== 429) break;
		} catch (e) {
			lastErr = e; // network error: retry. Storage ignores bytes it already has.
		}
	}
	throw lastErr;
}

/** Encrypts `file` and uploads it to `url` in UPLOAD_CHUNK pieces. */
export async function uploadEncrypted(
	url: string,
	keys: Keys,
	file: Blob,
	total: number,
	onProgress: Progress
): Promise<void> {
	let buf = new Uint8Array(UPLOAD_CHUNK);
	let fill = 0;
	let sent = 0;
	await encryptFile(keys, file, async (piece) => {
		let off = 0;
		while (off < piece.length) {
			const n = Math.min(piece.length - off, UPLOAD_CHUNK - fill);
			buf.set(piece.subarray(off, off + n), fill);
			fill += n;
			off += n;
			// Hold a full buffer back until we know it isn't the final chunk.
			if (fill === UPLOAD_CHUNK && sent + fill < total) {
				await putChunk(url, buf, sent, total);
				sent += fill;
				fill = 0;
				buf = new Uint8Array(UPLOAD_CHUNK);
				onProgress(sent, total);
			}
		}
	});
	await putChunk(url, buf.slice(0, fill), sent, total);
	onProgress(total, total);
}

/** Fetches the ciphertext at `url` and returns the decrypted file. */
export async function downloadDecrypted(
	url: string,
	keys: Keys,
	total: number,
	meta: FileMeta,
	onProgress: Progress
): Promise<Blob> {
	let resp: Response | undefined;
	// The signed URL is valid for about a minute; retry a network blip once.
	for (let attempt = 0; attempt < 2 && !resp; attempt++) {
		try {
			resp = await fetch(url, { cache: 'no-store' });
		} catch (e) {
			if (attempt === 1) throw e;
		}
	}
	if (!resp?.ok || !resp.body) throw new Error(`download failed: HTTP ${resp?.status}`);

	const dec = new Decryptor(keys, total);
	const parts: BlobPart[] = [];
	const reader = resp.body.getReader();
	let got = 0;
	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		got += value.length;
		for (const p of await dec.push(value)) parts.push(p);
		onProgress(got, total);
	}
	dec.finish();
	return new Blob(parts, { type: meta.type || 'application/octet-stream' });
}

export function saveBlob(blob: Blob, name: string) {
	const href = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = href;
	a.download = name;
	a.rel = 'noopener';
	document.body.appendChild(a);
	a.click();
	a.remove();
	setTimeout(() => URL.revokeObjectURL(href), 60_000);
}
