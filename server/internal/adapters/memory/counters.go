package memory

import (
	"context"
	"sync"
	"time"

	"librehold/internal/app"
)

// Counters keeps the limit windows of one process, on the app's clock; windows that closed are dropped as they
// are met, and all of them every sweepEvery adds.
type Counters struct {
	mu      sync.Mutex
	clock   app.Clock
	windows map[string]*window
	adds    int
}

type window struct {
	n        int
	closesAt int64
}

const sweepEvery = 1024

func NewCounters(clock app.Clock) *Counters {
	return &Counters{clock: clock, windows: map[string]*window{}}
}

// open is key's window if it is still open, dropping a closed one.
func (c *Counters) open(key string, now int64) *window {
	w := c.windows[key]
	if w != nil && now >= w.closesAt {
		delete(c.windows, key)
		return nil
	}
	return w
}

func (c *Counters) Add(_ context.Context, key string, d time.Duration) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock.NowMs()
	if c.adds++; c.adds%sweepEvery == 0 {
		for k := range c.windows {
			c.open(k, now)
		}
	}
	w := c.open(key, now)
	if w == nil {
		w = &window{closesAt: now + d.Milliseconds()}
		c.windows[key] = w
	}
	w.n++
	return w.n, nil
}

func (c *Counters) Undo(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if w := c.open(key, c.clock.NowMs()); w != nil && w.n > 0 {
		w.n--
	}
	return nil
}

func (c *Counters) Clear(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.windows, key)
	return nil
}
