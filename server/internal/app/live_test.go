package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

func live(t *testing.T, f *instantFixture) app.LiveStrip {
	t.Helper()
	l, err := f.svc.Live(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// longWorld creates a long world with the given number of bots, started at the clock's time.
func longWorld(t *testing.T, f *instantFixture, bots int) int64 {
	t.Helper()
	row := apptest.NewWorldRow(bots)
	row.StartedAtMs = f.clock.NowMs()
	id, err := f.worlds.Create(context.Background(), row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestLiveCountsRunningInstantGames(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	f.playIn(t, f.guest(t), id)
	l := live(t, f)
	cfg := game.InstantWorldConfig
	if l.Games != 1 || l.Humans != 2 || l.Bots != cfg.BotCount || l.OpenSeats != cfg.HumanSeats-2 {
		t.Fatalf("live %+v", l)
	}
}

func TestLiveSaysNothingWhenNoGameRuns(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(context.Background()); err != nil {
		t.Fatal(err)
	}
	l := live(t, f)
	if l.Games != 0 || l.Humans != 0 || l.Bots != 0 || l.OpenSeats != 0 || l.LastBattleAt != nil {
		t.Fatalf("live %+v", l)
	}
	b, _ := json.Marshal(l)
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(b, &raw)
	if string(raw["events"]) != "[]" || string(raw["lastBattleAt"]) != "null" {
		t.Fatalf("json %s", b)
	}
}

func TestLiveLeavesLongWorldsOut(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	id := longWorld(t, f, 0)
	err := f.worlds.Run(context.Background(), id, func(w app.WorldTx) error {
		_, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if l := live(t, f); l.Humans != 0 || l.Games != 0 {
		t.Fatalf("live %+v", l)
	}
}

func TestLiveShowsTheFiveNewestLargeEvents(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	id := longWorld(t, f, 0)
	for i := range 7 {
		troops := 200 + i
		if i == 6 {
			troops = 50
		}
		err := f.worlds.Run(context.Background(), id, func(w app.WorldTx) error {
			w.SetNow(int64(i+1) * 1000)
			w.AddPublicEvent(game.PublicEvent{Kind: "battle", Attacker: "Ala", Troops: troops})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	l := live(t, f)
	if len(l.Events) != 5 {
		t.Fatalf("events %+v", l.Events)
	}
	for i, e := range l.Events {
		if e.Troops != 205-i || e.WorldID != id || e.Instant {
			t.Fatalf("event %d %+v", i, e)
		}
	}
	if newest := f.clock.NowMs() + 7000; l.LastBattleAt == nil || *l.LastBattleAt != newest {
		t.Fatalf("last battle %v, want %d", l.LastBattleAt, newest)
	}
}

func TestLiveIsCachedThirtySeconds(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	f.play(t, f.guest(t))
	if l := live(t, f); l.Humans != 1 {
		t.Fatalf("live %+v", l)
	}
	f.play(t, f.guest(t))
	f.clock.Add((30*time.Second - time.Millisecond).Milliseconds())
	if l := live(t, f); l.Humans != 1 {
		t.Fatalf("within 30 s %+v", l)
	}
	f.clock.Add(1)
	if l := live(t, f); l.Humans != 2 {
		t.Fatalf("after 30 s %+v", l)
	}
}
