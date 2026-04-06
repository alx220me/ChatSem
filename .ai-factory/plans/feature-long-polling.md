# Plan: Long Polling

**Branch:** `feature/long-polling`
**Created:** 2026-04-02
**Milestone:** Long Polling

## Settings
- **Testing:** yes — unit тесты (mock broker/repo) + интеграционные (TEST_REDIS_URL + TEST_DATABASE_URL)
- **Logging:** verbose — DEBUG на входе каждого метода, INFO на успехе, WARN на fail-open
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Long Polling"
**Rationale:** Шестой milestone — real-time доставка сообщений клиентам через long polling нужна до React Widget.

## Tasks

### Phase 1: RedisBroker

**Task 30 — RedisBroker: fan-out реализация**

`services/chat/internal/broker/redis_broker.go`:

Структура:
```go
type fanout struct {
    clients map[<-chan longpoll.Message]chan longpoll.Message
    cancel  context.CancelFunc
}

type RedisBroker struct {
    rdb    *redis.Client
    mu     sync.Mutex
    rooms  map[uuid.UUID]*fanout
}
```

`Subscribe(chatID uuid.UUID) <-chan longpoll.Message`:
- При первом клиенте: `startReader(chatID)` — горутина Redis Pub/Sub
- Клиентский канал буферизован (buffer=1): медленный клиент пропускает сообщение, не блокирует fan-out
- `slog.Debug("broker: new subscriber", "chat_id", chatID)`

`Unsubscribe(chatID uuid.UUID, ch <-chan longpoll.Message)`:
- Удалить клиентский канал из fanout
- При последнем клиенте: `cancel()` горутины → `sub.Close()`
- `slog.Debug("broker: last client left, closing redis sub", "chat_id", chatID)`

`startReader(chatID uuid.UUID)`:
- `rdb.Subscribe(ctx, fmt.Sprintf("chat:%s", chatID))`
- Читать сообщения, доставлять всем клиентам в fanout.clients
- Panic recovery: `recover()` → `slog.Error(...)` → 1s sleep → перезапуск если ctx ещё активен

`Publish(ctx, chatID uuid.UUID, data []byte) error`:
- `rdb.Publish(ctx, fmt.Sprintf("chat:%s", chatID), data)`

Redis-ключ: `chat:{uuid-string}` (строковый формат `%s`)

LOGGING:
- `slog.Debug("[RedisBroker.Subscribe] new client", "chat_id", chatID, "total_clients", n)`
- `slog.Debug("[RedisBroker.Unsubscribe] client removed", "chat_id", chatID, "remaining", n)`
- `slog.Error("[RedisBroker.startReader] panic recovered, restarting", "chat_id", chatID, "err", r)`
- `slog.Warn("[RedisBroker.startReader] redis read error", "chat_id", chatID, "err", err)`

Files: `services/chat/internal/broker/redis_broker.go`

### Phase 2: InMemoryBroker

**Task 31 — InMemoryBroker: pure Go реализация**

`shared/pkg/longpoll/broker.go`:

```go
type Message struct {
    ChatID uuid.UUID
    Data   []byte
}

const (
    LongPollTimeout    = 25 * time.Second
    LongPollSettleDelay = 50 * time.Millisecond
)

type Broker interface {
    Subscribe(chatID uuid.UUID) <-chan Message
    Unsubscribe(chatID uuid.UUID, ch <-chan Message)
    Publish(ctx context.Context, chatID uuid.UUID, data []byte) error
}

type InMemoryBroker struct {
    mu      sync.Mutex
    clients map[uuid.UUID][]chan Message
}
```

- Клиентский канал: buffer=10 (для тестов достаточно)
- `Publish`: доставить всем подписчикам chatID; non-blocking send (select + default)
- Без Redis-зависимостей (pure Go, только `github.com/google/uuid`)

LOGGING: минимальный (это тестовая реализация)

