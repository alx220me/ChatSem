# Plan: Admin Panel (React SPA)

**Branch:** `feature/admin-panel`
**Created:** 2026-04-02
**Milestone:** Admin Panel (React SPA)

## Settings
- **Testing:** yes — Vitest + React Testing Library
- **Logging:** verbose — `console.debug` в DEV (`import.meta.env.DEV`), `console.warn/error` всегда
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "Admin Panel (React SPA)"
**Rationale:** Десятый milestone — UI для управления событиями, модерации и экспорта нужен после Admin Service backend.

## Tasks

### Phase 1: Foundation

**[x] Task 48 — TypeScript types + Admin API client + Vite SPA setup**

`frontend/admin/src/types/index.ts`:
```ts
export interface Event { id: string; name: string; allowedOrigin: string; createdAt: string }
export interface Chat { id: string; eventId: string; type: 'parent' | 'child'; externalRoomId: string | null; settings: Record<string, unknown> }
export interface User { id: string; externalId: string; eventId: string; name: string; role: 'user' | 'moderator' | 'admin' }
export interface Ban { id: string; userId: string; eventId: string; reason: string; createdAt: string; expiresAt: string | null }
export interface Mute { id: string; chatId: string; userId: string; reason: string; createdAt: string; expiresAt: string | null }
```

`frontend/admin/src/api/adminClient.ts` — класс `AdminApiClient(baseUrl, getToken)`:
- Events: `listEvents()`, `createEvent(name, allowedOrigin, apiSecret)`
- Chats: `listChats(eventId)`, `updateChatSettings(chatId, settings)`
- Users: `listUsers(eventId, limit, offset)`, `updateUserRole(userId, role)`
- Bans: `createBan(userId, eventId, reason, expiresAt?)`, `deleteBan(banId)`, `listBans(eventId)`
- Mutes: `createMute(chatId, userId, reason, expiresAt?)`, `deleteMute(muteId)`, `listMutes(chatId)`
- Export: `exportUrl(chatId, format: 'csv' | 'json')` → string URL с `?token=<jwt>`

`frontend/admin/package.json`:
- deps: `react@18`, `react-dom@18`, `react-router-dom@6`
- devDeps: `typescript`, `vite@5`, `@vitejs/plugin-react`, `vitest`, `@testing-library/react`, `@testing-library/user-event`, `jsdom`
- scripts: `build`, `dev`, `test`

`frontend/admin/vite.config.ts` — обычный SPA build (не IIFE)

Logging:
- `console.debug('[AdminClient] request', method, url)` — DEV only
- `console.warn('[AdminClient] auth error', status)`

Files: `frontend/admin/src/types/index.ts`, `frontend/admin/src/api/adminClient.ts`, `frontend/admin/package.json`, `frontend/admin/vite.config.ts`, `frontend/admin/tsconfig.json`

### Phase 2: Auth + App shell

**[x] Task 49 — Auth (Login page + JWT context) + App shell**

`frontend/admin/src/context/AuthContext.tsx`:
- JWT в `sessionStorage` (admin panel — trusted env, в отличие от виджета)
- `{ token, login, logout }`
- `login(eventId, apiSecret, name)` → `POST /api/auth/token` с `role=admin` → сохранить
- `logout()` → clear sessionStorage → redirect /login

`frontend/admin/src/pages/LoginPage.tsx`:
- Форма: Event ID (UUID), API Secret (password), Name
- Submit → `auth.login(...)` → redirect /events
- 401 → "Invalid secret"

`frontend/admin/src/components/Layout.tsx`:
- Sidebar nav: Events / Chats / Users / Moderation / Export
- Header: имя пользователя + Logout
- Текущий route подсвечен

`frontend/admin/src/App.tsx`:
- Routes: `/login`, `/events`, `/events/:eventId/chats`, `/events/:eventId/users`, `/events/:eventId/moderation`, `/events/:eventId/export`
- `<PrivateRoute>` — redirect /login если нет token

Logging:
- `console.debug('[AuthContext] login attempt', eventId)` — DEV
- `console.info('[AuthContext] logged in', eventId)`
- `console.warn('[LoginPage] auth failed', error)`

Files: `frontend/admin/src/context/AuthContext.tsx`, `frontend/admin/src/pages/LoginPage.tsx`, `frontend/admin/src/components/Layout.tsx`, `frontend/admin/src/App.tsx`, `frontend/admin/src/main.tsx`

