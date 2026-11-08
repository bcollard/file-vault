# file-vault — invariants for editors

Read [README.md](README.md) first for setup and architecture. This file
captures non-obvious rules that aren't visible from the code alone. Each is
load-bearing — don't remove or "simplify" without thinking about why.

## Security model

- **Capability URLs are the only auth.** The download token and admin token
  ARE the credentials. Do not add login screens, passwords, or sessions
  without explicit discussion — the no-account model is deliberate.
- **Tokens and the key live in the URL fragment** (`/d#<token>.<key>`,
  `/a#<admin>.<key>`). Browsers never send the fragment, so tokens can't reach
  app, Cloud Run or load-balancer request logs. The API takes tokens **only in
  POST bodies**. Never add a route with a token in its path or query string,
  and never log request bodies. `TestFullFlow` greps the logs for tokens.
- **The server never sees plaintext.** The browser encrypts content and
  metadata (`web/src/lib/crypto.ts`, format documented in its header). Don't
  add server-side fields that hold the file name, type, or anything derived
  from plaintext. Changing the format needs a new magic (`FVE2`) and a decoder
  that still reads `FVE1` until old shares expire (7 days).
- **Tokens are stored as HMAC(pepper, token).** Raw tokens are unrecoverable
  by design. Do not add a "show this URL again" feature; the only way to
  recover access is to issue a new token from the admin console.
- **Client IP = second-from-right X-Forwarded-For entry** (`TRUSTED_PROXY_HOPS=2`,
  Google external ALB). The leftmost entry is client-controlled. If the
  proxy chain changes (Cloud Armor edge, CDN, another proxy), adjust the hop
  count, or the rate limiter and audit IPs become wrong or spoofable.
- **Per-IP rate limiter is per-Cloud-Run-instance.** Each replica counts
  independently. DDoS protection lives at the LB (Cloud Armor), not in the
  Go limiter.

## Request flow gotchas

- **Do not collapse peek and burn.** Opening `/d` only calls
  `POST /api/download/peek` (no state change); the burn happens on
  `POST /api/download/burn` after an explicit click. Link unfurlers and
  Safe-Links-style scanners load pages (some run JS) but don't click.
- **Signed URLs always force `response-content-disposition=attachment` and
  `application/octet-stream`.** Content is ciphertext, but keep the headers so
  the bucket never serves anything a browser would render.
- **Uploads never pass through the service.** The server opens a GCS resumable
  session with `X-Upload-Content-Length` (size pinned), `ifGenerationMatch=0`
  and an `Origin` header (CORS), and finalize re-checks the object size. Don't
  add a proxying upload endpoint — Cloud Run caps requests at 32 MiB.
- **Non-final upload chunks must be multiples of 256 KiB** (GCS rule).
  `UPLOAD_CHUNK` in `transfer.ts` is 8 MiB; keep it a multiple.

## Operational constraints

- **Nothing deployment-specific in git.** Project, bucket, domain and state
  bucket come from `terraform.tfvars` / `backend.hcl` (gitignored) locally,
  and from repo variables + the `TFVARS` environment secret in CI. Keep
  examples generic (`example.com`, `your-gcp-project`).
- **Firestore uses a named database** (`file-vault-<env>`), never
  `(default)`, which other apps in the project may own.
- **Ingress = `INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER`.** Only the external
  LB reaches the service; `TRUSTED_PROXY_HOPS` depends on that.
- **The bucket needs CORS for `PUBLIC_ORIGIN`** (GET for signed downloads;
  PUT for uploads). Terraform sets it from `var.domain`. The UI's CSP
  `connect-src` allows `https://storage.googleapis.com`.
- **`MAX_UPLOAD_MB` is bounded by browser memory**: downloads are decrypted
  into a Blob. Raising it far past 1–2 GiB needs a streaming save
  (File System Access API / service worker) first.

## Repo conventions

- **One binary.** `make build` copies `web/build` into
  `api/internal/webui/dist` (gitignored except `.gitkeep`) and embeds it. The
  root `Dockerfile` does the same. Without a UI build the binary serves 503.
- **`LOCAL_MODE=1`** swaps Firestore/GCS for `store.Memory` and `blob.Local`.
  `make dev` and `make e2e` use it. Never set it in Terraform.
- **Store changes go in both backends** and the shared suite in
  `store_test.go`. Run it against the Firestore emulator
  (`FIRESTORE_EMULATOR_HOST`) when touching `firestore.go`.
- **Kit 3 imports:** use `#lib/...` (Node subpath imports in `package.json`)
  with explicit `.ts` extensions. `$lib` no longer exists.
- **`web/package-lock.json` and `docs/package-lock.json` are committed.** CI uses `npm ci`.
- **Releases:** push a `vX.Y.Z` tag; `release.yaml` creates the GitHub release and dispatches the docs build, whose header badge shows the latest release (read at build time). Deploys don't depend on tags: every push to `main` deploys.
- **Docs live in `docs/` (Starlight)** and publish to GitHub Pages via `docs.yaml`. Keep them in sync with behaviour changes, and keep them generic: site URL and base path come from the Pages config, not from hardcoded values.
- **Don't search other customers' projects for older `populate_gdoc.py`,
  bootstrap scripts, or similar.** This project has its own canonical copies
  under `cicd/` and `terraform/`.

## What's intentional and not a bug

- Admin tokens have a TTL field on Firestore (`expires_at`) — same TTL as the
  share — so orphaned admin docs self-clean. Lookups also check expiry
  because Firestore TTL deletion can lag by a day.
- Peek/burn return 404 (not 410) on unknown/expired/revoked/exhausted —
  deliberate, to avoid enumeration signal.
- `RevokeOutstandingTokens` filters client-side. A server-side inequality
  filter would need a composite Firestore index we don't ship.
- Pending shares (upload never finalized) have no download token and expire
  with the share TTL; the bucket lifecycle rule removes any stray object.
- There is no sweeper job. Firestore TTL (shares, tokens, admin docs, audit)
  and the bucket lifecycle rule (age = `share_ttl_days`) do all expiry, and
  every lookup checks `expires_at` itself because TTL deletion lags.
