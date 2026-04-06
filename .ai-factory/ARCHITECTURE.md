# Architecture: Multi-Service Monorepo

## Overview

ChatSem is organized as a **monorepo с тремя независимыми Go-сервисами**: `chat`, `auth`, `admin`.
Каждый сервис — отдельный бинарник с собственным HTTP-сервером, внутренней Clean Architecture
и независимым деплоем. Общий код (domain-типы, утилиты) вынесен в `shared/`
Паттерн выбран потому, что ChatSem имеет три явно разграниченных домена:
1. **Chat** — реальная работа с чатами, сообщениями, иерархией, long polling
2. **Auth** — SSO, JWT, сессии; может масштабироваться и тестироваться независимо
3. **Admin** — модерация, управление событиями; отдельный endpoint с другими правами доступа

Один репозиторий позволяет переиспользовать типы из `shared/` без network-вызовов между сервисами для базовых структур.

## Decision Rationale

- **Project type:** Embeddable chat service for online events
- **Tech stack:** Go 1.24, chi router, pgx v5, PostgreSQL, Redis
- **Scale target:** 10 000 одновременных пользователей
- **Key factor:** Три домена с разными зонами ответственности и потенциально разными требованиями к масштабированию; при этом команда небольшая, поэтому отдельные репозитории излишни

## Repository Structure

```
ChatSem/
├── go.work                    # Go workspace — links all Go modules
│
├── services/                  # Go backend services
│   ├── chat/                  # Chat service :8080 — чаты, сообщения, long poll
│   │   ├── cmd/main.go        # Точка входа, DI
│   │   ├── internal/
│   │   │   ├── domain/        # Entities + repository interfaces (no external deps)
│   │   │   ├── service/       # Business logic: chat hierarchy, messages, long poll
│   │   │   ├── repository/postgres/  # pgx implementations
│   │   │   ├── handler/       # HTTP handlers (chi)
│   │   │   └── middleware/    # JWT auth middleware
│   │   └── go.mod
│   │
│   ├── auth/                  # Auth service :8081 — SSO, JWT, сессии
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── domain/        # User, Session, SSOConfig entities
│   │   │   ├── service/       # JWT validation, session store, token exchange
│   │   │   ├── repository/postgres/
│   │   │   └── handler/       # /auth/validate, /auth/session
│   │   └── go.mod
│   │
│   └── admin/                 # Admin service :8082 — управление, модерация, экспорт
│       ├── cmd/main.go
│       ├── internal/
│       │   ├── domain/
│       │   ├── service/
│       │   ├── repository/postgres/
│       │   └── handler/
│       └── go.mod
│
├── shared/                    # Общий Go-код (no pgx/Redis/chi)
│   ├── domain/                # Базовые типы: Chat, Message, User, Event
│   └── pkg/
│       ├── longpoll/          # Интерфейс Broker + тип Message (pure Go, без Redis)
│       ├── jwt/               # JWT helpers
│       └── response/          # Стандартные HTTP-ответы
│
├── frontend/                  # React фронтенд (TypeScript + Vite)
│   ├── widget/                # Встраиваемый виджет чата
│   │   ├── src/
│   │   │   ├── components/    # ChatWindow, MessageList, MessageInput, UserAvatar
│   │   │   ├── hooks/         # useLongPoll, useAuth, useChat
│   │   │   ├── api/           # fetch-клиент к chat-service
│   │   │   ├── types/         # TypeScript types
│   │   │   └── index.tsx      # Точка входа (монтирует виджет в DOM)
│   │   ├── package.json
│   │   └── vite.config.ts     # Собирается в один JS-бандл (IIFE)
│   │
│   └── admin/                 # Панель администратора (SPA)
│       ├── src/
│       │   ├── pages/         # Events, Chats, Moderation, Export
│       │   ├── components/
│       │   ├── api/           # fetch-клиент к admin-service
│       │   └── main.tsx
│       ├── package.json
│       └── vite.config.ts
│
├── migrations/                # SQL-миграции, общие для всех сервисов
│   ├── 001_create_events.sql
│   ├── 002_create_chats.sql
│   └── 003_create_messages.sql
│
└── deploy/
    ├── docker-compose.yml          # все сервисы + postgres + pgbouncer + redis + nginx
    ├── nginx.conf                  # Reverse proxy: /api/chat → chat:8080, etc.
    ├── pgbouncer/
    │   └── pgbouncer.ini           # transaction pooling, pool_size=50
    ├── services/chat/Dockerfile
    ├── services/auth/Dockerfile
    ├── services/admin/Dockerfile
    ├── frontend/widget/Dockerfile
    └── frontend/admin/Dockerfile
```

## ID Strategy

Все сущности используют **UUID v4** в качестве первичного ключа.

**Правила:**
- SQL: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` (PostgreSQL 13+ — встроенная функция, расширений не нужно)
- Внешние ключи: `UUID NOT NULL REFERENCES ...` (не BIGINT)
- Go: `github.com/google/uuid` — тип `uuid.UUID` во всех domain-структурах и сигнатурах методов
- Redis-ключи: `chat:<uuid-string>`, `ban:<eventID>:<userID>` — строковый формат (`%s`)
- JSON API: UUID передаётся как строка `"id": "550e8400-e29b-41d4-a716-446655440000"`

**Почему UUID, не BIGSERIAL:**
- Горизонтальные инстансы генерируют ID независимо, без координации через БД
- ID безопасны для отображения в URL и API (не предсказуемы как последовательные int)
- Упрощает будущие шардирование и репликацию

## Service Responsibilities

| Сервис | Порт | Зона ответственности | Масштабирование |
|--------|------|----------------------|-----------------|
| `chat` | 8080 | Чаты, сообщения, иерархия, long polling, история | Горизонтально (2-4 инстанса) |
| `auth` | 8081 | SSO JWT validation, сессии, token exchange | 1-2 инстанса |
| `admin` | 8082 | Управление событиями, модерация, экспорт, бан | 1 инстанс |

## Scale Architecture (10k–30k concurrent users)

### Long Polling + Redis Pub/Sub + PgBouncer

Go легко держит 30k goroutines (~240MB RAM). Узкие места — синхронизация между инстансами
и количество соединений к PostgreSQL. Оба решаются без смены протокола.

**Схема:**

```
Clients (10k–30k)
    │  HTTP long poll (GET /chat/{id}/poll?after={seq})
    ▼
nginx (load balancer, keepalive)
    │  round-robin
    ├── chat-svc:8080  ──┐
    ├── chat-svc:8080  ──┤── Redis Pub/Sub
    └── chat-svc:8080  ──┘   PUBLISH/SUBSCRIBE
                              channel: chat:{chatID}
                                   │
                                   ▼
                             PgBouncer :5432        ← connection pooler
                             (N×20 app conn → 50 real conn)
                                   │
                                   ▼
                             PostgreSQL :5433
