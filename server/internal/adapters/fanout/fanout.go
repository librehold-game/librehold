// Package fanout delivers values to every current subscriber. Values are refresh hints: a subscriber that
// falls more than its buffer behind loses the overflow rather than blocking the publisher.
package fanout

import (
	"context"
	"sync"
)

const buffer = 1024

type Hub[T any] struct {
	mu   sync.Mutex
	subs map[chan T]struct{}
}

func New[T any]() *Hub[T] { return &Hub[T]{subs: map[chan T]struct{}{}} }

// Subscribe returns a channel of every value published from now on; it closes when ctx ends.
func (h *Hub[T]) Subscribe(ctx context.Context) (<-chan T, error) {
	ch := make(chan T, buffer)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.subs, ch)
		close(ch)
		h.mu.Unlock()
	}()
	return ch, nil
}

func (h *Hub[T]) Publish(v T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- v:
		default:
		}
	}
}
