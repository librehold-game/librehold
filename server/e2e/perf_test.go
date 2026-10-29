//go:build e2e && perf

package e2e

import (
	"context"
	"fmt"
	"librehold/internal/adapters/memory"
	"slices"
	"sync"
	"testing"
	"time"

	"librehold/internal/adapters/postgres"
	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

func percentile(xs []time.Duration, p float64) time.Duration {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s))*p))]
}

// DoD 11: 500 mixed orders from two players in a world of 100 bots, through HTTP, at real time.
func TestPerfOrderLatencyP95(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1)
	a := s.admin(t)
	wid := a.world(func(cfg *game.WorldConfig) { cfg.BotCount, cfg.BeginnerProtectionDays = 100, 0 })
	var players []*client
	for _, name := range []string{"ala", "bob"} {
		c := s.register(t, name)
		c.join(wid)
		boost(t, s.db, wid, c.home(wid).ID, army)
		players = append(players, c)
	}
	homes := map[*client]*game.Village{}
	barbs := map[*client]*app.MapTile{}
	for _, c := range players {
		v := c.home(wid)
		homes[c] = v
		barbs[c] = nearest(c.tiles(wid, v.X, v.Y, 20), v, func(t app.MapTile) bool { return t.OwnerID == "" })
	}
	orders := []func(c *client, i int) (string, any){
		func(c *client, _ int) (string, any) {
			return "build", map[string]string{"villageId": homes[c].ID, "building": "wood"}
		},
		func(c *client, _ int) (string, any) {
			return "recruit", map[string]any{"villageId": homes[c].ID, "unit": "spear", "count": 1}
		},
		func(c *client, i int) (string, any) {
			return "rename-village", map[string]string{"villageId": homes[c].ID, "name": fmt.Sprintf("Café Hill %d", i)}
		},
		func(c *client, i int) (string, any) { return "set-farm-filters", map[string]any{"radius": 10 + i%5} },
		func(c *client, _ int) (string, any) {
			b := barbs[c]
			return "send", map[string]any{"originId": homes[c].ID, "x": b.X, "y": b.Y, "units": map[string]int{"scout": 1}, "type": "attack"}
		},
		func(c *client, _ int) (string, any) { return "mark-all-reports-read", map[string]string{} },
	}
	var mu sync.Mutex
	var took []time.Duration
	var wg sync.WaitGroup
	for _, c := range players {
		wg.Go(func() {
			for i := range 250 {
				name, body := orders[i%len(orders)](c, i)
				start := time.Now()
				r := c.order(wid, name, body)
				d := time.Since(start)
				if r.status != 200 && r.status != 409 {
					t.Errorf("%s: %d %s", name, r.status, r.body)
					return
				}
				mu.Lock()
				took = append(took, d)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	p50, p95 := percentile(took, 0.5), percentile(took, 0.95)
	t.Logf("DoD 11: %d orders, p50 %v, p95 %v, max %v", len(took), p50, p95, percentile(took, 1))
	if len(took) != 500 || p95 > 200*time.Millisecond {
		t.Fatalf("p95 %v over %d orders", p95, len(took))
	}
}

// testClock runs scale times faster than the wall clock from its start, and reads as an evening whatever the
// hour the test runs at: bots sleep at night, so a night run would measure an idle world.
type testClock struct {
	start int64
	scale int64
}

func newTestClock(scale int64) testClock {
	return testClock{start: time.Now().UnixMilli(), scale: scale}
}

func (c testClock) NowMs() int64 {
	now := time.UnixMilli(c.start)
	evening := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, time.Local).UnixMilli()
	return evening + (time.Now().UnixMilli()-c.start)*c.scale
}

// latenessStore records, for every event a unit of work pops, how far behind world time it is.
type latenessStore struct {
	app.WorldStore
	clock app.Clock
	mu    sync.Mutex
	late  []time.Duration
}

type latenessTx struct {
	app.WorldTx
	s *latenessStore
}

func (s *latenessStore) Run(ctx context.Context, id int64, fn func(app.WorldTx) error) error {
	return s.WorldStore.Run(ctx, id, func(w app.WorldTx) error { return fn(&latenessTx{WorldTx: w, s: s}) })
}

func (t *latenessTx) PopDue(until int64) (string, int64, bool) {
	key, at, ok := t.WorldTx.PopDue(until)
	if ok {
		late := app.WorldNow(t.Row(), t.s.clock.NowMs()) - at
		t.s.mu.Lock()
		t.s.late = append(t.s.late, time.Duration(late)*time.Millisecond)
		t.s.mu.Unlock()
	}
	return key, at, ok
}

func inProcess(t *testing.T, clock app.Clock, poll time.Duration) (*app.Service, *latenessStore) {
	t.Helper()
	pool := pgtest.New(t)
	store := &latenessStore{WorldStore: postgres.NewWorlds(pool), clock: clock}
	svc := app.NewService(postgres.NewAccounts(pool), store, memory.NewCounters(clock), clock, security.NewArgon2(), security.NewTokens(),
		app.Config{WorkerPoll: poll})
	return svc, store
}

var perfAdmin = &app.Account{ID: 1, Username: "admin", IsAdmin: true}

// agedWorld is a world of 100 bots that has already lived two days (all processed), so the bots play at
// their usual pace rather than the quiet first hours.
func agedWorld(t *testing.T, svc *app.Service, store *latenessStore) int64 {
	t.Helper()
	cfg := game.DefaultWorldConfig
	cfg.BotCount = 100
	id, err := svc.CreateWorld(context.Background(), perfAdmin, "Benchmark", cfg)
	if err != nil {
		t.Fatal(err)
	}
	shift(t, store, id, 2*24*time.Hour)
	catchUp(t, svc, id)
	store.mu.Lock()
	store.late = nil
	store.mu.Unlock()
	return id
}

// shift moves a world's start back: as if that much wall time had passed without anything running.
func shift(t *testing.T, store *latenessStore, id int64, d time.Duration) {
	t.Helper()
	if err := store.WorldStore.Run(context.Background(), id, func(w app.WorldTx) error {
		w.Row().StartedAtMs -= d.Milliseconds()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func catchUp(t *testing.T, svc *app.Service, id int64) int {
	t.Helper()
	events := 0
	for {
		n, more, err := svc.ProcessDue(context.Background(), id, 500)
		if err != nil {
			t.Fatal(err)
		}
		events += n
		if !more {
			return events
		}
	}
}

// DoD 11: with 100 bots, events are processed within a second of being due, over ten minutes of real time.
// Real time on purpose: worker sleeps are measured on the clock, so a sped-up clock would only measure the
// poll interval.
func TestPerfEventLateness(t *testing.T) {
	const scale = 1
	clock := newTestClock(scale)
	svc, store := inProcess(t, clock, 30*time.Second)
	agedWorld(t, svc, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.RunWorkers(ctx, (&quietWakeups{}).feed()) }()
	time.Sleep(10 * time.Minute / scale)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	late := slices.Clone(store.late)
	store.mu.Unlock()
	if len(late) < 20 {
		t.Fatalf("only %d events in ten world minutes", len(late))
	}
	p50, p99 := percentile(late, 0.5), percentile(late, 0.99)
	t.Logf("DoD 11: %d events, lateness in world time p50 %v, p99 %v, max %v", len(late), p50, p99, percentile(late, 1))
	if p99 > time.Second {
		t.Fatalf("p99 lateness %v", p99)
	}
}

// DoD 11: a world of 100 bots that nobody processed for an hour catches up in under a minute.
func TestPerfCatchUpOneHour(t *testing.T) {
	clock := newTestClock(1)
	svc, store := inProcess(t, clock, time.Hour)
	id := agedWorld(t, svc, store)
	shift(t, store, id, time.Hour)
	start := time.Now()
	events := catchUp(t, svc, id)
	took := time.Since(start)
	t.Logf("DoD 11: caught up an hour (%d events) in %v", events, took)
	if events < 100 || took > time.Minute {
		t.Fatalf("%d events in %v", events, took)
	}
}

// quietWakeups is an EventWakeups that never fires: the workers rely on their own timers.
type quietWakeups struct{}

func (q *quietWakeups) feed() app.EventWakeups { return q }

func (*quietWakeups) Subscribe(ctx context.Context) (<-chan int64, error) {
	ch := make(chan int64)
	go func() { <-ctx.Done(); close(ch) }()
	return ch, nil
}
