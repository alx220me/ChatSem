# Auto-Generate API Secret on Event Creation

**Branch:** `feature/auto-generate-api-secret`
**Created:** 2026-04-06

## Settings
- Testing: yes
- Logging: verbose
- Docs: no

## Roadmap Linkage
- Milestone: none
- Rationale: все milestones уже выполнены; фича — QoL улучшение существующей функциональности

## Context

Сейчас при создании события администратор вручную придумывает и вводит `api_secret`.
Это неудобно и небезопасно (слабые секреты, повторное использование).

**Цель:** сервер генерирует криптостойкий 64-символьный hex-secret (32 случайных байта через `crypto/rand`),
хеширует его bcrypt, хранит хеш. Plaintext secret возвращается **один раз** в ответе на создание —
аналогично GitHub Personal Access Tokens.

## Tasks

### Phase 1 — Backend

- [x] Task #9: Генерировать secret в EventService
  - `services/admin/internal/service/event_service.go`
  - Убрать параметр `apiSecret`, генерировать через `crypto/rand` → hex
  - Возвращать `(event, plainSecret string, error)`

- [x] Task #10: Обновить EventHandler
  - `services/admin/internal/handler/event_handler.go`
  - Убрать `api_secret` из `createEventRequest`
  - Добавить `createEventResponse` с полями события + `api_secret` (plaintext)

### Phase 2 — Frontend

- [x] Task #11: Обновить AdminApiClient
  - `frontend/admin/src/api/adminClient.ts`
  - `createEvent(name, allowedOrigin)` — без apiSecret
  - Тело: `{ name, allowed_origin }` (snake_case)
  - Ответ: `Event & { api_secret: string }`

- [x] Task #12: Обновить CreateEventModal
  - `frontend/admin/src/pages/EventsPage.tsx`
  - Убрать поле ввода "API Secret"
  - `setCreatedSecret(event.api_secret)` из ответа

### Phase 3 — Tests

- [x] Task #13: Обновить тесты
  - `frontend/admin/src/pages/EventsPage.test.tsx`
  - `services/admin/internal/service/event_service_test.go`

## Commit Plan

**Checkpoint 1** (после Task #10) — backend готов:
```
feat(admin): auto-generate api_secret on event creation
```

**Checkpoint 2** (после Task #13) — фича полностью готова:
```
feat(admin/frontend): remove api_secret input, show generated secret
```
