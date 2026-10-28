package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func newEmptyWorld(t *testing.T) *gametest.MemWorld {
	t.Helper()
	cfg := game.DefaultWorldConfig
	cfg.WorldSpeed = 1
	cfg.BotCount = 0
	return gametest.NewMemWorld(1, cfg, 0)
}

func TestEvents(t *testing.T) {
	t.Run("order at equal times = scheduling order, not id string order", func(t *testing.T) {
		w := newEmptyWorld(t)
		var fired []string
		defer game.RegisterTestHandler("t", func(_ game.World, ref string) error {
			fired = append(fired, ref)
			return nil
		})()
		for _, ref := range []string{"c_9", "c_10", "c_a"} {
			w.Schedule("t:"+ref, 1000)
		}
		if err := gametest.Advance(w, 2000); err != nil {
			t.Fatal(err)
		}
		want := []string{"c_9", "c_10", "c_a"}
		if len(fired) != 3 || fired[0] != want[0] || fired[1] != want[1] || fired[2] != want[2] {
			t.Fatalf("fired %v, want %v", fired, want)
		}
	})

	t.Run("rescheduling a key invalidates the previous entry", func(t *testing.T) {
		w := newEmptyWorld(t)
		var fired []int64
		defer game.RegisterTestHandler("t", func(w game.World, _ string) error {
			fired = append(fired, w.Now())
			return nil
		})()
		w.Schedule("t:x", 1000)
		w.Schedule("t:x", 5000)
		if err := gametest.Advance(w, 10_000); err != nil {
			t.Fatal(err)
		}
		if len(fired) != 1 || fired[0] != 5000 {
			t.Fatalf("fired %v, want [5000]", fired)
		}
	})

	t.Run("a cancelled event does not fire", func(t *testing.T) {
		w := newEmptyWorld(t)
		n := 0
		defer game.RegisterTestHandler("t", func(game.World, string) error { n++; return nil })()
		w.Schedule("t:x", 1000)
		if !w.IsScheduled("t:x") {
			t.Fatal("not scheduled")
		}
		w.Unschedule("t:x")
		if w.IsScheduled("t:x") {
			t.Fatal("still scheduled")
		}
		if err := gametest.Advance(w, 10_000); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("fired %d times", n)
		}
	})

	t.Run("a time in the past moves to now", func(t *testing.T) {
		w := newEmptyWorld(t)
		w.SetNow(5000)
		w.Schedule("t:x", 1000)
		key, at, ok := w.PopDue(5000)
		if !ok || key != "t:x" || at != 5000 {
			t.Fatalf("PopDue = %q %d %v", key, at, ok)
		}
	})

	t.Run("PopDue does not pop future events", func(t *testing.T) {
		w := newEmptyWorld(t)
		w.Schedule("t:a", 3000)
		w.Schedule("t:b", 1000)
		if key, _, ok := w.PopDue(2000); !ok || key != "t:b" {
			t.Fatalf("first pop %q %v", key, ok)
		}
		if _, _, ok := w.PopDue(2000); ok {
			t.Fatal("popped a future event")
		}
		if !w.IsScheduled("t:a") {
			t.Fatal("future event lost")
		}
	})

	t.Run("Advance sets each event's time and the target time at the end", func(t *testing.T) {
		w := newEmptyWorld(t)
		var seen []int64
		defer game.RegisterTestHandler("t", func(w game.World, _ string) error {
			seen = append(seen, w.Now())
			return nil
		})()
		w.Schedule("t:a", 700)
		if err := gametest.Advance(w, 9000); err != nil {
			t.Fatal(err)
		}
		if len(seen) != 1 || seen[0] != 700 || w.Now() != 9000 {
			t.Fatalf("seen %v now %d", seen, w.Now())
		}
	})
}

func TestDispatchUnknownKindErrors(t *testing.T) {
	w := newEmptyWorld(t)
	if err := game.Dispatch(w, "nope:1"); err == nil {
		t.Fatal("expected an error for an unknown event kind")
	}
	w.Schedule("nope:1", 10)
	if err := gametest.Advance(w, 100); err == nil {
		t.Fatal("Advance must surface the dispatch error")
	}
}

func TestSpyEventsSeesEveryKey(t *testing.T) {
	w := newEmptyWorld(t)
	defer game.RegisterTestHandler("t", func(game.World, string) error { return nil })()
	var keys []string
	restore := game.SpyEvents(func(key string) { keys = append(keys, key) })
	w.Schedule("t:1", 10)
	w.Schedule("t:2", 20)
	if err := gametest.Advance(w, 100); err != nil {
		t.Fatal(err)
	}
	restore()
	w.Schedule("t:3", 200)
	if err := gametest.Advance(w, 300); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "t:1" || keys[1] != "t:2" {
		t.Fatalf("spy saw %v", keys)
	}
}

func TestNextGridTime(t *testing.T) {
	cases := []struct{ now, off, interval, want int64 }{
		{0, 0, 100, 100},
		{100, 0, 100, 200},
		{150, 30, 100, 230},
		{29, 30, 100, 30},
	}
	for _, c := range cases {
		if got := game.NextGridTime(c.now, c.off, c.interval); got != c.want {
			t.Errorf("NextGridTime(%d,%d,%d) = %d, want %d", c.now, c.off, c.interval, got, c.want)
		}
	}
}

func TestNewIDBase36(t *testing.T) {
	w := newEmptyWorld(t)
	first := w.NewID("c")
	if first != "c_1" {
		t.Fatalf("first id %q", first)
	}
	for range 34 {
		w.NewID("c")
	}
	if got := w.NewID("c"); got != "c_10" {
		t.Fatalf("36th id %q, want c_10", got)
	}
}
