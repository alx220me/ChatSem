package longpoll

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// LongPollTimeout is the maximum time a poll request waits for new messages.
	LongPollTimeout = 25 * time.Second
	// LongPollSettleDelay waits briefly after the first message to batch closely-timed messages.
	LongPollSettleDelay = 50 * time.Millisecond
)

// Message is the unit delivered by the broker to poll subscribers.
type Message struct {
	ChatID uuid.UUID
	Data   []byte
}

// Broker is the interface for fan-out message delivery to long-poll subscribers.
type Broker interface {
	Subscribe(chatID uuid.UUID) <-chan Message
	Unsubscribe(chatID uuid.UUID, ch <-chan Message)
	Publish(ctx context.Context, chatID uuid.UUID, data []byte) error
}

// InMemoryBroker is a pure-Go in-process broker for testing.
// It does not use Redis; use RedisBroker in production.
type InMemoryBroker struct {
	mu   sync.Mutex
	subs map[uuid.UUID][]chan Message
}

// NewInMemoryBroker creates an InMemoryBroker.
func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{subs: make(map[uuid.UUID][]chan Message)}
}

// Subscribe returns a buffered channel that receives messages for chatID.
func (b *InMemoryBroker) Subscribe(chatID uuid.UUID) <-chan Message {
	ch := make(chan Message, 1)
	b.mu.Lock()
	b.subs[chatID] = append(b.subs[chatID], ch)
	b.mu.Unlock()
	slog.Debug("inmemory broker: new subscriber", "chat_id", chatID)
	return ch
}

// Unsubscribe removes the channel from the chatID fan-out list.
func (b *InMemoryBroker) Unsubscribe(chatID uuid.UUID, ch <-chan Message) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := b.subs[chatID]
	for i, c := range list {
		if c == ch {
			b.subs[chatID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	if len(b.subs[chatID]) == 0 {
		delete(b.subs, chatID)
		slog.Debug("inmemory broker: last subscriber left", "chat_id", chatID)
	}
}

// Publish delivers data to all current subscribers of chatID.
// Slow subscribers are skipped (non-blocking send).
func (b *InMemoryBroker) Publish(_ context.Context, chatID uuid.UUID, data []byte) error {
	b.mu.Lock()
	list := make([]chan Message, len(b.subs[chatID]))
	copy(list, b.subs[chatID])
	b.mu.Unlock()

	msg := Message{ChatID: chatID, Data: data}
	for _, ch := range list {
		select {
		case ch <- msg:
		default:
			slog.Debug("inmemory broker: slow subscriber, message dropped", "chat_id", chatID)
		}
	}
	return nil
}
