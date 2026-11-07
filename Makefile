.PHONY: help dev dev-api dev-web build embed web-build web-check web-test api-build api-test test e2e fmt clean

help:
	@echo "Targets:"
	@echo "  dev          run the Go server in LOCAL_MODE (:8080) + Vite with hot reload (:5173)"
	@echo "  build        build the UI, embed it, and compile ./bin/file-vault"
	@echo "  test         Go tests (race) + svelte-check + crypto tests"
	@echo "  e2e          full upload/download round trip against a LOCAL_MODE server"
	@echo "  fmt          format Go + Terraform"

# Everything runs locally: in-memory store, blobs in $TMPDIR. No GCP needed.
# Open http://localhost:5173 — Vite proxies /api and /_local to Go.
dev:
	@trap 'kill 0' INT TERM EXIT; \
		$(MAKE) --no-print-directory dev-api & \
		$(MAKE) --no-print-directory dev-web & \
		wait

dev-api:
	cd api && LOCAL_MODE=1 PUBLIC_ORIGIN=http://localhost:5173 go run ./cmd/file-vault

dev-web:
	cd web && npm run dev

web-build:
	cd web && npm ci --no-audit --no-fund && npm run build

web-check:
	cd web && npm run check

web-test:
	cd web && npm test

# Replaces the Go embed directory's contents (except .gitkeep) with the UI build.
embed:
	find api/internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R web/build/. api/internal/webui/dist/

build: web-build embed
	cd api && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../bin/file-vault ./cmd/file-vault

api-build:
	cd api && go build ./...

api-test:
	cd api && go test -race ./...

test: api-test web-check web-test

# Starts the built binary in LOCAL_MODE and drives the real browser code
# (crypto, chunked upload, finalize, burn, decrypt) against it from Node.
e2e: build
	@dir=$$(mktemp -d); \
		LOCAL_MODE=1 PORT=18080 LOCAL_DIR=$$dir ./bin/file-vault > /dev/null 2>&1 & pid=$$!; \
		trap "kill $$pid" EXIT; \
		for i in $$(seq 1 50); do curl -sf http://localhost:18080/healthz >/dev/null && break; sleep 0.1; done; \
		cd web && E2E_BASE_URL=http://localhost:18080 node --test test/e2e.test.ts

fmt:
	cd api && go fmt ./...
	terraform -chdir=terraform fmt -recursive

clean:
	rm -rf web/.svelte-kit web/build web/node_modules bin
	find api/internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
