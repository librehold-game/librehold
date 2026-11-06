package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestFarmRaidsCarryTheFarmMark(t *testing.T) {
	t.Run("templates and the auto-farm mark their raids, the rally point does not", func(t *testing.T) {
		w, v := farmSetup(t)
		x, y := gametest.NearestBarbarianXY(w, v)
		a, err := game.SendFarmTemplate(w, "h1", v.ID, "A", x, y)
		if err != nil || !a.Farm {
			t.Fatalf("template A err %v farm %v", err, a != nil && a.Farm)
		}
		c, err := game.SendTemplateC(w, "h1", v.ID, x, y)
		if err != nil || !c.Farm {
			t.Fatalf("template C err %v farm %v", err, c != nil && c.Farm)
		}
		manual, err := game.SendCommand(w, "h1", v.ID, x, y, game.UnitCounts{game.ULightCav: 5}, "attack", game.SendOpts{})
		if err != nil || manual.Farm {
			t.Fatalf("rally point err %v farm %v", err, manual != nil && manual.Farm)
		}

		w.Human("h1").FarmAssistant.AutoFarmUnlocked = true
		if err := game.SetFarmFilters(w, "h1", game.FarmFiltersPatch{Radius: ptr(6.0), MinMinutesSinceAttack: ptr(0.0)}); err != nil {
			t.Fatal(err)
		}
		if err := game.SetAutoFarm(w, "h1", true, []string{v.ID}); err != nil {
			t.Fatal(err)
		}
		before := len(w.Commands())
		if sent := game.AutoFarmTick(w, "h1", v.ID); sent == 0 {
			t.Fatal("auto-farm sent nothing")
		}
		auto := 0
		for _, cmd := range w.Commands() {
			if cmd.ID != a.ID && cmd.ID != c.ID && cmd.ID != manual.ID {
				auto++
				if !cmd.Farm {
					t.Fatalf("auto-farm raid %s has no farm mark", cmd.ID)
				}
			}
		}
		if auto != len(w.Commands())-before {
			t.Fatalf("auto %d of %d new", auto, len(w.Commands())-before)
		}
	})

	t.Run("the way home keeps the mark", func(t *testing.T) {
		w, v := farmSetup(t)
		x, y := gametest.NearestBarbarianXY(w, v)
		game.EnsureVillage(w, x, y).Units = game.Units{}
		raid, err := game.SendFarmTemplate(w, "h1", v.ID, "A", x, y)
		if err != nil {
			t.Fatal(err)
		}
		manual, err := game.SendCommand(w, "h1", v.ID, x, y, game.UnitCounts{game.ULightCav: 5}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		advance(t, w, max(raid.ArrivesAt, manual.ArrivesAt))
		var farm, plain int
		for _, cmd := range w.Commands() {
			if cmd.Type != "return" {
				continue
			}
			if cmd.Farm {
				farm++
			} else {
				plain++
			}
		}
		if farm != 1 || plain != 1 {
			t.Fatalf("returns: %d marked, %d plain", farm, plain)
		}
	})

	t.Run("a scouting raid from a template comes home marked too", func(t *testing.T) {
		w, v := farmSetup(t)
		v.Units[game.UScout] = 10
		if err := game.SetFarmTemplate(w, "h1", "B", game.UnitCounts{game.UScout: 1}); err != nil {
			t.Fatal(err)
		}
		x, y := gametest.NearestBarbarianXY(w, v)
		game.EnsureVillage(w, x, y).Units = game.Units{}
		raid, err := game.SendFarmTemplate(w, "h1", v.ID, "B", x, y)
		if err != nil {
			t.Fatal(err)
		}
		advance(t, w, raid.ArrivesAt)
		for _, cmd := range w.Commands() {
			if cmd.Type == "return" && !cmd.Farm {
				t.Fatal("scouts came home without the farm mark")
			}
		}
	})
}

func TestHideFarmOnMap(t *testing.T) {
	w, _ := gametest.NewTestWorld(2)
	if w.Human("h1").FarmAssistant.HideOnMap {
		t.Fatal("farming hidden by default")
	}
	if err := game.HideFarmOnMap(w, "h1", true); err != nil || !w.Human("h1").FarmAssistant.HideOnMap {
		t.Fatalf("err %v hidden %v", err, w.Human("h1").FarmAssistant.HideOnMap)
	}
	if err := game.HideFarmOnMap(w, "h1", false); err != nil || w.Human("h1").FarmAssistant.HideOnMap {
		t.Fatalf("err %v hidden %v", err, w.Human("h1").FarmAssistant.HideOnMap)
	}
	if err := game.HideFarmOnMap(w, "nobody", true); err == nil {
		t.Fatal("a player without a human state changed the setting")
	}
}
