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
	r, err := f.svc.Play(context.Background(), a, 0)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *instantFixture) playIn(t *testing.T, a *app.Account, id int64) app.PlayResult {
	t.Helper()
	r, err := f.svc.Play(context.Background(), a, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func validationCode(err error) string {
	var ve *app.ErrValidation
	if errors.As(err, &ve) {
		return ve.Code
	}
	return ""
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
			r, err := f.svc.Play(ctx, g, 0)
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

func TestJoinRunningGameUntilItEnds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	f.clock.Add(59 * time.Minute.Milliseconds())
	if got := f.playIn(t, f.guest(t), first).WorldID; got != first {
		t.Fatalf("late joiner went to %d", got)
	}
	f.clock.Add(2 * time.Minute.Milliseconds())
	_, err := f.svc.Play(ctx, f.guest(t), first)
	if validationCode(err) != "worldClosed" && !errors.Is(err, app.ErrWorldEnded) {
		t.Fatalf("joined an ended game: %v", err)
	}
}

func TestPlayWithoutWorldStartsNewGame(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	second := f.play(t, f.guest(t)).WorldID
	if second == 0 || second == first {
		t.Fatalf("second guest went to %d (first %d)", second, first)
	}
}

func TestPlayChosenWorld(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	b := f.guest(t)
	if got := f.playIn(t, b, first).WorldID; got != first {
		t.Fatalf("went to %d", got)
	}
	list, err := f.svc.InstantGames(ctx, b)
	if err != nil || len(list.Games) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
	g := list.Games[0]
	if g.ID != first || g.Humans != 2 || !g.Mine || g.Seats != game.InstantWorldConfig.HumanSeats {
		t.Fatalf("game %+v", g)
	}
}

func TestOneRunningInstantGameAtATime(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	first := f.play(t, a).WorldID
	second := f.play(t, f.guest(t)).WorldID
	_, err := f.svc.Play(ctx, a, second)
	if validationCode(err) != "inAnotherGame" {
		t.Fatalf("second game: %v", err)
	}
	if err := f.svc.LeaveWorld(ctx, a, first); err != nil {
		t.Fatal(err)
	}
	if got := f.playIn(t, a, second).WorldID; got != second {
		t.Fatalf("after leaving went to %d", got)
	}
}

// Review focus 1: a game that is over does not count as the player's game.
func TestEndedGameDoesNotBlock(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	first := f.play(t, a).WorldID
	f.clock.Add(30 * time.Minute.Milliseconds())
	other := f.play(t, f.guest(t)).WorldID
	f.clock.Add(31 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, first, 0); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.InstantGames(ctx, a)
	if err != nil || !list.CanStart {
		t.Fatalf("list %+v %v", list, err)
	}
	if got := f.playIn(t, a, other).WorldID; got != other {
		t.Fatalf("went to %d", got)
	}
}

// Review focus 5: clicking Join twice gives one membership in one world.
func TestPlaySameWorldTwice(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	b := f.guest(t)
	f.playIn(t, b, first)
	if got := f.playIn(t, b, first).WorldID; got != first {
		t.Fatalf("second join went to %d", got)
	}
	if ms, _ := f.worlds.MembershipsOf(ctx, b.ID); len(ms) != 1 {
		t.Fatalf("memberships %+v", ms)
	}
}

func TestInstantGamesList(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	a, b := f.guest(t), f.guest(t)
	first := f.play(t, a).WorldID
	f.clock.Add(5 * time.Minute.Milliseconds())
	second := f.play(t, b).WorldID
	list, err := f.svc.InstantGames(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Games) != 2 || list.Games[0].ID != second || !list.Games[0].Mine || list.Games[1].ID != first {
		t.Fatalf("list %+v", list)
	}
	for _, g := range list.Games {
		row, _ := f.worlds.Get(ctx, g.ID)
		if g.Seats != 12 || g.EndsAt <= f.clock.NowMs() || g.GameNumber != row.GameNumber {
			t.Fatalf("game %+v", g)
		}
		if g.Leader == "" || g.LeaderPoints <= 0 {
			t.Fatalf("no leader: %+v", g)
		}
	}
	if list.CanStart {
		t.Fatalf("a player in a game can start another: %+v", list)
	}
	fresh, _ := f.svc.InstantGames(ctx, f.guest(t))
	if !fresh.CanStart || fresh.WaitMs != 0 || fresh.Games[0].Mine {
		t.Fatalf("fresh %+v", fresh)
	}
}

// Review focus 5: when every allowed world is running and none has a free seat, Play says how long to wait.
func TestCapacityFullReturnsWait(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{InstantMaxWorlds: 1})
	first := f.play(t, f.guest(t)).WorldID
	for range game.InstantWorldConfig.HumanSeats - 1 {
		f.playIn(t, f.guest(t), first)
	}
	late := f.guest(t)
	r := f.play(t, late)
	if r.WorldID != 0 || r.WaitMs < 1000 || r.WaitMs > 60*time.Minute.Milliseconds() {
		t.Fatalf("full: %+v", r)
	}
	list, err := f.svc.InstantGames(ctx, late)
	if err != nil || list.CanStart || list.WaitMs < 1000 {
		t.Fatalf("list %+v %v", list, err)
	}
	f.clock.Add(61 * time.Minute.Milliseconds())
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
		if id := f.playIn(t, g, first).WorldID; id != first {
			t.Fatalf("a seat freed by a leaver went unused: world %d instead of %d", id, first)
		}
		if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
			t.Fatal(err)
		}
	}
	if other := f.playIn(t, f.guest(t), first).WorldID; other != first {
		t.Fatalf("seats taken by leavers: a new guest went to %d", other)
	}
}

