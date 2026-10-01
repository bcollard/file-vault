.PHONY: help api-build api-test web-dev web-build web-check fmt clean

help:
	@echo "Targets:"
	@echo "  web-dev      run the SvelteKit UI locally in preview mode (no GCP needed)"
	@echo "  web-build    install deps and produce a production build of the UI"
	@echo "  web-check    svelte-check the UI (type errors)"
	@echo "  api-build    go build the API service"
	@echo "  api-test     go test the API service"
	@echo "  fmt          format Go + Terraform"

# Launch the SvelteKit UI locally with stubbed API responses. No credentials needed.
# Visit http://localhost:5173 — every screen is browsable.
web-dev:
	cd web && \
		PREVIEW_MODE=1 \
		PUBLIC_DOMAIN=localhost:5173 \
		PORT=5173 \
		npm run dev

web-build:
	cd web && npm install --no-audit --no-fund && npm run build

web-check:
	cd web && npx svelte-kit sync && npx svelte-check --tsconfig ./tsconfig.json

api-build:
	cd api && go build ./...

api-test:
	cd api && go test ./...

fmt:
	cd api && go fmt ./...
	terraform -chdir=terraform fmt -recursive

clean:
	rm -rf web/.svelte-kit web/build web/node_modules
