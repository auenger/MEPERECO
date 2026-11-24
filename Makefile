.PHONY: server daemon build sqlc dev web desktop
server:
	cd core && go run ./cmd/server
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