```

**Long polling flow:**
1. Клиент делает `GET /poll?chatID=X&after=seqN` — соединение висит open
2. Handler подписывается на Redis channel `chat:X`
3. При новом сообщении: `message_service` сохраняет в PG, делает `PUBLISH chat:X {msg}`
4. Все инстансы получают событие и отвечают своим waiting-клиентам
5. Таймаут 25с → ответ `{"messages": []}`, клиент переподключается

**Параметры:**

```go
// pgxpool подключается к PgBouncer, не к PostgreSQL напрямую
// PgBouncer сам держит пул реальных соединений к PostgreSQL (pool_size=50)
// Сервисы могут открывать больше app-соединений — PgBouncer их мультиплексирует
pool.Config().MaxConns = 20   // per instance → PgBouncer
pool.Config().MinConns = 5

// HTTP server
srv := &http.Server{
    ReadTimeout:  5 * time.Second,
    WriteTimeout: 30 * time.Second,  // long poll timeout + buffer
    IdleTimeout:  60 * time.Second,
}

// Long poll timeout
const LongPollTimeout = 25 * time.Second
```

**nginx:**
```nginx
upstream chat_backend {
    server chat1:8080;
    server chat2:8080;
    keepalive 200;
}
location /api/chat/ {
    proxy_pass         http://chat_backend;
    proxy_http_version 1.1;
    proxy_read_timeout 35s;
}
```

## Message Flow

### Отправка сообщения пользователем

```
React Widget                chat-service (любой инстанс)       Redis        PostgreSQL
     │                               │                            │               │
     │  POST /api/chat/{id}/messages │                            │               │
     │  Authorization: Bearer <JWT>  │                            │               │
     │  {"text": "Hello"}            │                            │               │
     │──────────────────────────────►│                            │               │
     │                               │ 1. middleware: validate JWT│               │
     │                               │    (shared/pkg/jwt, local) │               │
     │                               │                            │               │
     │                               │ 2. service.SendMessage()   │               │
     │                               │    - проверить права       │               │
     │                               │    - проверить бан/мут     │               │
     │                               │    - фильтр контента       │               │
     │                               │                            │               │
     │                               │ 3. repo.SaveMessage()      │               │
     │                               │───────────────────────────────────────────►│
     │                               │    INSERT messages ...     │               │
     │                               │    RETURNING id, seq, ts   │               │
     │                               │◄───────────────────────────────────────────│
     │                               │                            │               │
     │                               │ 4. PUBLISH chat:{id}       │               │
     │                               │    {id,seq,text,user,ts}   │               │
     │                               │───────────────────────────►│               │
     │                               │                            │               │
     │  201 Created                  │                            │               │
     │  {"id":42,"seq":17,"ts":"…"}  │                            │               │
     │◄──────────────────────────────│                            │               │
```

### Получение сообщений (Long Polling)

```
React Widget         chat-svc A (клиент подключён)   Redis     chat-svc B (другой инстанс)
     │                        │                         │                │
     │  GET /api/chat/{id}/poll│                         │                │
     │  ?after=16&timeout=25   │                         │                │
     │────────────────────────►│                         │                │
     │                         │ SUBSCRIBE chat:{id}     │                │
     │                         │────────────────────────►│                │
     │                         │   ... ждём 25 сек ...   │                │
     │                         │                         │                │
     │                         │         ← другой пользователь шлёт сообщение →
     │                         │                         │                │
     │                         │   PUBLISH chat:{id}     │                │
     │                         │         {msg}           │◄───────────────│
     │                         │◄────────────────────────│                │
     │                         │                         │                │
     │                         │ UNSUBSCRIBE chat:{id}   │                │
     │  200 OK                 │                         │                │
     │  {"messages":[{seq:17}]}│                         │                │
     │◄────────────────────────│                         │                │
     │                         │                         │                │
     │  (клиент сразу          │                         │                │
     │   переподключается      │                         │                │
     │   с ?after=17)          │                         │                │
```

### Таймаут без новых сообщений

```
     │  GET /poll?after=16&timeout=25   │
     │─────────────────────────────────►│
     │       ... 25 секунд тишины ...   │
     │  200 OK                          │
     │  {"messages":[]}                 │  ← пустой ответ
     │◄─────────────────────────────────│
     │  (клиент переподключается снова) │
```

### Правила реализации Message Flow

- **JWT валидируется локально** в `middleware/auth.go` через `shared/pkg/jwt` с pre-shared secret из конфига — HTTP-вызова к auth-service нет и быть не должно
- **Сначала сохранить в БД, потом PUBLISH** — если Redis недоступен, сообщение не потеряется; клиент получит его при следующем poll через seq-based запрос
- **Broker с fan-out** — handler вызывает `broker.Subscribe(chatID)`, Broker держит одну Redis-подписку на чат и раздаёт события в памяти всем клиентам этого чата; при отписке последнего клиента Redis-подписка закрывается
- **Горизонтальные инстансы** — Redis pub/sub гарантирует, что PUBLISH от chat-svc B доберётся до всех подписчиков на chat-svc A, C, D

### shared/pkg/longpoll: интерфейс Broker (pure Go)

`shared/pkg/longpoll` содержит **только** тип `Message` и интерфейс `Broker` — без зависимостей на Redis.
Redis-backed реализация живёт в `services/chat/internal/broker/` (только chat использует long polling).

```go
// shared/pkg/longpoll/broker.go  — pure Go, no Redis import

type Message struct {
    ChatID uuid.UUID
    Data   []byte  // JSON сообщения
}

// Broker — интерфейс для DI и тестирования.
type Broker interface {
    // Subscribe регистрирует клиента. При первом клиенте реализация запускает горутину-читатель.
    Subscribe(chatID uuid.UUID) <-chan Message

    // Unsubscribe удаляет клиента. При последнем клиенте реализация останавливает горутину.
    Unsubscribe(chatID uuid.UUID, ch <-chan Message)

    // Publish отправляет сообщение (например, в Redis-канал chat:{chatID}).
    Publish(ctx context.Context, chatID uuid.UUID, data []byte) error
}

const (
    LongPollTimeout    = 25 * time.Second
    LongPollSettleDelay = 50 * time.Millisecond
)
```

**Где живёт Redis-backed реализация:**

```
services/chat/internal/broker/
    redis_broker.go   ← struct RedisBroker implements longpoll.Broker
```

**Проблема без fan-out:**
1000 клиентов в одном чате → 1000 `SUBSCRIBE chat:X` в Redis → лишние соединения и нагрузка.

**Решение: RedisBroker держит одну Redis-подписку на чат, раздаёт в памяти:**

```
Redis pub/sub
    │  1 подписка на chat:X (для всего инстанса)
    ▼
RedisBroker.chatSub[chatID]     ← горутина-читатель Redis
    │  fan-out
    ├── clientCh[0]  →  handler клиента A
    ├── clientCh[1]  →  handler клиента B
    └── clientCh[N]  →  handler клиента N
```

**Структура RedisBroker (в services/chat/internal/broker/):**

```go
// services/chat/internal/broker/redis_broker.go

type RedisBroker struct {
    rdb *redis.Client
    mu  sync.RWMutex
    // одна запись на chatID, пока есть хоть один подписчик
    subs map[uuid.UUID]*fanout
}

type fanout struct {
    mu      sync.Mutex
    clients map[chan longpoll.Message]struct{}
    cancel  context.CancelFunc  // отменяет горутину-читатель Redis
}

func NewRedisBroker(rdb *redis.Client) *RedisBroker

