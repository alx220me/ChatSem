# Plan: Message Service

**Branch:** `feature/message-service`
**Created:** 2026-04-02
**Milestone:** Message Service

## Settings
- **Testing:** yes — unit тесты (mock repos/redis) + интеграционные (seq monotonicity)
- **Logging:** verbose — DEBUG на входе каждого метода, INFO на успехе, WARN на fail-open
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Message Service"
**Rationale:** Пятый milestone — отправка/листинг/удаление сообщений и модерация нужны до Long Polling и React Widget.

## Tasks

### Phase 1: Сервисный слой

**Task 25 — MessageService: Send, List, SoftDelete**

`services/chat/internal/service/message_service.go`:

`SendMessage(ctx, chatID, userID uuid.UUID, text string) (*domain.Message, error)`:
1. Валидация: len == 0 → ErrEmptyMessage; len > 4096 → ErrMessageTooLong
2. Ban check (service layer): `rdb.Exists(ban:{eventID}:{userID})`
3. Mute check: `repo.GetActiveMute(ctx, chatID, userID)` → ErrUserMuted
4. `repo.Create(ctx, chatID, userID, text)` — атомарный seq через CTE
5. `broker.Publish(ctx, chatID, json)` — fail safe (WARN если недоступен, не возвращать ошибку)

`GetMessages(ctx, chatID uuid.UUID, afterSeq int64, limit int)`:
- cap limit ≤ 100
- `repo.GetByChatAfterSeq(ctx, chatID, afterSeq, limit)`

`SoftDelete(ctx, msgID, requestorID uuid.UUID, role string)`:
- `repo.GetByID` → проверить owner: requestorID == msg.UserID ИЛИ role ∈ {moderator, admin}
- Иначе → `domain.ErrForbidden`
- `repo.SoftDelete(ctx, msgID, requestorID)`

Добавить в MessageRepository: `GetByID(ctx, msgID uuid.UUID) (*Message, error)`

LOGGING:
- `slog.Debug("[MessageService.SendMessage] sending", "chat_id", chatID, "user_id", userID, "text_len", len(text))`
- `slog.Info("[MessageService.SendMessage] sent", "chat_id", chatID, "seq", msg.Seq, "id", msg.ID)`
- `slog.Warn("[MessageService.SendMessage] broker publish failed, message saved", "err", publishErr)`
- `slog.Debug("[MessageService.SoftDelete] deleting", "msg_id", msgID, "by", requestorID, "role", role)`

Files: `services/chat/internal/service/message_service.go`, обновить `shared/domain/repository.go`

### Phase 2: Middleware

**Task 26 — Rate limiting middleware**

`services/chat/internal/middleware/ratelimit.go`:

Функция `allow(ctx, rdb, key, limit, window)`:
- ZRemRangeByScore + ZAdd + ZCard + Expire в pipeline
- **Fail open** при Redis ошибке (возвращает true)

`IPRateLimit(rdb)`: ключ `rl:ip:{ip}` → 100 req/60s → 429 + `Retry-After: 60`

`MessageRateLimit(rdb)`: ключ `rl:msg:{eventID}:{chatID}:{userID}` → 10/10s
- Moderator: лимит × 3; Admin: без ограничений
- 429 + `Retry-After: 10`

```go
const (
    IPRateLimit         = 100
    IPRateWindow        = 60 * time.Second
    MessageRateLimit    = 10
    MessageRateWindow   = 10 * time.Second
    ModeratorMultiplier = 3
)
```

LOGGING:
- `slog.Warn("[IPRateLimit] exceeded", "ip", ip)`, `slog.Warn("[MessageRateLimit] exceeded", "user_id", userID, "chat_id", chatID)`
- `slog.Warn("[*RateLimit] redis error, failing open", "err", err)`

Files: `services/chat/internal/middleware/ratelimit.go`

**Task 27 — Ban check middleware + admin ban/unban**

`services/chat/internal/middleware/ban.go`:
- `BanCheck(rdb)` → ключ `ban:{eventID}:{userID}` → EXISTS → 403
- Fail open при Redis ошибке

