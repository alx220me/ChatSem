package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"chatsem/shared/domain"

	"github.com/google/uuid"
)

const exportBatchSize = 100

// messageReader is the minimal interface ExportService needs from a message repository.
type messageReader interface {
	GetByChatRange(ctx context.Context, chatID uuid.UUID, from, to *time.Time, limit, offset int) ([]*domain.Message, error)
}

// ExportService streams chat messages to an io.Writer in CSV or NDJSON format.
type ExportService struct {
	messages messageReader
}

// NewExportService creates an ExportService backed by the given message reader.
func NewExportService(messages messageReader) *ExportService {
	return &ExportService{messages: messages}
}

// ExportMessages streams all non-deleted messages for chatID within the optional time range
// to w, formatted as "csv" or "json" (NDJSON). Each batch is flushed immediately.
// Returns domain.ErrInvalidFormat for unknown formats.
func (s *ExportService) ExportMessages(ctx context.Context, w io.Writer, chatID uuid.UUID, format string, from, to *time.Time) error {
	slog.Debug("[ExportService] starting", "chat_id", chatID, "format", format)

	switch format {
	case "csv":
		return s.exportCSV(ctx, w, chatID, from, to)
	case "json":
		return s.exportNDJSON(ctx, w, chatID, from, to)
	default:
		slog.Warn("[ExportService] unknown format", "format", format)
		return domain.ErrInvalidFormat
	}
}

func (s *ExportService) exportCSV(ctx context.Context, w io.Writer, chatID uuid.UUID, from, to *time.Time) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "seq", "text", "user_id", "created_at", "reply_to_id"}); err != nil {
		return fmt.Errorf("ExportService.exportCSV: write header: %w", err)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("ExportService.exportCSV: flush header: %w", err)
	}

	batchCount := 0
	offset := 0
	for {
		batch, err := s.messages.GetByChatRange(ctx, chatID, from, to, exportBatchSize, offset)
		if err != nil {
			return fmt.Errorf("ExportService.exportCSV: fetch batch %d: %w", batchCount, err)
		}
		if len(batch) == 0 {
			break
		}
		batchCount++
		slog.Debug("[ExportService] batch written", "format", "csv", "batch_num", batchCount, "count", len(batch))

		for _, msg := range batch {
			replyToID := ""
			if msg.ReplyToID != nil {
				replyToID = msg.ReplyToID.String()
			}
			if err := cw.Write([]string{
				msg.ID.String(),
				strconv.FormatInt(msg.Seq, 10),
				msg.Text,
				msg.UserID.String(),
				msg.CreatedAt.Format(time.RFC3339),
				replyToID,
			}); err != nil {
				return fmt.Errorf("ExportService.exportCSV: write row: %w", err)
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return fmt.Errorf("ExportService.exportCSV: flush batch %d: %w", batchCount, err)
		}
		offset += exportBatchSize
	}

	slog.Info("[ExportService] done", "chat_id", chatID, "format", "csv", "total_batches", batchCount)
	return nil
}

func (s *ExportService) exportNDJSON(ctx context.Context, w io.Writer, chatID uuid.UUID, from, to *time.Time) error {
	enc := json.NewEncoder(w)

	batchCount := 0
	offset := 0
	for {
		batch, err := s.messages.GetByChatRange(ctx, chatID, from, to, exportBatchSize, offset)
		if err != nil {
			return fmt.Errorf("ExportService.exportNDJSON: fetch batch %d: %w", batchCount, err)
		}
		if len(batch) == 0 {
			break
		}
		batchCount++
		slog.Debug("[ExportService] batch written", "format", "json", "batch_num", batchCount, "count", len(batch))

		for _, msg := range batch {
			if err := enc.Encode(msg); err != nil {
				return fmt.Errorf("ExportService.exportNDJSON: encode message: %w", err)
			}
		}
		offset += exportBatchSize
	}

	slog.Info("[ExportService] done", "chat_id", chatID, "format", "json", "total_batches", batchCount)
	return nil
}