// Реализует интерфейс longpoll.Broker
func (b *RedisBroker) Subscribe(chatID uuid.UUID) <-chan longpoll.Message
func (b *RedisBroker) Unsubscribe(chatID uuid.UUID, ch <-chan longpoll.Message)
func (b *RedisBroker) Publish(ctx context.Context, chatID uuid.UUID, data []byte) error
```

**Горутина-читатель Redis (запускается один раз на чат при первом Subscribe):**

```go
func (b *RedisBroker) startReader(ctx context.Context, chatID uuid.UUID, f *fanout) {
    sub := b.rdb.Subscribe(ctx, fmt.Sprintf("chat:%s", chatID))
    defer sub.Close()
    for {
        select {
        case msg := <-sub.Channel():
            f.mu.Lock()
            for ch := range f.clients {
                select {
                case ch <- longpoll.Message{ChatID: chatID, Data: []byte(msg.Payload)}:
                default: // клиент не читает — пропускаем, не блокируем
                }
            }
            f.mu.Unlock()
        case <-ctx.Done():
            return
        }
    }
}
```

**Long-poll handler использует интерфейс Broker:**

```go
func (h *PollHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    chatID, afterSeq := parseParams(r)

    ch := h.broker.Subscribe(chatID)
    defer h.broker.Unsubscribe(chatID, ch)

    select {
    case <-ch:
        time.Sleep(longpoll.LongPollSettleDelay) // settling window 50мс
    case <-time.After(longpoll.LongPollTimeout):
        // таймаут — вернуть пустой ответ
    }

    // r.Context() не используем: клиент мог отключиться во время sleep —
    // контекст уже отменён, запрос немедленно вернёт ошибку и создаст шум в логах.
    // Используем отдельный контекст с таймаутом — DB-запрос всегда завершается.
    dbCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()
    msgs, _ := h.repo.GetMessages(dbCtx, chatID, afterSeq)
    response.JSON(w, 200, msgs)
}
```

**Правила реализации Broker:**
- Одна горутина-читатель Redis на chatID на весь инстанс — не на клиента
- `Unsubscribe` последнего клиента → `cancel()` горутины → `sub.Close()` → Redis отписка
- Клиентский канал буферизован (buffer=1): если handler не успевает читать — сообщение пропускается (не блокирует fan-out)
- `Publish` не знает о клиентах — только пишет в Redis; fan-out внутри RedisBroker
- RedisBroker инициализируется один раз в `cmd/main.go`, передаётся в сервисы через DI как `longpoll.Broker`

### Seq: гонка и потеря сообщений

**Проблема 1 — гонка при генерации seq**

Если два инстанса одновременно делают `SELECT MAX(seq)+1`, оба получат одинаковый номер.

**Решение: атомарный инкремент через отдельную таблицу счётчиков**

```sql
-- Таблица счётчиков (одна строка на чат)
CREATE TABLE chat_seqs (
    chat_id UUID PRIMARY KEY REFERENCES chats(id),
    last_seq BIGINT NOT NULL DEFAULT 0
);

-- Атомарная вставка сообщения: seq назначается и сообщение вставляется в одной CTE
WITH next_seq AS (
    UPDATE chat_seqs
    SET last_seq = last_seq + 1
    WHERE chat_id = $1
    RETURNING last_seq
)
INSERT INTO messages (chat_id, user_id, text, seq, created_at)
SELECT $1, $2, $3, next_seq.last_seq, NOW()
FROM next_seq
RETURNING id, seq, created_at;
```

`UPDATE ... RETURNING` на одной строке в PostgreSQL сериализован — гонка невозможна.
Если транзакция откатится — seq «сгорает» (пропуск в нумерации), сообщение не теряется.

---

**Проблема 2 — потеря сообщения при пропуске seq**

Сценарий:
```
T1: получает seq=17, ещё не закоммитился
T2: получает seq=18, коммитит, делает PUBLISH
Клиент: получает seq=18, обновляет after=18
T1: коммитит seq=17 — клиент уже не увидит его никогда
```

**Решение: settling window в long-poll handler**

После получения события из Redis handler **не отвечает немедленно**, а ждёт короткий settling window (50мс), затем запрашивает из БД все сообщения `WHERE seq > after`:

```go
// pkg/longpoll/broker.go — после получения Redis-события
select {
case <-redisCh:
    time.Sleep(50 * time.Millisecond) // settling window
case <-time.After(LongPollTimeout):
    // таймаут — вернуть пустой ответ
}
// Запросить ВСЕ новые сообщения из БД, не только то, что пришло в Redis
msgs, _ := repo.GetMessages(ctx, chatID, afterSeq)
```

За 50мс параллельные транзакции успевают закоммититься. Клиент получает все сообщения с seq > after одним запросом к БД.

**Trade-off: 10k горутин, спящих одновременно**

`time.Sleep(50ms)` держит горутину заблокированной. При 10k одновременных клиентов — 10k горутин ждут одновременно при burst-нагрузке сообщениями.

Это осознанный выбор:
- Go-горутина в состоянии sleep ≈ 2–4 KB RAM → 10k горутин ≈ 20–40 MB — приемлемо
- Sleep не занимает OS-поток (Go runtime паркует горутину) → CPU не тратится
- Альтернатива — `time.AfterFunc` с явным wake-up — усложняет код без выигрыша при данном масштабе
- При росте до 100k+ клиентов на инстанс стоит пересмотреть (сейчас target 10k)

**Клиентская сторона (дополнительная защита):**

При реконнекте клиент запрашивает `after = lastKnownSeq - 1` и дедуплицирует по `id` — защита от повторной доставки при сетевых сбоях.

---

**Итоговые правила seq:**

- `chat_seqs` — обязательная таблица, создаётся вместе с чатом (`INSERT INTO chat_seqs (chat_id) VALUES ($1)`)
- seq назначается только через `UPDATE chat_seqs SET last_seq = last_seq + 1 ... RETURNING` внутри той же транзакции, что и INSERT в `messages`
- Long-poll handler всегда читает сообщения из БД (`WHERE seq > after`) после события — не доверяет payload из Redis
- Settling window = **50мс** (константа `LongPollSettleDelay` в `shared/pkg/longpoll`)

### Инвалидация JWT при бане пользователя

**Проблема:** JWT stateless — токен валиден до истечения (exp), даже если пользователь забанен.
Локальная валидация подписи это не видит.

**Решение: ban-check через Redis без HTTP-вызова к auth-service**

```
Бан пользователя (admin-service):
  1. INSERT INTO bans (event_id, user_id, reason, banned_at, banned_by)
  2. SET ban:{eventID}:{userID} 1 EX {jwt_max_ttl_seconds}
     (TTL = максимальный срок жизни JWT, чтобы ключ не висел вечно)

Разбан (admin-service):
  1. UPDATE bans SET unbanned_at = NOW() WHERE ...
  2. DEL ban:{eventID}:{userID}

Middleware chat-service (порядок проверок):
  1. Validate JWT signature → shared/pkg/jwt → 401 если невалиден
  2. Check Redis EXISTS ban:{eventID}:{userID} → 403 если ключ есть
  3. Proceed → req.Context() с Claims
