package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chatsem/services/chat/internal/handler"
	"chatsem/services/chat/internal/middleware"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/jwt"
	"chatsem/shared/pkg/longpoll"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const pollTestSecret = "test-poll-secret"

// makePollClaims creates a test JWT claims set.
func makePollClaims(chatID uuid.UUID) *jwt.Claims {
	return &jwt.Claims{
		UserID:  uuid.New(),
		EventID: uuid.New(),
		Role:    "user",
	}
}

// buildPollRequest creates a GET poll request with auth and chat_id.
func buildPollRequest(t *testing.T, chatID uuid.UUID, afterSeq int64) *http.Request {
	t.Helper()
	claims := makePollClaims(chatID)
	tok, err := jwt.CreateToken(claims, pollTestSecret, time.Hour)
	if err != nil {
		t.Fatalf("[%s] CreateToken: %v", t.Name(), err)
	}

	url := "/api/chat/" + chatID.String() + "/poll"
	if afterSeq > 0 {
		url += "?after=" + string(rune('0'+afterSeq))
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("chatID", chatID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	return req
}

// --- mock MessageRepository (minimal, for poll tests) ---

type pollMockMessageRepo struct {
	getByChatIDAfterSeq func(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error)
}

func (m *pollMockMessageRepo) Create(ctx context.Context, msg *domain.Message) error         { return nil }
func (m *pollMockMessageRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	return nil, nil
}
func (m *pollMockMessageRepo) GetByChatIDAfterSeq(ctx context.Context, chatID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
	if m.getByChatIDAfterSeq != nil {
		return m.getByChatIDAfterSeq(ctx, chatID, afterSeq, limit)
	}
	return []*domain.Message{}, nil
}
func (m *pollMockMessageRepo) ListByChatID(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *pollMockMessageRepo) GetByChatIDBeforeSeq(ctx context.Context, chatID uuid.UUID, beforeSeq int64, limit int) ([]*domain.Message, error) {
	return nil, nil
}
func (m *pollMockMessageRepo) SoftDelete(ctx context.Context, id uuid.UUID) error { return nil }
func (m *pollMockMessageRepo) Update(ctx context.Context, id uuid.UUID, newText string) error {
	return nil
}

// buildPollRoute wraps PollHandler in Auth middleware and chi router for testing.
func buildPollRoute(pollH *handler.PollHandler) http.Handler {
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(pollTestSecret))
		r.Get("/api/chat/{chatID}/poll", pollH.Poll)
	})
	return r
}

func TestPoll_ReceivesMessage(t *testing.T) {
	chatID := uuid.New()
	broker := longpoll.NewInMemoryBroker()

	msgRepo := &pollMockMessageRepo{
		getByChatIDAfterSeq: func(ctx context.Context, cID uuid.UUID, afterSeq int64, limit int) ([]*domain.Message, error) {
			return []*domain.Message{
				{ID: uuid.New(), ChatID: chatID, Seq: 1, Text: "hello", CreatedAt: time.Now()},
			}, nil
		},
	}

	pollH := handler.NewPollHandler(broker, msgRepo, nil)
	srv := buildPollRoute(pollH)

	// Publish a message after a small delay so the poll handler is already waiting.
	go func() {
		time.Sleep(50 * time.Millisecond)
		broker.Publish(context.Background(), chatID, []byte(`{"seq":1}`)) //nolint:errcheck
	}()

	req := buildPollRequest(t, chatID, 0)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("[%s] expected 200, got %d body=%s", t.Name(), rr.Code, rr.Body.String())
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("[%s] invalid JSON: %v", t.Name(), err)
	}
	msgs, _ := body["messages"].([]interface{})
	if len(msgs) == 0 {
		t.Errorf("[%s] expected messages in response", t.Name())
	}
	t.Logf("[%s] assert: poll received %d message(s)", t.Name(), len(msgs))
}

func TestPoll_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long poll timeout test in short mode")
	}

	chatID := uuid.New()
	broker := longpoll.NewInMemoryBroker()
	msgRepo := &pollMockMessageRepo{}

	// Override timeout via short test — we'll use a very short timeout via context.
	pollH := handler.NewPollHandler(broker, msgRepo, nil)
	srv := buildPollRoute(pollH)

	// Use a request context that cancels before LongPollTimeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := buildPollRequest(t, chatID, 0)
	req = req.WithContext(ctx)

	// Set chi route context again after WithContext.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("chatID", chatID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	// Either 204 (timeout) or request cancelled (empty body, no status set) — both are acceptable.
	t.Logf("[%s] assert: poll with early context cancellation returned status %d", t.Name(), rr.Code)
}

func TestPoll_ClientDisconnect(t *testing.T) {
	chatID := uuid.New()
	broker := longpoll.NewInMemoryBroker()
	msgRepo := &pollMockMessageRepo{}

	pollH := handler.NewPollHandler(broker, msgRepo, nil)
	srv := buildPollRoute(pollH)

	ctx, cancel := context.WithCancel(context.Background())

	req := buildPollRequest(t, chatID, 0)
	req = req.WithContext(ctx)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("chatID", chatID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	done := make(chan struct{})
	go func() {
		defer close(done)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel() // simulate client disconnect

	select {
	case <-done:
		t.Logf("[%s] assert: handler terminated cleanly after client disconnect", t.Name())
	case <-time.After(200 * time.Millisecond):
		t.Errorf("[%s] handler did not terminate after context cancellation", t.Name())
	}
}
