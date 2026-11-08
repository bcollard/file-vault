---
title: Local development
description: Run the whole app on your machine without GCP, and run the tests.
---

`LOCAL_MODE=1` swaps Firestore and Cloud Storage for an in-memory store and a local directory that speaks the same resumable-upload protocol. The full flow, encryption included, runs without any cloud account.

## Requirements

- Go 1.26 or later
- Node 24 or later

## Run it

```bash
make dev
```

This starts the Go server on `:8080` in local mode and Vite on `:5173` with hot reload. Open http://localhost:5173. Vite proxies `/api` and `/_local` to Go.

To run the production binary instead:

```bash
make build                       # → ./bin/file-vault, UI embedded
LOCAL_MODE=1 ./bin/file-vault    # → http://localhost:8080
```

## Tests

| Command | What it runs |
|---|---|
| `make api-test` | Go tests with the race detector: the store suite and the HTTP handlers |
| `make web-check` | `svelte-check` type checking |
| `make web-test` | Encryption tests: round trips at record boundaries, and rejection of truncation, reordering, tampering and wrong keys |
| `make test` | All three |
| `make e2e` | Builds the binary, starts it in local mode, and runs a full encrypt, upload, finalize, burn, decrypt round trip with the browser modules from Node |

The store suite also runs against Firestore when `FIRESTORE_EMULATOR_HOST` is set:

```bash
gcloud emulators firestore start --host-port=localhost:8081 &
FIRESTORE_EMULATOR_HOST=localhost:8081 make api-test
```

## Repository layout

```
api/                 Go service
  cmd/file-vault/      entrypoint
  internal/server/     handlers, client IP, security headers, rate limits
  internal/store/      Firestore and in-memory stores, shared test suite
  internal/blob/       GCS and local-disk blob backends
  internal/webui/      go:embed of the built UI
web/                 SvelteKit UI, built statically
  src/lib/crypto.ts    encryption format
  src/lib/transfer.ts  chunked upload, streaming download and decrypt
terraform/           modules and environments
docs/                this site (Starlight)
cicd/                Workload Identity bootstrap script
```

## Docs site

```bash
cd docs && npm ci && npm run dev    # → http://localhost:4321
```