```

**Почему Redis, а не PostgreSQL:**
- Redis `EXISTS` = O(1), ~0.1мс — не блокирует hot path
- PostgreSQL SELECT на каждый запрос создаёт лишнюю нагрузку на пул соединений
- TTL на ключе гарантирует автоочистку — ключ исчезнет сам, когда истекут все старые токены

**Namespacing Redis-ключей:**
- Бан: `ban:{eventID}:{userID}` — per-event (пользователь может быть забанен в одном ивенте, но не в другом)
- Глобальный бан (если нужен): `ban:global:{userID}`

**Правила реализации:**
- `admin-service` — единственный, кто пишет/удаляет ключи `ban:*`
- `chat-service` и `admin-service` — только читают `EXISTS ban:*` в middleware
- TTL ключа = значение env `JWT_MAX_TTL` (должно совпадать во всех сервисах)
- При разбане ключ удаляется немедленно (`DEL`), не ждёт TTL

## Dependency Rules (внутри каждого сервиса)

Внутри каждого сервиса соблюдается Clean Architecture:

```
handler → service → domain ← repository
```

- ✅ `handler` → `service` (вызывает use cases)
- ✅ `service` → `domain` (использует entities и interfaces)
- ✅ `repository/postgres` → `domain` (реализует interfaces)
- ✅ Любой сервис → `shared/` (базовые типы и утилиты)
- ❌ `domain` ЗАПРЕЩЕНО импортировать `service`, `repository`, `handler`, pgx, Redis
- ❌ `service` ЗАПРЕЩЕНО импортировать `handler` или конкретные `repository/postgres`
- ❌ `chat` ЗАПРЕЩЕНО импортировать `admin` или `auth` внутренние пакеты (только `shared/`)

## Inter-Service Communication

```
widget (JS)
    │
    ▼ HTTP long poll / REST
┌─────────┐    JWT validate    ┌──────────┐
│  chat   │ ─────────────────► │   auth   │
│ :8080   │  (или shared/jwt)  │  :8081   │
└─────────┘                    └──────────┘
                                     │
┌─────────┐                    admin reads
│  admin  │ ──── pgx ────────► PostgreSQL
│ :8082   │                    (shared DB)
└─────────┘
```

**Правила коммуникации:**
- `chat`, `admin` валидируют JWT **локально** через `shared/pkg/jwt` с pre-shared secret из конфига — HTTP-вызова к `auth` нет
- `auth` — единственный сервис, который **выдаёт** токены (SSO token exchange, создание сессий); остальные сервисы только валидируют
- Сервисы **не вызывают друг друга** для запросов к базе — каждый читает из shared PostgreSQL напрямую
- Redis — общий; namespacing по prefix: `chat:`, `auth:`, `admin:`

## SSO Flow и Widget Embedding

### Как хост-сайт встраивает виджет

Виджет — IIFE-бандл, загружается через `<script>` тег. **Не iframe** — нет postMessage.
Инициализация через глобальную функцию `window.ChatSem.init(config)`:

```html
<!-- Хост-сайт HTML -->
<div id="chatsem-widget"></div>
<script src="https://cdn.chatsem.io/widget.js"></script>
<script>
  window.ChatSem.init({
    containerId: "chatsem-widget",
    eventId: 123,
    token: "{{ chat_token }}",         // JWT, рендерится сервер-сайд
    onTokenExpired: async () => {       // опционально: callback для обновления токена
      const r = await fetch("/api/my-chat-token");
      return (await r.json()).token;
    }
  });
</script>
```

**Почему не `data-` атрибуты на script-теге:**
- `data-token` виден в HTML-source и легко извлекается ботами
- Нет возможности передать callback для refresh

**Почему не cookie:**
- Виджет работает на домене хост-сайта (same-origin), но chat-service на другом домене — cross-origin cookie запрещены по умолчанию
- Усложняет CORS и SameSite политику

### Token Exchange: как хост получает JWT для виджета

Хост-сайт **не выдаёт JWT сам** — он обращается к auth-service по серверному каналу:

```
Хост-сайт (backend)          auth-service :8081
      │                              │
      │  POST /api/auth/token        │
      │  Authorization: Bearer <pre-shared-secret>
      │  {                           │
      │    "external_user_id": "u42",│
      │    "event_id": 123,          │
      │    "name": "Иван",           │
      │    "role": "user"            │
      │  }                           │
      │ ─────────────────────────►  │
      │                              │  1. Проверить pre-shared-secret
      │                              │  2. Upsert user в БД
      │                              │  3. Выдать JWT
      │  { "token": "eyJ..." }       │
      │ ◄─────────────────────────  │
      │                              │
      │  Рендерит token в HTML       │
      │  или возвращает в JS         │
```

**pre-shared-secret** — статический секрет организатора, хранится в конфиге auth-service.
Каждый event может иметь свой секрет (поле `events.api_secret` в БД).

**auth-service при token exchange:**
```go
// services/auth/internal/handler/token.go

type TokenRequest struct {
    ExternalUserID string    `json:"external_user_id"`
    EventID        uuid.UUID `json:"event_id"`
    Name           string    `json:"name"`
    Role           string    `json:"role"` // "user" | "moderator"
}

// 1. Validate Authorization: Bearer <secret> против events.api_secret
// 2. INSERT INTO users (external_id, event_id, name, role)
//    ON CONFLICT (external_id, event_id) DO UPDATE SET name=$3, role=$4
// 3. Sign JWT: Claims{UserID, ExternalID, EventID, Role, exp: now+TTL}
// 4. Return {"token": "<jwt>"}
```

### Хранение токена в виджете

Токен хранится **только в памяти JS** (переменная модуля) — не в `localStorage`, не в `sessionStorage`.

**Почему не localStorage:**
- XSS на хост-сайте → атакующий читает токен из localStorage и делает запросы от имени пользователя
- В памяти модуля токен недоступен из других скриптов на странице

**Срок жизни JWT:** `exp = now + 4h`. При истечении виджет вызывает `onTokenExpired()` callback.
Если callback не задан — показывает сообщение "сессия истекла, обновите страницу".

### CORS

chat-service и auth-service разрешают запросы только с доменов организатора.
Разрешённый origin хранится в `events.allowed_origin` (конфигурируется в admin panel).

```go
// services/chat/internal/middleware/cors.go

