# Запуск для разработки

## Полный стек (Docker)

```bash
cd deploy
docker compose up -d
```

Поднимает всё: PostgreSQL, Redis, все три Go-сервиса, фронтенд и Nginx на порту 80.

---

## Только инфраструктура + сервисы на хосте

Удобно для разработки — сервисы запускаются напрямую через `go run`, горячая перезагрузка без пересборки образов.

### 1. Запустить инфраструктуру

```bash
cd deploy
docker compose up -d postgres redis
```

`docker-compose.override.yml` подхватывается автоматически и пробрасывает Redis на `localhost:6379`.

Порты на хосте:
- PostgreSQL → `localhost:5433`
- Redis → `localhost:6379`

### 2. Настроить окружение

Скопируй и отредактируй:
```bash
cp deploy/.env.example .env.local
```

Готовый `.env.local` для локального запуска:

```env
DATABASE_URL=postgres://chatsem:<password>@localhost:5433/chatsem
POSTGRES_DB=chatsem
POSTGRES_USER=chatsem
POSTGRES_PASSWORD=<password>

REDIS_ADDR=localhost:6379

JWT_SECRET=<secret>
JWT_MAX_TTL=5m

CHAT_ADDR=:8080
AUTH_ADDR=:8081
ADMIN_ADDR=:8082

ADMIN_USERNAME=admin
ADMIN_PASSWORD=<password>

LOG_LEVEL=debug
```

### 3. Запустить сервисы

Каждый сервис в отдельном терминале:

```bash
source .env.local

# Терминал 1
go run ./services/chat/cmd/

# Терминал 2
go run ./services/auth/cmd/

# Терминал 3
go run ./services/admin/cmd/
```

### 4. Тестовый виджет

```bash
go run ./deploy/test/server.go
# → http://localhost:8090
```

> **Важно:** в `deploy/test/server.go` замени константы `apiSecret` и `eventID` на актуальные значения из БД.

---

## Переменные окружения

| Переменная | Сервис | Дефолт |
|---|---|---|
| `DATABASE_URL` | chat, auth, admin | `postgres://chatsem:chatsem@localhost:5432/chatsem` |
| `REDIS_ADDR` | chat, auth | `localhost:6379` |
| `JWT_SECRET` | chat, auth, admin | `change-me-in-production` |
| `JWT_MAX_TTL` | auth | `4h` |
| `CHAT_ADDR` | chat | `:8080` |
| `AUTH_ADDR` | auth | `:8081` |
| `ADMIN_ADDR` | admin | `:8082` |
| `ADMIN_USERNAME` | admin | — |
| `ADMIN_PASSWORD` | admin | — |
| `LOG_LEVEL` | все | `info` (`debug` / `warn` / `error`) |

---

## Интеграционные тесты

```bash
TEST_DATABASE_URL="postgres://chatsem:<password>@localhost:5433/chatsem" \
  go test ./services/admin/internal/service/... -v
```
