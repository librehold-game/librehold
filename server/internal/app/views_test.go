package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

var bartek = &app.Account{ID: 2, Username: "bartek"}

type duel struct {
	*adminFixture
	world  int64
	v1, v2 string // the home villages of ala (h1) and bartek (h2)
}

// newDuel is a world without bots where ala and bartek joined and lost their beginner protection.
func newDuel(t *testing.T) *duel {
	t.Helper()
	ctx := context.Background()
	d := &duel{adminFixture: newAdminFixture(t)}
	d.world = d.namedWorld(t, "Duel")
	for _, a := range []*app.Account{ala, bartek} {
		if _, err := d.svc.JoinWorld(ctx, a, d.world, game.QuadrantRandom); err != nil {
			t.Fatal(err)
		}
	}
	err := d.worlds.Run(ctx, d.world, func(w app.WorldTx) error {
		for _, id := range []game.PlayerID{"h1", "h2"} {
			p := w.Player(id)
			p.ProtectedUntil = 0
			if id == "h1" {
				d.v1 = p.VillageIDs[0]
			} else {
				d.v2 = p.VillageIDs[0]
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func (d *duel) send(t *testing.T, from *app.Account, origin, target, typ string, units game.UnitCounts) *game.Command {
	t.Helper()
	var c *game.Command
	err := d.svc.InWorld(context.Background(), from, d.world, func(w app.WorldTx, me game.PlayerID) error {
		to := w.Village(target)
		var err error
		c, err = game.SendCommand(w, me, origin, to.X, to.Y, units, typ, game.SendOpts{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (d *duel) state(t *testing.T, acct *app.Account) *app.PlayerState {
	t.Helper()
	st, err := d.svc.State(context.Background(), acct, d.world)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestStateHidesOthers(t *testing.T) {
	d := newDuel(t)
	var attack *game.Command
	wall := game.BWall
	err := d.svc.InWorld(context.Background(), ala, d.world, func(w app.WorldTx, me game.PlayerID) error {
		w.Village(d.v1).Units[game.UCatapult] = 2
		to := w.Village(d.v2)
		var err error
		attack, err = game.SendCommand(w, me, d.v1, to.X, to.Y, game.UnitCounts{game.USpear: 5, game.UCatapult: 2}, "attack",
			game.SendOpts{CatapultTarget: &wall})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	st := d.state(t, bartek)
	if st.Me != "h2" || st.WorldID != d.world || st.WorldName != "Duel" || st.Now != 0 {
		t.Fatalf("header %+v", st)
	}
	if len(st.Villages) != 1 || st.Villages[d.v2] == nil {
		t.Fatalf("villages %v", st.Villages)
	}
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, banned := range []string{`"worldSeed"`, `"seed"`, `"isHuman"`} {
		if strings.Contains(js, banned) {
			t.Errorf("the state leaks %s", banned)
		}
	}
	var incoming map[string]any
	for _, c := range st.Commands {
		if c.ID == attack.ID {
			raw, _ := json.Marshal(c)
			_ = json.Unmarshal(raw, &incoming)
		}
	}
	if incoming == nil {
		t.Fatal("the incoming attack is missing")
	}
	// What someone else's command shows, and nothing more (no troops, no catapult target).
	keys := []string{}
	for k := range incoming {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"arrivesAt", "departedAt", "id", "originId", "targetId", "type"}; !slices.Equal(keys, want) {
		t.Fatalf("the incoming attack shows %v, want only %v", keys, want)
	}
	if incoming["arrivesAt"].(float64) != float64(attack.ArrivesAt) || incoming["originId"] != d.v1 {
		t.Fatalf("incoming %v", incoming)
	}
	if p, ok := st.Players["h1"]; !ok || p.Name != "ala" || len(p.VillageIDs) != 1 {
		t.Fatalf("players %+v", st.Players)
	}
	mine := d.state(t, ala)
	var own map[string]any
	for _, c := range mine.Commands {
		if c.ID == attack.ID {
			raw, _ := json.Marshal(c)
			_ = json.Unmarshal(raw, &own)
		}
	}
	if units, ok := own["units"].(map[string]any); !ok || units["spear"] != float64(5) {
		t.Fatalf("own attack %v", own)
	}
	if mine.Human == nil || mine.Config.WorldSpeed != 1 {
		t.Fatalf("own state %+v", mine)
	}
	if mine.Player == nil || mine.Player.ID != "h1" || mine.Player.BuildQueueSlots == 0 {
		t.Fatalf("own player record %+v", mine.Player)
	}
}

func TestSupportVisibleToHost(t *testing.T) {
	d := newDuel(t)
	support := d.send(t, ala, d.v1, d.v2, "support", game.UnitCounts{game.USpear: 4})
	d.clock.Set(accountsStart + support.ArrivesAt + 1)
	st := d.state(t, bartek)
	raw, _ := json.Marshal(st.Villages[d.v2])
	var v struct {
		SupportFrom map[string]map[string]int `json:"supportFrom"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.SupportFrom[d.v1]["spear"] != 4 {
		t.Fatalf("support seen by the host: %s", raw)
	}
}

func TestMapRegionClampsAndLimits(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	tiles, err := d.svc.MapRegion(ctx, ala, d.world, -10, -10, 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, tile := range tiles {
		if tile.X < 0 || tile.X > 5 || tile.Y < 0 || tile.Y > 5 {
			t.Fatalf("tile outside the clamp: %+v", tile)
		}
	}
	_, err = d.svc.MapRegion(ctx, ala, d.world, 0, 0, 150, 10)
	wantValidation(t, err, "region", "tooLarge")
	// x1 − x0 overflows to −1 here: the span must be checked on the clamped region.
	_, err = d.svc.MapRegion(ctx, ala, d.world, math.MinInt, math.MinInt, math.MaxInt, math.MaxInt)
	wantValidation(t, err, "region", "tooLarge")

	var home game.Village
	_ = d.worlds.Read(ctx, d.world, func(w app.WorldTx) error { home = *w.Village(d.v1); return nil })
	tiles, err = d.svc.MapRegion(ctx, ala, d.world, home.X-3, home.Y-3, home.X+3, home.Y+3)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tile := range tiles {
		if tile.ID == d.v1 {
			found = tile.OwnerID == "h1" && tile.Name == home.Name && tile.Points == home.Points
		}
	}
	if !found {
		t.Fatalf("own village missing from %+v", tiles)
	}

	var ex, ey int
	ok := false
	_ = d.worlds.Read(ctx, d.world, func(w app.WorldTx) error {
		for x := 0; x < w.Config().MapSize && !ok; x++ {
			for y := 0; y < w.Config().MapSize && !ok; y++ {
				if !game.VillageExistsAt(w, x, y) {
					ex, ey, ok = x, y, true
				}
			}
		}
		return nil
	})
	if !ok {
		t.Fatal("no empty tile")
	}
	tiles, err = d.svc.MapRegion(ctx, ala, d.world, ex, ey, ex, ey)
	if err != nil || tiles == nil || len(tiles) != 0 {
		t.Fatalf("empty region: %v %v", tiles, err)
	}
}

func TestNewsShowsOnlyPublicAndOwn(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	err := d.worlds.Run(ctx, d.world, func(w app.WorldTx) error {
		game.AddNewsItem(w, "test.all", nil, "", false, "")
		game.AddNewsItem(w, "test.ala", nil, "", true, "h1")
		game.AddNewsItem(w, "test.bartek", nil, "", true, "h2")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := d.svc.News(ctx, ala, d.world, 10)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, n := range items {
		texts = append(texts, n.Key)
	}
	if strings.Join(texts, "|") != "test.ala|test.all" {
		t.Fatalf("news %v", texts)
	}
}

func TestFarmTargetsView(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	targets, err := d.svc.FarmTargets(ctx, ala, d.world, d.v1)
	if err != nil || len(targets) == 0 {
		t.Fatalf("targets %d %v", len(targets), err)
	}
	for _, ft := range targets {
		if ft.Status == "" || ft.Distance <= 0 {
			t.Fatalf("target %+v", ft)
		}
	}
	foreign, err := d.svc.FarmTargets(ctx, ala, d.world, d.v2)
	if err != nil || foreign == nil || len(foreign) != 0 {
		t.Fatalf("someone else's village: %v %v", foreign, err)
	}
}

func TestRankingAndProfile(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	for _, tab := range []string{"points", "villages", "od", "daily", "conquests"} {
		if _, err := d.svc.Ranking(ctx, ala, d.world, tab); err != nil {
			t.Fatalf("%s: %v", tab, err)
		}
	}
	_, err := d.svc.Ranking(ctx, ala, d.world, "secret")
	wantValidation(t, err, "tab", "unknown")
	rows, _ := d.svc.Ranking(ctx, ala, d.world, "points")
	raw, _ := json.Marshal(rows)
	if !strings.Contains(string(raw), `"name":"bartek"`) || strings.Contains(string(raw), "isHuman") {
		t.Fatalf("points %s", raw)
	}
	prof, err := d.svc.Profile(ctx, ala, d.world, "h2")
	if err != nil || prof.Player.Name != "bartek" || len(prof.Villages) != 1 || prof.Villages[0].ID != d.v2 {
		t.Fatalf("profile %+v %v", prof, err)
	}
	if _, err := d.svc.Profile(ctx, ala, d.world, "h9"); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing player: %v", err)
	}
	if _, err := d.svc.State(ctx, &app.Account{ID: 7}, d.world); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("stranger state: %v", err)
	}
}

// Bots are labelled as bots in every view a player sees; humans are not.
func TestViewsFlagBots(t *testing.T) {
	ctx := context.Background()
	f := newAdminFixture(t)
	cfg := gametest.TestConfig()
	cfg.BotCount = 3
	wid, err := f.svc.CreateWorld(ctx, admin, "With bots", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.JoinWorld(ctx, ala, wid, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	var bot game.PlayerID
	var botVillage *game.Village
	err = f.worlds.Run(ctx, wid, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			if !p.IsHuman && len(p.VillageIDs) > 0 {
				bot, botVillage = p.ID, w.Village(p.VillageIDs[0])
				v := w.Village(w.Player("h1").VillageIDs[0])
				game.RecordConquest(w, v, game.Ref(bot), "h1")
				return nil
			}
		}
		return nil
	})
	if err != nil || bot == "" {
		t.Fatalf("no bot village: %v", err)
	}
	st, _ := f.svc.State(ctx, ala, wid)
	if !st.Players[bot].IsBot || st.Players["h1"].IsBot {
		t.Fatalf("state players %+v", st.Players)
	}
	rows, _ := f.svc.Ranking(ctx, ala, wid, "points")
	for _, r := range rows.([]app.PlayerRow) {
		if r.IsBot != (r.ID != "h1") {
			t.Fatalf("ranking row %+v", r)
		}
	}
	conq, _ := f.svc.Ranking(ctx, ala, wid, "conquests")
	if c := conq.([]app.ConquestRow); len(c) != 1 || !c[0].FromIsBot || c[0].ToIsBot {
		t.Fatalf("conquest rows %+v", c)
	}
	tiles, _ := f.svc.MapRegion(ctx, ala, wid, botVillage.X-1, botVillage.Y-1, botVillage.X+1, botVillage.Y+1)
	found := false
	for _, tile := range tiles {
		if tile.ID == botVillage.ID {
			found = tile.OwnerIsBot
		}
	}
	if !found {
		t.Fatalf("bot tile not flagged: %+v", tiles)
	}
	prof, _ := f.svc.Profile(ctx, ala, wid, bot)
	if !prof.Player.IsBot {
		t.Fatalf("profile %+v", prof.Player)
	}
}

// The client maps over every list it gets, so a fresh player's views carry empty lists, never null; only the
// farm target fields the client types as `number | null` (nothing known yet) may be null.
func TestViewsHaveNoNullsForFreshPlayer(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	views := map[string]func() (any, error){
		"state":   func() (any, error) { return d.svc.State(ctx, ala, d.world) },
		"reports": func() (any, error) { return d.svc.Reports(ctx, ala, d.world, 0, 100) },
		"news":    func() (any, error) { return d.svc.News(ctx, ala, d.world, 50) },
		"profile": func() (any, error) { return d.svc.Profile(ctx, ala, d.world, "h1") },
		"farm":    func() (any, error) { return d.svc.FarmTargets(ctx, ala, d.world, d.v1) },
	}
	for _, tab := range []string{"points", "villages", "od", "daily", "conquests"} {
		views["ranking "+tab] = func() (any, error) { return d.svc.Ranking(ctx, ala, d.world, tab) }
	}
	for name, view := range views {
		v, err := view()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		raw, _ := json.Marshal(v)
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		for _, path := range nullPaths(doc, name) {
			t.Errorf("null at %s", path)
		}
	}
}

var unknownFarmFields = []string{"lastAttackAt", "lastHaul", "lastHaulCapacity", "lastLosses", "wall", "defenders"}

func nullPaths(v any, path string) []string {
	switch v := v.(type) {
	case nil:
		return []string{path}
	case map[string]any:
		var out []string
		for k, x := range v {
			if x == nil && strings.HasPrefix(path, "farm[") && slices.Contains(unknownFarmFields, k) {
				continue
			}
			out = append(out, nullPaths(x, path+"."+k)...)
		}
		return out
	case []any:
		var out []string
		for i, x := range v {
			out = append(out, nullPaths(x, fmt.Sprintf("%s[%d]", path, i))...)
		}
		return out
	}
	return nil
}

func TestStateCarriesQuestProgressWithoutPersistingIt(t *testing.T) {
	ctx := context.Background()
	f := newAdminFixture(t)
	world := f.namedWorld(t, "Quests")
	if _, err := f.svc.JoinWorld(ctx, ala, world, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.State(ctx, ala, world)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := st.Human.Quests.Progress["wood1"]; !ok || v != 0 {
		t.Fatalf("progress %v", st.Human.Quests.Progress)
	}
	err = f.worlds.Run(ctx, world, func(w app.WorldTx) error {
		if p := w.Human("h1").Quests.Progress; p != nil {
			t.Errorf("stored progress %v", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
