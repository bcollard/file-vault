---
title: HTTP API
description: The JSON endpoints the UI uses.
---

All endpoints accept and return JSON. Tokens are 43-character base64url strings and always travel in the request body. Errors return `{"error": "<message>"}`.

Any failure involving a token (unknown, expired, revoked, used up, malformed) returns `404`, so responses don't reveal whether a token exists.

## `GET /api/config`

The limits the UI needs.

```json
{
  "default_ttl_hours": 48, "min_ttl_hours": 1, "max_ttl_hours": 168,
  "default_max_uses": 1, "max_uses_per_token": 100,
  "max_upload_bytes": 1073741824, "share_ttl_hours": 168
}
```

## `POST /api/shares`

Opens a pending share and a resumable upload session. Rate-limited as an upload.

| Field | Type | Notes |
|---|---|---|
| `size` | number | Ciphertext size in bytes, at least 20 and at most `max_upload_bytes` |
| `meta` | string | Encrypted metadata, base64url, at most 4096 characters |
| `ttl_hours` | number | First download link's expiry, clamped to bounds |
| `max_uses` | number | First download link's use count, clamped to bounds |

Returns `201`:

```json
{ "admin_token": "…", "upload_url": "https://storage.googleapis.com/upload/…", "expires_at": "…" }
```

Upload the ciphertext to `upload_url` with `PUT` requests carrying `Content-Range: bytes <start>-<end>/<size>`. Non-final chunks must be multiples of 256 KiB. The storage answers `308` for intermediate chunks and `200` or `201` for the last.

## `POST /api/shares/finalize`

```json
{ "admin_token": "…" }
```

Checks the object size, marks the share ready and creates the first download token. Returns `409` if the upload is incomplete or the share is already finalized, `400` on a size mismatch (the object is deleted).

```json
{ "download_token": "…", "download_expires_at": "…", "max_uses": 1, "expires_at": "…" }
```

## `POST /api/download/peek`

```json
{ "token": "…" }
```

Validates a download token without consuming it.

```json
{ "meta": "…", "size": 1048596, "expires_at": "…", "uses_remaining": 1, "max_uses": 1 }
```

## `POST /api/download/burn`

```json
{ "token": "…" }
```

Consumes one use in a transaction and returns a signed URL for the ciphertext, valid for `DOWNLOAD_URL_TTL_SECONDS`.

```json
{ "url": "https://storage.googleapis.com/…", "size": 1048596 }
```

## Admin endpoints

All take `{"admin_token": "…"}` plus the fields listed.

| Endpoint | Extra fields | Returns |
|---|---|---|
| `POST /api/admin` | — | Share metadata, status (`pending` or `ready`) and every download token with status, uses and last use |
| `POST /api/admin/issue` | `ttl_hours`, `max_uses` | `201` with a new `download_token`. `409` if the share isn't ready |
| `POST /api/admin/revoke` | — | `{"revoked": <count>}` |
| `POST /api/admin/delete` | — | `204`. Deletes the object, the share and every token |

## `GET /healthz`

`200` with an empty body.
