# Feature: Обработка 429 и всплывающее сообщение

**Branch:** `feature/429-rate-limit-toast`
**Created:** 2026-04-13
**Scope:** frontend/widget + frontend/admin

## Settings

- **Testing:** yes
- **Logging:** verbose (DEBUG/WARN в DEV-режиме)
- **Docs:** no

## Roadmap Linkage

Milestone: "none"
Rationale: Skipped by user

---

## Context

`HttpError` в виджете уже содержит поле `retryAfter` (парсится из заголовка `Retry-After`).
Компонент `Toast` с вариантами `success`/`error` уже есть в виджете и используется в `ChatWindow`.
В admin-панели ни Toast, ни обработки 429 нет — нужно создать с нуля.

Текущее поведение:
- **Виджет**: 429 в `handleSend` падает до `throw err` (необработанный). В остальных хендлерах тихо логируется.
- **Admin**: 429 бросает generic `Error`, страницы показывают его текст где попало.

Целевое поведение:
- При 429 пользователь видит всплывающий тост: *"Слишком много запросов. Попробуйте через N сек."* (или без таймера если `retryAfter = 0`).

---

## Tasks

### Фаза 1 — Виджет

#### [x] Task 1 — Обработка 429 в ChatWindow (виджет)
**File:** `frontend/widget/src/components/ChatWindow.tsx`

Добавить перехват `HttpError` с `status === 429` во все action-хендлеры:

- **`handleSend`** — после блока `err instanceof HttpError && err.code === 'muted'`, добавить:
  ```ts
  if (err instanceof HttpError && err.status === 429) {
    const msg = err.retryAfter > 0
      ? `Слишком много запросов. Попробуйте через ${err.retryAfter} сек.`
      : 'Слишком много запросов. Попробуйте позже.'
    if (import.meta.env.DEV) console.warn('[ChatWindow] rate limited, retryAfter', err.retryAfter)
    setToast({ message: msg, variant: 'error' })
    return
  }
  ```
- **`handleEdit`** — аналогично в catch, вместо `console.warn`
- **`handleDelete`** — аналогично
- **`loadOlderMessages`** — аналогично

Логирование: `console.warn('[ChatWindow] rate limited, retryAfter', err.retryAfter)` в DEV.

---

### Фаза 2 — Admin-панель

#### [x] Task 2 — Toast компонент для admin
**File:** `frontend/admin/src/components/Toast.tsx` (новый)

Адаптировать из виджетового `Toast.tsx`. Добавить вариант `'warning'` (оранжевый `#d97706`).
Экспортировать: `ToastVariant`, `ToastState`, `Toast`.

Логирование: `console.debug('[Toast] show', ...)` в DEV.

#### [x] Task 3 — ToastContext для admin
**File:** `frontend/admin/src/context/ToastContext.tsx` (новый)

```ts
interface ToastContextValue {
  showToast: (message: string, variant: ToastVariant) => void
}
```

- `ToastProvider` — хранит `state: ToastState | null`, рендерит `<Toast>` поверх `{children}`.
- `useToast()` — хук, кидает ошибку если вызван вне провайдера.

#### [x] Task 4 — on429 callback в AdminApiClient
**File:** `frontend/admin/src/api/adminClient.ts`

Добавить параметр `on429?: (retryAfter: number) => void` в конструктор.
В `request()` при `res.status === 429`:
```ts
const retryAfter = parseInt(res.headers.get('Retry-After') ?? '0', 10) || 0
console.warn('[AdminClient] rate limited, retryAfter', retryAfter)
this.on429?.(retryAfter)
const msg = retryAfter > 0
  ? `Слишком много запросов. Попробуйте через ${retryAfter} сек.`
  : 'Слишком много запросов. Попробуйте позже.'
throw new Error(msg)
```
Обработку добавить **до** общего `!res.ok` блока.

#### [x] Task 5 — Подключить ToastProvider и on429 в admin App
**Files:** `frontend/admin/src/App.tsx`, `frontend/admin/src/context/AuthContext.tsx`

1. В `App.tsx` обернуть `<AuthProvider>` в `<ToastProvider>`.
2. В `AuthContext.tsx` получить `useToast()`, при создании `AdminApiClient` передать:
   ```ts
   on429: (retryAfter) => {
     const msg = retryAfter > 0
       ? `Слишком много запросов. Попробуйте через ${retryAfter} сек.`
       : 'Слишком много запросов. Попробуйте позже.'
     showToast(msg, 'warning')
   }
   ```

> **Commit checkpoint 1** (после Task 5): `feat(frontend): handle 429 rate limit with toast in widget and admin`

---

### Фаза 3 — Тесты

#### Task 6 — Тесты: 429 toast в виджете
**File:** `frontend/widget/src/components/__tests__/ChatWindow.test.tsx`

Добавить тест-кейс в существующий файл:
- Мокать `api.sendMessage` чтобы бросал `new HttpError(429, 'Too Many Requests', 5)`
- Отправить сообщение через `MessageInput`
- Проверить что в DOM появляется элемент с текстом, содержащим "Слишком много запросов" и "5 сек"
- Проверить что оптимистичное сообщение удалено из списка

Логирование: нет (тест-файл).

#### Task 7 — Тесты: on429 в AdminApiClient
**File:** `frontend/admin/src/api/adminClient.test.ts` (новый)

- Мокать `fetch` глобально, возвращать `{ status: 429, ok: false, headers: Map{'Retry-After': '10'} }`
- Создать `AdminApiClient` с `on429` mock-функцией
- Вызвать любой метод (напр. `listEvents()`)
- Проверить что `on429` вызван с `10`
- Проверить что `Promise` reject с сообщением про "Слишком много запросов"

> **Commit checkpoint 2** (после Task 7): `test(frontend): add 429 rate limit handling tests`

---

## Commit Plan

| Checkpoint | После задач | Сообщение |
|---|---|---|
| 1 | Task 1–5 | `feat(frontend): handle 429 rate limit with toast in widget and admin` |
| 2 | Task 6–7 | `test(frontend): add 429 rate limit handling tests` |
