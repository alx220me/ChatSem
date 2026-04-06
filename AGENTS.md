# AGENTS.md

> Project map for AI agents. Keep this file up-to-date as the project evolves.

## Project Overview

ChatSem is a chat service for online events. Multi-service monorepo: 3 Go backend services
(chat, auth, admin) + 2 React frontend apps (widget, admin panel), shared PostgreSQL + Redis.

## Tech Stack

### Backend
- **Language:** Go 1.24
- **Framework:** net/http + chi router (per service)
- **Database:** PostgreSQL (pgx v5, raw SQL, no ORM)
- **Cache:** Redis (long polling, sessions, rate limiting)
- **Workspace:** go.work (multi-module)

### Frontend
- **Widget:** React 18 + TypeScript + Vite (builds to IIFE bundle, embedded via `<script>`)
- **Admin panel:** React 18 + TypeScript + Vite + React Router (SPA)

## Project Structure

```
ChatSem/
├── go.work                         # Go workspace
├── services/
│   ├── chat/                       # Chat service :8080
│   │   ├── cmd/main.go             # Entry point + DI
│   │   ├── internal/
│   │   │   ├── ports/              # Repository interfaces (at point of use)
│   │   │   ├── service/            # Business logic
│   │   │   ├── repository/postgres/
│   │   │   ├── handler/            # chi HTTP handlers
│   │   │   └── middleware/
│   │   └── go.mod
│   ├── auth/                       # Auth service :8081
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── ports/              # Repository interfaces (at point of use)
│   │   │   ├── service/
│   │   │   ├── repository/postgres/
│   │   │   └── handler/
│   │   └── go.mod
│   └── admin/                      # Admin service :8082
│       ├── cmd/main.go
│       ├── internal/
│       │   ├── ports/              # Repository interfaces (at point of use)
│       │   ├── service/
│       │   ├── repository/postgres/
│       │   └── handler/
│       └── go.mod
├── shared/                         # Shared pure-Go (no pgx/Redis/chi)
│   ├── domain/                     # Domain entities only: Chat, Message, User, Event, errors
│   └── pkg/
│       ├── postgres/               # Shared pgxpool.Pool factory (NewPool)
│       ├── longpoll/               # Redis long polling broker
│       ├── jwt/                    # JWT helpers
│       └── response/               # Standard HTTP error format
├── frontend/
│   ├── widget/                     # Embeddable React chat widget
│   │   ├── src/
│   │   │   ├── components/         # ChatWindow, MessageList, MessageInput, UserAvatar
│   │   │   ├── hooks/              # useLongPoll, useAuth, useChat
│   │   │   ├── api/                # fetch client (ApiClient) → chat-service
│   │   │   ├── types/              # TypeScript interfaces (Chat, Message, WidgetConfig…)
│   │   │   └── index.tsx           # window.ChatSem.init entry point
│   │   ├── package.json
│   │   └── vite.config.ts          # IIFE build
│   └── admin/                      # Admin panel SPA
│       ├── src/
│       │   ├── pages/
│       │   ├── components/
│       │   └── api/                # fetch client → admin-service
│       ├── package.json
│       └── vite.config.ts
├── migrations/                     # SQL migrations (all services)
└── deploy/
    ├── docker-compose.yml
    └── nginx.conf
```

## Key Entry Points

| File | Purpose |
|------|---------|
| `services/chat/cmd/main.go` | Chat service bootstrap |
| `services/auth/cmd/main.go` | Auth service bootstrap |
| `services/admin/cmd/main.go` | Admin service bootstrap |
| `shared/domain/` | Shared domain entities (Chat, Message, User, Event, errors) |
| `shared/pkg/postgres/` | Shared DB connection pool factory |
| `frontend/widget/src/index.tsx` | Widget entry point |
| `frontend/admin/src/main.tsx` | Admin panel entry point |
| `migrations/` | Database schema |

## Documentation

| Document | Path | Description |
|----------|------|-------------|
| Project spec | .ai-factory/DESCRIPTION.md | Tech stack and features |
| Architecture | .ai-factory/ARCHITECTURE.md | Architecture decisions |
| Roadmap | .ai-factory/ROADMAP.md | Project milestones |

## AI Context Files

| File | Purpose |
|------|---------|
| AGENTS.md | This file — project structure map |
| .ai-factory/DESCRIPTION.md | Project specification and tech stack |
| .ai-factory/ARCHITECTURE.md | Architecture decisions and guidelines |
| .ai-factory/ROADMAP.md | Strategic milestones |
| .ai-factory/plans/ | 12 implementation plans (one per milestone branch) |

## Agent Rules

- Never combine shell commands with `&&`, `||`, or `;` — execute as separate Bash calls
- Go: run `go vet ./...` from the service directory after changes
- DB changes go in `migrations/` — never inline in service code
- No ORM — raw SQL via pgx v5 only
- `shared/domain/` содержит только сущности и ошибки — интерфейсы репозиториев живут в `internal/ports/` каждого сервиса
- `shared/` stays pure Go — no Redis, chi imports (pgx разрешён только в `shared/pkg/postgres`)
- Services share PostgreSQL DB — no cross-service HTTP on hot path
- JWT validation is local via `shared/pkg/jwt`, not HTTP call to auth service
- Frontend: widget builds to IIFE bundle; admin panel builds to SPA