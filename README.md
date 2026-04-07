# ChatSem

> Встраиваемый чат-сервис для онлайн-мероприятий.

ChatSem — бэкенд на Go + React-виджет, который подключается к сайту одной строкой кода.
Поддерживает иерархические чат-комнаты, SSO-аутентификацию, long polling и экспорт истории.

## Быстрый старт

```bash
cp deploy/.env.example deploy/.env   # настройте секреты
make docker-build                    # сборка образов
make docker-up                       # запуск сервисов
export DATABASE_URL=postgres://chatsem:change-me@localhost:5433/chatsem?sslmode=disable
make migrate-up                      # применить миграции
```

Сервисы будут доступны по адресам:

| Сервис       | URL                        |
|--------------|----------------------------|
| API (nginx)  | http://localhost           |
| Виджет       | http://localhost/widget/   |
| Админ-панель | http://localhost/admin/    |

## Встраивание виджета

```html
<script src="https://your-host/widget/chat.iife.js"></script>
<script>
  ChatSem.init({ eventId: "42", token: "sso-jwt-token" });
</script>
```

## Ключевые возможности

- **Иерархические чаты** — родительский чат события + дочерние комнаты/залы
- **Встраиваемый виджет** — React-бандл, подключается через `<script>` без iframe
- **SSO-аутентификация** — валидация JWT от сайта-хоста, внешний user ID
- **Long polling** — сообщения в реальном времени через Redis pub/sub
- **Модерация** — бан/мут пользователей, удаление сообщений, роли admin/moderator
- **Экспорт истории** — выгрузка сообщений чата в CSV или JSON

---

## Документация

| Раздел | Описание |
|--------|----------|
| [Начало работы](docs/getting-started.md) | Установка, настройка, первый запуск |
| [Архитектура](docs/architecture.md) | Структура сервисов, паттерны, data flow |
| [API](docs/api.md) | Справочник по эндпоинтам (chat, auth, admin) |
| [Конфигурация](docs/configuration.md) | Переменные окружения, Docker Compose |

## Лицензия

MIT
