# Migrations

SQL migrations for all ChatSem services, managed with [goose](https://github.com/pressly/goose).

## Numbering convention

| Range     | Domain              |
|-----------|---------------------|
| 001–099   | Shared / core tables (events, chats, users) |
| 100–199   | Messaging (messages, chat_seqs)             |
| 300–399   | Moderation (bans, mutes)                    |

## File format

```
<NNN>_<description>.sql
```

Examples: `001_create_events.sql`, `100_create_messages.sql`

## Running migrations

```bash
# Apply all pending
make migrate-up DATABASE_URL=postgres://...

# Roll back last
make migrate-down DATABASE_URL=postgres://...

# Check status
make migrate-status DATABASE_URL=postgres://...

# Roll back everything
make migrate-reset DATABASE_URL=postgres://...
```

## Safe migration rules

1. **Always use `IF NOT EXISTS`** — makes migrations re-entrant and safe to replay.
2. **Create indexes with `CREATE INDEX CONCURRENTLY`** in production (no table lock).
   In migration files, plain `CREATE INDEX IF NOT EXISTS` is acceptable for initial schema.
3. **Never drop columns / rename columns** without an intermediate `Expand` migration first
   (Expand/Contract pattern).
4. **Soft deletes only** — never `DELETE` from messages; use `deleted_at TIMESTAMPTZ`.
5. **Goose Up/Down sections** are mandatory in every file:
   ```sql
   -- +goose Up
   ...
   -- +goose Down
   ...
   ```