func CORS(repo EventRepository) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")
            eventID := extractEventID(r) // из query param или JWT

            allowed, _ := repo.GetAllowedOrigin(r.Context(), eventID)
            if origin == allowed {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Credentials", "true")
            }
            // preflight
            if r.Method == http.MethodOptions {
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
                w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
                w.WriteHeader(http.StatusNoContent)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

**Wildcard `*` запрещён** — токен в Authorization header, credentials=true несовместимы с wildcard origin.

### Полный flow от загрузки страницы до первого сообщения

```
Пользователь открывает страницу хост-сайта
    │
    │  1. Хост-backend: POST /api/auth/token → auth-service
    │     Получает JWT, рендерит в HTML
    │
    │  2. Браузер загружает widget.js + вызывает ChatSem.init({token, eventId})
    │
    │  3. Виджет: GET /api/chat/events/{id}/chats → chat-service
    │     Authorization: Bearer <jwt>
    │     Получает список чатов (parent + child для текущего зала)
    │
    │  4. Виджет: GET /api/chat/chats/{id}/messages?limit=50 → chat-service
    │     Загружает последние 50 сообщений
    │
    │  5. Виджет: GET /api/chat/chats/{id}/poll?after=<seq> → chat-service
    │     Соединение висит open (long poll), ждёт новых сообщений
    │
    ▼
Пользователь видит чат, готов писать
```

## Child Chat Auto-Creation (Lazy)

### Триггер: первый вход пользователя в зал

Child-чат не создаётся заранее. Он создаётся в момент, когда виджет впервые обращается к chat-service с `eventID` + `roomID`.

```
Widget (пользователь входит в зал roomID=5)
    │
    │  POST /api/chat/join
    │  {"event_id": 1, "room_id": "hall-5"}
    ▼
chat-service: ChatService.GetOrCreateChildChat(eventID, roomID)
    │
    │  1. SELECT id FROM chats
    │     WHERE event_id=$1 AND external_room_id=$2
    │     → нашли → вернуть
    │
    │  2. Не нашли → создать:
    │     SELECT id, settings FROM chats        ← parent chat
    │     WHERE event_id=$1 AND type='parent'
    │
    │  3. INSERT INTO chats
    │     (event_id, parent_id, external_room_id, type)
    │     VALUES ($1, $2, $3, 'child')          ← settings не хранятся, всегда из parent
    │     ON CONFLICT (event_id, external_room_id) DO NOTHING
    │     RETURNING id
    │
    │  4. Если RETURNING пустой (конкурентная вставка уже выполнена):
    │     SELECT id FROM chats
    │     WHERE event_id=$1 AND external_room_id=$2
    │
    │  5. INSERT INTO chat_seqs (chat_id) VALUES ($1)
    │     ON CONFLICT DO NOTHING               ← счётчик seq для нового чата
    │
    ▼
    Вернуть {chat_id, settings} виджету
```

### Защита от race condition

Уникальный индекс в PostgreSQL:

```sql
CREATE UNIQUE INDEX uq_chats_event_room
    ON chats (event_id, external_room_id)
    WHERE type = 'child';
```

`INSERT ... ON CONFLICT DO NOTHING` — атомарная операция. При двух одновременных вставках одна пройдёт, другая получит пустой `RETURNING` и затем сделает `SELECT`. Дублей не будет.

### Наследование settings

Settings **всегда берутся из parent-чата динамически** — child-чат не хранит собственные settings в БД. При изменении настроек parent все child-чаты немедленно отражают новые настройки.

```
Parent settings изменились в admin-service
    │
    ▼
UPDATE chats SET settings=$1 WHERE id=$2 AND type='parent'
    │
    └── Все child-чаты автоматически видят новые настройки
        при следующем обращении — без дополнительных действий
```

**Схема БД:** у child-чатов колонка `settings` отсутствует (или NULL). При чтении репозиторий всегда делает JOIN с parent:

```sql
-- Получить child-чат с актуальными settings от parent
SELECT
    c.id,
    c.event_id,
    c.parent_id,
    c.external_room_id,
    c.type,
    c.created_at,
    p.settings   -- settings всегда из parent
FROM chats c
JOIN chats p ON p.id = c.parent_id
WHERE c.event_id = $1 AND c.external_room_id = $2 AND c.type = 'child';
```

```go
// services/chat/internal/service/chat_service.go

func (s *ChatService) GetOrCreateChildChat(ctx context.Context,
    eventID uuid.UUID, roomID string) (*domain.Chat, error) {

    // быстрый путь: чат уже существует
    if chat, err := s.repo.FindByRoom(ctx, eventID, roomID); err == nil {
        slog.Debug("child chat found", "chat_id", chat.ID, "room_id", roomID)
        return chat, nil
    }

    // медленный путь: первый вход в зал
    parent, err := s.repo.FindParent(ctx, eventID)
    if err != nil {
        return nil, fmt.Errorf("parent chat not found for event %d: %w", eventID, err)
    }

    child := &domain.Chat{
        EventID:        eventID,
        ParentID:       &parent.ID,
        ExternalRoomID: roomID,
        Type:           domain.ChatTypeChild,
        // Settings не хранятся — всегда читаются из parent динамически
    }

    created, err := s.repo.CreateChild(ctx, child) // INSERT ... ON CONFLICT DO NOTHING
    if err != nil {
        return nil, err
    }
    if created == nil {
        // конкурентная вставка выиграла — читаем то, что создал другой инстанс
        return s.repo.FindByRoom(ctx, eventID, roomID)
    }

    slog.Info("child chat auto-created",
        "chat_id", created.ID, "event_id", eventID, "room_id", roomID)
    return created, nil
}
```

### Parent chat: создаётся явно через admin-service

Parent-чат создаётся организатором через admin API при создании события — не лениво.

```
POST /api/admin/events
  → admin-service создаёт Event + ParentChat + chat_seqs в одной транзакции
  → ParentChat хранит настройки по умолчанию для всех залов
  → INSERT INTO chat_seqs (chat_id) VALUES (<parent_id>) — обязательно при создании
```

### Правила auto-creation

- `external_room_id` — ID зала из системы организатора (string, любой формат)
- Уникальный индекс `(event_id, external_room_id)` WHERE type='child' — в БД, не в коде
- `chat_seqs` запись создаётся вместе с чатом: `INSERT ... ON CONFLICT DO NOTHING`
- Child-чат **не удаляется** при отсутствии пользователей — история сохраняется
- Settings child-чата **не хранятся в БД** — репозиторий всегда делает JOIN с parent при чтении
- Изменение settings parent-чата немедленно применяется ко всем child-чатам без дополнительных операций

## Key Principles

1. **Отдельный go.mod на каждый сервис** — независимые зависимости и версии. `shared/` — отдельный Go-модуль (`module ChatSem/shared`), импортируется через `replace` в `go.work` или прямым path.

2. **Единая база данных, отдельные схемы логически** — все сервисы работают с одним PostgreSQL, но каждый сервис трогает только свои таблицы. Нет cross-service JOIN в runtime.

3. **Migrations в корне, не в сервисах** — `migrations/` в корне репозитория применяются единожды и создают всю схему.

4. **`shared/` — только pure Go** — никаких pgx, Redis, chi в `shared/`. Только domain-типы, интерфейсы и stateless утилиты.

5. **Конфиг через env** — каждый сервис читает свой `.env` файл / env переменные. Секреты не шарятся через код.

## Code Examples

### Структура go.work (workspace)

```
// go.work
go 1.24

use (
    ./services/chat
    ./services/auth
    ./services/admin
    ./shared
)
```

### Импорт shared из сервиса

```go
// services/chat/internal/service/chat_service.go
package service

import (
    "ChatSem/shared/domain"   // shared domain types
    "ChatSem/shared/pkg/jwt"  // JWT validation
)
```

### Точка входа сервиса (cmd/main.go)

```go
// services/chat/cmd/main.go
package main

import (
    "log/slog"
    "net/http"
    "os"
    "ChatSem/services/chat/internal/broker"   // Redis-backed реализация Broker
    "ChatSem/services/chat/internal/config"
    "ChatSem/services/chat/internal/handler"
    "ChatSem/services/chat/internal/repository/postgres"
    "ChatSem/services/chat/internal/service"
    // shared/pkg/longpoll не импортируется напрямую в main —
    // handler и service принимают longpoll.Broker как интерфейс
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    cfg := config.Load()

    db := postgres.Connect(cfg.DatabaseURL)
    rdb := redis.NewClient(cfg.RedisAddr)
    b := broker.NewRedisBroker(rdb) // implements longpoll.Broker

    chatRepo := postgres.NewChatRepo(db)
    msgRepo  := postgres.NewMessageRepo(db)

    chatSvc := service.NewChatService(chatRepo, logger)
    msgSvc  := service.NewMessageService(msgRepo, chatRepo, b, logger)

    r := handler.NewRouter(chatSvc, msgSvc, cfg, logger)

    logger.Info("chat service starting", "addr", cfg.Addr)
    http.ListenAndServe(cfg.Addr, r)
}
```

### Domain entity в shared/

```go
// shared/domain/chat.go
package domain

import "time"

type Chat struct {
    ID        uuid.UUID
    EventID   uuid.UUID
    ParentID  *uuid.UUID
    Type      ChatType
    Settings  ChatSettings
    CreatedAt time.Time
}

// Settings child-чата не хранятся в БД.
// Репозиторий всегда возвращает Chat.Settings, заполненные из parent через SQL JOIN.
// Для parent-чата Chat.Settings содержит актуальные настройки напрямую.
// domain-слой не вычисляет наследование — это ответственность repository.
```

## Rate Limiting

### Уровни защиты от флуда

```
Входящий запрос POST /messages
        │
        ▼
[1] Per-IP middleware          ← защита от анонимного флуда до аутентификации
        │ pass
        ▼
[2] JWT validation middleware  ← аутентификация
        │ pass
        ▼
[3] Ban check middleware       ← Redis EXISTS ban:{eventID}:{userID}
        │ pass
        ▼
[4] Per-user per-chat rate limit ← основная защита от флуда (Redis sliding window)
        │ pass
        ▼
    message_service.Send()
```

### Алгоритм: Redis sliding window (per-user per-chat)

Скользящее окно точнее фиксированного (INCR+EXPIRE): не сбрасывается в начале периода.

Реализация живёт в `services/chat/internal/middleware/ratelimit.go` — не в `shared/` (Redis-зависимость).

```go
// services/chat/internal/middleware/ratelimit.go

const (
    MessageRateLimit  = 10              // сообщений
    MessageRateWindow = 10 * time.Second // за 10 секунд
)

// allow возвращает true, если запрос разрешён.
// key — уникальный идентификатор субъекта (user+chat или IP).
func allow(ctx context.Context, rdb *redis.Client, key string, limit int, window time.Duration) (bool, error) {
    now := time.Now().UnixMilli()
    windowStart := now - window.Milliseconds()

    pipe := rdb.Pipeline()
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
    pipe.ZCard(ctx, key)
    pipe.Expire(ctx, key, window)
    results, err := pipe.Exec(ctx)
    if err != nil {
        return true, err  // fail open: при недоступности Redis не блокируем
    }
    count := results[2].(*redis.IntCmd).Val()
    return count <= int64(limit), nil
}
```

### Лимиты по уровням

| Уровень | Redis-ключ | Лимит | Окно | Ответ |
|---------|-----------|-------|------|-------|
| Per-IP (все endpoints) | `rl:ip:{ip}` | 100 req | 60s | 429 |
| Per-user per-chat (отправка) | `rl:msg:{eventID}:{chatID}:{userID}` | 10 msg | 10s | 429 |
| Per-user global (отправка) | `rl:msg:global:{userID}` | 30 msg | 10s | 429 |
| Per-IP poll (реконнекты) | `rl:poll:{ip}` | 60 req | 60s | 429 |

**Исключения по роли** (из JWT claims):
- `role=moderator` — лимит отправки ×3
- `role=admin` — rate limiting отключён

### Middleware в chat-service

```go
// services/chat/internal/middleware/ratelimit.go

func MessageRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := ClaimsFromCtx(r.Context())

            // Администраторы и модераторы — без лимита / повышенный лимит
            limit := 10
            if claims.Role == "moderator" { limit = 30 }
            if claims.Role == "admin" {
                next.ServeHTTP(w, r)
                return
            }

            chatID := chi.URLParam(r, "chatID")
            key := fmt.Sprintf("rl:msg:%d:%s:%d", claims.EventID, chatID, claims.UserID)

            ok, err := ratelimit.Allow(r.Context(), rdb, key, limit, 10*time.Second)
            if err != nil {
                slog.Warn("rate limit redis error, failing open", "err", err)
            }
            if !ok {
                response.Error(w, 429, "rate_limit_exceeded",
                    "too many messages, slow down")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### Ответ клиенту при превышении лимита

```json
HTTP 429 Too Many Requests
Retry-After: 10

{
  "error": "too many messages, slow down",
  "code": "rate_limit_exceeded"
}
```

Виджет при получении 429 показывает сообщение пользователю и блокирует поле ввода на `Retry-After` секунд.

### Правила реализации Rate Limiting

- Алгоритм (Redis sliding window, ~15 строк) дублируется в каждом сервисе в `internal/middleware/ratelimit.go` — нет `shared/pkg/ratelimit` (Redis-зависимость нарушала бы правило 4: shared/ — pure Go)
- chat-service: flood-защита сообщений; admin-service: лимит экспорта истории (2 req/min)
- **Fail open** — при недоступности Redis лимит не применяется (чат важнее защиты от редкого сбоя)
- **Заголовок `Retry-After`** — обязателен в ответе 429; значение = длина окна в секундах
- Per-IP лимит применяется в `middleware.RealIP` + отдельном middleware **до** JWT-валидации
- Ключи rate limit не пересекаются с ключами банов: префикс `rl:*` vs `ban:*`
- `ZRemRangeByScore` в pipeline очищает устаревшие записи — ключи не растут бесконечно

## Error Recovery

### Матрица отказов и поведение системы

| Компонент | Сценарий | Поведение | Восстановление |
|-----------|----------|-----------|----------------|
| Redis | Недоступен при PUBLISH | Сообщение сохранено в PG, не доставлено real-time | Клиент получит при следующем poll по seq |
| Redis | Недоступен при Subscribe | Long poll сразу возвращает 200 + последние сообщения из DB | Клиент переподключается, пробует снова |
| Redis | Недоступен при ban-check | **Fail closed** — запрос блокируется, 503 | Retry на клиенте |
| Redis | Недоступен при rate limit | **Fail open** — лимит не применяется | Автовосстановление при возврате Redis |
| PostgreSQL | Недоступен при INSERT | 503 Service Unavailable | Клиент показывает ошибку, пользователь повторяет |
| PostgreSQL | Пул соединений исчерпан | pgxpool ждёт `MaxConnWaitDuration` (5с), затем 503 | Автовосстановление при освобождении соединений |
| PgBouncer | Недоступен | Сервис не может подключиться к DB → 503 | Health check → nginx убирает инстанс из ротации |
| chat-service | Краш/рестарт | Открытые long-poll соединения рвутся | Клиент reconnects с `after=lastSeq`, сообщения не теряются |
| Broker goroutine | Паника | Горутина-читатель Redis падает | recover() в goroutine, перезапуск с backoff |

### Fail closed vs fail open

```
ban-check    → FAIL CLOSED (безопасность важнее доступности)
rate-limit   → FAIL OPEN   (доступность важнее защиты от редкого сбоя)
seq-insert   → НЕТ fallback (транзакция откатывается, клиент получает 503)
```

### Startup: retry с exponential backoff

Сервис не должен падать при временной недоступности Redis или PostgreSQL при старте.

```go
// shared/pkg/retry/retry.go

func Connect(ctx context.Context, name string, fn func() error) error {
    backoff := 500 * time.Millisecond
    for attempt := 1; ; attempt++ {
        if err := fn(); err == nil {
            return nil
        }
        if attempt >= 10 {
            return fmt.Errorf("%s: failed after %d attempts", name, attempt)
        }
        slog.Warn("connection failed, retrying",
            "component", name, "attempt", attempt, "next_in", backoff)
        select {
        case <-time.After(backoff):
        case <-ctx.Done():
            return ctx.Err()
        }
        if backoff < 30*time.Second {
            backoff *= 2
        }
    }
}
```

```go
// services/chat/cmd/main.go
retry.Connect(ctx, "postgres", func() error { return db.Ping(ctx) })
retry.Connect(ctx, "redis",    func() error { return rdb.Ping(ctx).Err() })
```

### Broker: panic recovery в горутине-читателе

```go
func (b *Broker) startReader(ctx context.Context, chatID uuid.UUID, f *fanout) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("broker reader panic, restarting",
                "chat_id", chatID, "panic", r)
            // перезапуск с задержкой, если контекст ещё активен
            time.Sleep(time.Second)
            if ctx.Err() == nil {
                go b.startReader(ctx, chatID, f)
            }
        }
    }()
    // ... основная логика ...
}
```

### Context propagation: соблюдать везде

Все операции с Redis и PostgreSQL получают контекст запроса. При обрыве клиентского соединения контекст отменяется → операции прерываются, ресурсы освобождаются.

```go
// handler: контекст запроса передаётся вниз
func (h *PollHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // отменяется при обрыве соединения с клиентом

    ch := h.broker.Subscribe(chatID)
    defer h.broker.Unsubscribe(chatID, ch) // всегда освобождаем

    select {
    case <-ch:
        time.Sleep(LongPollSettleDelay)
    case <-time.After(LongPollTimeout):
    case <-ctx.Done(): // клиент отключился → выходим немедленно
        return
    }
    // ...
}
```

### Health Check: реальное состояние зависимостей

`GET /health` должен проверять реальную доступность, не просто `200 OK`.

```go
// services/*/internal/handler/health.go

