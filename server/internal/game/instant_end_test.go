package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestInstantEndsAtTimeLimit(t *testing.T) {
	w, _ := newInstant(t, 3)
	limit := int64(game.InstantWorldConfig.DurationMin) * game.MsPerMinute
	if err := gametest.Advance(w, limit-1); err != nil {
		t.Fatal(err)
	}
	if w.State().EndedAt != nil {
		t.Fatal("ended before the limit")
	}
	if err := gametest.Advance(w, limit); err != nil {
		t.Fatal(err)
	}
	s := w.State()
	if s.EndedAt == nil || *s.EndedAt != limit || s.EndReason != "time" || s.WinnerID == "" {
		t.Fatalf("state %+v", s)
	}
}

func TestLongWorldNeverEnds(t *testing.T) {
	w, _ := gametest.NewWorldWithHuman(3, gametest.TestConfig())
	if err := gametest.Advance(w, 3*game.MsPerDay); err != nil {
		t.Fatal(err)
	}
	if w.State().EndedAt != nil {
		t.Fatalf("a long world ended: %+v", w.State())
	}
}

// takeVillages hands the owner barbarian villages next to its home until it holds n.
func takeVillages(t *testing.T, w *gametest.MemWorld, owner game.PlayerID, n int) {
	t.Helper()
	home := w.Village(w.Player(owner).VillageIDs[0])
	for r := 1; len(w.Player(owner).VillageIDs) < n && r < 40; r++ {
		for x := home.X - r; x <= home.X+r && len(w.Player(owner).VillageIDs) < n; x++ {
			for y := home.Y - r; y <= home.Y+r && len(w.Player(owner).VillageIDs) < n; y++ {
				v := game.EnsureVillage(w, x, y)
				if v == nil || v.OwnerID != "" {
					continue
				}
				game.TransferVillage(w, v, game.Ref(owner), home.ID)
			}
		}
	}
}

func TestDominationEndsEarlyOnlyForHumans(t *testing.T) {
	w, _ := newInstant(t, 3)
	cfg := w.Config()
	cfg.VillageTotal = 10
	var bot game.PlayerID
	for _, p := range w.Players() {
		if !p.IsHuman {
			bot = p.ID
			break
		}
	}
	takeVillages(t, w, bot, 7)
	if err := gametest.Advance(w, w.Now()+1000); err != nil {
		t.Fatal(err)
	}
	if w.State().EndedAt != nil {
		t.Fatalf("a bot ended the world by domination: %+v", w.State())
	}
	takeVillages(t, w, "h1", 6)
	if err := gametest.Advance(w, w.Now()+1000); err != nil {
		t.Fatal(err)
	}
	s := w.State()
	if s.EndedAt == nil || s.EndReason != "domination" || s.WinnerID != "h1" {
		t.Fatalf("state %+v", s)
	}
}

func TestMarchInFlightAtEndFreezes(t *testing.T) {
	w, v := newInstant(t, 3)
	limit := int64(game.InstantWorldConfig.DurationMin) * game.MsPerMinute
	if err := gametest.Advance(w, limit-2000); err != nil {
		t.Fatal(err)
	}
	x, y := gametest.NearestBarbarianXY(w, v)
	c, err := game.SendCommand(w, "h1", v.ID, x, y, game.UnitCounts{game.UAxe: 10}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, c.ArrivesAt+game.MsPerMinute); err != nil {
		t.Fatal(err)
	}
	if w.Command(c.ID) == nil {
		t.Fatal("the march resolved after the world ended")
	}
}

func TestInstantCountsAttacksAndTheLargestBattle(t *testing.T) {
	w, v := newInstant(t, 3)
	x, y := gametest.NearestBarbarianXY(w, v)
	c, err := game.SendCommand(w, "h1", v.ID, x, y, game.UnitCounts{game.UAxe: 40}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, c.ArrivesAt+1); err != nil {
		t.Fatal(err)
	}
	tot := w.State().Totals
	if tot.AttacksSent < 1 || tot.LargestBattleUnits < 40 {
		t.Fatalf("totals %+v", tot)
	}
}
