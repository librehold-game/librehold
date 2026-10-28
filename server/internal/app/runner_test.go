package app_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"librehold/internal/adapters/memory"
	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

type fakeClock struct{ ms atomic.Int64 }

func (c *fakeClock) NowMs() int64 { return c.ms.Load() }
func (c *fakeClock) Set(ms int64) { c.ms.Store(ms) }
func (c *fakeClock) Add(ms int64) { c.ms.Add(ms) }

type fixture struct {
	svc    *app.Service
	worlds *memory.Worlds
	clock  *fakeClock
	world  int64
	acct   *app.Account
	home   string
}

// newFixture is a world without bots started at the clock's start, with account 1 joined as h1.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{worlds: memory.NewWorlds(), clock: &fakeClock{}, acct: &app.Account{ID: 1, Username: "ala"}}
	row := apptest.NewWorldRow(0)
	f.clock.Set(row.StartedAtMs)
	f.svc = app.NewService(memory.NewAccounts(), f.worlds, memory.NewCounters(f.clock), f.clock, nil, nil, app.Config{})
	id, err := f.worlds.Create(context.Background(), row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	f.world = id
	err = f.worlds.Run(context.Background(), id, func(w app.WorldTx) error {
		v, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		if v != nil {
			f.home = v.ID
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) order(t *testing.T, fn func(w app.WorldTx, me game.PlayerID) error) error {
	t.Helper()
	return f.svc.InWorld(context.Background(), f.acct, f.world, fn)
}

func (f *fixture) village(t *testing.T) *game.Village {
	t.Helper()
	var v *game.Village
	err := f.worlds.Read(context.Background(), f.world, func(w app.WorldTx) error { v = w.Village(f.home); return nil })
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestWorldNowExcludesPause(t *testing.T) {
	row := &app.WorldRow{StartedAtMs: 1000, PausedTotalMs: 500}
	if got := app.WorldNow(row, 5000); got != 3500 {
		t.Fatalf("running: %d", got)
	}
	paused := int64(4000)
	row.PausedAtMs = &paused
	if got := app.WorldNow(row, 9999); got != 2500 {
		t.Fatalf("paused: %d", got)
	}
}

func TestProcessDueInOrderAndOnce(t *testing.T) {
	f := newFixture(t)
	start := f.village(t).Buildings
	var ends []int64
	for _, b := range []game.BuildingID{game.BWood, game.BClay} {
		err := f.order(t, func(w app.WorldTx, me game.PlayerID) error {
			_, err := game.EnqueueBuild(w, me, f.home, b)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, j := range f.village(t).BuildQueue {
		ends = append(ends, j.CompletesAt)
	}
	if len(ends) != 2 || ends[0] >= ends[1] {
		t.Fatalf("queue ends %v", ends)
	}
	var seen []string
	restore := game.SpyEvents(func(key string) { seen = append(seen, key) })
	defer restore()

	f.clock.Add(ends[1] + 1)
	n, more, err := f.svc.ProcessDue(context.Background(), f.world, 1)
	if err != nil || n != 1 || !more {
		t.Fatalf("limited: %d %v %v", n, more, err)
	}
	if lv := f.village(t).Buildings; lv[game.BWood] != start[game.BWood]+1 || lv[game.BClay] != start[game.BClay] {
		t.Fatalf("after one event: wood %d clay %d", lv[game.BWood], lv[game.BClay])
	}
	n2, more, err := f.svc.ProcessDue(context.Background(), f.world, 0)
	if err != nil || n2 < 1 || more {
		t.Fatalf("rest: %d %v %v", n2, more, err)
	}
	if lv := f.village(t).Buildings; lv[game.BClay] != start[game.BClay]+1 {
		t.Fatalf("clay %d", lv[game.BClay])
	}
	builds := 0
	for _, k := range seen {
		if k == "build:"+f.home {
			builds++
		}
	}
	if builds != 2 {
		t.Fatalf("build events %v", seen)
	}
	seen = nil
	if n, more, err := f.svc.ProcessDue(context.Background(), f.world, 0); err != nil || n != 0 || more || len(seen) != 0 {
		t.Fatalf("again: %d %v %v %v", n, more, err, seen)
	}
}

func TestConcurrentOrdersSerialise(t *testing.T) {
	f := newFixture(t)
	err := f.worlds.Run(context.Background(), f.world, func(w app.WorldTx) error {
		v := w.Village(f.home)
		v.Resources = game.BuildCost(game.BMain, v.Buildings[game.BMain]+1)
		v.ResourcesUpdatedAt = w.Now()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			<-start
			results <- f.order(t, func(w app.WorldTx, me game.PlayerID) error {
				_, err := game.EnqueueBuild(w, me, f.home, game.BMain)
				return err
			})
		})
	}
	close(start)
	wg.Wait()
	close(results)
	ok, blocked := 0, 0
	for err := range results {
		var be *game.BlockedError
		switch {
		case err == nil:
			ok++
		case errors.As(err, &be):
			blocked++
		default:
			t.Fatalf("unexpected %v", err)
		}
	}
	if ok != 1 || blocked != 1 {
		t.Fatalf("ok %d blocked %d", ok, blocked)
	}
	if r := f.village(t).Resources; r.Wood < 0 || r.Clay < 0 || r.Iron < 0 {
		t.Fatalf("resources %+v", r)
	}
}

func TestOrderAfterDueEvents(t *testing.T) {
	f := newFixture(t)
	before := f.village(t).Buildings[game.BMain]
	var end int64
	err := f.order(t, func(w app.WorldTx, me game.PlayerID) error {
		job, err := game.EnqueueBuild(w, me, f.home, game.BMain)
		if job != nil {
			end = job.CompletesAt
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Add(end + 1000)
	err = f.order(t, func(w app.WorldTx, me game.PlayerID) error {
		if got := w.Village(f.home).Buildings[game.BMain]; got != before+1 {
			t.Errorf("the order saw main %d, want %d", got, before+1)
		}
		if w.Now() != end+1000 {
			t.Errorf("now %d, want %d", w.Now(), end+1000)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOrderErrorRollsBackAndNonMembersAreRefused(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("boom")
	err := f.order(t, func(w app.WorldTx, me game.PlayerID) error {
		w.Village(f.home).Name = "Renamed"
		return boom
	})
	if !errors.Is(err, boom) || f.village(t).Name == "Renamed" {
		t.Fatalf("err %v, name %q", err, f.village(t).Name)
	}
	called := false
	err = f.svc.InWorld(context.Background(), &app.Account{ID: 99}, f.world, func(app.WorldTx, game.PlayerID) error {
		called = true
		return nil
	})
	if !errors.Is(err, app.ErrNotMember) || called {
		t.Fatalf("stranger: %v %v", err, called)
	}
}

func TestPausedWorldRefusesOrders(t *testing.T) {
	f := newFixture(t)
	err := f.worlds.Run(context.Background(), f.world, func(w app.WorldTx) error {
		at := f.clock.NowMs()
		w.Row().PausedAtMs = &at
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	err = f.order(t, func(app.WorldTx, game.PlayerID) error { called = true; return nil })
	if !errors.Is(err, app.ErrWorldPaused) || called {
		t.Fatalf("paused: %v %v", err, called)
	}
}

func TestStateCarriesTheInstantClock(t *testing.T) {
	ctx := context.Background()
	worlds, clock := memory.NewWorlds(), &fakeClock{}
	row := apptest.NewWorldRow(0)
	row.Config = game.InstantWorldConfig
	row.Config.BotCount = 2
	clock.Set(row.StartedAtMs)
	svc := app.NewService(memory.NewAccounts(), worlds, memory.NewCounters(clock), clock, nil, nil, app.Config{})
	id, err := worlds.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	ala := &app.Account{ID: 1, Username: "ala"}
	err = worlds.Run(ctx, id, func(w app.WorldTx) error {
		_, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := svc.State(ctx, ala, id)
	if err != nil || st.Instant == nil || st.Instant.EndsAt != 60*game.MsPerMinute || st.Instant.JoinUntil != 10*game.MsPerMinute || st.Instant.Ended != nil {
		t.Fatalf("instant view %+v %v", st.Instant, err)
	}
	clock.Add(61 * game.MsPerMinute)
	st, err = svc.State(ctx, ala, id)
	if err != nil || st.Instant.Ended == nil || st.Instant.Ended.Reason != "time" || st.Instant.Ended.WinnerName == "" {
		t.Fatalf("after the end %+v %v", st.Instant, err)
	}
	long, _ := newFixture(t).svc.State(ctx, &app.Account{ID: 1}, 1)
	if long != nil && long.Instant != nil {
		t.Fatal("a long world carries an instant view")
	}
}

// Review focus 2: once a world has ended it is read-only — orders are refused, the state stays readable.
func TestOrdersRefusedAfterEnd(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	err := f.worlds.Run(ctx, f.world, func(w app.WorldTx) error {
		at := w.Now()
		w.State().EndedAt, w.State().EndReason = &at, "time"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Add(1)
	ran := false
	if err := f.order(t, func(w app.WorldTx, me game.PlayerID) error { ran = true; return nil }); !errors.Is(err, app.ErrWorldEnded) || ran {
		t.Fatalf("order after the end: %v (ran %v)", err, ran)
	}
	if _, err := f.svc.State(ctx, f.acct, f.world); err != nil {
		t.Fatalf("state after the end: %v", err)
	}
}
