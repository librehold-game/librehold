package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

func publicWorlds(t *testing.T, f *instantFixture) app.PublicWorldList {
	t.Helper()
	l, err := f.svc.PublicWorlds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func change(t *testing.T, f *instantFixture, id int64, fn func(w app.WorldTx)) {
	t.Helper()
	if err := f.worlds.Run(context.Background(), id, func(w app.WorldTx) error { fn(w); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestPublicWorldsListsRunningLongAndFinished(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	instant := f.play(t, f.guest(t)).WorldID
	long := longWorld(t, f, 2)
	change(t, f, long, func(w app.WorldTx) { w.Row().StartedAtMs -= 36 * time.Hour.Milliseconds() })
	err := f.worlds.SaveArchive(ctx, app.ArchivedGame{WorldID: 900, Name: "Game #3", GameNumber: 3, EndedAtMs: accountsStart - 1000})
	if err != nil {
		t.Fatal(err)
	}
	l := publicWorlds(t, f)
	if len(l.Running) != 1 || len(l.Long) != 1 || len(l.Finished) != 1 {
		t.Fatalf("list %+v", l)
	}
	row, _ := f.worlds.Get(ctx, instant)
	r := l.Running[0]
	start := row.StartedAtMs + row.PausedTotalMs
	if r.ID != instant || !r.Instant || r.Humans != 1 || r.StartedAt != start ||
		r.EndsAt != start+int64(game.InstantWorldConfig.DurationMin)*game.MsPerMinute || r.Day != 0 ||
		r.OpenSeats != game.InstantWorldConfig.HumanSeats-1 {
		t.Fatalf("running %+v", r)
	}
	g := l.Long[0]
	if g.ID != long || g.Instant || g.Status != "open" || g.Day != 2 || g.EndsAt != 0 || g.Bots != 2 || g.Speed != 1 || g.OpenSeats != 0 {
		t.Fatalf("long %+v", g)
	}
	if fin := l.Finished[0]; fin != (app.FinishedGameCard{WorldID: 900, Name: "Game #3", GameNumber: 3, EndedAt: accountsStart - 1000}) {
		t.Fatalf("finished %+v", fin)
	}
}

// Review focus 4: closed and paused long worlds are still worlds people play in.
func TestPublicWorldsListsClosedAndPausedLongWorlds(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	closed, paused, open := longWorld(t, f, 0), longWorld(t, f, 0), longWorld(t, f, 0)
	change(t, f, closed, func(w app.WorldTx) { w.Row().Status = "closed" })
	change(t, f, paused, func(w app.WorldTx) { at := f.clock.NowMs(); w.Row().PausedAtMs = &at })
	l := publicWorlds(t, f)
	if len(l.Long) != 3 || l.Long[0].ID != open || l.Long[1].ID != closed || l.Long[2].ID != paused ||
		l.Long[1].Status != "closed" || !l.Long[2].Paused || l.Long[0].Paused {
		t.Fatalf("long %+v", l.Long)
	}
}

func TestPublicWorldsHidesReadyGamesAndOtherKinds(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	other := longWorld(t, f, 0)
	change(t, f, other, func(w app.WorldTx) { w.Row().Config.Kind = game.WorldKind("player") })
	if l := publicWorlds(t, f); len(l.Running) != 0 || len(l.Long) != 0 {
		t.Fatalf("list %+v", l)
	}

	f = newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	row, _ := f.worlds.Get(ctx, id)
	if l := publicWorlds(t, f); len(l.Running) != 1 || l.Running[0].GameNumber == 0 || l.Running[0].GameNumber != row.GameNumber {
		t.Fatalf("list %+v, game number %d", l, row.GameNumber)
	}
}

func TestPublicWorldCardAndEvents(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, other := longWorld(t, f, 15), longWorld(t, f, 0)
	for _, e := range []struct {
		world  int64
		troops int
	}{{id, 500}, {id, 20}, {other, 800}} {
		change(t, f, e.world, func(w app.WorldTx) {
			w.SetNow(1000)
			w.AddPublicEvent(game.PublicEvent{Kind: "battle", Attacker: "Bot", Troops: e.troops})
		})
	}
	pw, err := f.svc.PublicWorld(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Bots != 15 || pw.Ended || pw.Settings.MapSize == 0 || !pw.Settings.FieldBattles {
		t.Fatalf("world %+v", pw)
	}
	if len(pw.Events) != 1 || pw.Events[0].WorldID != id || pw.Events[0].Troops != 500 {
		t.Fatalf("events %+v", pw.Events)
	}
	raw, _ := json.Marshal(pw)
	if strings.Contains(string(raw), `"top"`) {
		t.Fatalf("the world page has one ranking, the ranking endpoint: %s", raw)
	}
}

func TestPublicMapHasEveryVillageAndNoSeed(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := longWorld(t, f, 3)
	change(t, f, id, func(w app.WorldTx) { w.Row().Seed = 912345678 })
	var stored []game.VillageRef
	colors := map[game.PlayerRef]string{}
	var cfg game.WorldConfig
	err := f.worlds.Read(ctx, id, func(w app.WorldTx) error {
		cfg = *w.Config()
		stored = w.VillagesIn(0, 0, cfg.MapSize-1, cfg.MapSize-1)
		for _, p := range w.Players() {
			colors[game.Ref(p.ID)] = p.Color
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	at := map[[2]int]game.VillageRef{}
	for _, v := range stored {
		at[[2]int{v.X, v.Y}] = v
	}
	want := len(stored)
	for x := range cfg.MapSize {
		for y := range cfg.MapSize {
			if _, ok := at[[2]int{x, y}]; !ok && game.TileHasVillage(912345678, &cfg, x, y) {
				want++
			}
		}
	}
	m, err := f.svc.PublicMap(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Size != cfg.MapSize || len(m.Villages) != want || len(m.Players) != 3 {
		t.Fatalf("map size %d, %d villages (want %d), players %+v", m.Size, len(m.Villages), want, m.Players)
	}
	for _, v := range m.Villages {
		ref, ok := at[[2]int{v[0], v[1]}]
		switch {
		case !ok || ref.OwnerID == "":
			if v[2] != -1 {
				t.Fatalf("barbarian village %v", v)
			}
		case v[2] < 0 || m.Players[v[2]].Color != colors[ref.OwnerID]:
			t.Fatalf("village %v of %s", v, ref.OwnerID)
		}
	}
	pw, err := f.svc.PublicWorld(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]any{"world": pw, "map": m} {
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), "912345678") || strings.Contains(strings.ToLower(string(b)), `"seed"`) {
			t.Fatalf("the %s gives the seed away", name)
		}
	}
}

// Review focus 1 and 2: long worlds and running instant games are public; everything else is not found.
func TestPublicWorldVisibility(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	long := longWorld(t, f, 0)
	running := f.play(t, f.guest(t)).WorldID
	paused := f.play(t, f.guest(t)).WorldID
	change(t, f, paused, func(w app.WorldTx) { at := f.clock.NowMs(); w.Row().PausedAtMs = &at })
	hosted := longWorld(t, f, 0)
	change(t, f, hosted, func(w app.WorldTx) { w.Row().Config.Kind = game.WorldKind("player") })
	for _, id := range []int64{long, running} {
		if _, err := f.svc.PublicWorld(ctx, id); err != nil {
			t.Fatalf("world %d: %v", id, err)
		}
		if _, err := f.svc.PublicMap(ctx, id); err != nil {
			t.Fatalf("map %d: %v", id, err)
		}
	}
	pw, _ := f.svc.PublicWorld(ctx, running)
	if !pw.Instant || pw.Settings.DurationMin <= 0 {
		t.Fatalf("instant world %+v", pw)
	}
	for _, id := range []int64{999, paused, hosted} {
		if _, err := f.svc.PublicWorld(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("world %d: %v", id, err)
		}
		if _, err := f.svc.PublicMap(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("map %d: %v", id, err)
		}
	}
	f.clock.Add(int64(game.InstantWorldConfig.DurationMin)*game.MsPerMinute + time.Minute.Milliseconds())
	if _, err := f.svc.PublicWorld(ctx, running); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("finished game: %v", err)
	}
	if _, err := f.svc.PublicMap(ctx, running); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("finished game map: %v", err)
	}
}

func TestPublicWorldSettingsAreComplete(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	long := longWorld(t, f, 3)
	human := game.SideModifiers{Resources: 1.5, Recruitment: 1.2, Building: 0.8}
	bots := game.SideModifiers{Resources: 0.9, Recruitment: 1.1, Building: 1}
	change(t, f, long, func(w app.WorldTx) {
		c := &w.Row().Config
		c.HumanModifiers, c.BotModifiers = human, bots
		c.Difficulty, c.BotStance = game.Difficulty("hard"), game.BotStance("aggressive")
	})
	pw, err := f.svc.PublicWorld(ctx, long)
	if err != nil {
		t.Fatal(err)
	}
	row, _ := f.worlds.Get(ctx, long)
	s := pw.Settings
	if s.HumanModifiers != human || s.BotModifiers != bots || s.BotCount != row.Config.BotCount ||
		s.Difficulty != "hard" || s.BotStance != "aggressive" || s.BarbarianDensity != string(row.Config.BarbarianDensity) ||
		s.BattleLosses != string(row.Config.BattleLosses) || s.SightRadius != game.SightRadiusOf(&row.Config) ||
		s.NobleCost != nil || s.NoCoins || s.HumanSeats != 0 {
		t.Fatalf("long settings %+v", s)
	}

	instant := f.play(t, f.guest(t)).WorldID
	price := game.Resources{Wood: 100, Clay: 200, Iron: 300}
	change(t, f, instant, func(w app.WorldTx) {
		c := &w.Row().Config
		c.NoCoins, c.NobleCost = true, price
		c.OwnedNobleDropMin, c.OwnedNobleDropMax, c.AcademyExtraNobles = 10, 20, 2
	})
	f.clock.Add(time.Minute.Milliseconds())
	iw, err := f.svc.PublicWorld(ctx, instant)
	if err != nil {
		t.Fatal(err)
	}
	row, _ = f.worlds.Get(ctx, instant)
	s = iw.Settings
	if !s.NoCoins || s.NobleCost == nil || *s.NobleCost != price || s.OwnedNobleDropMin != 10 || s.OwnedNobleDropMax != 20 ||
		s.AcademyExtraNobles != 2 || s.HumanSeats != row.Config.HumanSeats || s.DominationShare != row.Config.DominationShare {
		t.Fatalf("instant settings %+v", s)
	}
}

func TestPublicWorldsCachedAMinute(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := longWorld(t, f, 0)
	if l := publicWorlds(t, f); len(l.Long) != 1 {
		t.Fatalf("list %+v", l)
	}
	before, err := f.svc.PublicWorld(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	longWorld(t, f, 0)
	change(t, f, id, func(w app.WorldTx) { w.Row().Name = "Renamed " + strconv.Itoa(1) })
	f.clock.Add(time.Minute.Milliseconds() - 1)
	if l := publicWorlds(t, f); len(l.Long) != 1 {
		t.Fatalf("within a minute %+v", l)
	}
	if pw, _ := f.svc.PublicWorld(ctx, id); pw.Name != before.Name {
		t.Fatalf("world within a minute %+v", pw)
	}
	f.clock.Add(1)
	if l := publicWorlds(t, f); len(l.Long) != 2 {
		t.Fatalf("after a minute %+v", l)
	}
	if pw, _ := f.svc.PublicWorld(ctx, id); pw.Name != "Renamed 1" {
		t.Fatalf("world after a minute %+v", pw)
	}
}
