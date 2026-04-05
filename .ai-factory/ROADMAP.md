# Project Roadmap

> Embeddable chat service for online events — Go multi-service backend + React widget + admin panel

## Milestones

- [x] **Project Bootstrap** — go.work, go.mod per service, config loading, chi router, slog, shared module
- [x] **Database Schema** — SQL migrations: events, chats (parent/child), messages, users, ban lists
- [x] **Shared Domain & Repositories** — shared/domain types, repository interfaces, pgx implementations per service
- [x] **Chat Hierarchy** — create/list events, parent/child chat management, settings inheritance
- [x] **Message Service** — send, list, delete messages; moderation (ban, mute, filter)
- [x] **Long Polling** — Redis pub/sub broker (shared/pkg/longpoll), /poll endpoint, backpressure handling
- [x] **Auth Service** — SSO JWT validation with pre-shared secret, session store (Redis), token exchange
- [x] **React Widget MVP** — embeddable React widget, long polling client, basic chat UI
- [x] **Admin Service** — event/chat management API, moderation endpoints, ban/unban
- [ ] **Admin Panel (React SPA)** — admin UI: events, chats, users, moderation, history export
- [ ] **History Export** — CSV + JSON streaming download endpoint
- [ ] **Production Readiness** — Docker multi-stage, docker-compose (all services + nginx), health checks, rate limiting

## Completed

| Milestone | Date |
|-----------|------|
| Project Bootstrap | —  |
| Database Schema | — |
| Shared Domain & Repositories | — |
| Chat Hierarchy | — |
| Message Service | — |
| Long Polling | — |
| Admin Service | — |
| Auth Service | 2026-04-04 |
| React Widget MVP | 2026-04-04 |
