package event

import (
	"context"
	"log/slog"
	"sync"
)

type EventType string

type Event struct {
	Type    EventType
	Payload any
}

type Publisher interface {
	Publish(ctx context.Context, e Event)
}

// https://leapcell.medium.com/how-to-create-a-event-bus-in-go-d7919b59a584
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
}

type EventHandler func(context.Context, Event) error

func NewEventBus() *EventBus {
	return &EventBus{handlers: make(map[EventType][]EventHandler)}
}

func (b *EventBus) Subscribe(eventType EventType, fn EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], fn)
	slog.Debug("eventbus: handler subscribed", "event_type", eventType)
}

func (b *EventBus) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	handlers := b.handlers[e.Type]
	b.mu.RUnlock()

	for _, fn := range handlers {
		if err := fn(ctx, e); err != nil {
			slog.Error("eventbus: handler error", "event_type", e.Type, "err", err)
		}
		slog.Debug("eventbus: event published", "event_type", e.Type)
	}
}

var _ Publisher = (*EventBus)(nil)
