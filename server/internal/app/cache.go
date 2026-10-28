package app

import (
	"sync"
	"time"
)

// ttlCache keeps each loaded value for ttl; its zero value is ready. Errors are never kept.
type ttlCache[K comparable, V any] struct {
	mu    sync.Mutex
	items map[K]cached[V]
}

type cached[V any] struct {
	at    int64
	value V
}

// get returns the value kept for key if it is younger than ttl, else loads, keeps and returns a fresh one.
// Two callers that miss at once may both load; the later one is kept.
func (c *ttlCache[K, V]) get(key K, nowMs int64, ttl time.Duration, load func() (V, error)) (V, error) {
	c.mu.Lock()
	it, ok := c.items[key]
	c.mu.Unlock()
	if ok && nowMs-it.at < ttl.Milliseconds() {
		return it.value, nil
	}
	v, err := load()
	if err != nil {
		var zero V
		return zero, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[K]cached[V]{}
	}
	for k, old := range c.items {
		if nowMs-old.at >= ttl.Milliseconds() {
			delete(c.items, k)
		}
	}
	c.items[key] = cached[V]{at: nowMs, value: v}
	return v, nil
}
