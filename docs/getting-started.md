[Back to README](../README.md) · [Архитектура →](architecture.md)

# Начало работы

## Требования

| Инструмент | Версия |
|------------|--------|
| Docker + Docker Compose | 24+ |
| Go | 1.24+ (для локальной разработки) |
| Node.js | 20+ (для локальной разработки фронтенда) |
| [goose](https://github.com/pressly/goose) | любая (для миграций) |

## Установка

### 1. Подготовь `userlist.txt` для PgBouncer

PgBouncer требует MD5-хэш пароля в формате `md5(password + username)`:

```bash
echo -n "change-mechatsem" | md5sum
# Например: a1b2c3d4e5f6...
```

Открой `deploy/pgbouncer/userlist.txt` и замени `md5REPLACEME` на полученный хэш:

```
"chatsem" "md5a1b2c3d4e5f6..."
```

> Если меняешь пароль в `.env`, генерируй хэш заново.

### 2. Создай `.env`

```bash
cp deploy/.env.example deploy/.env
```

Отредактируй `deploy/.env` — поменяй `POSTGRES_PASSWORD` и `JWT_SECRET`:

```env
POSTGRES_PASSWORD=your-strong-password
JWT_SECRET=your-32-char-secret-key
```

### 3. Собери и подними сервисы

```bash
make docker-build   # сборка всех Docker-образов (Go + frontend)
make docker-up      # запуск всех сервисов в фоне
```

Проверь, что всё поднялось:

```bash
make docker-logs    # смотреть логи в реальном времени (Ctrl+C для выхода)
```

### 4. Примени миграции

```bash
export DATABASE_URL=postgres://chatsem:your-password@localhost:5433/chatsem?sslmode=disable
make migrate-up
```

Замени `your-password` на значение `POSTGRES_PASSWORD` из `deploy/.env`.

### 5. Проверь работу

```bash
curl http://localhost/health
```

Ожидаемый ответ: `{"service":"...","status":"ok"}`

## Адреса после запуска

| Сервис       | URL                                |
|--------------|------------------------------------|
| API (nginx)  | http://localhost                   |
| Chat-сервис  | http://localhost:8080 (напрямую)   |
| Auth-сервис  | http://localhost:8081 (напрямую)   |
| Admin-сервис | http://localhost:8082 (напрямую)   |
| Виджет       | http://localhost/widget/           |
| Админ-панель | http://localhost/admin/            |

## Полезные команды

```bash
make docker-logs          # логи всех сервисов
make docker-down          # остановить сервисы
make docker-clean         # остановить + удалить volumes (сброс БД)
make migrate-status       # статус миграций
make migrate-down         # откатить последнюю миграцию
make docker-scale-chat    # запустить 2 инстанса chat-сервиса
```

## Локальная сборка Go-сервисов

```bash
make build-chat    # собрать → bin/chat
make build-auth    # собрать → bin/auth
make build-admin   # собрать → bin/admin
```

## See Also

- [Конфигурация](configuration.md) — переменные окружения и их значения по умолчанию
- [Архитектура](architecture.md) — как устроен проект изнутри
