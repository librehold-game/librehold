package apptest

import (
	"context"
	"sync"
	"testing"
	"time"

	"librehold/internal/app"
)

// RunCountersContract checks a Counters; newCounters returns an empty store, its window length for the test and a
// way to let time pass.
func RunCountersContract(t *testing.T, newCounters func(t *testing.T) (c app.Counters, window time.Duration, wait func(time.Duration))) {
	ctx := context.Background()
	add := func(t *testing.T, c app.Counters, key string, window time.Duration) int {
		t.Helper()
		n, err := c.Add(ctx, key, window)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	t.Run("counts per key", func(t *testing.T) {
		c, window, _ := newCounters(t)
		for i := 1; i <= 3; i++ {
			if n := add(t, c, "a", window); n != i {
				t.Fatalf("event %d of a counts %d", i, n)
			}
		}
		if n := add(t, c, "b", window); n != 1 {
			t.Fatalf("first event of b counts %d", n)
		}
	})

	t.Run("events at once get one count each", func(t *testing.T) {
		c, window, _ := newCounters(t)
		const events = 50
		seen := make([]bool, events+1)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for range events {
			wg.Go(func() {
				n, err := c.Add(ctx, "a", window)
				mu.Lock()
				defer mu.Unlock()
				if err != nil || n < 1 || n > events || seen[n] {
					t.Errorf("add returned %d, %v", n, err)
					return
				}
				seen[n] = true
			})
		}
		wg.Wait()
	})

	t.Run("the window opens with the first event and is not extended", func(t *testing.T) {
		c, window, wait := newCounters(t)
		add(t, c, "a", window)
		wait(window / 2)
		if n := add(t, c, "a", window); n != 2 {
			t.Fatalf("within the window: %d", n)
		}
		wait(window/2 + window/4)
		if n := add(t, c, "a", window); n != 1 {
			t.Fatalf("after the first event's window: %d", n)
		}
	})

	t.Run("undo takes back one event", func(t *testing.T) {
		c, window, _ := newCounters(t)
		add(t, c, "a", window)
		add(t, c, "a", window)
		if err := c.Undo(ctx, "a"); err != nil {
			t.Fatal(err)
		}
		if n := add(t, c, "a", window); n != 2 {
			t.Fatalf("after an undo: %d", n)
		}
	})

	t.Run("undo of a closed window opens none", func(t *testing.T) {
		c, window, wait := newCounters(t)
		add(t, c, "a", window)
		wait(window + window/4)
		if err := c.Undo(ctx, "a"); err != nil {
			t.Fatal(err)
		}
		if err := c.Undo(ctx, "never"); err != nil {
			t.Fatal(err)
		}
		if n := add(t, c, "a", window); n != 1 {
			t.Fatalf("after undoing a closed window: %d", n)
		}
		wait(window + window/4)
		if n := add(t, c, "a", window); n != 1 {
			t.Fatalf("the window after an undo never closed: %d", n)
		}
	})

	t.Run("clear forgets one key", func(t *testing.T) {
		c, window, _ := newCounters(t)
		add(t, c, "a", window)
		add(t, c, "b", window)
		if err := c.Clear(ctx, "a"); err != nil {
			t.Fatal(err)
		}
		if err := c.Clear(ctx, "never"); err != nil {
			t.Fatal(err)
		}
		if a, b := add(t, c, "a", window), add(t, c, "b", window); a != 1 || b != 2 {
			t.Fatalf("after clear a=%d b=%d", a, b)
		}
	})
}
