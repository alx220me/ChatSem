# Plan: Admin Service

**Branch:** `feature/admin-service`
**Created:** 2026-04-02
**Milestone:** Admin Service

## Settings
- **Testing:** yes — интеграционные тесты (TEST_DATABASE_URL + TEST_REDIS_URL)
- **Logging:** verbose — DEBUG на входе каждого метода, INFO на успехе
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Admin Service"
**Rationale:** Девятый milestone — management API, moderation и ban/unban нужны до Admin Panel (React SPA).

## Context: что уже запланировано

Части Admin Service покрыты предыдущими планами:
- **Task 18** (feature/shared-domain-repos): `admin/repository/postgres/event_repo.go`, `chat_repo.go`, `ban_repo.go`
- **Task 23** (feature/chat-hierarchy): `EventService` (CreateEvent, ListEvents), `EventHandler`, `middleware/auth.go`, `middleware/rbac.go`
- **Task 27** (feature/message-service): `BanService` (CreateBan, UnbanUser), `BanHandler`

Этот план добавляет **недостающие части**: мут-система, chat/user management endpoints.

## Tasks

### Phase 1: Mute domain + schema

**[x] Task 38 — Mute domain type + migration + repository interface**

`shared/domain/mute.go`:
```go
type Mute struct {
    ID        uuid.UUID
    ChatID    uuid.UUID
    UserID    uuid.UUID
    MutedBy   uuid.UUID
    Reason    string
    CreatedAt time.Time
    ExpiresAt *time.Time
}
```

Добавить `ErrUserMuted` в `shared/domain/errors.go` (если ещё нет).

`migrations/301_create_mutes.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS mutes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    muted_by   UUID NOT NULL REFERENCES users(id),
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ
);

-- Для mute-check: "замьючен ли пользователь X в чате Y?"
CREATE INDEX IF NOT EXISTS mutes_chat_user_idx
    ON mutes (chat_id, user_id)
    WHERE expires_at IS NULL OR expires_at > now();

-- +goose Down
DROP TABLE IF EXISTS mutes;
```

`MuteRepository` в `shared/domain/repository.go`:
```go
type MuteRepository interface {
    Create(ctx context.Context, mute *Mute) error
    GetActive(ctx context.Context, chatID, userID uuid.UUID) (*Mute, error)
    Expire(ctx context.Context, muteID uuid.UUID) error
    List(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*Mute, error)
}
```

Добавить в `UserRepository`: `ListByEvent(ctx, eventID uuid.UUID, limit, offset int) ([]*User, error)` и `UpdateRole(ctx, userID uuid.UUID, role string) error`

Files: `shared/domain/mute.go`, `shared/domain/errors.go`, `shared/domain/repository.go`, `migrations/301_create_mutes.sql`

### Phase 2: Repository implementations

**[x] Task 39 — MuteRepository pgx реализации**

`services/admin/internal/repository/postgres/mute_repo.go`:
- `Create`: INSERT, вернуть с заполненным ID + CreatedAt
- `GetActive`: `WHERE chat_id=$1 AND user_id=$2 AND (expires_at IS NULL OR expires_at > NOW()) LIMIT 1` → `domain.ErrNotFound` если нет
- `Expire`: `UPDATE SET expires_at=NOW() WHERE id=$1`
- `List`: активные муты для чата (expires_at IS NULL OR > NOW()) с LIMIT/OFFSET

`services/chat/internal/repository/postgres/mute_repo.go`:
- Только `GetActive` — chat service только проверяет при отправке сообщения

LOGGING:
- `slog.Debug("[MuteRepo.Create] creating mute", "chat_id", chatID, "user_id", userID, "expires_at", expiresAt)`
- `slog.Debug("[MuteRepo.GetActive] checking", "chat_id", chatID, "user_id", userID)`
- `slog.Debug("[MuteRepo.Expire] expiring", "mute_id", muteID)`

Files: `services/admin/internal/repository/postgres/mute_repo.go`, `services/chat/internal/repository/postgres/mute_repo.go`

<!-- Commit checkpoint: tasks 38-39 -->

### Phase 3: Сервисный слой + HTTP API

**[x] Task 40 — MuteService + MuteHandler**

`services/admin/internal/service/mute_service.go`:

`CreateMute(ctx, chatID, userID, mutedBy uuid.UUID, reason string, expiresAt *time.Time) (*domain.Mute, error)`:
1. `repo.GetActive(ctx, chatID, userID)` → если уже замьючен → вернуть idempotent
2. `repo.Create(ctx, &domain.Mute{ChatID, UserID, MutedBy, Reason, ExpiresAt})`

