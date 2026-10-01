# file-vault — invariants for editors

Read [README.md](README.md) first for setup and architecture. This file
captures non-obvious rules that aren't visible from the code alone. Each is
load-bearing — don't remove or "simplify" without thinking about why.

## Security model

- **Capability URLs are the only auth.** The download token and admin token
  ARE the credentials. Do not add login screens, passwords, or sessions
  without explicit discussion — the no-account model is deliberate.
- **Tokens are stored as HMAC(pepper, token).** Raw tokens are unrecoverable
  by design. Do not add a "show this URL again" feature; the only way to
  recover access is to issue a new token from the admin console.
- **Per-IP rate limiter is per-Cloud-Run-instance.** Each replica counts
  independently. DDoS protection lives at the LB (Cloud Armor), not in the
  Go limiter. Don't conflate the two.

## Request flow gotchas

- **Do not collapse `GET /d/{token}` and the burn into one step.** The GET
  renders an HTML confirmation page (no state change); the actual burn happens
  on `POST /d/{token}/confirm`. This split defeats link-unfurl preflight
  (Slack, Outlook Safe Links, iMessage) — without it, a shared link gets
  consumed before the recipient ever clicks.
- **Signed URLs always force `response-content-disposition=attachment`.**
  Uploaded HTML would otherwise execute as a page on `storage.googleapis.com`.
  The filename encoding in `signer.go` is RFC 5987 dual (ASCII + UTF-8) — keep
  both.
- **303 See Other, not 302.** The burn handler redirects with 303 to force the
  browser to GET the signed URL after the POST. 302 doesn't guarantee that.

## Operational constraints

- **Cloud Run has a hard 32 MiB request limit.** `maxUploadBytes` in
  `api/internal/server/server.go` is set to match. Bigger uploads require
  signed-resumable upload URLs (browser → GCS direct) — not bumping the
  middleware limit.
- **API ingress = `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER`.** The web service
  reaches it via VPC egress `ALL_TRAFFIC` so `*.run.app` calls route through
  the VPC and satisfy the internal-ingress check. Changing API ingress to
  public would expose the internal admin endpoints — only do it if you also
  add ID-token verification at the API layer.

## Repo conventions

- **`web/` runs in two modes.** Setting `PREVIEW_MODE=1` swaps the HTTP API
  client for an in-process stub returning canned data. `make web-dev` sets it
  for local browsing without GCP credentials.
- **`web/package-lock.json` is committed.** CI uses `npm ci`. Don't add it to
  `.gitignore`.
- **Don't search other customers' projects for older `populate_gdoc.py`,
  bootstrap scripts, or similar.** This project has its own canonical copies
  under `cicd/` and `terraform/`.

## What's intentional and not a bug

- Admin tokens have a TTL field on Firestore (`expires_at`) — same TTL as the
  share — so orphaned admin docs self-clean.
- `/d/{token}` returns 404 (not 410) on unknown/expired/revoked/exhausted —
  deliberate, to avoid enumeration signal.
- `RevokeOutstandingTokens` filters client-side. The previous server-side
  inequality filter would have required a composite Firestore index we don't
  ship.
