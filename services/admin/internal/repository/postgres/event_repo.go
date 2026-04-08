package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventRepo implements domain.EventRepository for the admin service using pgx.
type EventRepo struct {
	db *pgxpool.Pool
}

// NewEventRepo creates an EventRepo backed by the given connection pool.
func NewEventRepo(db *pgxpool.Pool) *EventRepo {
	return &EventRepo{db: db}
}

// GetByID returns the event with the given ID.
func (r *EventRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	slog.Debug("[AdminEventRepo.GetByID] query", "event_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, name, settings, allowed_origin, api_secret, created_at
		FROM events WHERE id = $1`, id)

	e := &domain.Event{}
	if err := row.Scan(&e.ID, &e.Name, &e.Settings, &e.AllowedOrigin, &e.APISecret, &e.CreatedAt); err != nil {
		return nil, fmt.Errorf("AdminEventRepo.GetByID: %w", err)
	}
	return e, nil
}

// Create inserts a new event (api_secret should be bcrypt hashed before calling).
func (r *EventRepo) Create(ctx context.Context, e *domain.Event) error {
	slog.Debug("[AdminEventRepo.Create] inserting event", "name", e.Name, "allowed_origin", e.AllowedOrigin)
	row := r.db.QueryRow(ctx, `
		INSERT INTO events (name, settings, allowed_origin, api_secret)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		e.Name, e.Settings, e.AllowedOrigin, e.APISecret)

	if err := row.Scan(&e.ID, &e.CreatedAt); err != nil {
		return fmt.Errorf("AdminEventRepo.Create: %w", err)
	}
	slog.Debug("[AdminEventRepo.Create] created", "event_id", e.ID)
	return nil
}

// UpdateAPISecret replaces the bcrypt-hashed api_secret for the given event.
// Returns an error if the event does not exist.
func (r *EventRepo) UpdateAPISecret(ctx context.Context, id uuid.UUID, hashedSecret string) error {
	slog.Debug("[AdminEventRepo.UpdateAPISecret] updating secret", "event_id", id)
	tag, err := r.db.Exec(ctx, `UPDATE events SET api_secret = $1 WHERE id = $2`, hashedSecret, id)
	if err != nil {
		return fmt.Errorf("AdminEventRepo.UpdateAPISecret: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("AdminEventRepo.UpdateAPISecret: event not found: %s", id)
	}
	slog.Debug("[AdminEventRepo.UpdateAPISecret] updated", "event_id", id)
	return nil
}

// List returns all events ordered by creation time.
func (r *EventRepo) List(ctx context.Context) ([]*domain.Event, error) {
	slog.Debug("[AdminEventRepo.List] query")
	rows, err := r.db.Query(ctx, `
		SELECT id, name, settings, allowed_origin, api_secret, created_at
		FROM events ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("AdminEventRepo.List: %w", err)
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		e := &domain.Event{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Settings, &e.AllowedOrigin, &e.APISecret, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("AdminEventRepo.List scan: %w", err)
		}
		events = append(events, e)
	}
	slog.Debug("[AdminEventRepo.List] fetched", "count", len(events))
	return events, rows.Err()
}
