package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"chatsem/services/admin/internal/service"
	"chatsem/shared/domain"
	"chatsem/shared/pkg/jwt"
	"chatsem/shared/pkg/response"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// exportSvc is the interface ExportHandler needs from ExportService.
type exportSvc interface {
	ExportMessages(ctx context.Context, w io.Writer, chatID uuid.UUID, format string, from, to *time.Time) error
}

// ExportHandler handles streaming message export requests.
type ExportHandler struct {
	svc       exportSvc
	jwtSecret string
}

// NewExportHandler creates an ExportHandler backed by the given service.
func NewExportHandler(svc *service.ExportService, jwtSecret string) *ExportHandler {
	return &ExportHandler{svc: svc, jwtSecret: jwtSecret}
}

// Export handles GET /api/admin/chats/{chatID}/export.
// Auth is accepted from Authorization header OR ?token= query param to support <a download> links.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	chatIDStr := chi.URLParam(r, "chatID")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "bad_request", "invalid chat_id")
		return
	}

	// Resolve JWT from header first, then query param.
	tokenStr := ""
	if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
		tokenStr = strings.TrimPrefix(hdr, "Bearer ")
	} else if q := r.URL.Query().Get("token"); q != "" {
		tokenStr = q
	}
	if tokenStr == "" {
		slog.Warn("[ExportHandler.Export] missing token", "chat_id", chatID)
		response.Error(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}

	claims, err := jwt.ValidateToken(tokenStr, h.jwtSecret)
	if err != nil {
		slog.Warn("[ExportHandler.Export] invalid token", "chat_id", chatID, "err", err)
		response.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	if claims.Role != "admin" && claims.Role != "moderator" {
		slog.Warn("[ExportHandler.Export] insufficient role", "user_id", claims.UserID, "role", claims.Role)
		response.Error(w, http.StatusForbidden, "forbidden", "admin or moderator role required")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	var from, to *time.Time
	if s := r.URL.Query().Get("from"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid from: use RFC3339")
			return
		}
		from = &t
	}
	if s := r.URL.Query().Get("to"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "bad_request", "invalid to: use RFC3339")
			return
		}
		to = &t
	}

	slog.Debug("[ExportHandler.Export] request", "chat_id", chatID, "format", format, "from", from, "to", to, "user_id", claims.UserID)

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="chat-%s.csv"`, chatID))
	case "json":
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="chat-%s.ndjson"`, chatID))
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Wrap writer to flush after each ExportService write.
	fw := &flushWriter{w: w}
	if flusher, ok := w.(http.Flusher); ok {
		fw.flusher = flusher
	}

	slog.Info("[ExportHandler.Export] streaming started", "chat_id", chatID)

	if err := h.svc.ExportMessages(r.Context(), fw, chatID, format, from, to); err != nil {
		if r.Context().Err() != nil {
			slog.Warn("[ExportHandler.Export] client disconnected", "chat_id", chatID)
			return
		}
		if err == domain.ErrInvalidFormat {
			response.Error(w, http.StatusBadRequest, "bad_request", "format must be csv or json")
			return
		}
		if err == domain.ErrChatNotFound {
			response.Error(w, http.StatusNotFound, "not_found", "chat not found")
			return
		}
		slog.Warn("[ExportHandler.Export] export error", "chat_id", chatID, "err", err)
		// Headers already sent; nothing more we can do.
		return
	}
}

// flushWriter wraps http.ResponseWriter and flushes after every write when available.
type flushWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.flusher != nil {
		fw.flusher.Flush()
	}
	return n, err
}