func TestPlayRefusesAPausedWorld(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	first := f.play(t, f.guest(t)).WorldID
	if err := f.svc.PauseWorld(ctx, admin, first); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Play(ctx, f.guest(t), first)
	if validationCode(err) != "worldClosed" {
		t.Fatalf("joined a paused world: %v", err)
	}
}

func TestPlayRefusesAGuestWhoRejoinedTooOften(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	first := f.play(t, g).WorldID
	for range 2 {
		if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
			t.Fatal(err)
		}
		if again := f.playIn(t, g, first).WorldID; again != first {
			t.Fatalf("a rejoin within the cap went to %d, not %d", again, first)
		}
	}
	if err := f.svc.LeaveWorld(ctx, g, first); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Play(ctx, g, first); validationCode(err) != "rejoinLimit" {
		t.Fatalf("past the cap: %v", err)
	}
}

func TestAbandonedInstantEndsAfterFiveMinutes(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	f.clock.Add(5*time.Minute.Milliseconds() - 1000)
	if n, err := f.svc.EndAbandonedInstant(ctx); err != nil || n != 0 {
		t.Fatalf("ended %d after 4m59s: %v", n, err)
	}
	f.clock.Add(2000)
	if n, err := f.svc.EndAbandonedInstant(ctx); err != nil || n != 1 {
		t.Fatalf("ended %d after 5m01s: %v", n, err)
	}
	row, _ := f.worlds.Get(ctx, id)
	if row.State.EndedAt == nil || row.State.EndReason != "abandoned" || row.State.WinnerID == "" {
		t.Fatalf("state %+v", row.State)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	if g, err := f.svc.PublicGame(ctx, id); err != nil || g.Summary.EndReason != "abandoned" {
		t.Fatalf("archive %+v %v", g, err)
	}
	if n, _ := f.svc.EndAbandonedInstant(ctx); n != 0 {
		t.Fatalf("ended again: %d", n)
	}
}

func TestPresenceKeepsInstantRunning(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	for range 5 {
		f.clock.Add(4 * time.Minute.Milliseconds())
		f.svc.MarkPresent(ctx, id)
		if n, err := f.svc.EndAbandonedInstant(ctx); err != nil || n != 0 {
			t.Fatalf("ended a game somebody has open: %d %v", n, err)
		}
	}
}

// Review focus 4: the ready world and a world paused by the admin never count as abandoned.
func TestReadyAndPausedWorldsNeverAbandoned(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	paused := f.play(t, f.guest(t)).WorldID
	if err := f.svc.PauseWorld(ctx, admin, paused); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(30 * time.Minute.Milliseconds())
	if n, err := f.svc.EndAbandonedInstant(ctx); err != nil || n != 0 {
		t.Fatalf("ended %d: %v", n, err)
	}
	list, _ := f.worlds.List(ctx)
	for _, w := range list {
		if w.State.EndedAt != nil {
			t.Fatalf("world %d ended: %+v", w.ID, w.State)
		}
	}
}

// Review finding: a game paused by the admin and resumed later is not abandoned at once.
func TestResumeAfterPauseDoesNotAbandon(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	f.clock.Add(20 * time.Minute.Milliseconds())
	f.svc.MarkPresent(ctx, id)
	if err := f.svc.PauseWorld(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(30 * time.Minute.Milliseconds())
	if err := f.svc.ResumeWorld(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	if n, err := f.svc.EndAbandonedInstant(ctx); err != nil || n != 0 {
		t.Fatalf("ended %d right after the resume: %v", n, err)
	}
	f.clock.Add(5*time.Minute.Milliseconds() + 1000)
	if n, _ := f.svc.EndAbandonedInstant(ctx); n != 1 {
		t.Fatalf("ended %d five minutes after the resume", n)
	}
}

// Review finding: a server that was down or just deployed has not seen anyone yet, so it ends nothing at first.
func TestRestartDoesNotAbandon(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	f.play(t, f.guest(t))
	f.clock.Add(20 * time.Minute.Milliseconds())
	again := app.NewService(memory.NewAccounts(), f.worlds, memory.NewCounters(f.clock), f.clock, plainHasher{}, security.NewTokens(), app.Config{})
	if n, err := again.EndAbandonedInstant(ctx); err != nil || n != 0 {
		t.Fatalf("a fresh server ended %d games: %v", n, err)
	}
	f.clock.Add(5*time.Minute.Milliseconds() + 1000)
	if n, _ := again.EndAbandonedInstant(ctx); n != 1 {
		t.Fatalf("ended %d after five minutes without anyone", n)
	}
}

// Review finding: worlds stored with the old ten-minute window still take players until they end.
func TestOldJoinWindowInStoredConfigIsIgnored(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	err := f.worlds.Run(ctx, id, func(w app.WorldTx) error { w.Config().JoinWindowMin = 10; return nil })
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Add(20 * time.Minute.Milliseconds())
	if got := f.playIn(t, f.guest(t), id).WorldID; got != id {
		t.Fatalf("went to %d", got)
	}
}

// Review finding: a game the admin paused still counts as the player's game.
func TestPausedGameStillCountsAsTheOneGame(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	first := f.play(t, a).WorldID
	other := f.play(t, f.guest(t)).WorldID
	if err := f.svc.PauseWorld(ctx, admin, first); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Play(ctx, a, other); validationCode(err) != "inAnotherGame" {
		t.Fatalf("joined a second game while the first is paused: %v", err)
	}
}

// Leaving a game that is over just lets the player go; the result stays as it was.
func TestLeavingAnEndedGameSucceeds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	id := f.play(t, a).WorldID
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LeaveWorld(ctx, a, id); err != nil {
		t.Fatalf("leave: %v", err)
	}
	row, _ := f.worlds.Get(ctx, id)
	if row.State.EndedAt == nil || row.State.EndReason != "time" {
		t.Fatalf("result changed: %+v", row.State)
	}
}

// A player can always get out: of a game the admin paused, and twice (two tabs).
func TestLeavingAPausedGameSucceeds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	first := f.play(t, a).WorldID
	other := f.play(t, f.guest(t)).WorldID
	if err := f.svc.PauseWorld(ctx, admin, first); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LeaveWorld(ctx, a, first); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if ms, _ := f.worlds.MembershipsOf(ctx, a.ID); len(ms) != 0 {
		t.Fatalf("still a member: %+v", ms)
	}
	if got := f.playIn(t, a, other).WorldID; got != other {
		t.Fatalf("went to %d", got)
	}
}

func TestLeavingTwiceSucceeds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a := f.guest(t)
	id := f.play(t, a).WorldID
	for range 2 {
		if err := f.svc.LeaveWorld(ctx, a, id); err != nil {
			t.Fatalf("leave: %v", err)
		}
	}
}