`UnmuteUser(ctx, muteID uuid.UUID) error`:
1. `repo.Expire(ctx, muteID)`

`services/admin/internal/handler/mute_handler.go`:

| Endpoint | Role | Описание |
|----------|------|----------|
| `POST /api/admin/mutes` | moderator/admin | Замьютить пользователя |
| `DELETE /api/admin/mutes/{muteID}` | moderator/admin | Снять мут |
| `GET /api/admin/chats/{chatID}/mutes` | moderator/admin | Список активных мутов |

`POST /api/admin/mutes` body: `{chat_id, user_id, reason?, expires_at?}`
Response: `201 {id, chat_id, user_id, reason, created_at, expires_at}`

LOGGING:
- `slog.Debug("[MuteService.CreateMute] muting", "chat_id", chatID, "user_id", userID)`
- `slog.Info("[MuteService.CreateMute] muted", "mute_id", mute.ID)`
- `slog.Info("[MuteService.UnmuteUser] unmuted", "mute_id", muteID)`
- `slog.Warn("[MuteService.CreateMute] already muted, returning existing", "mute_id", existing.ID)`
- `slog.Debug("[MuteHandler.Create] request", "chat_id", chatID, "user_id", userID, "by", claims.UserID)`

Files: `services/admin/internal/service/mute_service.go`, `services/admin/internal/handler/mute_handler.go`, обновить `router.go`

**[x] Task 41 — Chat и User management endpoints**

`services/admin/internal/handler/chat_handler.go`:

| Endpoint | Role | Описание |
|----------|------|----------|
| `GET /api/admin/events/{eventID}/chats` | moderator/admin | Список чатов (parent + children) |
| `PATCH /api/admin/chats/{chatID}/settings` | admin | Обновить настройки чата |

`PATCH` body: JSONB patch настроек; Response: `200 {id, settings}`

`services/admin/internal/handler/user_handler.go`:

| Endpoint | Role | Описание |
|----------|------|----------|
| `GET /api/admin/events/{eventID}/users` | moderator/admin | Список пользователей (`?limit=50&offset=0`) |
| `PATCH /api/admin/users/{userID}/role` | admin | Изменить роль (`{role: "moderator"}`) |

`PATCH /api/admin/users/{userID}/role`: допустимые значения `user`, `moderator`; `admin` — нельзя назначить через API (только вручную через БД).

LOGGING:
- `slog.Debug("[ChatHandler.ListChats] request", "event_id", eventID)`
- `slog.Debug("[ChatHandler.UpdateSettings] request", "chat_id", chatID)`
- `slog.Debug("[UserHandler.List] request", "event_id", eventID, "limit", limit, "offset", offset)`
- `slog.Debug("[UserHandler.UpdateRole] request", "user_id", userID, "new_role", role, "by", claims.UserID)`
- `slog.Info("[UserHandler.UpdateRole] role updated", "user_id", userID, "role", role)`

Files: `services/admin/internal/handler/chat_handler.go`, `user_handler.go`, обновить `router.go`

<!-- Commit checkpoint: tasks 40-41 -->

### Phase 4: Tests

**[x] Task 42 — Интеграционные тесты Admin Service**

Интеграционные (TEST_DATABASE_URL + TEST_REDIS_URL):

`services/admin/internal/service/ban_service_test.go`:
- `TestCreateBan_SetsRedisKey`: ban → `EXISTS ban:{eventID}:{userID}` == 1
- `TestUnbanUser_DeletesRedisKey`: unban → `EXISTS ban:{eventID}:{userID}` == 0
- `TestCreateBan_WithExpiry`: ban c expires_at → Redis TTL выставлен

`services/admin/internal/service/mute_service_test.go`:
- `TestCreateMute_GetActive`: мут создан → GetActive вернул его
- `TestCreateMute_Idempotent`: двойной вызов CreateMute → одна запись
- `TestUnmuteUser_GetActiveNotFound`: после Expire → GetActive возвращает ErrNotFound

`services/admin/internal/service/event_service_test.go`:
- `TestCreateEvent_CreatesParentChat`: CreateEvent → parent chat + chat_seq в БД

LOGGING: `t.Logf("[%s] setup: applying migrations", t.Name())`, `t.Logf("[%s] assert: %s", t.Name(), desc)`

Files: `services/admin/internal/service/ban_service_test.go`, `mute_service_test.go`, `event_service_test.go`

<!-- Commit checkpoint: task 42 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 38–39 | `feat: mute domain type, migration and pgx repository` |
| 2 | 40–41 | `feat: mute service, chat and user management admin endpoints` |
| 3 | 42 | `test: integration tests for admin service` |