type HealthHandler struct {
    db  *pgxpool.Pool
    rdb *redis.Client
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    status := map[string]string{"postgres": "ok", "redis": "ok"}
    code := 200

    if err := h.db.Ping(ctx); err != nil {
        status["postgres"] = "unavailable"
        code = 503
        slog.Error("health: postgres unavailable", "err", err)
    }
    if err := h.rdb.Ping(ctx).Err(); err != nil {
        status["redis"] = "unavailable"
        code = 503
        slog.Error("health: redis unavailable", "err", err)
    }

    response.JSON(w, code, status)
}
```

nginx убирает инстанс из upstream при 503 на `/health`.

### Клиентская сторона (React Widget): стратегия reconnect

```
POST /messages → 503/timeout → показать "не удалось отправить", кнопка "Повторить"
GET  /poll     → любая ошибка → reconnect с exponential backoff (1s, 2s, 4s, max 30s)
GET  /poll     → 401          → показать форму авторизации (токен истёк)
POST /messages → 429          → заблокировать ввод на Retry-After секунд
```

### Правила реализации Error Recovery

- `shared/pkg/retry` — общий пакет для startup-ретраев
- Все HTTP-хендлеры оборачиваются `middleware.Recoverer` (chi) — паника не роняет сервис
- Broker goroutine имеет свой `recover()` + перезапуск
- `context.WithTimeout(2s)` на health-check запросы — не висим вечно
- Логировать все ошибки с уровнем ERROR и контекстом (`chat_id`, `user_id`, `err`)
- 503 возвращается клиенту при недоступности PG; Redis-ошибки — по таблице выше

## History Export

### Контекст и проблема

Экспорт истории чата — тяжёлый запрос. Синхронный SELECT всего в память не подходит — OOM и таймаут nginx.
**Ограничение: максимум 3 дня за запрос** — делает объём предсказуемым и позволяет использовать простой streaming без async jobs.

### Решение: streaming HTTP response с курсором pgx

**Endpoint:** `GET /api/admin/chats/{id}/export?format=csv&from=<RFC3339>&to=<RFC3339>`

Сервис читает строки через `pgx.Rows` и пишет в `http.ResponseWriter` сразу — данные никогда не накапливаются в памяти.

```go
// services/admin/internal/handler/export.go

