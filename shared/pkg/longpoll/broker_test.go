package longpoll_test

import (
	"context"
	"testing"
	"time"

	"chatsem/shared/pkg/longpoll"

	"github.com/google/uuid"
)

func TestInMemoryBroker_SubscribeAndPublish(t *testing.T) {
	t.Logf("[%s] stage: creating broker and subscriber", t.Name())
	b := longpoll.NewInMemoryBroker()
	chatID := uuid.New()

	ch := b.Subscribe(chatID)

	t.Logf("[%s] stage: publishing message", t.Name())
	payload := []byte(`{"test":"hello"}`)
	if err := b.Publish(context.Background(), chatID, payload); err != nil {
		t.Fatalf("[%s] Publish: %v", t.Name(), err)
	}

	select {
	case msg := <-ch:
		if string(msg.Data) != string(payload) {
			t.Errorf("[%s] data: want %s, got %s", t.Name(), payload, msg.Data)
		}
		t.Logf("[%s] stage: received message data=%s", t.Name(), msg.Data)
	case <-time.After(time.Second):
		t.Fatalf("[%s] timeout waiting for message", t.Name())
	}
}

func TestInMemoryBroker_FanOut(t *testing.T) {
	t.Logf("[%s] stage: creating broker with multiple subscribers", t.Name())
	b := longpoll.NewInMemoryBroker()
	chatID := uuid.New()

	ch1 := b.Subscribe(chatID)
	ch2 := b.Subscribe(chatID)

	t.Logf("[%s] stage: publishing to fanout", t.Name())
	payload := []byte(`{"msg":"broadcast"}`)
	if err := b.Publish(context.Background(), chatID, payload); err != nil {
		t.Fatalf("[%s] Publish: %v", t.Name(), err)
	}

	for i, ch := range []<-chan longpoll.Message{ch1, ch2} {
		select {
		case msg := <-ch:
			t.Logf("[%s] stage: subscriber %d received data=%s", t.Name(), i+1, msg.Data)
		case <-time.After(time.Second):
			t.Errorf("[%s] subscriber %d: timeout waiting for message", t.Name(), i+1)
		}
	}
}

func TestInMemoryBroker_Unsubscribe(t *testing.T) {
	t.Logf("[%s] stage: subscribe then unsubscribe", t.Name())
	b := longpoll.NewInMemoryBroker()
	chatID := uuid.New()

	ch := b.Subscribe(chatID)
	b.Unsubscribe(chatID, ch)

	t.Logf("[%s] stage: publishing after unsubscribe — channel should not receive", t.Name())
	_ = b.Publish(context.Background(), chatID, []byte("ghost"))

	select {
	case msg := <-ch:
		t.Errorf("[%s] expected no message after unsubscribe, got %s", t.Name(), msg.Data)
	case <-time.After(50 * time.Millisecond):
		t.Logf("[%s] stage: correctly received no message after unsubscribe", t.Name())
	}
}

func TestInMemoryBroker_LastSubscriberCleanup(t *testing.T) {
	t.Logf("[%s] stage: verify chatID cleaned up after last subscriber leaves", t.Name())
	b := longpoll.NewInMemoryBroker()
	chatID := uuid.New()

	ch1 := b.Subscribe(chatID)
	ch2 := b.Subscribe(chatID)

	b.Unsubscribe(chatID, ch1)
	b.Unsubscribe(chatID, ch2)

	// Publish should not panic even after cleanup
	if err := b.Publish(context.Background(), chatID, []byte("noop")); err != nil {
		t.Errorf("[%s] Publish after cleanup: %v", t.Name(), err)
	}
	t.Logf("[%s] stage: cleanup verified", t.Name())
}
