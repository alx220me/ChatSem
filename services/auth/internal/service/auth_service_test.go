package service_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"chatsem/services/auth/internal/ports"
	"chatsem/services/auth/internal/service"
	authpostgres "chatsem/services/auth/internal/repository/postgres"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/jwt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// --- mock EventRepository ---

type mockEventRepo struct {
	getByID func(ctx context.Context, id uuid.UUID) (*domain.Event, error)
}

func (m *mockEventRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Event, error) {
	return m.getByID(ctx, id)
}
// --- mock UserRepository ---

type mockUserRepo struct {
	upsert func(ctx context.Context, u *domain.User) (*domain.User, error)
}

func (m *mockUserRepo) Upsert(ctx context.Context, u *domain.User) (*domain.User, error) {
	return m.upsert(ctx, u)
}

// --- helpers ---

const testSecret = "raw-api-secret"
const testJWTSecret = "jwt-test-secret"

func bcryptHash(t *testing.T, secret string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("[%s] bcrypt: %v", t.Name(), err)
	}
	return string(h)
}

func newTestSvc(eventRepo ports.EventRepository, userRepo ports.UserRepository) *service.AuthService {
	return service.NewAuthService(eventRepo, userRepo, testJWTSecret, time.Hour)
}

func makeEvent(t *testing.T) *domain.Event {
	return &domain.Event{
		ID:        uuid.New(),
		APISecret: bcryptHash(t, testSecret),
	}
}

func makeUser(eventID uuid.UUID) *domain.User {
	return &domain.User{
		ID:         uuid.New(),
		ExternalID: "ext-123",
		EventID:    eventID,
		Name:       "Test User",
		Role:       domain.RoleUser,
	}
}

// --- unit tests ---

func TestExchangeToken_ValidSecret(t *testing.T) {
	event := makeEvent(t)
	user := makeUser(event.ID)

	svc := newTestSvc(
		&mockEventRepo{getByID: func(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
			return event, nil
		}},
		&mockUserRepo{upsert: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return user, nil
		}},
	)

	token, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: user.ExternalID,
		EventID:        event.ID,
		Name:           user.Name,
		Role:           string(domain.RoleUser),
		APISecret:      testSecret,
	})
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if token == "" {
		t.Errorf("[%s] expected non-empty token", t.Name())
	}

	claims, err := jwt.ValidateToken(token, testJWTSecret)
	if err != nil {
		t.Fatalf("[%s] ValidateToken: %v", t.Name(), err)
	}
	if claims.UserID != user.ID {
		t.Errorf("[%s] claims.UserID=%s, want %s", t.Name(), claims.UserID, user.ID)
	}
	if claims.EventID != user.EventID {
		t.Errorf("[%s] claims.EventID=%s, want %s", t.Name(), claims.EventID, user.EventID)
	}
	if claims.Role != string(domain.RoleUser) {
		t.Errorf("[%s] claims.Role=%s, want user", t.Name(), claims.Role)
	}
	t.Logf("[%s] assert: valid secret → token issued, claims match", t.Name())
}

func TestExchangeToken_InvalidSecret(t *testing.T) {
	event := makeEvent(t)

	svc := newTestSvc(
		&mockEventRepo{getByID: func(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
			return event, nil
		}},
		&mockUserRepo{upsert: func(_ context.Context, u *domain.User) (*domain.User, error) {
			t.Error("Upsert must not be called on invalid secret")
			return nil, nil
		}},
	)

	_, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: "ext-1",
		EventID:        event.ID,
		Name:           "X",
		Role:           string(domain.RoleUser),
		APISecret:      "wrong-secret",
	})
	if !errors.Is(err, domain.ErrInvalidSecret) {
		t.Errorf("[%s] expected ErrInvalidSecret, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: wrong secret → ErrInvalidSecret", t.Name())
}

func TestExchangeToken_EventNotFound(t *testing.T) {
	svc := newTestSvc(
		&mockEventRepo{getByID: func(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
			return nil, domain.ErrNotFound
		}},
		&mockUserRepo{upsert: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return nil, nil
		}},
	)

	_, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: "ext-1",
		EventID:        uuid.New(),
		Name:           "X",
		Role:           string(domain.RoleUser),
		APISecret:      testSecret,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("[%s] expected ErrNotFound, got %v", t.Name(), err)
	}
	t.Logf("[%s] assert: event not found → ErrNotFound", t.Name())
}

func TestExchangeToken_InvalidRole(t *testing.T) {
	svc := newTestSvc(
		&mockEventRepo{getByID: func(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
			return nil, nil
		}},
		&mockUserRepo{upsert: func(_ context.Context, _ *domain.User) (*domain.User, error) {
			return nil, nil
		}},
	)

	for _, role := range []string{"superuser", "guest", ""} {
		_, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
			ExternalUserID: "ext-1",
			EventID:        uuid.New(),
			Name:           "X",
			Role:           role,
			APISecret:      testSecret,
		})
		if !errors.Is(err, domain.ErrInvalidRole) {
			t.Errorf("[%s] role=%q: expected ErrInvalidRole, got %v", t.Name(), role, err)
		}
	}
	t.Logf("[%s] assert: invalid roles → ErrInvalidRole", t.Name())
}

