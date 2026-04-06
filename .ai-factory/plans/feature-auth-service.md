# Plan: Auth Service

**Branch:** `feature/auth-service`
**Created:** 2026-04-02
**Milestone:** Auth Service

## Settings
- **Testing:** yes — unit тесты (mock repos) + интеграционные (TEST_DATABASE_URL)
- **Logging:** verbose — DEBUG на входе каждого метода, INFO на успехе, WARN на fail
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Auth Service"
**Rationale:** Седьмой milestone — сервис выдачи JWT через token exchange нужен для виджета и admin panel до React Widget MVP.

## Tasks

### Phase 1: JWT signing + domain errors

**[x] Task 34 — CreateToken в shared/pkg/jwt + domain errors**

Добавить `CreateToken(claims Claims, secret string, ttl time.Duration) (string, error)` в `shared/pkg/jwt/jwt.go`.

Добавить sentinel errors в `shared/domain/errors.go`:
- `ErrInvalidSecret` — неверный pre-shared API secret
- `ErrInvalidRole` — role не в {user, moderator}

(`ErrNotFound` уже определён — использовать для EventNotFound)

LOGGING:
- `slog.Debug("[jwt.CreateToken] signing", "user_id", claims.UserID, "event_id", claims.EventID, "ttl", ttl)`

Files: `shared/pkg/jwt/jwt.go`, `shared/domain/errors.go`

### Phase 2: Сервисный слой

**[x] Task 35 — AuthService: token exchange**

`services/auth/internal/service/auth_service.go`:

```go
type TokenRequest struct {
    ExternalUserID string
    EventID        uuid.UUID
    Name           string
    Role           string
    APISecret      string // из Authorization header (Bearer <secret>)
}
```

`ExchangeToken(ctx context.Context, req TokenRequest) (string, error)`:
1. Валидация: `req.Role` ∉ {`user`, `moderator`} → `ErrInvalidRole`
2. `eventRepo.GetAPISecret(ctx, req.EventID)` → `domain.ErrNotFound` если не найден
3. `bcrypt.CompareHashAndPassword(hash, []byte(req.APISecret))` → `ErrInvalidSecret` при несовпадении
4. `userRepo.GetOrCreate(ctx, req.ExternalUserID, req.EventID, req.Name, req.Role)` → `*domain.User`
5. `jwt.CreateToken(Claims{UserID: user.ID, ExternalID: user.ExternalID, EventID: user.EventID, Role: user.Role}, jwtSecret, ttl)` → token string
6. Вернуть token

LOGGING:
- `slog.Debug("[AuthService.ExchangeToken] request", "external_user_id", req.ExternalUserID, "event_id", req.EventID, "role", req.Role)`
- `slog.Info("[AuthService.ExchangeToken] token issued", "user_id", user.ID, "event_id", req.EventID)`
- `slog.Warn("[AuthService.ExchangeToken] invalid secret attempt", "event_id", req.EventID)`

Files: `services/auth/internal/service/auth_service.go`

### Phase 3: HTTP API

**[x] Task 36 — Token handler + CORS middleware**

`services/auth/internal/handler/token_handler.go`:

`POST /api/auth/token`:
- Извлечь Bearer из `Authorization` header → `apiSecret`
- Декодировать JSON body: `{external_user_id, event_id, name, role}`
- Вызвать `service.ExchangeToken(ctx, TokenRequest{...})`
- Ответ: `200 {"token": "eyJ..."}`
- Маппинг ошибок: `domain.ErrNotFound` → 404; `ErrInvalidSecret` → 401; `ErrInvalidRole` → 400

`services/auth/internal/middleware/cors.go`:
- `CORS(eventRepo domain.EventRepository)` — per-event allowed_origin
- Wildcard `*` запрещён (несовместим с credentials=true)
- Preflight OPTIONS → 204

Роутинг (`services/auth/internal/handler/router.go`):
```go
r.Use(middleware.CORS(eventRepo))
r.Post("/api/auth/token", h.ExchangeToken)
```

LOGGING:
- `slog.Debug("[TokenHandler.ExchangeToken] request", "event_id", eventID, "role", role)`
- `slog.Info("[TokenHandler.ExchangeToken] token issued")`
- `slog.Warn("[CORSMiddleware] origin rejected", "origin", origin, "allowed", allowed)`
- `slog.Debug("[CORSMiddleware] origin allowed", "origin", origin)`

Files: `services/auth/internal/handler/token_handler.go`, `services/auth/internal/middleware/cors.go`, обновить `services/auth/internal/handler/router.go`

### Phase 4: Tests

**[x] Task 37 — Тесты Auth Service**

Unit `services/auth/internal/service/auth_service_test.go`:
- `TestExchangeToken_ValidSecret` → token возвращён, Claims внутри JWT корректны
- `TestExchangeToken_InvalidSecret` → ErrInvalidSecret
- `TestExchangeToken_EventNotFound` → domain.ErrNotFound
- `TestExchangeToken_InvalidRole` → ErrInvalidRole (role="admin" или "superuser")
- `TestExchangeToken_UserUpsertCalled` → mock userRepo.GetOrCreate вызван с правильными аргументами

Интеграционные (TEST_DATABASE_URL):
- `TestExchangeToken_CreatesUser`: первый вызов → user создан в БД, токен парсится
- `TestExchangeToken_UpdatesUser`: второй вызов с другим name → name обновлён в БД
- `TestExchangeToken_ValidJWT`: `jwt.ValidateToken(token, secret)` → Claims.UserID совпадает

Files: `services/auth/internal/service/auth_service_test.go`

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 34–36 | `feat: auth service token exchange with JWT signing and CORS` |
| 2 | 37 | `test: unit and integration tests for auth service` |
