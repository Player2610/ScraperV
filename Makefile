-include .env
export

.PHONY: help dev dev-quick dev-reset build test test-integration lint migrate-up migrate-down scraper-dry-run generate

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## [a-z]' Makefile | sort | awk 'BEGIN {FS = ": "}; {printf "  \033[36m%-20s\033[0m %s\n", $$2, $$3}'

## dev: Start dev environment — syncs from prod, runs migrations, starts API
dev:
	docker compose up --build

## dev-quick: Restart API only — skips prod sync and migrations (DB must already be up)
dev-quick:
	docker compose up --no-deps api --build

## dev-reset: Wipe local DB volume and do a full restart (re-sync from prod)
dev-reset:
	docker compose down -v
	docker compose up --build

## build: Build all Go binaries (api, scraper, migrate)
build:
	go build -trimpath -ldflags="-s -w" -o bin/api     ./cmd/api
	go build -trimpath -ldflags="-s -w" -o bin/scraper ./cmd/scraper
	go build -trimpath -ldflags="-s -w" -o bin/migrate ./cmd/migrate

## test: Run all tests with race detector
test:
	go test -race -timeout 120s ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## migrate-up: Apply all pending database migrations
migrate-up:
	go run ./cmd/migrate

## migrate-down: Roll back the last database migration
migrate-down:
	@if [ -z "$$DATABASE_URL" ]; then \
		echo "DATABASE_URL is not set"; exit 1; \
	fi
	goose -dir db/migrations postgres "$$DATABASE_URL" down

## scraper-dry-run: Run the scraper in dry-run mode (no DB writes, prints to stdout)
scraper-dry-run:
	go run ./cmd/scraper --dry-run

## test-integration: Run integration tests (requires TEST_DATABASE_URL)
test-integration:
	go test -tags integration -v -timeout 120s ./internal/scraping/...

## generate: Run all code generators (sqlc, go generate)
generate:
	go generate ./...