func (h *ExportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    chatID := chi.URLParam(r, "id")
    from, to, err := parseTimeRange(r) // валидирует: to-from <= 72h
    if err != nil {
        response.Error(w, 400, "invalid_range", err.Error())
        return
    }

    w.Header().Set("Content-Type", "text/csv")
    w.Header().Set("Content-Disposition", `attachment; filename="chat-`+chatID+`.csv"`)
    // chunked transfer: Content-Length не указываем

    rows, err := h.repo.StreamMessages(r.Context(), chatID, from, to)
    if err != nil {
        response.Error(w, 500, "db_error", "failed to start export")
        return
    }
    defer rows.Close()

    enc := csv.NewWriter(w)
    enc.Write([]string{"id", "user_id", "text", "seq", "created_at"})
    for rows.Next() {
        var m domain.Message
        rows.Scan(&m.ID, &m.UserID, &m.Text, &m.Seq, &m.CreatedAt)
        enc.Write([]string{...})
        enc.Flush() // сбрасываем каждую строку — не буферизуем
    }
}
```

**Репозиторий возвращает курсор, не slice:**

```go
// services/admin/internal/repository/postgres/message_repo.go

func (r *MessageRepo) StreamMessages(ctx context.Context, chatID uuid.UUID, from, to time.Time) (pgx.Rows, error) {
    return r.db.Query(ctx,
        `SELECT id, user_id, text, seq, created_at FROM messages
         WHERE chat_id = $1 AND created_at BETWEEN $2 AND $3
           AND deleted_at IS NULL
         ORDER BY seq ASC`,
        chatID, from, to,
    )
    // pgx.Rows — lazy cursor, строки читаются по одной
}
```

### Ограничения

| Параметр | Значение | Причина |
|----------|----------|---------|
| Максимальный диапазон | **3 дня (72ч)** | Предсказуемый объём, не нужен async job |
| Дефолтный диапазон | Последние 24ч | Если `from`/`to` не указаны |
| nginx timeout | `proxy_read_timeout 120s` | Стриминг медленнее обычного запроса |
| Rate limit экспорта | 2 req/min на user | Защита от параллельных тяжёлых запросов |

Если нужно больше 3 дней — несколько запросов с разными диапазонами.

### Авторизация

Экспорт живёт только в admin-service:

```
role=admin      → экспорт любого чата в любом event
role=moderator  → только чаты своего event (claims.EventID == chat.EventID)
role=user       → 403 Forbidden
```

### Форматы ответа

**CSV** (`format=csv`, дефолт):
```
id,user_id,text,seq,created_at
1,42,"Hello",1,2026-03-01T10:00:00Z
```

**JSON** (`format=json`) — JSON Lines (одна строка = один объект), не массив:
```
{"id":1,"user_id":42,"text":"Hello","seq":1,"created_at":"2026-03-01T10:00:00Z"}
{"id":2,"user_id":43,"text":"Hi","seq":2,"created_at":"2026-03-01T10:00:05Z"}
```

JSON Lines стримится построчно — весь JSON-документ не нужно держать в памяти.

### Почему не async job

При лимите 3 дня объём предсказуем (< 500k сообщений в активном чате).
Стриминг справляется синхронно — async job добавляет сложность без выгоды.
Пересмотреть если: появится требование экспорта за месяц+ или отправки архива на email.

## Migration Strategy

### Контекст: общая БД, независимый деплой

Все три сервиса (`chat`, `auth`, `admin`) работают с одной PostgreSQL. При независимом деплое возможна ситуация, когда старая версия сервиса работает одновременно с новой схемой БД — или наоборот.

```
Деплой v2:
  t=0  Запускаем миграции (схема v2)
  t=1  chat-svc v1 (старый код) + схема v2   ← переходный период
  t=2  chat-svc v2 деплоится
  t=3  Все инстансы chat-svc v1 завершены