### Phase 3: Core pages

**[x] Task 50 — Events page + Chats page**

`frontend/admin/src/pages/EventsPage.tsx`:
- Таблица событий: Name, Allowed Origin, Created At, Actions
- Кнопка "Create Event" → модаль: name, allowed_origin, api_secret (показывается 1 раз, copy-to-clipboard)
- После создания → список обновить

`frontend/admin/src/pages/ChatsPage.tsx` (URL: `/events/:eventId/chats`):
- Дерево чатов: parent + children (external_room_id)
- "Settings" → боковая панель с JSONB-редактором parent chat settings
- `PATCH /api/admin/chats/{chatId}/settings` → тост "Settings saved"

`frontend/admin/src/components/ConfirmDialog.tsx` — переиспользуемый диалог для destructive actions

Logging:
- `console.debug('[EventsPage] createEvent', name)` — DEV
- `console.info('[EventsPage] event created', eventId)`
- `console.debug('[ChatsPage] updateSettings', chatId)` — DEV
- `console.warn('[ChatsPage] settings update failed', error)`

Files: `frontend/admin/src/pages/EventsPage.tsx`, `frontend/admin/src/pages/ChatsPage.tsx`, `frontend/admin/src/components/ConfirmDialog.tsx`

**[x] Task 51 — Users page + Moderation page**

`frontend/admin/src/pages/UsersPage.tsx` (URL: `/events/:eventId/users`):
- Таблица: external_id, name, role, Actions
- Inline role select (user / moderator) — только для admin
- Пагинация: limit=50, Prev/Next

`frontend/admin/src/pages/ModerationPage.tsx` (URL: `/events/:eventId/moderation`):
- Вкладки: **Bans** / **Mutes**
- Bans: таблица + форма "Ban User" (userId, reason, expires_at?) + Unban через ConfirmDialog
- Mutes: selector чата → таблица + форма "Mute User" (chatId, userId, reason, expires_at?) + Unmute

Logging:
- `console.debug('[ModerationPage] ban', userId)` — DEV
- `console.info('[ModerationPage] banned', banId)`
- `console.warn('[ModerationPage] action failed', error)`

Files: `frontend/admin/src/pages/UsersPage.tsx`, `frontend/admin/src/pages/ModerationPage.tsx`

<!-- Commit checkpoint: tasks 48-51 -->

### Phase 4: Export + Tests

**[x] Task 52 — History Export page**

`frontend/admin/src/pages/ExportPage.tsx` (URL: `/events/:eventId/export`):
- Selector чата из `GET /api/admin/events/{eventId}/chats`
- Radio: CSV / JSON
- Кнопка "Download" → `<a href={exportUrl(chatId, format)} download>` с `?token=<jwt>` в URL
- Placeholder если бэкенд не вернул 200 (backend реализуется в milestone "History Export")

Logging:
- `console.debug('[ExportPage] export', chatId, format)` — DEV
- `console.warn('[ExportPage] export failed', error)`

Files: `frontend/admin/src/pages/ExportPage.tsx`

**[x] Task 53 — Тесты Admin Panel**

`frontend/admin/src/pages/LoginPage.test.tsx`:
- `TestLogin_Success`: form submit → `login` вызван → redirect /events
- `TestLogin_InvalidSecret`: 401 mock → error message "Invalid secret"

`frontend/admin/src/pages/EventsPage.test.tsx`:
- `TestEventsList_Renders`: mock list → events в таблице
- `TestCreateEvent_Form`: модаль → submit → `createEvent` вызван → список обновлён

`frontend/admin/src/pages/ModerationPage.test.tsx`:
- `TestBanUser_CallsApi`: форма → submit → `createBan` с правильными args
- `TestUnban_Confirm`: Unban → ConfirmDialog → confirm → `deleteBan` вызван

`frontend/admin/src/pages/UsersPage.test.tsx`:
- `TestChangeRole_Updates`: select moderator → `updateUserRole` вызван

`frontend/admin/vitest.config.ts`: `environment: 'jsdom'`, setupFiles с `@testing-library/jest-dom`

<!-- Commit checkpoint: tasks 52-53 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 48–51 | `feat: admin panel SPA with auth, events, chats, users and moderation pages` |
| 2 | 52–53 | `feat: history export page and Vitest tests for admin panel` |
