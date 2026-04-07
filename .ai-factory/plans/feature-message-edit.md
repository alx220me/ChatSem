# Feature: Редактирование своих сообщений

**Branch:** `feature/message-edit`
**Created:** 2026-04-07
**Type:** feature

## Settings

- **Testing:** Yes — backend unit-тесты (service + handler) + frontend component-тесты
- **Logging:** Verbose — детальные DEBUG-логи на каждом этапе
- **Docs:** No — WARN [docs], обязательного чекпоинта нет

## Roadmap Linkage

Milestone: "none"
Rationale: Все milestone в ROADMAP.md выполнены; задача — post-MVP улучшение.

## Overview

Реализовать редактирование сообщений собственным автором. Удаление уже реализовано (soft-delete, PATCH-эндпоинта нет только). Основная работа:

1. **Backend:** Миграция `edited_at`, метод `EditMessage` в сервисе, `PATCH /api/chat/messages/{msgID}`, трекинг правок через Redis edit_log в poll-ответе.
2. **Frontend:** Типы, ApiClient, useLongPoll (editedMessages), UI кнопка ✏️ + inline-textarea в MessageList, обновление ChatWindow.

**Правило доступа:** только владелец сообщения может редактировать (в отличие от удаления, где мод тоже может). Ошибка `ErrEditForbidden` (403).

---

## Phase 1: Backend Core

### [x] Task 1 — Миграция edited_at
**File:** `migrations/304_add_edited_at_to_messages.sql`
```sql
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ NULL;
```

### [x] Task 2 — Domain: EditedAt, ErrEditForbidden, scanMessages
**Files:** `shared/domain/message.go`, `shared/domain/errors.go`, `services/chat/internal/repository/postgres/message_repo.go`
- Добавить `EditedAt *time.Time` в `Message` (после `DeletedAt`)
- Добавить `ErrEditForbidden = errors.New("only the message owner can edit")`
- Обновить все SQL-SELECT и `scanMessages` / `GetByID` — добавить `m.edited_at`

_Зависит от: Task 1_

### [x] Task 3 — Repository: Update
**Files:** `services/chat/internal/ports/ports.go`, `services/chat/internal/repository/postgres/message_repo.go`
- Добавить в интерфейс `MessageRepository`: `Update(ctx, id, newText) error`
- Реализация: `UPDATE messages SET text=$1, edited_at=NOW() WHERE id=$2 AND deleted_at IS NULL`
- Логирование: `slog.Debug("[MessageRepo.Update]", ...)` на входе и выходе

_Зависит от: Task 2_

### [x] Task 4 — Service: EditMessage
**File:** `services/chat/internal/service/message_service.go`
- Метод `EditMessage(ctx, msgID, requestorID uuid.UUID, newText string) (*domain.Message, error)`
- Валидация текста (empty / too long)
- GetByID → проверка владельца (только owner, не мод) → ErrEditForbidden
- repo.Update → публикация в broker (`{"type":"edit","id":"..."}`)
- Redis edit_log: INCR `chat:{chatID}:edit_seq`, ZADD `chat:{chatID}:edit_log` (score=seq, member=JSON{id,text,edited_at}), EXPIRE 1h
- Возврат обновлённого сообщения
- Логирование: DEBUG на входе, WARN при Redis-ошибках (fail-open), INFO при успехе

_Зависит от: Task 3_

---

## Phase 2: Backend API + Poll

### [x] Task 5 — Handler + Router: PATCH endpoint
**Files:** `services/chat/internal/handler/message_handler.go`, `services/chat/internal/handler/router.go`
- `MessageHandler.Edit`: PATCH `/api/chat/messages/{msgID}`
  - Parse msgID, claims, body `{ "text": "..." }`
  - ErrEmptyMessage/ErrMessageTooLong → 400; ErrEditForbidden → 403; ErrNotFound → 404
  - Success → 200 JSON `{ "id", "text", "edited_at" }`
- router.go: `r.Patch("/api/chat/messages/{msgID}", msgH.Edit)` в authenticated group
- Логирование: DEBUG на входе, INFO при успехе, WARN при ошибках

_Зависит от: Task 4_

### [x] Task 6 — PollHandler: edited_messages в ответе
**File:** `services/chat/internal/handler/poll_handler.go`
- Добавить `editLogKey`, `editSeqKey` (по аналогии с deleteLogKey/deleteSeqKey)
- Тип `editEntry { ID, Text, EditedAt string }` + `fetchEditsSince` (аналог fetchDeletesSince)
- Poll читает `after_edit_seq` из query params
- `buildResult` вызывает fetchEditsSince → результат в `pollResult`
- Ответ: добавить `edited_messages` и `last_edit_seq`
- Fast-path: учитывать `len(res.editedMessages) > 0`
- Логирование: DEBUG при получении правок

_Зависит от: Task 4_

