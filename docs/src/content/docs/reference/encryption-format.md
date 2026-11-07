---
title: Encryption format
description: The FVE1 ciphertext format, key derivation and metadata encoding.
---

All cryptography runs in the browser with WebCrypto. The implementation is `web/src/lib/crypto.ts`.

## Key

- A fresh 256-bit key is generated for every share with `crypto.getRandomValues`.
- It is encoded as 43 characters of unpadded base64url and placed in the link fragment after the token: `#<token>.<key>`.
- HKDF-SHA256 derives two AES-256-GCM keys from it, with a 32-byte zero salt:

| Info string | Key used for |
|---|---|
| `file-vault/v1/content` | File content |
| `file-vault/v1/meta` | Metadata |

## Content: format FVE1

```
"FVE1" | record_0 | record_1 | … | record_n-1
```

- The 4-byte magic `FVE1` identifies the format version.
- The plaintext is split into 1 MiB chunks. The last chunk may be shorter, and an empty file has one empty chunk.
- Each record is `AES-GCM(contentKey, nonce_i, aad_i, chunk_i)`: the chunk's ciphertext followed by a 16-byte tag.

| Parameter | Value |
|---|---|
| `nonce_i` | 4 zero bytes followed by the record index as a 64-bit big-endian integer |
| `aad_i` | 1 byte: `0x01` for the last record, `0x00` otherwise |

Ciphertext size for a plaintext of `n` bytes:

```
4 + n + 16 × max(1, ceil(n / 1 MiB))
```

### Why it's built this way

- **Counter nonces are safe** because every share has its own key, so a nonce is never used twice with the same key.
- **The index in the nonce** makes swapped or duplicated records fail authentication.
- **The final flag in the additional data** makes truncation detectable: a stream cut at a record boundary ends with a record that wasn't encrypted as final, so decryption fails.
- **1 MiB records** let both sides stream: the uploader holds one record at a time, and the downloader decrypts while it receives.

The decryptor knows the total ciphertext length from the server, so it knows where the final record must end. It rejects data after the final record and streams that end early.

## Metadata

```
base64url( nonce(12 random bytes) | AES-GCM(metaKey, nonce, aad = "file-vault/v1/meta", json) )
```

The JSON is `{"name": "<file name>", "type": "<MIME type>", "size": <plaintext bytes>}`. The server stores the base64url string as-is and returns it to holders of a valid token.

## Changing the format

A new format needs a new magic (`FVE2`), and the decoder must keep reading `FVE1` until every share created before the change has expired (7 days by default).
