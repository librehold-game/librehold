package game_test

import (
	"errors"
	"strconv"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func wantRule(t *testing.T, err error, kind string) {
	t.Helper()
	var re *game.RuleError
	if !errors.As(err, &re) || re.Code != kind {
		t.Fatalf("err %v, want %s", err, kind)
	}
}

func TestBuildPlanHoldsAtMost200Steps(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	steps := make([]game.BuildingID, 200)
	for i := range steps {
		steps[i] = game.BWall
	}
	wantRule(t, game.SetBuildPlan(w, "h1", v.ID, append(steps, game.BWall)), "planTooLong")
	if len(v.BuildPlan) != 0 {
		t.Fatalf("a refused plan left %d steps", len(v.BuildPlan))
	}
	if err := game.SetBuildPlan(w, "h1", v.ID, steps[:150]); err != nil {
		t.Fatal(err)
	}
	wantRule(t, game.SetBuildPlan(w, "h1", v.ID, steps[:len(steps)-len(v.BuildPlan)+1]), "planTooLong")
	if err := game.SetBuildPlan(w, "h1", v.ID, steps[:200-len(v.BuildPlan)]); err != nil || len(v.BuildPlan) > 200 {
		t.Fatalf("filling the plan to 200: %d steps, %v", len(v.BuildPlan), err)
	}
}

func TestBuildTemplatesAreCapped(t *testing.T) {
	w, _ := gametest.NewTestWorld(5)
	for len(w.Human("h1").BuildTemplates) < game.MaxBuildTemplates {
		if err := game.SaveBuildTemplate(w, "h1", game.BuildTemplate{Name: "t", Steps: []game.BuildingID{game.BWall}}); err != nil {
			t.Fatal(err)
		}
	}
	wantRule(t, game.SaveBuildTemplate(w, "h1", game.BuildTemplate{Name: "one more"}), "tooManyTemplates")
	last := w.Human("h1").BuildTemplates[game.MaxBuildTemplates-1]
	if err := game.SaveBuildTemplate(w, "h1", game.BuildTemplate{ID: last.ID, Name: "renamed"}); err != nil {
		t.Fatalf("changing a template at the cap: %v", err)
	}
}

func TestGroupsAreCappedAndKeepEachVillageOnce(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	if err := game.SaveGroup(w, "h1", game.VillageGroup{Name: "g", VillageIDs: []string{v.ID, v.ID, v.ID}}); err != nil {
		t.Fatal(err)
	}
	if ids := w.Human("h1").Groups[0].VillageIDs; len(ids) != 1 {
		t.Fatalf("group villages %v", ids)
	}
	for i := 1; i < game.MaxGroups; i++ {
		if err := game.SaveGroup(w, "h1", game.VillageGroup{Name: "g" + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	wantRule(t, game.SaveGroup(w, "h1", game.VillageGroup{Name: "one more"}), "tooManyGroups")
}

func TestAutoFarmKeepsEachVillageOnce(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	if err := game.SetAutoFarm(w, "h1", false, []string{v.ID, v.ID}); err != nil {
		t.Fatal(err)
	}
	if ids := w.Human("h1").FarmAssistant.AutoFarmVillages; len(ids) != 1 {
		t.Fatalf("auto-farm villages %v", ids)
	}
}

func TestScheduledSendsAreCapped(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	x, y := gametest.NearestBarbarianXY(w, v)
	in := game.ScheduleSendInput{OriginID: v.ID, X: x, Y: y, Units: game.UnitCounts{game.UAxe: 1}, Type: "attack", Mode: "sendAt",
		At: w.Now() + game.MsPerHour}
	for range game.MaxScheduledSends {
		mustSchedule(t, w, "h1", in)
	}
	_, err := game.ScheduleSend(w, "h1", in)
	wantRule(t, err, "tooManyScheduled")
}
