# Project: ChatSem — Embeddable Chat Service for Online Events

## Overview

A chat service for online event websites. Backend — Go monorepo (3 services). Frontend — React.js
embeddable widget, delivered as a `<script>` tag on the host website. Supports hierarchical chat
rooms (parent/child), SSO authentication, long polling, and chat history export.

Reference product: https://www.chatbro.com/ru/

## Core Features

- **Hierarchical chats**: Parent chat per event with dynamically created child chats (rooms/halls).
  Child chats inherit parent settings but remain fully isolated.
- **React widget**: Embeddable React app loaded on the host website via `<script>` tag;
  communicates with chat-service via long polling.
- **Long polling**: Server-side long polling endpoint; clients wait for new messages.
- **SSO / External auth**: Authentication via the host site's credentials (JWT token validation).
  Users are identified by the host site's user ID.
- **Chat history export**: Export messages per chat (CSV, JSON).
- **Admin panel**: React SPA for managing events, chats, settings, moderation, banning.
- **Moderation**: Message filtering, user banning per chat, message deletion.

## Tech Stack

### Backend
- **Language:** Go 1.24
- **Framework:** net/http + chi router (per service)
- **Database:** PostgreSQL
- **Cache:** Redis (long polling queues, session tokens, rate limiting)
- **Query:** pgx v5 (raw SQL, no ORM)
- **Config:** environment variables (`DATABASE_URL`, `REDIS_ADDR`, `JWT_SECRET`, …)
- **Shared DB pool:** `shared/pkg/postgres` — `NewPool(ctx, connString)` используется всеми сервисами
- **Repository interfaces:** объявляются в `internal/ports/` каждого сервиса (по месту использования, не в shared)

### Frontend
- **Widget:** React 18 + TypeScript, Vite build, delivered as embeddable `<script>` bundle
- **Admin panel:** React 18 + TypeScript + React Router, Vite build (SPA)
- **HTTP client:** fetch API (long polling)
- **Styling:** CSS Modules или Tailwind CSS

## Architecture
See `.ai-factory/ARCHITECTURE.md` for detailed architecture guidelines.
Pattern: Multi-Service Monorepo — 3 Go services (chat :8080, auth :8081, admin :8082) + 2 React apps (widget, admin-panel)

## Non-Functional Requirements

- Logging: structured JSON logging via `log/slog`
- Error handling: structured error responses `{"error": "...", "code": "..."}`
- Security: JWT validation, rate limiting per IP/user, input sanitization, CORS
- Scale target: 10 000–30 000 одновременных пользователей; long polling через Redis pub/sub
- Scalability: stateless HTTP handlers; 2-6 горизонтальных инстансов chat-service; Redis pub/sub для синхронизации; PgBouncer (transaction pooling) перед PostgreSQL
