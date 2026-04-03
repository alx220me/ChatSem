package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventRepo implements domain.EventRepository for the chat service using pgx.
type EventRepo struct {
	db *pgxpool.Pool
}

// NewEventRepo creates an EventRepo backed by the given connection pool.
func NewEventRepo(db *pgxpool.Pool) *EventRepo {
	return &EventRepo{db: db}
}

// GetByID returns the event with the given ID.
func (r *EventRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	slog.Debug("[ChatEventRepo.GetByID] query", "event_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, name, settings, allowed_origin, api_secret, created_at
		FROM events WHERE id = $1`, id)

	e := &domain.Event{}
	if err := row.Scan(&e.ID, &e.Name, &e.Settings, &e.AllowedOrigin, &e.APISecret, &e.CreatedAt); err != nil {
		return nil, fmt.Errorf("ChatEventRepo.GetByID: %w", err)
	}
	slog.Debug("[ChatEventRepo.GetByID] found", "event_id", id, "name", e.Name)
	return e, nil
}

// Create inserts a new event and sets e.ID.
func (r *EventRepo) Create(ctx context.Context, e *domain.Event) error {
	slog.Debug("[ChatEventRepo.Create] inserting event", "name", e.Name)
	row := r.db.QueryRow(ctx, `
		INSERT INTO events (name, settings, allowed_origin, api_secret)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		e.Name, e.Settings, e.AllowedOrigin, e.APISecret)

	if err := row.Scan(&e.ID, &e.CreatedAt); err != nil {
		return fmt.Errorf("ChatEventRepo.Create: %w", err)
	}
	slog.Debug("[ChatEventRepo.Create] done", "event_id", e.ID)
	return nil
}

// List returns all events ordered by creation time.
func (r *EventRepo) List(ctx context.Context) ([]*domain.Event, error) {
	slog.Debug("[ChatEventRepo.List] query")
	rows, err := r.db.Query(ctx, `
		SELECT id, name, settings, allowed_origin, api_secret, created_at
		FROM events ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("ChatEventRepo.List: %w", err)
	}
	defer rows.Close()

	var events []*domain.Event
	for rows.Next() {
		e := &domain.Event{}
		if err := rows.Scan(&e.ID, &e.Name, &e.Settings, &e.AllowedOrigin, &e.APISecret, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("ChatEventRepo.List scan: %w", err)
		}
		events = append(events, e)
	}
	slog.Debug("[ChatEventRepo.List] fetched", "count", len(events))
	return events, rows.Err()
}
