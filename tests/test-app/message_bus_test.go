package testapp_test

import (
	"context"
	"strconv"
	"sync"

	"github.com/kruczekrobert/bididi"
)

type memoryMessageBus struct {
	mu        sync.Mutex
	nextID    int
	handlers  map[string]func(context.Context, []byte) error
	observers map[string][]*memoryMessageObserver
}

type memoryMessageObserver struct {
	bus         *memoryMessageBus
	destination string
	messages    chan bdd.Message
}

func newMemoryMessageBus() *memoryMessageBus {
	return &memoryMessageBus{
		handlers:  make(map[string]func(context.Context, []byte) error),
		observers: make(map[string][]*memoryMessageObserver),
	}
}

func (b *memoryMessageBus) Publish(ctx context.Context, destination string, data []byte) error {
	b.mu.Lock()
	b.nextID++
	id := strconv.Itoa(b.nextID)
	observers := append([]*memoryMessageObserver(nil), b.observers[destination]...)
	handler := b.handlers[destination]
	b.mu.Unlock()

	for _, observer := range observers {
		message := bdd.Message{ID: id, Data: append([]byte(nil), data...)}
		select {
		case observer.messages <- message:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if handler != nil {
		return handler(ctx, data)
	}
	return nil
}

func (b *memoryMessageBus) Observe(_ context.Context, destination string) (bdd.MessageObserver, error) {
	observer := &memoryMessageObserver{
		bus:         b,
		destination: destination,
		messages:    make(chan bdd.Message, 32),
	}
	b.mu.Lock()
	b.observers[destination] = append(b.observers[destination], observer)
	b.mu.Unlock()
	return observer, nil
}

func (o *memoryMessageObserver) Next(ctx context.Context) (bdd.Message, error) {
	select {
	case message := <-o.messages:
		return message, nil
	case <-ctx.Done():
		return bdd.Message{}, ctx.Err()
	}
}

func (o *memoryMessageObserver) Close() error {
	o.bus.mu.Lock()
	defer o.bus.mu.Unlock()
	observers := o.bus.observers[o.destination]
	for i, observer := range observers {
		if observer == o {
			o.bus.observers[o.destination] = append(observers[:i], observers[i+1:]...)
			break
		}
	}
	return nil
}
