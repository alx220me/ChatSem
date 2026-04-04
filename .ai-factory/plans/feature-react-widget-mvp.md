# Plan: React Widget MVP

**Branch:** `feature/react-widget-mvp`
**Created:** 2026-04-02
**Milestone:** React Widget MVP

## Settings
- **Testing:** no
- **Logging:** verbose — `console.debug` в dev-режиме (`import.meta.env.DEV`), `console.warn/error` всегда
- **Docs:** no — WARN [docs]

## Roadmap Linkage
**Milestone:** "React Widget MVP"
**Rationale:** Восьмой milestone — встраиваемый виджет нужен для демонстрации end-to-end flow после Auth Service и Long Polling.

## Tasks

### Phase 1: Foundation

**[x] Task 43 — TypeScript types + API client**

`frontend/widget/src/types/index.ts`:
```ts
export interface Chat { id: string; eventId: string; parentId: string | null; type: 'parent' | 'child'; externalRoomId: string | null }
export interface Message { id: string; chatId: string; userId: string; text: string; seq: number; createdAt: string }
export interface SendResponse { id: string; seq: number; ts: string }
export interface PollResponse { messages: Message[] }
export interface WidgetConfig {
  containerId: string
  eventId: string
  token: string
  roomId?: string
  onTokenExpired?: () => Promise<string>
}
```

`frontend/widget/src/api/client.ts` — класс `ApiClient`:
- `listChats(eventId)` — `GET /api/chat/events/{eventId}/chats`
- `joinRoom(eventId, roomId)` — `POST /api/chat/join`
- `getMessages(chatId, limit)` — `GET /api/chat/chats/{chatId}/messages?limit={limit}`
- `sendMessage(chatId, text)` — `POST /api/chat/{chatId}/messages`
- `poll(chatId, afterSeq, signal: AbortSignal)` — `GET /api/chat/{chatId}/poll?after={afterSeq}`
- При 401 → `onTokenExpired()` → обновить токен → повторить 1 раз

Logging (DEV only):
- `console.debug('[ApiClient] request', method, url)`
- `console.warn('[ApiClient] token expired, refreshing')`

Files: `frontend/widget/src/types/index.ts`, `frontend/widget/src/api/client.ts`, `frontend/widget/package.json`, `frontend/widget/tsconfig.json`

### Phase 2: Hooks

**[x] Task 44 — Core React hooks**

`frontend/widget/src/hooks/useAuth.ts`:
- Токен хранится **в памяти модуля** (не state, не localStorage) — требование безопасности
- `refreshToken()` → `config.onTokenExpired?.()` → обновить токен; если callback нет → `console.warn('[useAuth] session expired, no refresh callback')`

`frontend/widget/src/hooks/useChat.ts`:
- `useChat(api, eventId, roomId?)` → `{ chat, messages, loading, error, sendMessage }`
- При монтировании: `listChats` → `joinRoom` (если roomId задан) → `getMessages(limit=50)`
- `sendMessage(text)` → optimistic message (seq=-1) → после ответа → заменить реальным

`frontend/widget/src/hooks/useLongPoll.ts`:
- `useLongPoll(api, chatId, onMessages)` — бесконечный polling loop
- Loop: `poll(chatId, afterSeq, signal)` → `onMessages(msgs)` → `afterSeq = max(msg.seq)`
- Пустой ответ (204 / messages:[]) → немедленный реконнект
- При unmount → `AbortController.abort()`
- Реконнект: `after = lastKnownSeq - 1`, дедупликация по `id`

Logging (DEV):
- `console.debug('[useLongPoll] poll', chatId, 'after', afterSeq)`
- `console.debug('[useLongPoll] received', messages.length, 'messages')`
- `console.warn('[useLongPoll] disconnected, reconnecting')`

Files: `frontend/widget/src/hooks/useAuth.ts`, `useChat.ts`, `useLongPoll.ts`

### Phase 3: UI Components

**[x] Task 45 — UI компоненты виджета**

`frontend/widget/src/components/MessageList.tsx`:
- Props: `messages: Message[]`, `loading: boolean`
- Автоскролл вниз при новых сообщениях (`useEffect` + `ref`)
- Скелетон-строки при `loading=true`

`frontend/widget/src/components/MessageInput.tsx`:
- Props: `onSend: (text: string) => void`, `disabled: boolean`
- Enter → отправить; Shift+Enter → перенос строки
- Очистка поля после отправки; disabled при `sending=true`

`frontend/widget/src/components/UserAvatar.tsx`:
- Props: `name: string`, `size?: 'sm' | 'md'`
- Отображает инициалы; цвет фона — из хэша `name`

`frontend/widget/src/components/ChatWindow.tsx`:
- Корневой компонент, объединяет `useChat` + `useLongPoll`
- Props: `config: WidgetConfig`, `api: ApiClient`
- Ошибка соединения → banner "Connection error, retrying..."

Logging:
- `console.debug('[ChatWindow] mounted', 'chat_id', chatId)` — DEV
- `console.warn('[ChatWindow] poll error', error)`

Files: `frontend/widget/src/components/ChatWindow.tsx`, `MessageList.tsx`, `MessageInput.tsx`, `UserAvatar.tsx`

### Phase 4: Entry point + Build

**[x] Task 46 — Widget entry point + Vite build config** ✅

`frontend/widget/src/index.tsx`:
```tsx
declare global {
  interface Window { ChatSem: { init: (config: WidgetConfig) => void } }
}

window.ChatSem = {
  init(config: WidgetConfig) {
    const container = document.getElementById(config.containerId)
    if (!container) {
      console.error('[ChatSem] container not found:', config.containerId)
      return
    }
    const api = new ApiClient('/api', getToken, config.onTokenExpired)
    createRoot(container).render(<ChatWindow config={config} api={api} />)
    console.info('[ChatSem] widget mounted', 'event_id', config.eventId)
  }
}
```

`frontend/widget/vite.config.ts` — IIFE build:
```ts
build: {
  lib: {
    entry: 'src/index.tsx',
    name: 'ChatSem',
    formats: ['iife'],
    fileName: () => 'widget.js',
  }
}
```

`frontend/widget/package.json` — зависимости:
- `react@18`, `react-dom@18`
- devDeps: `vite@5`, `@vitejs/plugin-react`, `typescript`
- Scripts: `build`, `dev`

Files: `frontend/widget/src/index.tsx`, `frontend/widget/vite.config.ts`, `frontend/widget/package.json`, `frontend/widget/tsconfig.json`

<!-- Commit checkpoint: tasks 43-46 -->

## Commit Plan

| Commit | Tasks | Сообщение |
|--------|-------|-----------|
| 1 | 43–46 | `feat: React widget MVP with long polling client and chat UI` |
