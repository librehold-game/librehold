package game_test

import (
	"math"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// localWorld is h1's world whose clock starts at 09:00 local time, so world days and local days differ.
func localWorld(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w := gametest.NewMemWorld(8, gametest.TestConfig(), clockOrigin)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	v, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	v.Units = game.Units{}
	return w, v
}

func TestTodayCountersFollowTheLocalDay(t *testing.T) {
	w, v := localWorld(t)
	target := ensureNearestBarbarian(w, v)
	disarm(w, target.ID)
	target.Resources = game.Resources{Wood: 500, Clay: 500, Iron: 500}
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 10})
	s := w.Human("h1").Stats
	if today := game.LocalDayIndex(w, w.Now()); s.AttacksToday.Day != game.LocalDayIndex(w, 0) || s.LootedToday.Day != today {
		t.Fatalf("attacks day %d, loot day %d, local day %d", s.AttacksToday.Day, s.LootedToday.Day, today)
	}
	if s.LootedToday.Total <= 0 {
		t.Fatalf("nothing looted: %+v", s.LootedToday)
	}

	w.SetNow(16 * game.MsPerHour) // 01:00 the next local day, still world day 0
	v.Units[game.UAxe] = 10
	mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 10}, "attack")
	if s := w.Human("h1").Stats; s.AttacksToday.Count != 1 || s.AttacksToday.Day != game.LocalDayIndex(w, w.Now()) {
		t.Fatalf("after local midnight: %+v", s.AttacksToday)
	}
}

func TestEstimatedLootIsUnknownWithoutOwnReports(t *testing.T) {
	w, v := farmSetup(t)
	target := ensureNearestBarbarian(w, v)
	if ft := farmTargetOf(w, v, target.ID); ft.EstimatedLoot != 0 {
		t.Fatalf("an estimate without any report: %+v", ft)
	}
}

func TestEstimatedLootIgnoresBuildingsNeverSeen(t *testing.T) {
	w, v := farmSetup(t)
	target := ensureNearestBarbarian(w, v)
	w.PutIntel("h1", target.ID, &game.Intel{
		LastFarmedAt: ptr(w.Now()),
		Intel:        &game.VillageIntel{At: w.Now(), LastHaul: &game.Resources{Wood: 10}, LastHaulCapacity: ptr(400.0), LastLosses: ptr(0)},
	})
	w.SetNow(w.Now() + 2*game.MsPerHour)
	before := farmTargetOf(w, v, target.ID).EstimatedLoot
	unseen := game.Village{Bonus: game.TileInfo(w, target.X, target.Y).Bonus}
	unseen.Buildings[game.BStorage] = 1
	want := 0.0
	for _, r := range game.ResourceIDs {
		want += game.VillageProductionPerHour(&unseen, r, w.Config()) * 2
	}
	if before != math.Floor(want) {
		t.Fatalf("two hours after a raid %v, want %v from mines at level 0", before, math.Floor(want))
	}
	for _, b := range []game.BuildingID{game.BWood, game.BClay, game.BIron, game.BStorage} {
		target.Buildings[b] = 20
	}
	if after := farmTargetOf(w, v, target.ID).EstimatedLoot; after != before {
		t.Fatalf("the estimate reads the real buildings: %v, then %v", before, after)
	}
}

func TestEstimatedLootUsesWhatTheScoutSaw(t *testing.T) {
	w, v := farmSetup(t)
	target := ensureNearestBarbarian(w, v)
	disarm(w, target.ID)
	for _, b := range []game.BuildingID{game.BWood, game.BClay, game.BIron, game.BStorage} {
		target.Buildings[b] = 15
	}
	target.Resources = game.Resources{Wood: 3000, Clay: 2000, Iron: 1000}
	if err := gametest.ScoutOnce(w, v, target); err != nil {
		t.Fatal(err)
	}
	seen := w.Intel("h1", target.ID).Intel.Scouted
	if seen == nil || seen.Buildings != target.Buildings || seen.At != w.Now() {
		t.Fatalf("scouted %+v", seen)
	}
	if got, want := farmTargetOf(w, v, target.ID).EstimatedLoot, game.SumResources(seen.Resources); got != want || want < 6000 {
		t.Fatalf("estimate right after scouting %v, want the %v seen", got, want)
	}
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 10})
	if in := w.Intel("h1", target.ID).Intel; in.Scouted == nil || in.Scouted.Buildings != target.Buildings {
		t.Fatalf("a raid forgot the scouted buildings: %+v", in)
	}
}

func TestFilterReasonSaysWhyATargetIsHidden(t *testing.T) {
	w, v := farmSetup(t)
	target := ensureNearestBarbarian(w, v)
	ft := farmTargetOf(w, v, target.ID)
	if r := game.FilterReason(w, "h1", *ft); r != "" {
		t.Fatalf("an untouched target hidden: %q", r)
	}
	for _, c := range []struct {
		edit func(*game.FarmTarget)
		want string
	}{
		{func(f *game.FarmTarget) { f.Inbound = true }, "inbound"},
		{func(f *game.FarmTarget) { f.LastLosses = ptr(2) }, "losses"},
		{func(f *game.FarmTarget) { f.Wall = ptr(9) }, "wall"},
		{func(f *game.FarmTarget) { f.LastAttackAt = ptr(w.Now()) }, "recent"},
	} {
		f := *ft
		c.edit(&f)
		if r := game.FilterReason(w, "h1", f); r != c.want || game.PassesFilters(w, "h1", f) {
			t.Fatalf("reason %q, want %q", r, c.want)
		}
	}
}
