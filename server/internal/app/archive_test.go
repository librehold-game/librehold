package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

// endedGame plays one instant world with one guest to its end and archives it.
func endedGame(t *testing.T, f *instantFixture) (int64, *app.Account) {
	t.Helper()
	ctx := context.Background()
	g := f.guest(t)
	id := f.play(t, g).WorldID
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	return id, g
}

func TestArchiveWrittenAtEnd(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, g := endedGame(t, f)
	game, err := f.svc.PublicGame(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if game.Summary.EndReason != "time" || game.Summary.Humans != 1 || game.Summary.Guests != 1 || game.Summary.Bots == 0 ||
		game.WinnerName == "" {
		t.Fatalf("summary %+v", game)
	}
	for _, r := range game.Rows {
		if r.AccountID != 0 || r.Key != "" {
			t.Fatalf("the public results name an account: %+v", r)
		}
	}
	stored, err := f.worlds.ArchivedGame(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(game.Rows) != game.Summary.Humans+game.Summary.Bots || game.Rows[0].Rank != 1 {
		t.Fatalf("rows %+v", game.Rows)
	}
	found := false
	for _, r := range stored.Rows {
		if !r.IsBot {
			found = r.AccountID == g.ID && r.Name == g.Username && r.Key == g.PlayerKey
		}
	}
	if !found {
		t.Fatalf("the guest's row %+v", stored.Rows)
	}
	if game.EndedAtMs <= game.StartedAtMs {
		t.Fatalf("times %d → %d", game.StartedAtMs, game.EndedAtMs)
	}
}

func TestArchiveOutlivesPurge(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	f.clock.Add(25 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeEndedInstant(ctx); err != nil || n != 1 {
		t.Fatalf("purge %d %v", n, err)
	}
	game, err := f.svc.PublicGame(ctx, id)
	if err != nil || game.Summary.Humans != 1 {
		t.Fatalf("after purge %+v %v", game, err)
	}
	stats, _ := f.svc.PublicStats(ctx)
	if stats.AllTime.Games != 1 {
		t.Fatalf("stats after purge %+v", stats)
	}
}

// Review focus 3: a purged guest's row stays on the leaderboard as a former player; the totals don't change.
func TestPurgedGuestBecomesFormerPlayer(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	before, _ := f.svc.PublicStats(ctx)
	f.clock.Add(25 * time.Hour.Milliseconds())
	if _, err := f.svc.PurgeEndedInstant(ctx); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(31 * 24 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeGuests(ctx); err != nil || n != 1 {
		t.Fatalf("purge guests %d %v", n, err)
	}
	game, _ := f.svc.PublicGame(ctx, id)
	for _, r := range game.Rows {
		if !r.IsBot && (r.AccountID != 0 || r.Name != "") {
			t.Fatalf("former player row %+v", r)
		}
	}
	after, _ := f.svc.PublicStats(ctx)
	if after.AllTime != before.AllTime {
		t.Fatalf("totals changed %+v → %+v", before.AllTime, after.AllTime)
	}
}

// A guest who left before the end is forgotten on purge like one who stayed.
func TestPurgedLeaverBecomesFormerPlayer(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	stay, leave := f.guest(t), f.guest(t)
	id := f.play(t, stay).WorldID
	if f.playIn(t, leave, id).WorldID != id {
		t.Fatal("the guests play different worlds")
	}
	if err := f.svc.LeaveWorld(ctx, leave, id); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	before, _ := f.svc.PublicStats(ctx)
	if before.AllTime.Players != 2 {
		t.Fatalf("players %+v", before.AllTime)
	}
	f.clock.Add(25 * time.Hour.Milliseconds())
	if _, err := f.svc.PurgeEndedInstant(ctx); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(31 * 24 * time.Hour.Milliseconds())
	if n, err := f.svc.PurgeGuests(ctx); err != nil || n != 2 {
		t.Fatalf("purge guests %d %v", n, err)
	}
	stored, _ := f.worlds.ArchivedGame(ctx, id)
	raw, _ := json.Marshal(stored)
	for _, name := range []string{stay.Username, leave.Username} {
		if strings.Contains(string(raw), name) {
			t.Fatalf("%s is still in the archive: %s", name, raw)
		}
	}
	after, _ := f.svc.PublicStats(ctx)
	if after.AllTime != before.AllTime {
		t.Fatalf("totals changed %+v → %+v", before.AllTime, after.AllTime)
	}
}

func TestPublicStatsHiddenUnderTwentyGames(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	endedGame(t, f)
	s, err := f.svc.PublicStats(ctx)
	if err != nil || s.Visible || s.AllTime.Games != 1 || s.Last24h.Games != 1 {
		t.Fatalf("stats %+v %v", s, err)
	}
}

func TestPublicStatsCountsHumansOnly(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	s, _ := f.svc.PublicStats(ctx)
	game, _ := f.svc.PublicGame(ctx, id)
	if s.AllTime.Players != 1 || s.AllTime.Conquests != int64(game.Summary.ConquestsHumans) || s.AllTime.UnitsKilled != game.Summary.HumanUnitsKilled {
		t.Fatalf("stats %+v, summary %+v", s.AllTime, game.Summary)
	}
}

func TestProfileTallyAggregatesArchive(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, g := endedGame(t, f)
	p, err := f.svc.PublicProfile(ctx, g.Username, nil)
	if err != nil || p.Tally.Games != 1 || len(p.Games) != 1 || p.Games[0].WorldID != id {
		t.Fatalf("profile %+v %v", p, err)
	}
}

func TestPublicGameShowsRunningInstantGame(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	row, _ := f.worlds.Get(ctx, id)
	g, err := f.svc.PublicGame(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	wantEnd := row.StartedAtMs + row.PausedTotalMs + int64(row.Config.DurationMin)*game.MsPerMinute
	if !g.Running || g.EndsAtMs != wantEnd || g.Summary.Humans != 1 || g.Summary.Bots == 0 || g.WinnerName != "" ||
		g.GameNumber != row.GameNumber {
		t.Fatalf("running game %+v (want end %d)", g, wantEnd)
	}
	for _, r := range g.Rows {
		if r.AccountID != 0 || r.Key != "" {
			t.Fatalf("the public results name an account: %+v", r)
		}
	}
}

func TestPublicGameHidesReadyWorld(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	list, _ := f.worlds.List(ctx)
	if _, err := f.svc.PublicGame(ctx, list[0].ID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("ready world: %v", err)
	}
}

func TestPublicGameNotForLongWorlds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, err := f.worlds.Create(ctx, apptest.NewWorldRow(1), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.PublicGame(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("long world: %v", err)
	}
}

func TestPublicGameRunningIsCached(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	if _, err := f.svc.PublicGame(ctx, id); err != nil {
		t.Fatal(err)
	}
	rename := func(w app.WorldTx) error {
		for _, p := range w.Players() {
			if p.IsHuman {
				p.Name = "Renamed"
			}
		}
		return nil
	}
	if err := f.worlds.Run(ctx, id, rename); err != nil {
		t.Fatal(err)
	}
	named := func() bool {
		g, err := f.svc.PublicGame(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range g.Rows {
			if r.Name == "Renamed" {
				return true
			}
		}
		return false
	}
	f.clock.Add(29 * time.Second.Milliseconds())
	if named() {
		t.Fatal("a change showed within the cache time")
	}
	f.clock.Add(2 * time.Second.Milliseconds())
	if !named() {
		t.Fatal("the change never showed")
	}
}
