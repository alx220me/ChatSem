# Plan: Chat Hierarchy

**Branch:** `feature/chat-hierarchy`
**Created:** 2026-04-02
**Milestone:** Chat Hierarchy

## Settings
- **Testing:** yes — unit тесты (mock repos) + интеграционный race-condition тест
- **Logging:** verbose — DEBUG на входе каждого сервисного метода и handler'а
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Chat Hierarchy"
**Rationale:** Четвёртый milestone — сервисный слой и HTTP API для иерархии чатов нужны до Message Service и Long Polling.

## Tasks

### Phase 1: Сервисный слой

**Task 20 — ChatService: бизнес-логика иерархии**

`services/chat/internal/service/chat_service.go`:

`GetOrCreateChildChat(ctx, eventID, roomID)`:
1. Fast path: `repo.GetByRoom` → вернуть если найден
2. Slow path: GetParent → Create child + InitChatSeq (транзакция) → при ON CONFLICT → GetByRoom (winner)

`GetParentChat(ctx, eventID)` — parent с settings

`GetChildren(ctx, eventID)` — children с settings из parent (JOIN в репо)

Добавить `InitChatSeq(ctx, chatID uuid.UUID) error` в `ChatRepository`:
- `INSERT INTO chat_seqs (chat_id, last_seq) VALUES ($1, 0)`
- Вызвать в Create транзакции

LOGGING:
- `slog.Debug("[ChatService.GetOrCreateChildChat] fast path hit", "event_id", eventID, "room_id", roomID)`
- `slog.Debug("[ChatService.GetOrCreateChildChat] slow path: creating", "event_id", eventID)`
- `slog.Debug("[ChatService.GetOrCreateChildChat] conflict race, fetching winner")`
- `slog.Info("[ChatService.GetOrCreateChildChat] child created", "chat_id", child.ID)`

Files: `services/chat/internal/service/chat_service.go`, обновить `shared/domain/repository.go`

### Phase 2: Middleware

**Task 21 — JWT и CORS middleware (chat service)**

`services/chat/internal/middleware/auth.go`:
- `Auth(jwtSecret string)` middleware: extract Bearer → `shared/pkg/jwt.ValidateToken` → Claims в context
- `ClaimsFromCtx(ctx) (*Claims, bool)` — helper
- 401 при ошибке: `response.Error(w, 401, "unauthorized", msg)`

`services/chat/internal/middleware/cors.go`:
- `CORS(eventRepo domain.EventRepository)` middleware
- Per-event allowed origin из `events.allowed_origin`
- Wildcard `*` **запрещён** — несовместим с `credentials=true`
- Preflight OPTIONS → 204

LOGGING:
- `slog.Debug("[AuthMiddleware] validated", "user_id", claims.UserID, "event_id", claims.EventID)`
- `slog.Warn("[AuthMiddleware] invalid token", "err", err)`
- `slog.Debug("[CORSMiddleware] origin allowed", "origin", origin)`
- `slog.Warn("[CORSMiddleware] origin rejected", "origin", origin, "allowed", allowed)`

Files: `services/chat/internal/middleware/auth.go`, `services/chat/internal/middleware/cors.go`

<!-- Commit checkpoint: tasks 20-21 -->

### Phase 3: HTTP API

**Task 22 — HTTP handlers: Chat Hierarchy**

`services/chat/internal/handler/chat_handler.go`:

| Endpoint | Auth | Описание |
|----------|------|----------|
| `GET /api/chat/events/{eventID}/chats` | нет (публичный) | parent + все children с settings |
| `POST /api/chat/join` | JWT | GetOrCreate child, тело `{event_id, room_id}` |
| `GET /api/chat/chats/{chatID}` | JWT | Один чат с settings |

Роутинг:
```
r.Get("/api/chat/events/{eventID}/chats", h.ListChats)
r.Group(func(r chi.Router) {
    r.Use(middleware.Auth(cfg.JWTSecret))
    r.Use(middleware.CORS(eventRepo))
    r.Post("/api/chat/join", h.JoinRoom)
    r.Get("/api/chat/chats/{chatID}", h.GetChat)
})
```

Ответ `POST /api/chat/join`: HTTP 201 если создан, 200 если существовал (определять по флагу из сервиса)

LOGGING:
- `slog.Debug("[ChatHandler.JoinRoom] request", "event_id", eventID, "room_id", roomID, "user_id", claims.UserID)`
- `slog.Info("[ChatHandler.JoinRoom] joined", "chat_id", chat.ID, "new", isNew)`

Files: `services/chat/internal/handler/chat_handler.go`, обновить `services/chat/internal/handler/router.go`

**Task 23 — Admin service: Events + parent chat management**

`services/admin/internal/service/event_service.go`:
- `CreateEvent(ctx, name, allowedOrigin, apiSecret)` → bcrypt apiSecret → INSERT → создать parent chat + chat_seqs (транзакция)
- `ListEvents(ctx, limit, offset)` → SELECT с пагинацией

`services/admin/internal/handler/event_handler.go`:
- `POST /api/admin/events` — role=admin
- `GET /api/admin/events` — role=admin или moderator
- `POST /api/admin/events/{eventID}/chat` — создать parent chat

`services/admin/internal/middleware/auth.go` + `rbac.go`:
- JWT validation аналогично chat service
- `RequireRole(roles ...string)` — 403 если роль не подходит

LOGGING:
- `slog.Info("[EventService.CreateEvent] created", "event_id", event.ID)`
- `slog.Info("[EventService.CreateParentChat] parent chat created", "event_id", eventID, "chat_id", chat.ID)`

Files: `services/admin/internal/service/event_service.go`, `handler/event_handler.go`, `middleware/auth.go`, `middleware/rbac.go`

<!-- Commit checkpoint: tasks 22-23 -->

### Phase 4: Tests

**Task 24 — Тесты Chat Hierarchy**

Unit тесты `services/chat/internal/service/chat_service_test.go`:
- `TestGetOrCreateChildChat_FastPath` — GetByRoom hit, нет обращения к GetParent
- `TestGetOrCreateChildChat_SlowPath` — полный путь создания
- `TestGetOrCreateChildChat_ConflictFallback` — ON CONFLICT → GetByRoom winner
- `TestGetParentChat_NotFound` — ErrChatNotFound propagation

Интеграционный тест `services/chat/internal/service/chat_service_integration_test.go`:
- `TestGetOrCreateChildChat_RaceCondition`: 10 goroutines → одинаковый chat.ID, одна запись в БД

Middleware unit тест `services/chat/internal/middleware/auth_test.go`:
- `TestAuth_ValidToken` → Claims в context
- `TestAuth_MissingToken` → 401
- `TestAuth_ExpiredToken` → 401

LOGGING: `t.Logf("[%s] goroutine %d got chat_id=%s", t.Name(), i, chatID)`

Files: `services/chat/internal/service/*_test.go`, `services/chat/internal/middleware/auth_test.go`

<!-- Commit checkpoint: task 24 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 20–21 | `feat: ChatService hierarchy logic and JWT/CORS middleware` |
| 2 | 22–23 | `feat: chat HTTP handlers and admin event management API` |
| 3 | 24 | `test: unit and integration tests for chat hierarchy` |
