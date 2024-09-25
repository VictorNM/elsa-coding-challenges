package event

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

const (
	defaultPoolSize = 10000
	defaultTimeout  = 30 * time.Second
)

type Event interface {
	Name() string
}

type Handler func(ctx context.Context, e Event) error

// Bus is an in-memory event bus.
type Bus struct {
	pool     chan struct{}
	wg       *sync.WaitGroup
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewBus create a new event bus. Caller should call Stop for graceful shutdown the bus.
func NewBus() *Bus {
	return &Bus{
		pool:     make(chan struct{}, defaultPoolSize),
		wg:       new(sync.WaitGroup),
		handlers: make(map[string][]Handler),
	}
}

// Subscribe to an event
func (b *Bus) Subscribe(name string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[name] = append(b.handlers[name], h)
}

// Publish an event
func (b *Bus) Publish(ctx context.Context, e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, h := range b.handlers[e.Name()] {
		// TODO: isolate pool size for each handler, so a slow handler won't block other handlers
		b.dispatch(ctx, h, e)
	}
}

func (b *Bus) dispatch(ctx context.Context, h Handler, e Event) {
	b.wg.Add(1)

	b.pool <- struct{}{}

	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultTimeout)
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "event: handler panic",
					"error", fmt.Errorf("%v, stack: %s", r, debug.Stack()),
				)
			}

			cancel()
			<-b.pool
			b.wg.Done()
		}()

		if err := h(ctx, e); err != nil {
			slog.ErrorContext(ctx, "event: handle event failed",
				"error", err,
			)
		}
	}()
}

// Stop waits for all handlers to finish
func (b *Bus) Stop() {
	b.wg.Wait()
}
