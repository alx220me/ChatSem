package broker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// fanout holds a Redis PubSub subscription and all local subscriber channels for one chat.
type fanout struct {
	sub     *redis.PubSub
	cancel  context.CancelFunc
	clients []chan longpoll.Message
}

// RedisBroker implements longpoll.Broker using Redis Pub/Sub for cross-instance fan-out.
// One Redis subscription per chatID per process; in-process fan-out to individual client channels.
type RedisBroker struct {
	rdb  *redis.Client
	mu   sync.Mutex
	fans map[uuid.UUID]*fanout
}

// NewRedisBroker creates a RedisBroker backed by the given Redis client.
func NewRedisBroker(rdb *redis.Client) *RedisBroker {
	return &RedisBroker{
		rdb:  rdb,
		fans: make(map[uuid.UUID]*fanout),
	}
}

// Subscribe returns a buffered channel that receives messages for chatID.
// The first subscriber starts a Redis sub goroutine.
func (b *RedisBroker) Subscribe(chatID uuid.UUID) <-chan longpoll.Message {
	ch := make(chan longpoll.Message, 1)
	b.mu.Lock()
	defer b.mu.Unlock()

	f, ok := b.fans[chatID]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		sub := b.rdb.Subscribe(ctx, redisChannel(chatID))
		f = &fanout{sub: sub, cancel: cancel}
		b.fans[chatID] = f
		go b.startReader(chatID, f, ctx)
		slog.Debug("redis broker: started redis sub", "chat_id", chatID)
	}
	f.clients = append(f.clients, ch)
	slog.Debug("redis broker: new subscriber", "chat_id", chatID, "total_clients", len(f.clients))
	return ch
}

// Unsubscribe removes the channel from chatID's fan-out.
// When the last client leaves, the Redis subscription is closed.
func (b *RedisBroker) Unsubscribe(chatID uuid.UUID, ch <-chan longpoll.Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	f, ok := b.fans[chatID]
	if !ok {
		return
	}
	for i, c := range f.clients {
		if c == ch {
			f.clients = append(f.clients[:i], f.clients[i+1:]...)
			break
		}
	}
	slog.Debug("redis broker: client unsubscribed", "chat_id", chatID, "remaining", len(f.clients))
	if len(f.clients) == 0 {
		f.cancel()
		f.sub.Close()
		delete(b.fans, chatID)
		slog.Debug("redis broker: last client left, closing redis sub", "chat_id", chatID)
	}
}

// Publish sends data to the Redis channel for chatID.
func (b *RedisBroker) Publish(ctx context.Context, chatID uuid.UUID, data []byte) error {
	err := b.rdb.Publish(ctx, redisChannel(chatID), data).Err()
	if err != nil {
		return fmt.Errorf("redis broker: publish chat %s: %w", chatID, err)
	}
	slog.Debug("redis broker: published", "chat_id", chatID, "bytes", len(data))
	return nil
}

// startReader reads messages from Redis and fan-outs to local client channels.
// Panics are recovered to keep the goroutine alive.
func (b *RedisBroker) startReader(chatID uuid.UUID, f *fanout, ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("redis broker: reader panic recovered", "chat_id", chatID, "panic", r)
		}
	}()

	ch := f.sub.Channel()
	for {
		select {
		case <-ctx.Done():
			slog.Debug("redis broker: reader context done", "chat_id", chatID)
			return
		case msg, ok := <-ch:
			if !ok {
				slog.Debug("redis broker: redis channel closed", "chat_id", chatID)
				return
			}
			b.fanOut(chatID, []byte(msg.Payload))
		}
	}
}

// fanOut delivers a message to all current client channels for chatID.
func (b *RedisBroker) fanOut(chatID uuid.UUID, data []byte) {
	b.mu.Lock()
	f, ok := b.fans[chatID]
	if !ok {
		b.mu.Unlock()
		return
	}
	clients := make([]chan longpoll.Message, len(f.clients))
	copy(clients, f.clients)
	b.mu.Unlock()

	msg := longpoll.Message{ChatID: chatID, Data: data}
	for _, ch := range clients {
		select {
		case ch <- msg:
		default:
			// Slow client: skip to avoid blocking fan-out goroutine.
			slog.Debug("redis broker: slow client, message dropped", "chat_id", chatID)
		}
	}
}

func redisChannel(chatID uuid.UUID) string {
	return fmt.Sprintf("chat:%s", chatID)
}
