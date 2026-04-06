# Admin Login/Password Authentication

**Branch:** `feature/admin-auth`
**Created:** 2026-04-06

## Settings
- Testing: yes
- Logging: verbose
- Docs: no

## Roadmap Linkage
- Milestone: none
- Rationale: все milestones уже выполнены; фича — базовая безопасность Admin панели

## Context

Сейчас авторизация в Admin панели работает через eventID + API Secret:
- Пользователь вводит Event ID, API Secret события и своё имя
- Frontend делает POST `/api/auth/token` (сервис auth) с api_secret как Bearer
- Получает JWT с role=admin, связанный с конкретным event

Это неудобно и концептуально неверно: admin должен управлять ВСЕМИ событиями,
а не быть привязан к одному.

**Цель:** реализовать логин по логину/паролю для единственного администратора.
Учётные данные задаются через env vars (ADMIN_USERNAME, ADMIN_PASSWORD).
Admin сервис сам выдаёт JWT при успешной аутентификации.

**Дизайн:**
- Новый endpoint: `POST /api/admin/auth/login` → `{ username, password }` → `{ token }`
- Пароль хранится в env (plaintext), хэшируется bcrypt при старте сервиса
- JWT: role=admin, event_id=uuid.Nil, user_id=uuid.Nil, TTL=8h
- Middleware /api/admin/** без изменений — уже проверяет Bearer JWT

## Tasks

### Phase 1 — Backend

- [x] Task #1: Обновить Config
  - `services/admin/internal/config/config.go`
  - Добавить поля `AdminUsername string` (env: ADMIN_USERNAME, default: admin)
  - Добавить `AdminPassword string` (env: ADMIN_PASSWORD, default: changeme)
  - Добавить `JWTMaxTTL time.Duration` (env: JWT_MAX_TTL, default: 8h)
  - Логировать: `slog.Info("config loaded", "admin_username", cfg.AdminUsername, "addr", cfg.Addr)`

- [x] Task #2: Создать AuthHandler
  - Новый файл: `services/admin/internal/handler/auth_handler.go`
  - Структура `AuthHandler` хранит `username string`, `passwordHash []byte`, `jwtSecret string`, `jwtTTL time.Duration`
  - Конструктор `NewAuthHandler(username, password, jwtSecret string, ttl time.Duration) (*AuthHandler, error)`:
    - bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    - slog.Debug("[AuthHandler] password hash generated")
  - Метод `Login(w, r)` — POST `/api/admin/auth/login`:
    - Декодирует `{ username, password }` из JSON body
    - slog.Debug("[AuthHandler] login attempt", "username", req.Username)
    - Сравнивает username (constant-time), пароль через bcrypt.CompareHashAndPassword
    - При неуспехе: slog.Warn("[AuthHandler] login failed", "username"), 401
    - При успехе: jwt.CreateToken с Claims{UserID:uuid.Nil, EventID:uuid.Nil, Role:"admin", Name:username}, ttl
    - slog.Info("[AuthHandler] login success", "username")
    - Возвращает `{ "token": "..." }`

- [x] Task #3: Зарегистрировать маршрут в Router
  - `services/admin/internal/handler/router.go`
  - NewRouter принимает дополнительный аргумент `authH *AuthHandler`
  - Добавить публичный маршрут (вне auth-группы): `r.Post("/api/admin/auth/login", authH.Login)`
  - Логировать в router setup: slog.Debug("[Router] auth handler registered")

- [x] Task #4: Обновить main.go и .env
  - `services/admin/cmd/main.go` — создать AuthHandler, передать в NewRouter
  - `deploy/.env` — добавить `ADMIN_USERNAME=admin`, `ADMIN_PASSWORD=changeme`

### Phase 2 — Frontend

- [x] Task #5: Обновить AuthContext
  - `frontend/admin/src/context/AuthContext.tsx`
  - Изменить сигнатуру `login(username: string, password: string): Promise<void>`
  - POST `/api/admin/auth/login` с `{ username, password }`
  - Убрать eventId из state и sessionStorage (admin не привязан к событию)
  - Обновить `AuthContextValue` интерфейс
  - console.debug('[AuthContext] login attempt', username) в DEV

- [x] Task #6: Обновить LoginPage
  - `frontend/admin/src/pages/LoginPage.tsx`
  - Убрать поля EventID, API Secret, Name
  - Добавить поля Username и Password
  - Вызов: `login(username, password)`

### Phase 3 — Tests

- [x] Task #7: Обновить тесты LoginPage
  - `frontend/admin/src/pages/LoginPage.test.tsx`
  - Обновить renderLogin: убрать eventId из value контекста
  - Обновить тесты: вместо EventID/APISecret/Name → Username/Password
  - `expect(login).toHaveBeenCalledWith('admin', 'password123')`

- [x] Task #8: Integration тест для AuthHandler
  - `services/admin/internal/handler/auth_handler_test.go`
  - TestAdminLogin_Success: правильный логин → 200 + token в ответе
  - TestAdminLogin_WrongPassword: неверный пароль → 401
  - TestAdminLogin_WrongUsername: неверный username → 401
  - TestAdminLogin_MissingBody: пустой body → 400
  - Использовать httptest.NewRecorder

## Commit Plan

**Checkpoint 1** (после Task #4) — backend готов:
```
feat(admin): add username/password login endpoint
```

**Checkpoint 2** (после Task #8) — фича полностью готова:
```
feat(admin/frontend): replace event+secret login with username/password
```
