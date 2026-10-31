package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
)

func TestObserveHuman(t *testing.T) {
	t.Run("what the player has: economy, queues, armies away and attacks coming", func(t *testing.T) {
		w, v := armedVillage(t)
		v.Units[game.USpear] = 20
		if _, err := game.EnqueueBuild(w, "h1", v.ID, game.BWood); err != nil {
			t.Fatal(err)
		}
		mustRecruit(t, w, v, game.USpear, 3)
		x, y := nearestUnstoredBarbarian(t, w, v, 1)
		out := mustSend(t, w, v, x, y, game.UnitCounts{game.USpear: 15}, "attack")
		w.PutCommand(&game.Command{ID: "c_in", Type: "attack", OriginID: "v:1:1", TargetID: v.ID,
			Units: game.UnitCounts{game.UAxe: 500}, ArrivesAt: w.Now() + game.MsPerHour})

		o := game.ObserveHuman(w, "h1")
		if len(o.Villages) != 1 {
			t.Fatalf("villages %+v", o.Villages)
		}
		ov := o.Villages[0]
		if ov.ID != v.ID || ov.Buildings[game.BMain] != 10 || ov.UnitsHome[game.USpear] != 5 || ov.UnitsAway[game.USpear] != 15 {
			t.Fatalf("village %+v", ov)
		}
		if ov.StorageCap != game.VillageStorageCapacity(v) || ov.PopCap != game.VillageFarmCapacity(v) ||
			ov.PopUsed != game.PopulationUsed(w, v) || ov.Production.Wood != game.VillageProductionPerHour(v, game.Wood, w.Config()) {
			t.Fatalf("economy %+v", ov)
		}
		if len(ov.BuildQueue) != 1 || ov.BuildQueue[0].Building != game.BWood || len(ov.RecruitQueue) != 1 ||
			ov.RecruitQueue[0].Unit != game.USpear || ov.RecruitQueue[0].Left != 3 {
			t.Fatalf("queues %+v %+v", ov.BuildQueue, ov.RecruitQueue)
		}
		if len(ov.Outgoing) != 1 || ov.Outgoing[0].ID != out.ID || ov.Outgoing[0].Units[game.USpear] != 15 {
			t.Fatalf("outgoing %+v", ov.Outgoing)
		}
		if len(ov.Incoming) != 1 || ov.Incoming[0].ID != "c_in" || ov.Incoming[0].Type != "attack" || ov.Incoming[0].Units != nil {
			t.Fatalf("incoming %+v — its troops are hidden from the player", ov.Incoming)
		}
		if o.Points != w.Player("h1").Points {
			t.Fatalf("points %d", o.Points)
		}
		if ov.Research != game.UnitCounts(v.Research) || o.FarmAssistant == nil ||
			len(o.FarmAssistant.Templates) != len(w.Human("h1").FarmAssistant.Templates) {
			t.Fatalf("research %v farm assistant %+v", ov.Research, o.FarmAssistant)
		}
	})

	t.Run("an order aimed at a tile: the target as the player knows it and the farms they could have picked", func(t *testing.T) {
		w, v := armedVillage(t)
		v.Units[game.USpear] = 100
		x, y := nearestUnstoredBarbarian(t, w, v, 1)
		c := mustSend(t, w, v, x, y, game.UnitCounts{game.USpear: 20}, "attack")
		advanceTouch(t, w, v, c.ArrivesAt*3)

		choice := game.ObserveChoice(w, "h1", v.ID, x, y)
		tg := choice.Target
		if tg.Owner != "barbarian" || tg.X != x || tg.Y != y || tg.Points == 0 || tg.Distance != game.Distance(v.X, v.Y, x, y) ||
			tg.LastHaul == nil || tg.Wall == nil || tg.Status == "untouched" {
			t.Fatalf("target %+v — want the barbarian with what the raid's report told", tg)
		}
		if n := len(choice.Options); n == 0 || n > game.ObservedOptions {
			t.Fatalf("%d options", n)
		}
		for i := 1; i < len(choice.Options); i++ {
			if choice.Options[i].Distance < choice.Options[i-1].Distance {
				t.Fatalf("options not nearest first: %+v", choice.Options)
			}
		}
		if !slices.ContainsFunc(choice.Options, func(f game.FarmTarget) bool { return f.ID == tg.ID }) {
			t.Fatal("the nearest farm is not among the options")
		}
		if own := game.ObserveChoice(w, "h1", v.ID, v.X, v.Y).Target; own.Owner != "own" {
			t.Fatalf("own village %+v", own)
		}
	})
}
