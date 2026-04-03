package postgres

import (
	"context"
	"fmt"
	"log/slog"

	"chatsem/shared/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserRepo implements domain.UserRepository for the chat service using pgx.
type UserRepo struct {
	db *pgxpool.Pool
}

// NewUserRepo creates a UserRepo backed by the given connection pool.
func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

// Upsert creates or updates the user by (external_id, event_id).
func (r *UserRepo) Upsert(ctx context.Context, u *domain.User) (*domain.User, error) {
	slog.Debug("[UserRepo.Upsert] upsert", "external_id", u.ExternalID, "event_id", u.EventID)
	row := r.db.QueryRow(ctx, `
		INSERT INTO users (external_id, event_id, name, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (external_id, event_id) DO UPDATE
		  SET name = EXCLUDED.name
		RETURNING id, external_id, event_id, name, role, created_at`,
		u.ExternalID, u.EventID, u.Name, u.Role)

	result := &domain.User{}
	if err := row.Scan(&result.ID, &result.ExternalID, &result.EventID, &result.Name, &result.Role, &result.CreatedAt); err != nil {
		return nil, fmt.Errorf("UserRepo.Upsert: %w", err)
	}
	slog.Debug("[UserRepo.Upsert] done", "user_id", result.ID, "role", result.Role)
	return result, nil
}

// GetByID returns the user with the given ID.
func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	slog.Debug("[UserRepo.GetByID] query", "user_id", id)
	row := r.db.QueryRow(ctx, `
		SELECT id, external_id, event_id, name, role, created_at
		FROM users WHERE id = $1`, id)

	u := &domain.User{}
	if err := row.Scan(&u.ID, &u.ExternalID, &u.EventID, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("UserRepo.GetByID: %w", err)
	}
	return u, nil
}

// GetByExternalID returns the user by (external_id, event_id).
func (r *UserRepo) GetByExternalID(ctx context.Context, externalID string, eventID uuid.UUID) (*domain.User, error) {
	slog.Debug("[UserRepo.GetByExternalID] query", "external_id", externalID, "event_id", eventID)
	row := r.db.QueryRow(ctx, `
		SELECT id, external_id, event_id, name, role, created_at
		FROM users WHERE external_id = $1 AND event_id = $2`, externalID, eventID)

	u := &domain.User{}
	if err := row.Scan(&u.ID, &u.ExternalID, &u.EventID, &u.Name, &u.Role, &u.CreatedAt); err != nil {
		return nil, fmt.Errorf("UserRepo.GetByExternalID: %w", err)
	}
	return u, nil
}

// ListByEventID returns users for an event, paginated.
func (r *UserRepo) ListByEventID(ctx context.Context, eventID uuid.UUID, limit, offset int) ([]*domain.User, error) {
	slog.Debug("[UserRepo.ListByEventID] query", "event_id", eventID, "limit", limit, "offset", offset)
	rows, err := r.db.Query(ctx, `
		SELECT id, external_id, event_id, name, role, created_at
		FROM users WHERE event_id = $1
		ORDER BY created_at ASC LIMIT $2 OFFSET $3`, eventID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("UserRepo.ListByEventID: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(&u.ID, &u.ExternalID, &u.EventID, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("UserRepo.ListByEventID scan: %w", err)
		}
		users = append(users, u)
	}
	slog.Debug("[UserRepo.ListByEventID] fetched", "count", len(users))
	return users, rows.Err()
}

// UpdateRole sets the role for the given user.
func (r *UserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role domain.UserRole) error {
	slog.Debug("[UserRepo.UpdateRole] update", "user_id", id, "role", role)
	_, err := r.db.Exec(ctx, `UPDATE users SET role=$2 WHERE id=$1`, id, role)
	if err != nil {
		return fmt.Errorf("UserRepo.UpdateRole: %w", err)
	}
	return nil
}