Files: `shared/pkg/longpoll/broker.go`

<!-- Commit checkpoint: tasks 30-31 -->

### Phase 3: HTTP API

**Task 32 — Poll handler**

`services/chat/internal/handler/poll_handler.go`:

`GET /api/chat/{chatID}/poll?after={seq}`:
1. Middleware: `PollIPRateLimit` → Auth → handler
2. `ch := broker.Subscribe(chatID)`; `defer broker.Unsubscribe(chatID, ch)`
3. Settling loop:
```go
select {
case <-ch:
    time.Sleep(longpoll.LongPollSettleDelay) // 50ms — ждём параллельные транзакции
case <-time.After(longpoll.LongPollTimeout): // 25s timeout
    w.WriteHeader(http.StatusNoContent)      // 204 — нет новых сообщений
    return
case <-r.Context().Done():
    return
}
```
4. После settle: `dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)` — независимый контекст для DB запроса (клиент уже мог отключиться)
5. `repo.GetByChatAfterSeq(dbCtx, chatID, afterSeq, 100)` → 200 с JSON

`PollIPRateLimit(rdb)`: ключ `rl:poll:{ip}` → 60 req/60s — fail open при Redis ошибке

Роутинг (обновить `router.go`):
```
r.Group(func(r chi.Router) {
    r.Use(middleware.PollIPRateLimit(rdb))
    r.Use(middleware.Auth(cfg.JWTSecret))
    r.Get("/api/chat/{chatID}/poll", h.Poll)
})
```

Ответ `200`:
```json
{"messages":[{"id":"uuid","seq":17,"text":"...","user_id":"uuid","created_at":"RFC3339"}]}
```

LOGGING:
- `slog.Debug("[PollHandler.Poll] waiting", "chat_id", chatID, "after_seq", afterSeq, "user_id", claims.UserID)`
- `slog.Debug("[PollHandler.Poll] event received, settling", "chat_id", chatID)`
- `slog.Info("[PollHandler.Poll] returning messages", "chat_id", chatID, "count", len(msgs))`
- `slog.Debug("[PollHandler.Poll] timeout, no messages", "chat_id", chatID)`

Files: `services/chat/internal/handler/poll_handler.go`, обновить `services/chat/internal/handler/router.go`

<!-- Commit checkpoint: task 32 -->

### Phase 4: Tests

**Task 33 — Тесты Long Polling**

Unit `shared/pkg/longpoll/broker_test.go` (InMemoryBroker):
- `TestSubscribePublish`: Subscribe → Publish → получить сообщение в канале
- `TestFanout`: 3 подписчика → все получают одно сообщение
- `TestUnsubscribe`: после Unsubscribe — канал не получает новые сообщения
- `TestPublishNoSubscribers`: Publish без подписчиков — нет паники

Unit `services/chat/internal/handler/poll_handler_test.go`:
- `TestPoll_ReceivesMessage`: mock broker отправляет сообщение → 200 с JSON
- `TestPoll_Timeout`: broker не отправляет → через 25s → 204
- `TestPoll_ClientDisconnect`: ctx.Done → handler завершается без паники
- `TestPoll_SettlingWindow`: сообщение прилетает → 50ms sleep → DB запрос после settle

Интеграционные (TEST_REDIS_URL + TEST_DATABASE_URL):
- `TestRedisBroker_PublishReceive`: Publish через Redis → Subscribe получает
- `TestPoll_EndToEnd`: sendMessage → poll получает через Redis → 200 с корректным seq

Files: `shared/pkg/longpoll/broker_test.go`, `services/chat/internal/handler/poll_handler_test.go`

<!-- Commit checkpoint: task 33 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 30–31 | `feat: RedisBroker fan-out and InMemoryBroker` |
| 2 | 32 | `feat: long polling HTTP handler with settling window` |
| 3 | 33 | `test: unit and integration tests for long polling` |
