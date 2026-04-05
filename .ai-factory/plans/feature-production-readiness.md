# Plan: Production Readiness

**Branch:** `feature/production-readiness`
**Created:** 2026-04-02
**Milestone:** Production Readiness

## Settings
- **Testing:** no — Docker/Compose/nginx конфигурации не покрываются unit-тестами
- **Logging:** verbose — build metadata (`version`, `built_at`) логируется при старте сервиса
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Production Readiness"
**Rationale:** Двенадцатый (последний) milestone — контейнеризация и инфраструктура нужны для деплоя всех сервисов вместе.

## Tasks

### Phase 1: Dockerfiles

**[x] Task 58 — Dockerfiles multi-stage (все сервисы)**

Go-сервисы (chat, auth, admin) — паттерн builder + distroless:
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY shared/ shared/
COPY services/<name>/ services/<name>/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /service ./services/<name>/cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /service /service
EXPOSE <port>
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD ["/service", "-health"] # или wget через отдельный busybox layer
ENTRYPOINT ["/service"]
```

Фронтенд — node builder + nginx:alpine:

`deploy/frontend/widget/Dockerfile`:
```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY frontend/widget/package*.json ./
RUN npm ci --prefer-offline
COPY frontend/widget/ .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist/ /usr/share/nginx/html/widget/
```

`deploy/frontend/admin/Dockerfile` — аналогично, dist → `/usr/share/nginx/html/admin/`

HEALTHCHECK для distroless — использовать отдельный `FROM busybox AS healthcheck` stage или `/bin/sh` через debug-вариант distroless.

Files: `deploy/services/chat/Dockerfile`, `deploy/services/auth/Dockerfile`, `deploy/services/admin/Dockerfile`, `deploy/frontend/widget/Dockerfile`, `deploy/frontend/admin/Dockerfile`

### Phase 2: Compose + конфигурация

**[x] Task 59 — docker-compose.yml + .env.example**

`deploy/docker-compose.yml`:
- `postgres:16-alpine` — volume `postgres_data`, healthcheck `pg_isready -U $$POSTGRES_USER`
- `redis:7-alpine` — healthcheck `redis-cli ping`
- `pgbouncer` (bitnami/pgbouncer:1) — depends_on postgres healthy, mount `pgbouncer.ini` + `userlist.txt`
- `chat` — build `./deploy/services/chat`, context `.`, env_file `.env`, depends_on pgbouncer + redis
- `auth` — аналогично, port 8081
- `admin` — аналогично, port 8082
- `widget` — build `./deploy/frontend/widget`, context `.`
- `admin-panel` — build `./deploy/frontend/admin`, context `.`
- `nginx` — build от nginx:alpine с COPY nginx.conf, ports `80:80`, depends_on все сервисы

Networks: `backend` (postgres, pgbouncer, redis, go-services), `frontend` (nginx + статика)

`deploy/.env.example`:
```env
DATABASE_URL=postgres://chatsem:password@pgbouncer:5432/chatsem
REDIS_ADDR=redis:6379
JWT_SECRET=change-me-in-production
JWT_MAX_TTL=4h
CHAT_ADDR=:8080
AUTH_ADDR=:8081
ADMIN_ADDR=:8082
POSTGRES_DB=chatsem
POSTGRES_USER=chatsem
POSTGRES_PASSWORD=change-me
```

Files: `deploy/docker-compose.yml`, `deploy/.env.example`

**[x] Task 60 — nginx.conf + pgbouncer.ini**

`deploy/nginx.conf`:
```nginx
upstream chat_backend {
    server chat:8080;
    keepalive 200;
}
upstream auth_backend  { server auth:8081; }
upstream admin_backend { server admin:8082; }

server {
    listen 80;

    location /api/chat/ {
        proxy_pass         http://chat_backend;
        proxy_http_version 1.1;
        proxy_set_header   Connection "";
        proxy_set_header   Host $host;
        proxy_read_timeout 35s;  # LongPollTimeout(25s) + буфер
    }
    location /api/auth/ {
        proxy_pass         http://auth_backend;
        proxy_http_version 1.1;
        proxy_read_timeout 10s;
    }
    location /api/admin/ {
        proxy_pass         http://admin_backend;
        proxy_http_version 1.1;
        proxy_read_timeout 60s;  # export может занимать время
    }
    location /widget/ {
        root /usr/share/nginx/html;
        try_files $uri $uri/ =404;
    }
    location / {
        root /usr/share/nginx/html/admin;
        try_files $uri /index.html;  # SPA fallback
    }
}
```

`deploy/pgbouncer/pgbouncer.ini`:
```ini
[databases]
chatsem = host=postgres port=5432 dbname=chatsem

[pgbouncer]
listen_addr = *
listen_port = 5432
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
pool_mode = transaction
max_client_conn = 200
default_pool_size = 50
server_round_robin = 1
log_connections = 0
log_disconnections = 0
```

`deploy/pgbouncer/userlist.txt` — placeholder: `"chatsem" "md5<hash>"` (заменить реальным хэшем).

Files: `deploy/nginx.conf`, `deploy/pgbouncer/pgbouncer.ini`, `deploy/pgbouncer/userlist.txt`

### Phase 3: Makefile + build metadata

**[x] Task 61 — Makefile targets + build metadata в сервисах**

Добавить в корневой `Makefile`:
```makefile
docker-build:
	docker compose -f deploy/docker-compose.yml build

docker-up:
	cp -n deploy/.env.example deploy/.env || true
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

docker-logs:
	docker compose -f deploy/docker-compose.yml logs -f

docker-clean:
	docker compose -f deploy/docker-compose.yml down -v --remove-orphans

docker-scale-chat:
	docker compose -f deploy/docker-compose.yml up -d --scale chat=2
```

В Makefile target `build` для Go-сервисов — embed version + time через ldflags:
```makefile
BUILD_VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME    := $(shell date -u +%FT%TZ)

build-chat:
	go build -ldflags="-s -w -X main.buildVersion=$(BUILD_VERSION) -X main.buildTime=$(BUILD_TIME)" \
	  -o bin/chat ./services/chat/cmd/main.go
```

Обновить `services/*/cmd/main.go` — добавить переменные и вывод при старте:
```go
var (
    buildVersion = "dev"
    buildTime    = "unknown"
)

func main() {
    slog.Info("service starting", "addr", cfg.Addr, "version", buildVersion, "built_at", buildTime)
    ...
}
```

Files: `Makefile` (обновить), `services/chat/cmd/main.go`, `services/auth/cmd/main.go`, `services/admin/cmd/main.go`
