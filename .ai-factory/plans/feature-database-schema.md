# Plan: Database Schema

**Branch:** `feature/database-schema`
**Created:** 2026-04-01
**Milestone:** Database Schema

## Settings

- **Testing:** yes — интеграционные тесты валидации схемы (up/down, constraints)
- **Logging:** verbose — goose выводит каждый применённый файл; в Makefile — явный вывод шагов; в тестах t.Logf для каждого этапа
- **Docs:** no — WARN [docs]

## Roadmap Linkage

**Milestone:** "Database Schema"
**Rationale:** Второй milestone roadmap — SQL-схема нужна до реализации репозиториев в любом сервисе.

## Overview

Создать SQL-миграции в `migrations/` с помощью goose:
- Структура файлов по доменным диапазонам: 001–099 shared, 100–199 messages, 300–399 bans
- Все таблицы с индексами в одном файле миграции каждая
- Makefile с goose-командами для удобного запуска
- Expand/Contract-совместимые миграции (IF NOT EXISTS, CREATE INDEX CONCURRENTLY)

**Таблицы:**
- `events` — мероприятия организатора
- `chats` — parent и child чаты, иерархия, unique constraint для lazy creation
- `chat_seqs` — атомарные счётчики seq (одна строка на чат)
- `users` — пользователи с внешними ID организатора (upsert при токен-обмене)
- `messages` — сообщения с seq, soft delete
- `bans` — баны пользователей (per-event или per-chat), поддержка Redis TTL через expires_at

## Tasks

### Phase 1: Структура + shared tables

**Task 1 — migrations/ + Makefile**
- Создать директорию `migrations/`
- Создать `Makefile` в корне с targets:
  ```
  migrate-up:    goose -dir ./migrations postgres "$(DATABASE_URL)" up
  migrate-down:  goose -dir ./migrations postgres "$(DATABASE_URL)" down
  migrate-status: goose -dir ./migrations postgres "$(DATABASE_URL)" status
  migrate-reset: goose -dir ./migrations postgres "$(DATABASE_URL)" reset
  ```
- Создать `migrations/README.md`: нумерация диапазонов, формат файлов, правила безопасных миграций (IF NOT EXISTS, CONCURRENTLY)
- Файлы: `Makefile`, `migrations/README.md`

**Task 2 — 001–003: events, chats, chat_seqs**

`migrations/001_create_events.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    settings       JSONB NOT NULL DEFAULT '{}',
    allowed_origin TEXT NOT NULL,          -- CORS: разрешённый домен хост-сайта
    api_secret     TEXT NOT NULL,          -- pre-shared secret для token exchange (хранить bcrypt-хэш)
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS events;
```

`migrations/002_create_chats.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS chats (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id         UUID NOT NULL REFERENCES events(id),
    parent_id        UUID REFERENCES chats(id),           -- NULL для parent-чата
    external_room_id TEXT,                                 -- ID зала из системы организатора (NULL для parent)
    type             TEXT NOT NULL CHECK (type IN ('parent', 'child')),
    settings         JSONB NOT NULL DEFAULT '{}',          -- только для parent; child читает через JOIN
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chats_type_parent_check CHECK (
        (type = 'parent' AND parent_id IS NULL AND external_room_id IS NULL) OR
        (type = 'child'  AND parent_id IS NOT NULL)
    )
);

-- Уникальный индекс для lazy child-chat creation (ON CONFLICT DO NOTHING)
CREATE UNIQUE INDEX IF NOT EXISTS chats_event_room_unique
    ON chats (event_id, external_room_id)
    WHERE type = 'child';

-- Для поиска parent-чата по event_id
CREATE INDEX IF NOT EXISTS chats_event_type_idx ON chats (event_id, type);

-- +goose Down
DROP TABLE IF EXISTS chats;
```

`migrations/003_create_chat_seqs.sql`:
```sql
-- +goose Up
-- Счётчики seq: одна строка на чат, создаётся вместе с чатом
CREATE TABLE IF NOT EXISTS chat_seqs (
    chat_id  UUID PRIMARY KEY REFERENCES chats(id) ON DELETE CASCADE,
    last_seq BIGINT NOT NULL DEFAULT 0  -- счётчик, не ID — остаётся BIGINT
);

-- +goose Down
DROP TABLE IF EXISTS chat_seqs;
```

Файлы: `migrations/001_create_events.sql`, `migrations/002_create_chats.sql`, `migrations/003_create_chat_seqs.sql`

### Phase 2: Users + Messages

**Task 3 — 004: users**

`migrations/004_create_users.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id TEXT NOT NULL,                               -- ID пользователя в системе организатора
    event_id    UUID NOT NULL REFERENCES events(id),
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user'
                    CHECK (role IN ('user', 'moderator', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Уникальность: один пользователь = одна запись на event
CREATE UNIQUE INDEX IF NOT EXISTS users_external_event_unique
    ON users (external_id, event_id);

-- +goose Down
DROP TABLE IF EXISTS users;
```

Файлы: `migrations/004_create_users.sql`

**Task 4 — 100: messages + performance indexes**

`migrations/100_create_messages.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id    UUID NOT NULL REFERENCES chats(id),
    user_id    UUID NOT NULL REFERENCES users(id),
    text       TEXT NOT NULL,
    seq        BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,                                  -- soft delete
    UNIQUE (chat_id, seq)                                    -- seq уникален в рамках чата
);

-- Основной паттерн long-poll: WHERE chat_id=$1 AND seq > $2 ORDER BY seq ASC
CREATE INDEX IF NOT EXISTS messages_chat_seq_idx
    ON messages (chat_id, seq ASC);

-- Паттерн экспорта истории: WHERE chat_id=$1 AND created_at BETWEEN $2 AND $3
CREATE INDEX IF NOT EXISTS messages_chat_created_at_idx
    ON messages (chat_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS messages;
```

Файлы: `migrations/100_create_messages.sql`

### Phase 3: Bans

**Task 5 — 300: bans**


`migrations/300_create_bans.sql`:
```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS bans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    event_id   UUID NOT NULL REFERENCES events(id),
    chat_id    UUID REFERENCES chats(id),                    -- NULL = бан на весь event
    banned_by  UUID NOT NULL REFERENCES users(id),           -- модератор/admin
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ                                    -- NULL = перманентный бан
);

-- Для ban-check запросов: "забанен ли пользователь X в event Y?"
CREATE INDEX IF NOT EXISTS bans_user_event_idx
    ON bans (user_id, event_id)
    WHERE expires_at IS NULL OR expires_at > now();
    -- Примечание: partial index с now() не обновляется автоматически;
    -- в production рассмотреть btree на (user_id, event_id, expires_at) и фильтровать в SQL

-- +goose Down
DROP TABLE IF EXISTS bans;
```

Файлы: `migrations/300_create_bans.sql`

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 1–2 | `feat: migrations setup and core schema (events, chats, chat_seqs)` |
| 2 | 3–4 | `feat: users and messages tables with performance indexes` |
| 3 | 5 | `feat: bans table with moderation schema` |
