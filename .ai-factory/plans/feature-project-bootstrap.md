# Plan: Project Bootstrap

**Branch:** `feature/project-bootstrap`
**Created:** 2026-03-30
**Updated:** 2026-04-01 (UUID ID Strategy: int64 → uuid.UUID во всех доменных типах)
**Milestone:** Project Bootstrap

## Settings

- **Testing:** yes — unit tests для shared/domain, shared/pkg/jwt, config, InMemoryBroker
- **Logging:** verbose — DEBUG-логи на старте каждого компонента
- **Docs:** no — WARN [docs], без обязательного checkpoint

## Roadmap Linkage

**Milestone:** "Project Bootstrap"
**Rationale:** Первый milestone roadmap — настройка go.work, модулей, DI, роутера, логирования.

## Overview

Настройка skeleton multi-service monorepo:
- `go.work` + отдельный `go.mod` на каждый из 4 модулей (chat, auth, admin, shared)
- `shared/` — pure Go: только domain-типы и утилиты без Redis/pgx/chi
- Вспомогательные пакеты: `shared/pkg/jwt`, `shared/pkg/response`, `shared/pkg/longpoll` (интерфейс + InMemoryBroker)
- Redis-backed `RedisBroker` в `services/chat/internal/broker/` (не в shared/)
- Config loading из env per service
- chi router + slog middleware per service
- DI wiring в `cmd/main.go` per service — chat импортирует `internal/broker`, не `shared/pkg/longpoll`
- Health check `GET /health` per service
- Unit тесты для shared пакетов, InMemoryBroker и config

## Tasks

### Phase 1: Module Structure

**Task 1 — go.work + go.mod для всех модулей**
- Создать `go.work` с `use` для chat, auth, admin, shared
- Создать `shared/go.mod` (module `chatsem/shared`) — зависимости: `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`; **без** redis/pgx/chi (pure Go правило)
- Создать `services/chat/go.mod` (module `chatsem/services/chat`) — зависимости: `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`
- Создать `services/auth/go.mod` (module `chatsem/services/auth`) — зависимости: `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/redis/go-redis/v9`, `github.com/golang-jwt/jwt/v5`
- Создать `services/admin/go.mod` (module `chatsem/services/admin`) — зависимости: `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`
- Файлы: `go.work`, `services/chat/go.mod`, `services/auth/go.mod`, `services/admin/go.mod`, `shared/go.mod`
- Logging: `slog.Debug("module initialized", "module", "<name>")`

**Task 2 — shared/domain: базовые типы**
- Создать `shared/domain/event.go` — `Event{ID uuid.UUID, Name, Settings, AllowedOrigin, CreatedAt}` (AllowedOrigin — для CORS per-event)
- Создать `shared/domain/chat.go` — `Chat{ID uuid.UUID, EventID uuid.UUID, ParentID *uuid.UUID, ExternalRoomID string, Type ChatType, CreatedAt time.Time}`
  - **Без поля Settings** у child-чатов и **без метода EffectiveSettings** — настройки всегда читаются через SQL JOIN с parent (динамическое наследование)
  - `ChatSettings` — отдельный тип, хранится только у parent-чата
  - `ChatType` — `const TypeParent, TypeChild`
- Создать `shared/domain/message.go` — `Message{ID uuid.UUID, ChatID uuid.UUID, UserID uuid.UUID, Text string, Seq int64, CreatedAt time.Time, DeletedAt *time.Time}` (Seq — счётчик, не ID — остаётся int64)
- Создать `shared/domain/user.go` — `User{ID uuid.UUID, ExternalID string, EventID uuid.UUID, Name string, Role string}`
- Создать `shared/domain/errors.go` — sentinel errors: `ErrNotFound`, `ErrForbidden`, `ErrChatNotFound`, `ErrUserBanned`
- Создать `shared/domain/repository.go` — интерфейсы: `ChatRepository`, `MessageRepository`, `UserRepository`, `EventRepository`
- Файлы: `shared/domain/*.go`

**Task 3 — shared/pkg + RedisBroker**
- `shared/pkg/jwt/jwt.go` — `ValidateToken(tokenStr, secret string) (*Claims, error)`, `Claims{UserID uuid.UUID, ExternalID string, EventID uuid.UUID, Role string, ExpiresAt time.Time}`; `slog.Debug("token validated", "user_id", claims.UserID)`
- `shared/pkg/response/response.go` — `JSON(w, status, v)`, `Error(w, status, code, message string)`; формат `{"error":"...","code":"..."}`
- `shared/pkg/longpoll/broker.go` — **только pure Go**:
  - `type Message struct{ChatID uuid.UUID; Data []byte}` — ID Strategy: uuid.UUID (github.com/google/uuid)
  - `type Broker interface{Subscribe(chatID uuid.UUID) <-chan Message; Unsubscribe(chatID uuid.UUID, ch <-chan Message); Publish(ctx context.Context, chatID uuid.UUID, data []byte) error}`
  - `const LongPollTimeout = 25*time.Second`, `LongPollSettleDelay = 50*time.Millisecond`
  - `type InMemoryBroker struct` — in-process реализация для тестов (без Redis)
  - **Без импорта `go-redis`** — shared/ pure Go
  - Зависимость `shared/go.mod`: добавить `github.com/google/uuid`