`services/admin/internal/service/ban_service.go`:
- `CreateBan(...)`: repo.Create + `rdb.Set("ban:{eventID}:{userID}", "1", ttl)`, ttl = до expiresAt или 24h
- `UnbanUser(banID)`: repo.Expire + `rdb.Del("ban:{eventID}:{userID}")`
- Redis недоступен → WARN, бан в БД сохранён

`services/admin/internal/handler/ban_handler.go`:
- `POST /api/admin/bans` — role=moderator/admin, тело `{user_id, event_id, chat_id?, reason, expires_at?}`
- `DELETE /api/admin/bans/{banID}` — unban
- `GET /api/admin/events/{eventID}/bans` — список с пагинацией

LOGGING:
- `slog.Debug("[BanCheck] checking", "event_id", claims.EventID, "user_id", claims.UserID)`
- `slog.Warn("[BanCheck] user banned", "event_id", claims.EventID, "user_id", claims.UserID)`
- `slog.Info("[BanService.CreateBan] created", "ban_id", ban.ID, "user_id", userID)`
- `slog.Warn("[BanService.CreateBan] redis SET failed, ban in DB only", "err", err)`
- `slog.Info("[BanService.UnbanUser] unbanned", "ban_id", banID)`

Files: `services/chat/internal/middleware/ban.go`, `services/admin/internal/service/ban_service.go`, `services/admin/internal/handler/ban_handler.go`

<!-- Commit checkpoint: tasks 25-27 -->

### Phase 3: HTTP API

**Task 28 — HTTP handlers Message Service**

`services/chat/internal/handler/message_handler.go`:

| Endpoint | Middleware chain | Описание |
|----------|-----------------|----------|
| `POST /api/chat/{chatID}/messages` | IPRateLimit → Auth → BanCheck → MessageRateLimit | Отправить сообщение |
| `GET /api/chat/{chatID}/messages` | IPRateLimit → Auth | Список: `?after=0&limit=50` |
| `DELETE /api/chat/messages/{msgID}` | IPRateLimit → Auth | Soft delete |

Ответы:
- Send: `201 {"id":"uuid","seq":17,"ts":"RFC3339"}`
- List: `200 {"messages":[{id,seq,text,user_id,created_at}]}`
- Delete: `204`

Маппинг ошибок: ErrEmptyMessage/TooLong → 400; ErrBanned/Muted/Forbidden → 403; ErrNotFound → 404

LOGGING:
- `slog.Debug("[MessageHandler.Send] request", "chat_id", chatID, "user_id", claims.UserID, "text_len", len(text))`
- `slog.Info("[MessageHandler.Send] sent", "seq", msg.Seq)`
- `slog.Debug("[MessageHandler.List] request", "chat_id", chatID, "after_seq", afterSeq, "limit", limit)`
- `slog.Debug("[MessageHandler.Delete] request", "msg_id", msgID, "user_id", claims.UserID)`

Files: `services/chat/internal/handler/message_handler.go`, обновить `services/chat/internal/handler/router.go`

<!-- Commit checkpoint: tasks 28 -->

### Phase 4: Tests

**Task 29 — Тесты Message Service**

Unit `services/chat/internal/service/message_service_test.go`:
- EmptyText, TooLongText → ошибки валидации
- BannedUser → ErrUserBanned (mock Redis EXISTS)
- Success → Message с seq; broker.Publish вызван
- BrokerFailure → fail safe (нет ошибки)
- SoftDelete permissions (owner, moderator, foreign user)

Unit `services/chat/internal/middleware/ratelimit_test.go`:
- UnderLimit → allow, OverLimit → deny, WindowSlides → старые записи удалены, RedisError → fail open

Unit `services/chat/internal/middleware/ban_test.go`:
- NotBanned → pass, Banned → 403, RedisError → fail open

Интеграционные (TEST_DATABASE_URL):
- `TestSendMessage_SeqMonotonic`: 5 sends → seq 1,2,3,4,5
- `TestSendMessage_ConcurrentSeq`: 10 goroutines → 10 уникальных seq без пропусков

Files: `services/chat/internal/service/message_service_test.go`, `middleware/ratelimit_test.go`, `middleware/ban_test.go`

<!-- Commit checkpoint: task 29 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 25–27 | `feat: MessageService, rate limiting and ban check middleware` |
| 2 | 28 | `feat: message HTTP handlers with full middleware chain` |
| 3 | 29 | `test: unit and integration tests for message service` |
