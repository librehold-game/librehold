package game_test

import (
	"slices"
	"sort"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// playerWorld is a player-hosted world on the instant preset, without beginner protection, with human h1.
func playerWorld(t *testing.T, tweak func(cfg *game.WorldConfig)) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	cfg := game.InstantWorldConfig
	cfg.Kind = game.KindPlayer
	cfg.BeginnerProtectionDays = 0
	if tweak != nil {
		tweak(&cfg)
	}
	w := gametest.NewMemWorld(7, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	v, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	return w, v
}

func TestIsShort(t *testing.T) {
	for kind, want := range map[game.WorldKind]bool{game.KindLong: false, game.KindInstant: true, game.KindPlayer: true} {
		if got := game.IsShort(&game.WorldConfig{Kind: kind}); got != want {
			t.Errorf("IsShort(%q) = %v", kind, got)
		}
	}
}

func TestPlayerMapSize(t *testing.T) {
	for players, want := range map[int]int{2: 20, 15: 34, 30: 40, 100: 40} {
		if got := game.PlayerMapSize(players); got != want {
			t.Errorf("PlayerMapSize(%d) = %d, want %d", players, got, want)
		}
	}
}

func TestPlayerWorldEndsAndDominatesLikeInstant(t *testing.T) {
	w, v := playerWorld(t, nil)
	limit := int64(w.Config().DurationMin) * game.MsPerMinute
	if !w.IsScheduled("end:time") {
		t.Fatal("no end at the time limit")
	}
	if w.Config().VillageTotal <= 0 {
		t.Fatal("no village total for domination")
	}
	target := ensureNearestBarbarian(w, v)
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 60})
	if w.State().Totals.AttacksSent == 0 {
		t.Fatal("attacks are not counted")
	}
	w.Config().VillageTotal = 10
	takeVillages(t, w, "h1", 6)
	if err := gametest.Advance(w, w.Now()+1000); err != nil {
		t.Fatal(err)
	}
	if s := w.State(); s.EndedAt == nil || s.EndReason != "domination" || s.WinnerID != "h1" || *s.EndedAt >= limit {
		t.Fatalf("state %+v", s)
	}

	timed, _ := playerWorld(t, nil)
	if err := gametest.Advance(timed, limit); err != nil {
		t.Fatal(err)
	}
	if s := timed.State(); s.EndedAt == nil || *s.EndedAt != limit || s.EndReason != "time" {
		t.Fatalf("state %+v", s)
	}
}

func TestPlayerWorldWritesNoPublicEvents(t *testing.T) {
	w, v := playerWorld(t, nil)
	raid := ensureNearestBarbarian(w, v)
	attackAndLand(t, w, v, raid.X, raid.Y, game.UnitCounts{game.UAxe: 60})
	target := ensureNearestBarbarian(w, v)
	disarm(w, target.ID)
	target.Loyalty = 1
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 50, game.UNoble: 1})
	if target.OwnerID != "h1" {
		t.Fatalf("not conquered: owner %q", target.OwnerID)
	}
	if events := w.PublicEvents(); len(events) != 0 {
		t.Fatalf("a player world wrote public events %+v", events)
	}
}

func TestKickTurnsVillagesBarbarianWithoutRespawn(t *testing.T) {
	w, _ := playerWorld(t, nil)
	takeVillages(t, w, "h1", 2)
	villages := slices.Clone(w.Player("h1").VillageIDs)
	if len(villages) != 2 {
		t.Fatalf("villages %v", villages)
	}
	total := len(w.Villages())
	w.TakeTimeline()
	game.KickPlayer(w, "h1")
	p := w.Player("h1")
	if p.Status != game.StatusQuit || len(p.VillageIDs) != 0 {
		t.Fatalf("player %+v", p)
	}
	for _, id := range villages {
		if o := w.Village(id).OwnerID; o != "" {
			t.Fatalf("village %s still owned by %q", id, o)
		}
	}
	if got := len(w.Villages()); got != total {
		t.Fatalf("villages %d → %d: a respawn", total, got)
	}
	if w.IsScheduled("abandon:h1") {
		t.Fatal("abandon still scheduled")
	}
	if owners := ofKind(w.TakeTimeline(), "owner"); len(owners) != 2 || owners[0].Owner != "" || owners[1].Owner != "" {
		t.Fatalf("timeline %+v", owners)
	}
}

