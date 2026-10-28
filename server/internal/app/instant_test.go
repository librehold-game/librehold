package app_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

type instantFixture struct {
	svc    *app.Service
	worlds *memory.Worlds
	clock  *fakeClock
	n      int
}

func newInstantFixture(t *testing.T, cfg app.Config) *instantFixture {
	t.Helper()
	f := &instantFixture{worlds: memory.NewWorlds(), clock: &fakeClock{}}
	f.clock.Set(accountsStart)
	f.svc = app.NewService(memory.NewAccounts(), f.worlds, memory.NewCounters(f.clock), f.clock, plainHasher{}, security.NewTokens(), cfg)
	return f
}

// guest opens a guest account from its own address, so the per-address limit never gets in the way.
func (f *instantFixture) guest(t *testing.T) *app.Account {
	t.Helper()
	f.n++
	_, a, err := f.svc.CreateGuest(context.Background(), "10.0.0."+strconv.Itoa(f.n), "en")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (f *instantFixture) play(t *testing.T, a *app.Account) app.PlayResult {
	t.Helper()
	r, err := f.svc.Play(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReadyWorldIsPausedUntilOpened(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	list, _ := f.worlds.List(ctx)
	if len(list) != 1 || !game.IsInstant(&list[0].Config) || list[0].PausedAtMs == nil || list[0].Humans != 0 {
		t.Fatalf("worlds %+v", list)
	}
	r := f.play(t, f.guest(t))
	got, _ := f.worlds.Get(ctx, r.WorldID)
	if r.WorldID != list[0].ID || got.PausedAtMs != nil {
		t.Fatalf("play %+v, row %+v", r, got)
	}
	list, _ = f.worlds.List(ctx)
	if len(list) != 2 {
		t.Fatalf("no new ready world after opening one: %d worlds", len(list))
	}
}

// Numbers come from one sequence, so deleting a world never hands its number (or a later one) out twice.
func TestInstantWorldsAreNumberedGlobally(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	list, _ := f.worlds.List(ctx)
	if len(list) != 1 || list[0].GameNumber != 1 || list[0].Name != "Game #1" {
		t.Fatalf("first world %+v", list)
	}
	a := f.play(t, f.guest(t)).WorldID
	list, _ = f.worlds.List(ctx)
	if len(list) != 2 || list[1].GameNumber != 2 {
		t.Fatalf("ready world after the first opened %+v", list)
	}
	b := list[1].ID
	if err := f.worlds.Delete(ctx, a); err != nil {
		t.Fatal(err)
	}
	g2 := f.guest(t)
	if id := f.play(t, g2).WorldID; id != b {
		t.Fatalf("guest 2 went to %d, not the ready world %d", id, b)
	}
	list, _ = f.worlds.List(ctx)
	if len(list) != 2 || list[1].GameNumber != 3 || list[1].Name != "Game #3" {
		t.Fatalf("worlds after a deletion %+v", list)
	}
	st, err := f.svc.State(ctx, g2, b)
	if err != nil || st.GameNumber != 2 {
		t.Fatalf("state %+v %v", st, err)
	}
}

// Review focus 1: a guest who clicks Play twice, even at once, gets one membership in one world.
func TestPlayTwiceReturnsSameWorld(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	results := make([]app.PlayResult, 8)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			r, err := f.svc.Play(ctx, g)
			if err != nil {
				t.Error(err)
			}
			results[i] = r
		})
	}
	wg.Wait()
	for _, r := range results {
		if r.WorldID == 0 || r.WorldID != results[0].WorldID {
			t.Fatalf("results %+v", results)
		}
	}
	ms, _ := f.worlds.MembershipsOf(ctx, g.ID)
	if len(ms) != 1 {
		t.Fatalf("memberships %+v", ms)
	}
}

