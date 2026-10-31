package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

type rankedWorld struct {
	id         int64
	kasia, ola *app.Account
	kasiaID    game.PlayerID
	olaID      game.PlayerID
}

// rankedLongWorld is a long world with two registered humans, bots and a conquest made at world time conquestAt.
func rankedLongWorld(t *testing.T, f *instantFixture, conquestAt int64) rankedWorld {
	t.Helper()
	ctx := context.Background()
	r := rankedWorld{id: longWorld(t, f, 3), kasia: f.registered(t, "kasia"), ola: f.registered(t, "ola")}
	var err error
	if r.kasiaID, err = f.svc.JoinWorld(ctx, r.kasia, r.id, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	if r.olaID, err = f.svc.JoinWorld(ctx, r.ola, r.id, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	change(t, f, r.id, func(w app.WorldTx) {
		w.Player(r.kasiaID).Points, w.Player(r.olaID).Points = 500, 300
		w.SetNow(conquestAt)
		bot := w.Players()[0]
		for _, p := range w.Players() {
			if !p.IsHuman {
				bot = p
				break
			}
		}
		v := w.Village(w.Player(r.olaID).VillageIDs[0])
		game.RecordConquest(w, v, game.PlayerRef(r.olaID), r.kasiaID)
		game.RecordConquest(w, v, "", bot.ID)
	})
	return r
}

func rowsJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestPublicRankingMatchesTheGame(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	r := rankedLongWorld(t, f, 1000)
	pending := f.registered(t, "pending")
	pendingID, err := f.svc.JoinWorld(ctx, pending, r.id, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	change(t, f, r.id, func(w app.WorldTx) { w.Player(pendingID).Status = game.StatusPending })
	for _, tab := range []string{"points", "villages", "od", "daily"} {
		game, err := f.svc.Ranking(ctx, r.kasia, r.id, tab)
		if err != nil {
			t.Fatal(err)
		}
		public, err := f.svc.PublicRanking(ctx, r.id, tab)
		if err != nil {
			t.Fatal(err)
		}
		if rowsJSON(t, game) != rowsJSON(t, public) {
			t.Fatalf("%s: game %s public %s", tab, rowsJSON(t, game), rowsJSON(t, public))
		}
	}
	rows, _ := f.svc.PublicRanking(ctx, r.id, "points")
	for _, p := range rows.([]app.PlayerRow) {
		if p.ID == pendingID {
			t.Fatalf("pending player is ranked: %+v", p)
		}
	}
	inGame, _ := f.svc.Ranking(ctx, r.kasia, r.id, "conquests")
	public, _ := f.svc.PublicRanking(ctx, r.id, "conquests")
	a, b := inGame.([]app.ConquestRow), public.([]app.ConquestRow)
	if len(a) != 2 || len(b) != 2 {
		t.Fatalf("conquests %d %d", len(a), len(b))
	}
	for i := range a {
		b[i].T = a[i].T
		if !reflect.DeepEqual(a[i], b[i]) {
			t.Fatalf("conquest %d: game %+v public %+v", i, a[i], b[i])
		}
	}
	_, err = f.svc.PublicRanking(ctx, r.id, "secret")
	wantValidation(t, err, "tab", "unknown")
}

func TestPublicConquestTimesAreWallClock(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	const x = 3_600_000
	r := rankedLongWorld(t, f, x)
	f.clock.Add(5 * time.Hour.Milliseconds())
	now := f.clock.NowMs()
	row, _ := f.worlds.Get(ctx, r.id)
	want := now - (app.WorldNow(row, now) - x)
	rows, err := f.svc.PublicRanking(ctx, r.id, "conquests")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range rows.([]app.ConquestRow) {
		if c.T != want {
			t.Fatalf("ranking conquest t %d, want %d", c.T, want)
		}
	}
	p, err := f.svc.PublicWorldPlayer(ctx, r.id, r.kasiaID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Conquests) != 1 || p.Conquests[0].T != want {
		t.Fatalf("player conquests %+v, want t %d", p.Conquests, want)
	}
	inGame, _ := f.svc.Ranking(ctx, r.kasia, r.id, "conquests")
	if got := inGame.([]app.ConquestRow)[0].T; got != x {
		t.Fatalf("the in-game ranking keeps world time: %d", got)
	}
}

// Review focus 1 and 2.
func TestPublicRankingHidesWorldsThatAreNotPublic(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	running := f.play(t, f.guest(t)).WorldID
	paused := f.play(t, f.guest(t)).WorldID
	change(t, f, paused, func(w app.WorldTx) { at := f.clock.NowMs(); w.Row().PausedAtMs = &at })
	hosted := longWorld(t, f, 0)
	change(t, f, hosted, func(w app.WorldTx) { w.Row().Config.Kind = game.KindPlayer })
	if _, err := f.svc.PublicRanking(ctx, running, "points"); err != nil {
		t.Fatalf("running game: %v", err)
	}
	for name, id := range map[string]int64{"paused": paused, "hosted": hosted, "unknown": 999} {
		if _, err := f.svc.PublicRanking(ctx, id, "points"); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("%s ranking: %v", name, err)
		}
		if _, err := f.svc.PublicWorldPlayer(ctx, id, "h1"); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("%s player: %v", name, err)
		}
	}
	f.clock.Add(int64(game.InstantWorldConfig.DurationMin)*game.MsPerMinute + time.Minute.Milliseconds())
	if _, err := f.svc.PublicRanking(ctx, running, "villages"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("ended game: %v", err)
	}
}

// Review focus 3 and 4.
func TestPublicWorldPlayer(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	r := rankedLongWorld(t, f, 1000)
	p, err := f.svc.PublicWorldPlayer(ctx, r.id, r.kasiaID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Username != "kasia" || p.Rank != 1 || p.Player.Name != "kasia" || len(p.Conquests) != 1 || p.Conquests[0].ToID != r.kasiaID {
		t.Fatalf("kasia %+v", p)
	}
	if o, _ := f.svc.PublicWorldPlayer(ctx, r.id, r.olaID); o.Rank != 2 || len(o.Conquests) != 1 || o.Conquests[0].FromID != game.PlayerRef(r.olaID) {
		t.Fatalf("ola %+v", o)
	}
	var botID game.PlayerID
	change(t, f, r.id, func(w app.WorldTx) {
		for _, q := range w.Players() {
			if !q.IsHuman {
				botID = q.ID
			}
		}
	})
	b, err := f.svc.PublicWorldPlayer(ctx, r.id, botID)
	if err != nil || b.Username != "" || !b.Player.IsBot {
		t.Fatalf("bot %+v %v", b, err)
	}
	change(t, f, r.id, func(w app.WorldTx) { w.Player(r.olaID).Status = game.StatusPending })
	f.clock.Add(time.Minute.Milliseconds())
	for _, id := range []game.PlayerID{r.olaID, "nobody"} {
		if _, err := f.svc.PublicWorldPlayer(ctx, r.id, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("player %q: %v", id, err)
		}
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rowsJSON(t, p)), &keys); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"player": true, "villages": true, "history": true, "username": true, "rank": true, "conquests": true}
	for k := range keys {
		if !allowed[k] {
			t.Fatalf("unexpected key %q in %v", k, keys)
		}
	}
	var player map[string]json.RawMessage
	_ = json.Unmarshal(keys["player"], &player)
	for _, k := range []string{"coins", "buildQueueSlots", "protectedUntil"} {
		if _, ok := player[k]; ok {
			t.Fatalf("player carries %q", k)
		}
	}
}

func TestPublicConquestTimesDoNotDriftWhilePaused(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	r := rankedLongWorld(t, f, 3_600_000)
	change(t, f, r.id, func(w app.WorldTx) { at := f.clock.NowMs(); w.Row().PausedAtMs = &at })
	read := func() int64 {
		rows, err := f.svc.PublicRanking(ctx, r.id, "conquests")
		if err != nil {
			t.Fatal(err)
		}
		return rows.([]app.ConquestRow)[0].T
	}
	before := read()
	f.clock.Add(3 * time.Hour.Milliseconds())
	if after := read(); after != before {
		t.Fatalf("a paused world's conquest moved by %d ms", after-before)
	}
}
