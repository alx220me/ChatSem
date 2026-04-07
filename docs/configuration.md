[← API](api.md) · [Back to README](../README.md)

# Конфигурация

## Переменные окружения

Все переменные задаются в файле `deploy/.env` (скопируй из `deploy/.env.example`).

### База данных

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `DATABASE_URL` | `postgres://chatsem:change-me@postgres:5432/chatsem` | URL подключения к PostgreSQL (через PgBouncer) |
| `POSTGRES_DB` | `chatsem` | Имя базы данных |
| `POSTGRES_USER` | `chatsem` | Пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | `change-me` | **Замени в production** |

### Redis

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `REDIS_ADDR` | `redis:6379` | Адрес Redis (host:port) |

### JWT и безопасность

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `JWT_SECRET` | `change-me-in-production` | Ключ подписи JWT. **Замени в production** |
| `JWT_MAX_TTL` | `4h` | Максимальное время жизни токена |

### Адреса сервисов

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `CHAT_ADDR` | `:8080` | Bind-адрес chat-сервиса |
| `AUTH_ADDR` | `:8081` | Bind-адрес auth-сервиса |
| `ADMIN_ADDR` | `:8082` | Bind-адрес admin-сервиса |

## Docker Compose

Конфигурация: `deploy/docker-compose.yml`

Поднимает следующие контейнеры:

| Контейнер | Образ | Описание |
|-----------|-------|----------|
| `chat` | сборка из `deploy/services/chat/Dockerfile` | Chat-сервис :8080 |
| `auth` | сборка из `deploy/services/auth/Dockerfile` | Auth-сервис :8081 |
| `admin` | сборка из `deploy/services/admin/Dockerfile` | Admin-сервис :8082 |
| `widget` | сборка из `deploy/frontend/widget/Dockerfile` | React-виджет (static) |
| `admin-panel` | сборка из `deploy/frontend/admin/Dockerfile` | Админ-панель (static) |
| `postgres` | `postgres:16` | PostgreSQL |
| `pgbouncer` | PgBouncer | Connection pooling (transaction mode, pool_size=50) |
| `redis` | `redis:7` | Кэш и pub/sub |
| `nginx` | `nginx:alpine` | Reverse proxy, раздача статики |

## PgBouncer

Конфигурация: `deploy/pgbouncer/pgbouncer.ini`

PgBouncer работает в режиме **transaction pooling** и требует MD5-хэш пароля в `deploy/pgbouncer/userlist.txt`:

```bash
# Сгенерировать хэш (password + username, без разделителя):
echo -n "your-passwordchatsem" | md5sum
```

Формат записи в `userlist.txt`:
```
"chatsem" "md5<хэш>"
```

## nginx

Конфигурация: `deploy/nginx.conf`

Маршруты:

| URL-префикс | Проксируется к |
|-------------|----------------|
| `/api/chat/` | `chat:8080` |
| `/api/auth/` | `auth:8081` |
| `/api/admin/` | `admin:8082` |
| `/widget/` | статика виджета |
| `/admin/` | статика админ-панели |

## Переменные для миграций

При запуске миграций через `make migrate-*` необходимо задать `DATABASE_URL`
с адресом, доступным с локальной машины (порт PgBouncer экспортируется как `:5433`):

```bash
export DATABASE_URL=postgres://chatsem:your-password@localhost:5433/chatsem?sslmode=disable
make migrate-up
```

## See Also

- [Начало работы](getting-started.md) — пошаговая инструкция по запуску
- [Архитектура](architecture.md) — как сервисы используют эти настройки