**Commit checkpoint 1** (после Tasks 1–6):
```
feat(chat): add message editing — migration, domain, repo, service, handler, poll
```

---

## Phase 3: Frontend

### [x] Task 7 — Types: editedAt + EditedMessage + PollResponse
**File:** `frontend/widget/src/types/index.ts`
- `Message.editedAt?: string`
- Новый тип `EditedMessage { id, text, edited_at: string }`
- `PollResponse.edited_messages?: EditedMessage[]`, `last_edit_seq?: number`

### [x] Task 8 — ApiClient: editMessage + poll с afterEditSeq
**File:** `frontend/widget/src/api/client.ts`
- `editMessage(msgId, text)`: PATCH `/chat/messages/{msgId}`
- `poll(chatId, afterSeq, afterDeleteSeq, afterEditSeq, signal)`: добавить `&after_edit_seq=${afterEditSeq}`

_Зависит от: Task 7_

### [x] Task 9 — useLongPoll: lastKnownEditSeq и editedMessages
**File:** `frontend/widget/src/hooks/useLongPoll.ts`
- Параметр `onMessages` расширяется: `(msgs, deletedIds, editedMessages: EditedMessage[]) => void`
- `let lastKnownEditSeq = 0`, обновляется из `response.last_edit_seq`
- Передаёт `afterEditSeq` в `api.poll()`
- Вызов `onMessages` при `editedMessages.length > 0`

_Зависит от: Task 8_

### [x] Task 10 — MessageList: кнопка ✏️ и inline-редактирование
**File:** `frontend/widget/src/components/MessageList.tsx`
- Prop `onEdit?: (msgId: string, newText: string) => Promise<void>`
- State `editingId: string | null` + `editText: string`
- `canEdit = msg.userId === currentUserId` — кнопка ✏️ при hover
- Inline-режим: textarea (autoFocus), Enter→сохранить, Escape→отмена
- Метка "(изм.)" если `msg.editedAt` заполнено
- Логирование: `console.debug('[MessageList] edit ...')`

_Зависит от: Task 9_

### [x] Task 11 — ChatWindow: handleEdit + poll-обновления правок
**File:** `frontend/widget/src/components/ChatWindow.tsx`
- `handleEdit(msgId, newText)`: вызывает `api.editMessage`, обновляет allMessages
- `handlePollMessages` добавляет третий параметр `editedMessages`, обновляет текст/editedAt в allMessages
- Передаёт `onEdit={handleEdit}` в MessageList

_Зависит от: Task 10_

**Commit checkpoint 2** (после Tasks 7–11):
```
feat(widget): implement message edit UI — types, client, poll, MessageList, ChatWindow
```

---

## Phase 4: Tests

### [x] Task 12 — Backend тесты: EditMessage service
**File:** `services/chat/internal/service/message_service_test.go`
- Добавить `update func(...)` в `mockMessageRepo`
- TestEditMessage_OwnerCanEdit
- TestEditMessage_NonOwnerForbidden → ErrEditForbidden
- TestEditMessage_NotFound → ErrNotFound
- TestEditMessage_EmptyText → ErrEmptyMessage
- TestEditMessage_TooLongText → ErrMessageTooLong
- Проверить компиляцию poll_handler_test.go (новый параметр after_edit_seq)

_Зависит от: Tasks 5, 6, 11_

### [x] Task 13 — Frontend тесты: MessageList edit UI
**File:** `frontend/widget/src/components/__tests__/MessageList.test.tsx`
- Edit button visible for own messages
- Edit button hidden for others' messages
- Inline edit mode активируется при клике ✏️
- Save: onEdit вызывается с правильными аргументами
- Cancel: onEdit не вызывается, текст восстанавливается
- Edited label: "(изм.)" при наличии editedAt

_Зависит от: Task 11_

**Commit checkpoint 3** (после Tasks 12–13):
```
test: backend and frontend tests for message edit feature
```

---

## Commit Plan

| Checkpoint | Tasks | Commit message |
|------------|-------|----------------|
| 1 | 1–6   | `feat(chat): add message editing — migration, domain, repo, service, handler, poll` |
| 2 | 7–11  | `feat(widget): implement message edit UI — types, client, poll, MessageList, ChatWindow` |
| 3 | 12–13 | `test: backend and frontend tests for message edit feature` |

---

## Key Design Decisions

- **Только владелец** может редактировать (мод — нет). Мод может удалять чужие, но не редактировать.
- **Redis edit_log** аналогичен delete_log: sorted set, score=edit_seq, member=JSON. TTL 1 час.
- **Poll ответ** расширяется полями `edited_messages` / `last_edit_seq` по аналогии с `deleted_ids` / `last_delete_seq`.
- **Soft edit** — исходный текст не хранится, только текущий + `edited_at` timestamp.
- **Broker publish** при редактировании — пробуждает long-poll у других клиентов (как при отправке/удалении).