```

Схема должна быть совместима с обеими версиями в t=1.

### Инструмент: goose

```
migrations/
├── 001_create_events.sql
├── 002_create_chats.sql
├── 003_create_messages.sql
├── 004_create_users.sql
├── 005_create_bans.sql
├── 006_create_chat_seqs.sql
└── ...

# Запуск:
goose -dir migrations postgres "$DATABASE_URL" up
```

Файлы именуются с префиксом порядкового номера. Один набор миграций на весь монорепо — не per-service.

### Кто запускает миграции

**Отдельный job, не сервис при старте.** Причины:
- При N инстансах каждый попытается накатить миграцию → race condition
- Сервис не должен стартовать с правами ALTER TABLE в продакшне
- Миграции должны пройти до деплоя нового кода

```yaml
# deploy/docker-compose.yml
services:
  migrate:
    image: chatsem/migrate
    command: goose -dir /migrations postgres "${DATABASE_URL}" up
    depends_on:
      postgres: {condition: service_healthy}
    restart: "no"   # выполнить один раз и завершить

  chat:
    depends_on:
      migrate: {condition: service_completed_successfully}
```

В CI/CD pipeline:
```
1. docker run migrate   ← применить миграции
2. deploy chat-svc      ← только после успешных миграций
3. deploy auth-svc
4. deploy admin-svc
```

### Паттерн Expand / Contract (обязательный)

Единственный безопасный способ изменять схему при независимом деплое сервисов.

**Фаза 1 — EXPAND** (обратно совместимое изменение):
- Добавить новую колонку / таблицу
- Старый код её не трогает, не ломается
- Задеплоить миграцию и новый код

**Фаза 2 — CONTRACT** (удаление старого):
- Удалить старую колонку / таблицу
- Только после того, как все инстансы перешли на новый код
- Отдельная миграция, отдельный деплой

```
❌ Одна миграция:  DROP COLUMN old_name + код без old_name
✅ Три шага:
   Step 1 (migrate): ADD COLUMN new_name
   Step 2 (deploy):  код читает new_name, пишет в оба
   Step 3 (migrate): DROP COLUMN old_name  ← через неделю, когда v1 точно убран
```

### Правила безопасных миграций

**Можно в любой момент (не ломают старый код):**
```sql
-- Новая таблица
CREATE TABLE IF NOT EXISTS new_table (...);

-- Новая nullable колонка
ALTER TABLE chats ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

-- Новая колонка с DEFAULT
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMPTZ DEFAULT NULL;

-- Новый индекс (CONCURRENTLY — не блокирует таблицу)
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_messages_chat_seq
    ON messages (chat_id, seq);
```

**Нельзя без Expand/Contract (сломают старый код):**
```sql
-- ❌ DROP COLUMN — старый код упадёт с "column not found"
ALTER TABLE chats DROP COLUMN settings;

-- ❌ NOT NULL без DEFAULT — INSERT от старого кода упадёт
ALTER TABLE messages ADD COLUMN priority INT NOT NULL;

-- ❌ Переименование — старый код ищет старое имя
ALTER TABLE chats RENAME COLUMN type TO chat_type;

-- ❌ Изменение типа — несовместимо
ALTER TABLE messages ALTER COLUMN seq TYPE BIGINT;
-- ✅ Вместо этого: ADD new_seq BIGINT, backfill, DROP seq, RENAME
```

### Rollback-стратегия

goose поддерживает `down`-миграции. Каждый `.sql` файл содержит оба направления:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS bans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id   UUID NOT NULL,
    user_id    UUID NOT NULL,
    banned_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    banned_by  UUID NOT NULL
);
CREATE INDEX idx_bans_lookup ON bans (event_id, user_id);

-- +goose Down
DROP TABLE IF EXISTS bans;
```

Rollback только до предыдущей версии: `goose down-to <version>`. Сначала откатить код, потом схему.

### Нумерация миграций по сервисам

Используем диапазоны для читаемости (не обязательно, но удобно):

```
001–099   shared (events, chats, chat_seqs)
100–199   messages, history
200–299   users, auth, sessions
300–399   bans, moderation
400–499   admin, settings
```

### Правила реализации

- `migrations/` в корне репо — один источник истины для всей схемы
- Каждая миграция идемпотентна: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`
- `CREATE INDEX` только через `CONCURRENTLY` — не блокирует таблицу при большом объёме
- Миграции **никогда** не запускаются внутри `cmd/main.go` сервиса
- `goose` хранит версию в таблице `goose_db_version` — не удалять вручную
- Перед `DROP` в CONTRACT-фазе: убедиться в логах, что старый код не обращается к колонке

## Anti-Patterns

- ❌ **Импортировать внутренние пакеты другого сервиса** — только `shared/`, никогда `services/auth/internal/...`
- ❌ **Cross-service JOIN в SQL** — не писать SQL, который объединяет таблицы двух сервисов в runtime-запросе
- ❌ **pgx или Redis в `shared/domain`** — `shared/domain` остаётся чистым Go
- ❌ **Дублировать domain-типы** в каждом сервисе — базовые типы живут в `shared/domain`
- ❌ **Глобальные переменные** — вся инициализация через конструкторы в `cmd/main.go`
- ❌ **HTTP-вызов к auth-service для валидации JWT** — валидация всегда локальная через `shared/pkg/jwt`; auth-service только выдаёт токены, не валидирует чужие запросы
- ❌ **Пропускать забаненного пользователя только по валидному JWT** — после проверки подписи обязательно `EXISTS ban:{eventID}:{userID}` в Redis
- ❌ **Писать/удалять ключи `ban:*` из chat-service** — только admin-service управляет банами; chat-service только читает
- ❌ **Подключать сервисы напрямую к PostgreSQL** — все Go-сервисы подключаются к PgBouncer (:5432), не к PostgreSQL (:5433) напрямую
- ❌ **Использовать prepared statements с PgBouncer в transaction mode** — pgx нужно настроить с `default_query_exec_mode=simple_protocol` при подключении через PgBouncer