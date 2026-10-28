package app_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

// startWorkers runs the service's workers until the test ends.
func startWorkers(t *testing.T, svc *app.Service, wake app.EventWakeups) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.RunWorkers(ctx, wake) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
}

func eventually(t *testing.T, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.After(within)
	for !cond() {
		select {
		case <-deadline:
			t.Fatalf("not within %v", within)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func workerFixture(t *testing.T, poll time.Duration) *fixture {
	t.Helper()
	f := newFixture(t)
	f.svc = app.NewService(memory.NewAccounts(), f.worlds, memory.NewCounters(f.clock), f.clock, nil, nil, app.Config{WorkerPoll: poll})
	return f
}

func TestWorkerFiresOnTime(t *testing.T) {
	f := workerFixture(t, 50*time.Millisecond)
	start := f.clock.NowMs()
	level := f.village(t).Buildings[game.BWood]
	var end int64
	err := f.order(t, func(w app.WorldTx, me game.PlayerID) error {
		job, err := game.EnqueueBuild(w, me, f.home, game.BWood)
		if job != nil {
			end = job.CompletesAt
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Set(start + end - 2000)
	startWorkers(t, f.svc, f.worlds.Wakeups())
	time.Sleep(100 * time.Millisecond)
	if f.village(t).Buildings[game.BWood] != level {
		t.Fatal("the build finished early")
	}
	f.clock.Set(start + end + 1)
	eventually(t, time.Second, func() bool { return f.village(t).Buildings[game.BWood] == level+1 })
}

func TestWorkerWakesOnEarlierEvent(t *testing.T) {
	f := workerFixture(t, time.Hour)
	start := f.clock.NowMs()
	level := f.village(t).Buildings[game.BWood]
	startWorkers(t, f.svc, f.worlds.Wakeups())
	time.Sleep(50 * time.Millisecond)
	// The worker now sleeps until the next event it knew of; a build finishing earlier must wake it.
	buildMs := game.BuildTimeMs(game.BWood, level+1, f.village(t).Buildings[game.BMain], 1)
	f.clock.Set(start + buildMs + 60_000)
	err := f.worlds.Run(context.Background(), f.world, func(w app.WorldTx) error {
		_, err := game.EnqueueBuild(w, "h1", f.home, game.BWood)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	eventually(t, time.Second, func() bool { return f.village(t).Buildings[game.BWood] == level+1 })
}

type keyLog struct {
	mu   sync.Mutex
	keys []string
}

func (l *keyLog) add(k string) { l.mu.Lock(); l.keys = append(l.keys, k); l.mu.Unlock() }
func (l *keyLog) take() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := l.keys
	l.keys = nil
	return k
}

// backlogWorld is 100 bots and account 1, three hours after the start with nothing processed yet.
func backlogWorld(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{worlds: memory.NewWorlds(), clock: &fakeClock{}, acct: &app.Account{ID: 1}}
	row := apptest.NewWorldRow(100)
	f.clock.Set(row.StartedAtMs)
	f.svc = app.NewService(memory.NewAccounts(), f.worlds, memory.NewCounters(f.clock), f.clock, nil, nil, app.Config{WorkerPoll: 50 * time.Millisecond})
	id, err := f.worlds.Create(context.Background(), row, func(w app.WorldTx) error {
		if err := game.NewWorld(w); err != nil {
			return err
		}
		_, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.world = id
	f.clock.Set(row.StartedAtMs + 3*3_600_000)
	return f
}

func TestWorkerCatchUpThenOrder(t *testing.T) {
	log := &keyLog{}
	restore := game.SpyEvents(log.add)
	defer restore()

	ref := backlogWorld(t)
	if _, more, err := ref.svc.ProcessDue(context.Background(), ref.world, 0); err != nil || more {
		t.Fatal(err, more)
	}
	want := log.take()
	if len(want) <= 500 {
		t.Fatalf("backlog of only %d events", len(want))
	}

	f := backlogWorld(t)
	startWorkers(t, f.svc, f.worlds.Wakeups())
	err := f.order(t, func(w app.WorldTx, me game.PlayerID) error {
		if key, at, ok := w.PopDue(w.Now()); ok {
			t.Errorf("the order ran before %s at %d", key, at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	eventually(t, 5*time.Second, func() bool {
		got = append(got, log.take()...)
		return len(got) >= len(want)
	})
	time.Sleep(100 * time.Millisecond)
	got = append(got, log.take()...)
	if len(got) != len(want) {
		t.Fatalf("%d dispatches, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dispatch %d: %s, want %s", i, got[i], want[i])
		}
	}
}