func TestExchangeToken_UserUpsertCalled(t *testing.T) {
	event := makeEvent(t)
	eventID := event.ID
	extID := "ext-upsert-test"
	name := "Upsert User"
	role := string(domain.RoleModerator)

	upsertCalled := false
	svc := newTestSvc(
		&mockEventRepo{getByID: func(_ context.Context, _ uuid.UUID) (*domain.Event, error) {
			return event, nil
		}},
		&mockUserRepo{upsert: func(_ context.Context, u *domain.User) (*domain.User, error) {
			upsertCalled = true
			if u.ExternalID != extID {
				t.Errorf("[%s] Upsert ExternalID=%s, want %s", t.Name(), u.ExternalID, extID)
			}
			if u.EventID != eventID {
				t.Errorf("[%s] Upsert EventID=%s, want %s", t.Name(), u.EventID, eventID)
			}
			if u.Name != name {
				t.Errorf("[%s] Upsert Name=%s, want %s", t.Name(), u.Name, name)
			}
			if string(u.Role) != role {
				t.Errorf("[%s] Upsert Role=%s, want %s", t.Name(), u.Role, role)
			}
			return makeUser(eventID), nil
		}},
	)

	_, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: extID,
		EventID:        eventID,
		Name:           name,
		Role:           role,
		APISecret:      testSecret,
	})
	if err != nil {
		t.Fatalf("[%s] unexpected error: %v", t.Name(), err)
	}
	if !upsertCalled {
		t.Errorf("[%s] Upsert was not called", t.Name())
	}
	t.Logf("[%s] assert: Upsert called with correct args", t.Name())
}

// --- integration tests ---

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("[%s] pgxpool.New: %v", t.Name(), err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func seedEvent(t *testing.T, pool *pgxpool.Pool, rawSecret string) uuid.UUID {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(rawSecret), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("[%s] bcrypt: %v", t.Name(), err)
	}
	var id uuid.UUID
	err = pool.QueryRow(context.Background(), `
		INSERT INTO events (name, allowed_origin, api_secret)
		VALUES ('auth-test-event', 'http://localhost', $1)
		RETURNING id`, string(hash)).Scan(&id)
	if err != nil {
		t.Fatalf("[%s] seedEvent: %v", t.Name(), err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE event_id=$1`, id)
		pool.Exec(context.Background(), `DELETE FROM events WHERE id=$1`, id)
	})
	return id
}

func newIntegrationSvc(t *testing.T, pool *pgxpool.Pool) *service.AuthService {
	t.Helper()
	return service.NewAuthService(
		authpostgres.NewEventRepo(pool),
		authpostgres.NewUserRepo(pool),
		testJWTSecret,
		time.Hour,
	)
}

func TestIntegration_ExchangeToken_CreatesUser(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool, testSecret)
	svc := newIntegrationSvc(t, pool)

	token, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: "int-ext-1",
		EventID:        eventID,
		Name:           "Integration User",
		Role:           string(domain.RoleUser),
		APISecret:      testSecret,
	})
	if err != nil {
		t.Fatalf("[%s] ExchangeToken: %v", t.Name(), err)
	}
	if token == "" {
		t.Errorf("[%s] expected non-empty token", t.Name())
	}

	var count int
	pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM users WHERE external_id=$1 AND event_id=$2`, "int-ext-1", eventID).Scan(&count)
	if count != 1 {
		t.Errorf("[%s] expected 1 user in DB, got %d", t.Name(), count)
	}
	t.Logf("[%s] assert: user created in DB, token returned", t.Name())
}

func TestIntegration_ExchangeToken_UpdatesUser(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool, testSecret)
	svc := newIntegrationSvc(t, pool)

	req := service.TokenRequest{
		ExternalUserID: "int-ext-update",
		EventID:        eventID,
		APISecret:      testSecret,
		Role:           string(domain.RoleUser),
	}

	req.Name = "Original Name"
	if _, err := svc.ExchangeToken(context.Background(), req); err != nil {
		t.Fatalf("[%s] first ExchangeToken: %v", t.Name(), err)
	}

	req.Name = "Updated Name"
	if _, err := svc.ExchangeToken(context.Background(), req); err != nil {
		t.Fatalf("[%s] second ExchangeToken: %v", t.Name(), err)
	}

	var name string
	pool.QueryRow(context.Background(),
		`SELECT name FROM users WHERE external_id=$1 AND event_id=$2`, "int-ext-update", eventID).Scan(&name)
	if name != "Updated Name" {
		t.Errorf("[%s] expected name 'Updated Name', got %q", t.Name(), name)
	}
	t.Logf("[%s] assert: user name updated on second call", t.Name())
}

func TestIntegration_ExchangeToken_ValidJWT(t *testing.T) {
	pool := testPool(t)
	eventID := seedEvent(t, pool, testSecret)
	svc := newIntegrationSvc(t, pool)

	token, err := svc.ExchangeToken(context.Background(), service.TokenRequest{
		ExternalUserID: "int-ext-jwt",
		EventID:        eventID,
		Name:           "JWT User",
		Role:           string(domain.RoleUser),
		APISecret:      testSecret,
	})
	if err != nil {
		t.Fatalf("[%s] ExchangeToken: %v", t.Name(), err)
	}

	claims, err := jwt.ValidateToken(token, testJWTSecret)
	if err != nil {
		t.Fatalf("[%s] ValidateToken: %v", t.Name(), err)
	}

	var userID uuid.UUID
	pool.QueryRow(context.Background(),
		`SELECT id FROM users WHERE external_id=$1 AND event_id=$2`, "int-ext-jwt", eventID).Scan(&userID)

	if claims.UserID != userID {
		t.Errorf("[%s] claims.UserID=%s, want %s", t.Name(), claims.UserID, userID)
	}
	t.Logf("[%s] assert: JWT claims.UserID matches DB user ID", t.Name())
}