func TestAbandonOfQuitHumanDoesNotRespawn(t *testing.T) {
	w, _ := gametest.NewTestWorld(7)
	total := len(w.Villages())
	game.QuitPlayer(w, "h1")
	if err := gametest.Advance(w, w.Now()+int64(game.BotConfig.AbandonAfterDays+1)*game.MsPerDay); err != nil {
		t.Fatal(err)
	}
	if p := w.Player("h1"); len(p.VillageIDs) != 0 || p.Status != game.StatusQuit {
		t.Fatalf("player %+v", p)
	}
	if got := len(w.Villages()); got != total {
		t.Fatalf("villages %d → %d: a respawn", total, got)
	}
}

func TestEndByHostPicksTopPlayer(t *testing.T) {
	w, _ := playerWorld(t, nil)
	if err := gametest.Advance(w, 5*game.MsPerMinute); err != nil {
		t.Fatal(err)
	}
	points := w.PointsByOwner()
	ps := w.Players()
	sort.SliceStable(ps, func(i, j int) bool {
		if points[ps[i].ID] != points[ps[j].ID] {
			return points[ps[i].ID] > points[ps[j].ID]
		}
		return ps[i].ID < ps[j].ID
	})
	game.EndByHost(w)
	s := w.State()
	if s.EndedAt == nil || *s.EndedAt != w.Now() || s.EndReason != "host" || s.WinnerID != ps[0].ID {
		t.Fatalf("state %+v, top %s", s, ps[0].ID)
	}
	game.EndByHost(w)
	if *w.State().EndedAt != *s.EndedAt {
		t.Fatal("a second end moved the end")
	}
}

func TestNoSightHidesPassingArmies(t *testing.T) {
	for _, noSight := range []bool{false, true} {
		w, a := gametest.NewTestWorld(41)
		w.Config().NoSight = noSight
		w.Player("h1").ProtectedUntil = 0
		gametest.NewHumanPlayer(w, "h2")
		gametest.NewHumanPlayer(w, "h3")
		b := gametest.AddStartVillage(w, "h2", a.X+10, a.Y+5)
		gametest.AddStartVillage(w, "h3", a.X+5, a.Y+3)
		b.Units[game.UAxe] = 100
		attack, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		mid := (attack.DepartedAt + attack.ArrivesAt) / 2
		w.SetNow(mid)
		windows, sighted := game.SightWindows(w, attack), game.Sighted(w, "h3", mid)
		if noSight && (len(windows) != 0 || len(sighted) != 0 || w.IsScheduled("sight:"+attack.ID+":h3:in")) {
			t.Fatalf("no-sight world: windows %+v, sighted %d", windows, len(sighted))
		}
		if !noSight && (len(windows) == 0 || len(sighted) == 0) {
			t.Fatal("the control world sees nothing")
		}
	}
}

func TestShortWorldHumanColoursSkipBotColours(t *testing.T) {
	w, _ := playerWorld(t, func(cfg *game.WorldConfig) { cfg.BotCount = 5 })
	for _, id := range []game.PlayerID{"h2", "h3"} {
		if _, err := game.JoinHuman(w, id, string(id), 2, game.QuadrantRandom); err != nil {
			t.Fatal(err)
		}
	}
	colours := map[string]bool{}
	for _, p := range w.Players() {
		colours[p.Color] = true
	}
	if len(w.Players()) != 8 || len(colours) != 8 {
		t.Fatalf("%d players, %d colours", len(w.Players()), len(colours))
	}
}
