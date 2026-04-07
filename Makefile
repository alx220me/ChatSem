## ── Database migrations (goose) ────────────────────────────────────────────────
## DATABASE_URL must be set, e.g.:
##   export DATABASE_URL=postgres://chatsem:password@localhost:5432/chatsem?sslmode=disable

GOOSE      := goose
GOOSE_DIR  := ./migrations
GOOSE_DB   := postgres

.PHONY: migrate-up migrate-down migrate-status migrate-reset
export DATABASE_URL=postgres://chatsem:q1w2e3r4@localhost:5433/chatsem?sslmode=disable
migrate-up:
	@echo "[migrate] applying all pending migrations..."
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DB) "$(DATABASE_URL)" up

migrate-down:
	@echo "[migrate] rolling back last migration..."
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DB) "$(DATABASE_URL)" down

migrate-status:
	@echo "[migrate] current migration status:"
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DB) "$(DATABASE_URL)" status

migrate-reset:
	@echo "[migrate] resetting all migrations (down-to 0)..."
	$(GOOSE) -dir $(GOOSE_DIR) $(GOOSE_DB) "$(DATABASE_URL)" reset

## ── Docker helpers ─────────────────────────────────────────────────────────────

.PHONY: docker-build docker-up docker-down docker-logs docker-clean docker-scale-chat

docker-build:
	docker compose -f deploy/docker-compose.yml build

docker-up:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d --force-recreate

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-logs:
	docker compose -f deploy/docker-compose.yml logs -f

docker-clean:
	docker compose -f deploy/docker-compose.yml down -v --remove-orphans

docker-scale-chat:
	docker compose -f deploy/docker-compose.yml up -d --scale chat=2

## ── Build (Go services with version metadata) ──────────────────────────────────

BUILD_VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME    := $(shell date -u +%FT%TZ)

.PHONY: build-chat build-auth build-admin

build-chat:
	@echo "[build] chat service $(BUILD_VERSION) @ $(BUILD_TIME)"
	go build -ldflags="-s -w -X main.buildVersion=$(BUILD_VERSION) -X main.buildTime=$(BUILD_TIME)" \
	  -o bin/chat ./services/chat/cmd/main.go

build-auth:
	@echo "[build] auth service $(BUILD_VERSION) @ $(BUILD_TIME)"
	go build -ldflags="-s -w -X main.buildVersion=$(BUILD_VERSION) -X main.buildTime=$(BUILD_TIME)" \
	  -o bin/auth ./services/auth/cmd/main.go

build-admin:
	@echo "[build] admin service $(BUILD_VERSION) @ $(BUILD_TIME)"
	go build -ldflags="-s -w -X main.buildVersion=$(BUILD_VERSION) -X main.buildTime=$(BUILD_TIME)" \
	  -o bin/admin ./services/admin/cmd/main.go