- `services/chat/internal/broker/redis_broker.go` — `RedisBroker implements longpoll.Broker`:
  - fan-out: одна Redis-подписка на chatID, горутина-читатель, `map[uuid.UUID]*fanout`
  - Redis-ключ: `fmt.Sprintf("chat:%s", chatID)` — строковый формат UUID
  - `Subscribe` — при первом клиенте запускает горутину `startReader`
  - `Unsubscribe` — при последнем клиенте `cancel()` горутины → `sub.Close()`
  - Клиентский канал буферизован (buffer=1): slow client пропускает сообщение, не блокирует fan-out
  - `func NewRedisBroker(rdb *redis.Client) *RedisBroker`
  - `slog.Debug("broker: new subscriber", "chat_id", chatID)`, `slog.Debug("broker: last client left, closing redis sub", "chat_id", chatID)`
- Файлы: `shared/pkg/jwt/jwt.go`, `shared/pkg/response/response.go`, `shared/pkg/longpoll/broker.go`, `services/chat/internal/broker/redis_broker.go`

### Phase 2: Per-Service Bootstrap

**Task 4 — Config loading (все 3 сервиса)**

Создать `internal/config/config.go` в каждом сервисе:
- `services/chat/internal/config/config.go`: `Config{Addr, DatabaseURL, RedisAddr, JWTSecret}` — читать из env с дефолтами; `Load() *Config`
- `services/auth/internal/config/config.go`: `Config{Addr, DatabaseURL, RedisAddr, JWTSecret}`
- `services/admin/internal/config/config.go`: `Config{Addr, DatabaseURL, JWTSecret}`
- Logging: `slog.Info("config loaded", "addr", cfg.Addr)`
- Файлы: `services/*/internal/config/config.go`

**Task 5 — chi router + middleware + health (все 3 сервиса)**

Создать `internal/handler/router.go` в каждом сервисе:
- chi router с middleware: `middleware.Logger` (slog), `middleware.Recoverer`, `middleware.RealIP`
- `GET /health` → `{"status":"ok","service":"<name>"}`
- Logging на каждый запрос: `slog.Debug("request", "method", r.Method, "path", r.URL.Path)`
- Файлы: `services/*/internal/handler/router.go`, `services/*/internal/middleware/logger.go`

**Task 6 — DI wiring в cmd/main.go (все 3 сервиса)**

- `services/chat/cmd/main.go`:
  - Инициализировать slog (JSON handler), config
  - Подключить `postgres.Connect(cfg.DatabaseURL)`, `redis.NewClient(cfg.RedisAddr)`
  - Создать `b := broker.NewRedisBroker(rdb)` — импорт `chatsem/services/chat/internal/broker` (**не** `shared/pkg/longpoll`)
  - Запустить `http.Server` с graceful shutdown: `os.Signal → context.WithCancel → srv.Shutdown(ctx)`
- `services/auth/cmd/main.go`: инициализировать slog, config, запустить `http.Server` с graceful shutdown
- `services/admin/cmd/main.go`: то же
- Logging: `slog.Info("service starting", "addr", cfg.Addr)`, `slog.Info("service stopped")`
- Файлы: `services/*/cmd/main.go`

### Phase 3: Tests

**Task 7 — Unit тесты**

- `shared/pkg/jwt/jwt_test.go` — тест валидации токена: корректный, истёкший, неверная подпись
- `shared/pkg/response/response_test.go` — тест формата JSON-ответа и error-ответа
- `shared/pkg/longpoll/broker_test.go` — тест `InMemoryBroker`: Subscribe/Publish/Unsubscribe, fan-out нескольким подписчикам, корректная отписка последнего клиента
- `services/chat/internal/config/config_test.go` — тест дефолтов и чтения из env
- Файлы: `shared/**/*_test.go`, `services/chat/internal/config/config_test.go`

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 1–2 | `feat: go.work multi-module setup and shared domain types` |
| 2 | 3 | `feat: shared/pkg jwt, response, longpoll interface and RedisBroker` |
| 3 | 4–5 | `feat: per-service config loading and chi router with health endpoint` |
| 4 | 6–7 | `feat: DI wiring in cmd/main.go and unit tests` |
