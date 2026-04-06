# Plan: Shared Domain & Repositories

**Branch:** `feature/shared-domain-repos`
**Created:** 2026-04-02
**Milestone:** Shared Domain & Repositories

## Settings
- **Testing:** yes — интеграционные тесты с реальной PostgreSQL (TEST_DATABASE_URL)
- **Logging:** verbose — DEBUG-логи на входе каждого метода репозитория
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Shared Domain & Repositories"
**Rationale:** Третий milestone roadmap — реализация интерфейсов и pgx-репозиториев обязательна до Chat Hierarchy и Message Service.

## Tasks

### Phase 1: Интерфейсы

**Task 15 — Полные интерфейсы репозиториев в shared/domain**

Определить полные сигнатуры методов для всех репозиториев. Зависит от domain-типов из Project Bootstrap.

Интерфейсы в `shared/domain/repository.go`:
- `ChatRepository`: GetByID, GetParentByEventID, GetChildren, GetOrCreateChild, Create, UpdateSettings
- `MessageRepository`: Create (с seq), GetByChatAfterSeq, GetByChatRange, SoftDelete
- `UserRepository`: GetOrCreate (upsert), GetByID, GetByExternalID
- `EventRepository`: GetByID, GetAPISecret
- `BanRepository`: Create, GetActive, Expire, List

Добавить `shared/domain/ban.go`:
- `Ban{ID uuid.UUID, UserID, EventID, ChatID *uuid.UUID, BannedBy uuid.UUID, Reason string, CreatedAt time.Time, ExpiresAt *time.Time}`

Files: `shared/domain/repository.go`, `shared/domain/ban.go`

### Phase 2: Chat Service Repositories

**Task 16 — pgx-репозитории chat service**

`services/chat/internal/repository/postgres/`:

`chat_repo.go`:
- GetOrCreateChild: `INSERT ... ON CONFLICT (event_id, external_room_id) DO NOTHING` + SELECT
- GetChildren: JOIN parent для settings через SQL (child-настройки наследуются из parent через JOIN)
- UpdateSettings

`message_repo.go`:
- Create с CTE для атомарного seq:
  ```sql
  WITH next_seq AS (
      UPDATE chat_seqs SET last_seq=last_seq+1 WHERE chat_id=$1 RETURNING last_seq
  )
  INSERT INTO messages (id, chat_id, user_id, text, seq, created_at)
  SELECT gen_random_uuid(), $1, $2, $3, next_seq.last_seq, NOW()
  FROM next_seq RETURNING id, seq, created_at
  ```
- GetByChatAfterSeq: `WHERE chat_id=$1 AND seq>$2 AND deleted_at IS NULL ORDER BY seq ASC LIMIT $3`
- SoftDelete: `UPDATE SET deleted_at=NOW(), deleted_by=$2 WHERE id=$1`

`user_repo.go`:
- GetOrCreate: upsert `ON CONFLICT (external_id, event_id) DO UPDATE SET name=$3, role=$4`

LOGGING: `slog.Debug("[ChatRepo.GetOrCreateChild] creating child", "event_id", eventID, "room_id", roomID)` и т.д.

Files: `services/chat/internal/repository/postgres/chat_repo.go`, `message_repo.go`, `user_repo.go`

<!-- Commit checkpoint: tasks 15-16 -->

### Phase 3: Auth & Admin Service Repositories

**Task 17 — pgx-репозитории auth service**

`services/auth/internal/repository/postgres/`:

`user_repo.go`:
- UpsertUser: `INSERT ... ON CONFLICT DO UPDATE SET name, role RETURNING *`

`event_repo.go`:
- GetAPISecret: `SELECT api_secret WHERE id=$1` (возвращает bcrypt-хэш, сравнение на уровне сервиса)

LOGGING: `slog.Debug("[UserRepo.UpsertUser] upsert", "external_id", externalID, "event_id", eventID)`

Files: `services/auth/internal/repository/postgres/user_repo.go`, `event_repo.go`

**Task 18 — pgx-репозитории admin service**

`services/admin/internal/repository/postgres/`:

`event_repo.go`: Create, GetByID, List, Update

`chat_repo.go`: GetByID, List, GetChildren, UpdateSettings

`ban_repo.go`:
- Create: INSERT + gen_random_uuid()
- GetActive: `WHERE user_id=$1 AND event_id=$2 AND (expires_at IS NULL OR expires_at > NOW()) LIMIT 1`
- Expire: `UPDATE SET expires_at=NOW() WHERE id=$1`
- List с пагинацией

LOGGING: `slog.Debug("[BanRepo.Create] creating ban", "user_id", ban.UserID, "event_id", ban.EventID, "expires_at", ban.ExpiresAt)`

Files: `services/admin/internal/repository/postgres/event_repo.go`, `chat_repo.go`, `ban_repo.go`

<!-- Commit checkpoint: tasks 17-18 -->

### Phase 4: Tests

**Task 19 — Интеграционные тесты репозиториев**

Подход: `TEST_DATABASE_URL` env var; `t.Skip` если не задан.
Setup: применить миграции, перед каждым тестом `TRUNCATE ... CASCADE`.

Тесты chat service:
- `TestGetOrCreateChild_Idempotent`: дважды → одна запись
- `TestMessageCreate_SeqMonotonic`: 3 сообщения → seq 1,2,3
- `TestMessageCreate_SeqIsolatedByChat`: разные чаты — независимые счётчики
- `TestGetByChatAfterSeq`: фильтрация и порядок
- `TestSoftDelete_HidesMessage`: после SoftDelete не возвращается в GetByChatAfterSeq
- `TestUserGetOrCreate_Upsert`: второй вызов обновляет name

Тесты auth service:
- `TestUpsertUser_CreatesAndUpdates`
- `TestGetAPISecret_ReturnsHashed`

LOGGING: `t.Logf("[%s] setup: applying migrations", t.Name())`, `t.Logf("[%s] assert: %s", t.Name(), desc)`

Files: `services/chat/internal/repository/postgres/*_test.go`, `services/auth/internal/repository/postgres/*_test.go`

<!-- Commit checkpoint: task 19 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 15–16 | `feat: repository interfaces and chat service pgx repos` |
| 2 | 17–18 | `feat: auth and admin service pgx repositories` |
| 3 | 19 | `test: integration tests for repository layer` |
