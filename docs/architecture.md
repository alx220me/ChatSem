[← Начало работы](getting-started.md) · [Back to README](../README.md) · [API →](api.md)

# Архитектура

## Обзор

ChatSem — **мультисервисный монорепозиторий**: 3 независимых Go-сервиса + 2 React-приложения.
Каждый сервис — отдельный HTTP-сервер со своей зоной ответственности.

```
                     ┌──────────────┐
  Браузер/виджет ───▶│    nginx     │─── /api/chat/* ──▶ chat-service  :8080
  Сайт-хост      ───▶│  (proxy)     │─── /api/auth/* ──▶ auth-service  :8081
  Админ-панель   ───▶│              │─── /api/admin/*──▶ admin-service :8082
                     └──────────────┘
                            │
              ┌─────────────┴─────────────┐
              ▼                           ▼
         PostgreSQL                     Redis
     (через PgBouncer)             (long polling,
                                    сессии, rate limit)
```

## Сервисы

| Сервис | Порт | Зона ответственности |
|--------|------|----------------------|
| **chat** | :8080 | Чаты, сообщения, long polling, online-счётчик |
| **auth** | :8081 | SSO-токен обмен, JWT, сессии |
| **admin** | :8082 | Управление событиями, модерация, экспорт |

## Структура репозитория

```
ChatSem/
├── go.work                         # Go workspace
├── services/
│   ├── chat/                       # Chat service :8080
│   │   ├── cmd/main.go             # Entry point, DI
│   │   └── internal/
│   │       ├── ports/              # Repository interfaces (at point of use)
│   │       ├── service/            # Business logic
│   │       ├── repository/postgres/# pgx implementations
│   │       ├── handler/            # chi HTTP handlers
│   │       └── middleware/         # JWT auth, rate limit, ban check
│   ├── auth/                       # Auth service :8081
│   └── admin/                      # Admin service :8082
├── shared/                         # Общий Go-код (без Redis/chi)
│   ├── domain/                     # Базовые типы: Chat, Message, User, Event
│   └── pkg/
│       ├── postgres/               # Shared pgxpool.Pool factory
│       ├── longpoll/               # Redis long polling broker
│       ├── jwt/                    # JWT helpers
│       └── response/               # Стандартный HTTP-ответ {"error":...}
├── frontend/
│   ├── widget/                     # Встраиваемый React-виджет (IIFE bundle)
│   └── admin/                      # Админ-панель (SPA)
├── migrations/                     # SQL-миграции (goose)
└── deploy/
    ├── docker-compose.yml
    ├── nginx.conf
    ├── pgbouncer/
    └── services/*/Dockerfile
```

## Внутренняя структура сервиса (Clean Architecture)

```
cmd/main.go          ← DI: wire всё вместе, запуск HTTP-сервера
internal/
  ports/             ← интерфейсы репозиториев (объявляются здесь, не в shared)
  service/           ← бизнес-логика (использует ports)
  repository/postgres/← реализация ports через pgx v5
  handler/           ← HTTP-обработчики (используют service)
  middleware/        ← JWT, rate limit, CORS, ban check
```

**Правило:** интерфейсы репозиториев живут в `internal/ports/` каждого сервиса,
не в `shared/`. `shared/domain/` содержит только сущности и ошибки.

## Data Flow: отправка сообщения

```
Виджет → POST /api/chat/{chatID}/messages
          ↓
     middleware: Auth → CORS → BanCheck → EventOwnership → MessageRateLimit
          ↓
     MessageHandler.Send()
          ↓
     MessageService.Send() → сохранить в PostgreSQL
          ↓
     Broker.Publish(chatID, message) → Redis pub/sub
          ↓
     Все клиенты с открытым GET /api/chat/{chatID}/poll получают ответ
```

## Data Flow: аутентификация

```
Виджет → POST /api/auth/token   { external_token: "host-site-jwt" }
          ↓
     auth-service: декодирует внешний JWT, создаёт внутренний JWT
          ↓
     Виджет сохраняет токен → использует в заголовке Authorization
```

## Ключевые паттерны

| Паттерн | Применение |
|---------|------------|
| Long polling | `GET /api/chat/{chatID}/poll` — клиент ждёт до 30с, Redis pub/sub уведомляет |
| Rate limiting | Per-IP через Redis (IP rate limit) + per-user (message rate limit) |
| PgBouncer | Transaction pooling перед PostgreSQL, pool_size=50 |
| Горизонтальное масштабирование | `make docker-scale-chat` — несколько инстансов chat через Redis pub/sub |
| Структурированные ошибки | `{"error": "...", "code": "..."}` — все сервисы |
| Логирование | `log/slog`, structured JSON |

## See Also

- [API](api.md) — все эндпоинты сервисов
- [Конфигурация](configuration.md) — переменные окружения
