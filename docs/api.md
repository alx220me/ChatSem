[← Архитектура](architecture.md) · [Back to README](../README.md) · [Конфигурация →](configuration.md)

# API Reference

Все сервисы доступны через nginx-прокси на `http://localhost`.
Прямые порты: chat `:8080`, auth `:8081`, admin `:8082`.

## Формат ответа

**Успех:** зависит от эндпоинта (JSON-объект или массив).

**Ошибка:**
```json
{ "error": "описание ошибки", "code": "ERROR_CODE" }
```

## Аутентификация

Большинство chat- и все admin-эндпоинты требуют заголовок:
```
Authorization: Bearer <jwt-token>
```

JWT получается через `POST /api/auth/token`.

---

## Auth Service

### POST /api/auth/token

Обменять внешний SSO-токен на внутренний JWT ChatSem.

**Request:**
```json
{ "external_token": "jwt-от-сайта-хоста" }
```

**Response `200`:**
```json
{ "token": "eyJ..." }
```

---

## Chat Service

### GET /health
Проверка доступности сервиса. Не требует авторизации.

**Response:** `{"status":"ok","service":"chat"}`

---

### GET /api/chat/events/{eventID}/chats
Список чатов события. Публичный (без авторизации).

**Response:**
```json
[
  { "id": "1", "name": "Главный зал", "parent_id": null },
  { "id": "2", "name": "Зал А", "parent_id": "1" }
]
```

---

### POST /api/chat/join  🔒
Вступить в чат-комнату.

**Request:**
```json
{ "event_id": "42", "room_name": "Зал А" }
```

**Response `200`:**
```json
{ "chat_id": "7" }
```

---

### GET /api/chat/chats/{chatID}  🔒
Получить информацию о чате.

---

### GET /api/chat/{chatID}/messages  🔒
История сообщений чата.

**Query params:**

| Параметр | Тип | Описание |
|----------|-----|----------|
| `before` | int64 | ID сообщения (пагинация вниз) |
| `limit` | int | Количество (по умолчанию 50) |

**Response:**
```json
[
  {
    "id": 123,
    "chat_id": "7",
    "user_id": "ext-user-42",
    "text": "Привет!",
    "created_at": "2026-04-07T10:00:00Z",
    "reply_to_id": null
  }
]
```

---

### POST /api/chat/{chatID}/messages  🔒
Отправить сообщение. Применяется rate limit.

**Request:**
```json
{ "text": "Привет!", "reply_to_id": 120 }
```

---

### DELETE /api/chat/messages/{msgID}  🔒
Удалить сообщение (роль: moderator или owner).

---

### PATCH /api/chat/messages/{msgID}  🔒
Редактировать своё сообщение.

**Request:**
```json
{ "text": "Исправленный текст" }
```

---

### GET /api/chat/{chatID}/poll  🔒
Long polling — ожидать новые сообщения (до 30 секунд).

**Query params:**

| Параметр | Тип | Описание |
|----------|-----|----------|
| `last_id` | int64 | ID последнего известного сообщения |

**Response `200`:** массив новых сообщений.
**Response `204`:** нет новых сообщений (timeout).

---

### POST /api/chat/{chatID}/heartbeat  🔒
Обновить метку присутствия (online-счётчик).

### DELETE /api/chat/{chatID}/heartbeat  🔒
Покинуть чат (убрать из online).

### GET /api/chat/{chatID}/online  🔒
Количество онлайн-пользователей.

**Response:** `{"count": 42}`

---

## Admin Service

Все эндпоинты (кроме `/api/admin/auth/login`) требуют роль `admin` или `moderator`.

### POST /api/admin/auth/login
Логин администратора/модератора.

**Request:**
```json
{ "username": "admin", "password": "secret" }
```

**Response `200`:**
```json
{ "token": "eyJ..." }
```

---

### POST /api/admin/events  🔒 `admin`
Создать событие.

### GET /api/admin/events  🔒 `admin|moderator`
Список событий.

### POST /api/admin/events/{eventID}/chat  🔒 `admin`
Создать родительский чат события.

### GET /api/admin/events/{eventID}/chats  🔒 `admin|moderator`
Список чатов события.

### PATCH /api/admin/chats/{chatID}/settings  🔒 `admin`
Обновить настройки чата.

---

### GET /api/admin/events/{eventID}/users  🔒 `admin|moderator`
Список пользователей события.

### PATCH /api/admin/users/{userID}/role  🔒 `admin`
Изменить роль пользователя.

**Request:** `{ "role": "moderator" }`

---

### POST /api/admin/bans  🔒 `admin|moderator`
Забанить пользователя.

**Request:**
```json
{ "event_id": "42", "user_id": "ext-user-5", "reason": "spam" }
```

### DELETE /api/admin/bans/{banID}  🔒 `admin|moderator`
Снять бан.

### GET /api/admin/events/{eventID}/bans  🔒 `admin|moderator`
Список банов события.

---

### POST /api/admin/mutes  🔒 `admin|moderator`
Заглушить пользователя в чате.

### DELETE /api/admin/mutes/{muteID}  🔒 `admin|moderator`
Снять мут.

### GET /api/admin/chats/{chatID}/mutes  🔒 `admin|moderator`
Список мутов чата.

---

### GET /api/admin/chats/{chatID}/export
Экспорт сообщений чата. Поддерживает `?token=` для скачивания из браузера.

**Query params:**

| Параметр | Тип | Описание |
|----------|-----|----------|
| `format` | string | `csv` или `json` |
| `token` | string | JWT (альтернатива заголовку Authorization) |

## See Also

- [Архитектура](architecture.md) — как устроены сервисы и маршрутизация
- [Конфигурация](configuration.md) — переменные окружения для JWT и URL сервисов
