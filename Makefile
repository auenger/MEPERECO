.PHONY: server daemon build sqlc dev web desktop cli-release
# Load the local server environment before changing into core. Use an alternate
# trusted shell-compatible file with make server SERVER_ENV_FILE=/path/server.env.
SERVER_ENV_FILE ?= .env
server:
	@set -a; if [ -f "$(SERVER_ENV_FILE)" ]; then . "$(abspath $(SERVER_ENV_FILE))" || exit 1; fi; set +a; cd core && exec go run ./cmd/server
daemon:
	cd core && go run ./cmd/silieco daemon start --foreground
build:
	cd core && go build ./...
	pnpm build
sqlc:
	cd core && sqlc generate
dev: web
web:
	pnpm dev:web
desktop:
	pnpm dev:desktop

# Publishable CLI archives with matching checksums (all six OS/arch combinations).
VERSION ?= 1.0.0
cli-release:
	python3 scripts/build-cli-release.py --version "$(VERSION)"