func TestPlayFillsOpenWorldThenOpensNext(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	for range game.InstantWorldConfig.HumanSeats - 1 {
		if id := f.play(t, f.guest(t)).WorldID; id != first {
			t.Fatalf("seat went to world %d, not %d", id, first)
		}
	}
	if id := f.play(t, f.guest(t)).WorldID; id == first || id == 0 {
		t.Fatalf("a full world took one more: %d", id)
	}
}

func TestJoinWindowClosesAfterTenMinutes(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	f.clock.Add(11 * time.Minute.Milliseconds())
	if id := f.play(t, f.guest(t)).WorldID; id == first || id == 0 {
		t.Fatalf("joined a world past its window: %d", id)
	}
}

// Review focus 5: when every allowed world is running and none takes players, Play says how long to wait.
func TestCapacityFullReturnsWait(t *testing.T) {
	f := newInstantFixture(t, app.Config{InstantMaxWorlds: 1})
	first := f.play(t, f.guest(t)).WorldID
	f.clock.Add(11 * time.Minute.Milliseconds())
	late := f.guest(t)
	r := f.play(t, late)
	if r.WorldID != 0 || r.WaitMs < 1000 || r.WaitMs > 50*time.Minute.Milliseconds() {
		t.Fatalf("full: %+v", r)
	}
	f.clock.Add(50 * time.Minute.Milliseconds())
	if r := f.play(t, late); r.WorldID == 0 || r.WorldID == first {
		t.Fatalf("after the first world ended: %+v", r)
	}
}

func TestPurgeDeletesEndedAfterKeep(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	id := f.play(t, g).WorldID
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if n, err := f.svc.PurgeEndedInstant(ctx); err != nil || n != 0 {
		t.Fatalf("purged a fresh result: %d %v", n, err)
	}
	f.clock.Add(25 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeEndedInstant(ctx); err != nil || n != 1 {
		t.Fatalf("purge %d %v", n, err)
	}
	if _, err := f.worlds.Get(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("world still there: %v", err)
	}
	if ms, _ := f.worlds.MembershipsOf(ctx, g.ID); len(ms) != 0 {
		t.Fatalf("memberships %+v", ms)
	}
}

func TestLobbyHidesInstantWorlds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	f.play(t, g)
	view, err := f.svc.Lobby(ctx, g)
	if err != nil || len(view.Mine) != 0 || len(view.Open) != 0 {
		t.Fatalf("lobby %+v %v", view, err)
	}
}

func TestGuestsMayJoinOnlyInstantWorlds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	r := f.play(t, g)
	if r.WorldID == 0 {
		t.Fatalf("a guest could not play: %+v", r)
	}
}

func TestLeaversFreeTheirSeats(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	for range 2 * game.InstantWorldConfig.HumanSeats {
		g := f.guest(t)
		if id := f.play(t, g).WorldID; id != first {
			t.Fatalf("a seat freed by a leaver went unused: world %d instead of %d", id, first)
		}
		if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
			t.Fatal(err)
		}
	}
	if other := f.play(t, f.guest(t)).WorldID; other != first {
		t.Fatalf("seats taken by leavers: a new guest went to %d", other)
	}
}

func TestPlaySkipsAPausedWorld(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	if err := f.svc.PauseWorld(context.Background(), admin, first); err != nil {
		t.Fatal(err)
	}
	if id := f.play(t, f.guest(t)).WorldID; id == first || id == 0 {
		t.Fatalf("a player was sent to the paused world: %d", id)
	}
}

func TestPlayMovesOnOnceAGuestRejoinedTooOften(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	first := f.play(t, g).WorldID
	for range 2 {
		if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
			t.Fatal(err)
		}
		if again := f.play(t, g).WorldID; again != first {
			t.Fatalf("a rejoin within the cap went to %d, not %d", again, first)
		}
	}
	if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
		t.Fatal(err)
	}
	if next := f.play(t, g).WorldID; next == first || next == 0 {
		t.Fatalf("past the cap play returned world %d", next)
	}
}
