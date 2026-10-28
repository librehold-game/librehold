package game_test

import (
	"errors"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestBuildPlan(t *testing.T) {
	t.Run("tops up the queue with buildings when slots and resources allow; skips impossible ones", func(t *testing.T) {
		w, v := gametest.NewTestWorld(2)
		v.Resources = game.Resources{Wood: 300, Clay: 300, Iron: 300}
		if err := game.SetBuildPlan(w, "h1", v.ID, []game.BuildingID{game.BWood, game.BClay, game.BBarracks, game.BIron, game.BStatue}); err != nil {
			t.Fatal(err)
		}
		var queued []game.BuildingID
		for _, j := range v.BuildQueue {
			queued = append(queued, j.Building)
		}
		if !slices.Equal(queued, []game.BuildingID{game.BWood, game.BClay}) || !slices.Equal(v.BuildPlan, []game.BuildingID{game.BBarracks, game.BIron, game.BStatue}) {
			t.Fatalf("queue %v plan %v", queued, v.BuildPlan)
		}
		advance(t, w, v.BuildQueue[1].CompletesAt)
		if !slices.ContainsFunc(v.BuildQueue, func(j game.BuildJob) bool { return j.Building == game.BIron }) || slices.Contains(v.BuildPlan, game.BBarracks) {
			t.Fatalf("queue %+v plan %v", v.BuildQueue, v.BuildPlan)
		}
	})

	t.Run("the plan waits for resources and tops up repeatedly", func(t *testing.T) {
		w, v := gametest.NewTestWorld(2)
		v.Buildings[game.BWood], v.Buildings[game.BClay], v.Buildings[game.BIron] = 5, 5, 5
		v.Resources = game.Resources{}
		if err := game.SetBuildPlan(w, "h1", v.ID, []game.BuildingID{game.BMain}); err != nil {
			t.Fatal(err)
		}
		if len(v.BuildQueue) != 0 || !slices.Equal(v.BuildPlan, []game.BuildingID{game.BMain}) {
			t.Fatal("queued without resources")
		}
		advance(t, w, 5*game.MsPerHour)
		if len(v.BuildQueue)+(v.Buildings[game.BMain]-1) < 1 || v.BuildPlan != nil || game.FeedBuildPlan(w, v.ID) != 0 {
			t.Fatalf("queue %d main %d plan %v", len(v.BuildQueue), v.Buildings[game.BMain], v.BuildPlan)
		}
	})

	t.Run("a build template as a plan; saving and deleting templates", func(t *testing.T) {
		w, v := gametest.NewTestWorld(2)
		v.Resources = game.Resources{}
		if err := game.ApplyBuildTemplate(w, "h1", v.ID, "tpl_def"); err != nil || len(v.BuildPlan) != 20 {
			t.Fatalf("err %v plan %d", err, len(v.BuildPlan))
		}
		if err := game.ClearBuildPlan(w, "h1", v.ID); err != nil || v.BuildPlan != nil || w.IsScheduled("plan:"+v.ID) {
			t.Fatal("plan not cleared")
		}
		tpl := game.BuildTemplate{ID: "mine", Name: "Mine", Steps: []game.BuildingID{game.BWood}}
		if err := game.SaveBuildTemplate(w, "h1", tpl); err != nil || len(w.Human("h1").BuildTemplates) != 4 {
			t.Fatal("save")
		}
		if err := game.DeleteBuildTemplate(w, "h1", "mine"); err != nil || len(w.Human("h1").BuildTemplates) != 3 {
			t.Fatal("delete")
		}
	})
}

func farmSetup(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w, v := gametest.NewTestWorld(8)
	v.Units[game.ULightCav], v.Units[game.USpear] = 200, 200
	if err := game.SetFarmTemplate(w, "h1", "A", game.UnitCounts{game.ULightCav: 5}); err != nil {
		t.Fatal(err)
	}
	if err := game.SetFarmTemplate(w, "h1", "C", game.UnitCounts{game.ULightCav: 1}); err != nil {
		t.Fatal(err)
	}
	return w, v
}

func farmTargetOf(w *gametest.MemWorld, v *game.Village, id string) *game.FarmTarget {
	for _, ft := range game.FarmTargets(w, "h1", v.ID) {
		if ft.ID == id {
			return &ft
		}
	}
	return nil
}

func TestFarmAssistant(t *testing.T) {
	t.Run("target list from the radius, statuses and filters", func(t *testing.T) {
		w, v := farmSetup(t)
		targets := game.FarmTargets(w, "h1", v.ID)
		if len(targets) <= 20 {
			t.Fatalf("%d targets", len(targets))
		}
		for _, ft := range targets {
			if ft.Distance > 15 || ft.Status != "untouched" {
				t.Fatalf("target %+v", ft)
			}
		}
		tv := game.EnsureVillage(w, targets[0].X, targets[0].Y)
		w.PutIntel("h1", tv.ID, &game.Intel{
			LastFarmedAt: ptr(w.Now()),
			Intel: &game.VillageIntel{At: w.Now(), Wall: 5, Loyalty: 100, LastHaul: &game.Resources{Wood: 10, Clay: 10, Iron: 10},
				LastHaulCapacity: ptr(400.0), LastLosses: ptr(0)},
		})
		again := farmTargetOf(w, v, tv.ID)
		if again.Status != "partial" || again.Wall == nil || *again.Wall != 5 || game.PassesFilters(w, "h1", *again) {
			t.Fatalf("target %+v", again)
		}
		if err := game.SetFarmFilters(w, "h1", game.FarmFiltersPatch{MaxWall: ptr(20), MinMinutesSinceAttack: ptr(0.0)}); err != nil {
			t.Fatal(err)
		}
		if !game.PassesFilters(w, "h1", *again) {
			t.Fatal("filters not applied")
		}
		w.Intel("h1", tv.ID).Intel.LastLosses = ptr(3)
		if game.PassesFilters(w, "h1", *farmTargetOf(w, v, tv.ID)) {
			t.Fatal("losses not filtered")
		}
	})

	t.Run("template C sizes the number of packs to the estimated loot", func(t *testing.T) {
		w, v := farmSetup(t)
		if u, ok := game.TemplateCUnits(w, "h1", v, 1000); !ok || u[game.ULightCav] != 13 {
			t.Fatalf("units %v", u)
		}
		if u, _ := game.TemplateCUnits(w, "h1", v, 100_000); u[game.ULightCav] != 200 {
			t.Fatalf("units %v", u)
		}
		if err := game.SetFarmTemplate(w, "h1", "C", game.UnitCounts{}); err != nil {
			t.Fatal(err)
		}
		if _, ok := game.TemplateCUnits(w, "h1", v, 1000); ok {
			t.Fatal("empty template")
		}
	})

	t.Run("auto-farm: sends template A to the filtered targets, deterministically offline", func(t *testing.T) {
		setup := func() (*gametest.MemWorld, *game.Village, int) {
			w, v := farmSetup(t)
			w.Human("h1").FarmAssistant.AutoFarmUnlocked = true
			if err := game.SetFarmFilters(w, "h1", game.FarmFiltersPatch{Radius: ptr(6.0)}); err != nil {
				t.Fatal(err)
			}
			if err := game.SetAutoFarm(w, "h1", true, []string{v.ID}); err != nil {
				t.Fatal(err)
			}
			sent := game.AutoFarmTick(w, "h1", v.ID)
			return w, v, sent
		}
		w, v, sent := setup()
		if sent == 0 || len(w.Commands()) != sent || v.Units[game.ULightCav] != 200-5*sent {
			t.Fatalf("sent %d commands %d lc %d", sent, len(w.Commands()), v.Units[game.ULightCav])
		}
		a, va, _ := setup()
		b, vb, _ := setup()
		advanceTouch(t, a, va, game.MsPerDay)
		for i := int64(1); i <= 500; i++ {
			advanceTouch(t, b, vb, game.MsPerDay*i/500)
		}
		if len(a.ReportsOf("h1")) != len(b.ReportsOf("h1")) || a.Human("h1").Stats.AttacksSent != b.Human("h1").Stats.AttacksSent ||
			a.Human("h1").Stats.AttacksSent <= sent || va.Units != vb.Units {
			t.Fatalf("jump %d/%d steps %d/%d", len(a.ReportsOf("h1")), a.Human("h1").Stats.AttacksSent, len(b.ReportsOf("h1")), b.Human("h1").Stats.AttacksSent)
		}
	})

	t.Run("sending a template from the chosen village", func(t *testing.T) {
		w, v := farmSetup(t)
		x, y := gametest.NearestBarbarianXY(w, v)
		c, err := game.SendFarmTemplate(w, "h1", v.ID, "A", x, y)
		if err != nil || c.Units != (game.UnitCounts{game.ULightCav: 5}) {
			t.Fatalf("err %v command %+v", err, c)
		}
		if _, err := game.SendFarmTemplate(w, "h1", v.ID, "B", x, y); err == nil {
			t.Fatal("empty template B sent")
		}
		if c, err := game.SendTemplateC(w, "h1", v.ID, x, y); err != nil || c.Units[game.ULightCav] < 1 {
			t.Fatalf("template C err %v", err)
		}
	})
}

func TestFarmTargetsHideOnlyOwnInbound(t *testing.T) {
	w, v1 := farmSetup(t)
	v2, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	near := game.FarmTargets(w, "h1", v1.ID)
	target := near[0]
	v2.Units[game.UAxe] = 10
	mustSend(t, w, v2, target.X, target.Y, game.UnitCounts{game.UAxe: 10}, "attack")
	got := farmTargetOf(w, v1, target.ID)
	if got == nil || got.Inbound || !game.PassesFilters(w, "h1", *got) {
		t.Fatalf("h2's attack visible to h1: %+v", got)
	}
	mustSend(t, w, v1, target.X, target.Y, game.UnitCounts{game.ULightCav: 5}, "attack")
	if !farmTargetOf(w, v1, target.ID).Inbound {
		t.Fatal("own attack not inbound")
	}
	if len(game.FarmTargets(w, "h2", v1.ID)) != 0 {
		t.Fatal("farm targets of a foreign village")
	}
}

func TestMassBuildSkipsForeign(t *testing.T) {
	w, v1 := gametest.NewTestWorld(5)
	v2, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	if n := game.MassBuild(w, "h1", []string{v1.ID, v2.ID}, game.BWood); n != 1 || len(v2.BuildQueue) != 0 || len(v1.BuildQueue) != 1 {
		t.Fatalf("built %d", n)
	}
	v1.Buildings[game.BBarracks] = 1
	v1.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	v2.Buildings[game.BBarracks] = 1
	if n := game.MassRecruit(w, "h1", []string{v1.ID, v2.ID}, game.USpear, 5); n != 5 || len(v2.RecruitQueues.Barracks) != 0 {
		t.Fatalf("recruited %d", n)
	}
}

func TestRenameVillageValidation(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	if err := game.RenameVillage(w, "h1", v.ID, "  Café Hill  "); err != nil || v.Name != "Café Hill" {
		t.Fatalf("err %v name %q", err, v.Name)
	}
	var re *game.RuleError
	for _, bad := range []string{"   ", "abcdefghijabcdefghijabcdefghijabc"} {
		if err := game.RenameVillage(w, "h1", v.ID, bad); !errors.As(err, &re) || re.Code != "nameLength" {
			t.Fatalf("%q: err %v", bad, err)
		}
	}
	gametest.NewHumanPlayer(w, "h2")
	var be *game.BlockedError
	if err := game.RenameVillage(w, "h2", v.ID, "Mine"); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("foreign rename err %v", err)
	}
}

func TestGroups(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	if err := game.SaveGroup(w, "h1", game.VillageGroup{ID: "g1", Name: "Front", VillageIDs: []string{v.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := game.SaveGroup(w, "h1", game.VillageGroup{ID: "g1", Name: "Rear", VillageIDs: []string{v.ID}}); err != nil || len(w.Human("h1").Groups) != 1 || w.Human("h1").Groups[0].Name != "Rear" {
		t.Fatal("update")
	}
	if err := game.SaveGroup(w, "h1", game.VillageGroup{ID: "g2", Name: "Obce", VillageIDs: []string{"v:0:0"}}); err == nil {
		t.Fatal("group with a foreign village")
	}
	if err := game.DeleteGroup(w, "h1", "g1"); err != nil || len(w.Human("h1").Groups) != 0 {
		t.Fatal("delete")
	}
}

func TestReportManagement(t *testing.T) {
	w, v := gametest.NewTestWorld(101)
	v.Units[game.UScout] = 3
	for range 3 {
		x, y := nearestBarbarianTile(w, v)
		c := mustSend(t, w, v, x, y, game.UnitCounts{game.UScout: 1}, "attack")
		advanceTouch(t, w, v, c.ArrivesAt)
	}
	reps := w.Reports("h1", 1<<62, 0)
	if len(reps) != 3 || reps[0].Time < reps[2].Time {
		t.Fatalf("%d reports, newest first", len(reps))
	}
	if err := game.MarkReportsRead(w, "h1", []string{reps[0].ID}, true); err != nil || !reps[0].Read || reps[1].Read {
		t.Fatal("mark one")
	}
	if err := game.DeleteReadReports(w, "h1"); err != nil || len(w.Reports("h1", 1<<62, 0)) != 2 {
		t.Fatal("delete read")
	}
	if err := game.MarkAllReportsRead(w, "h1"); err != nil {
		t.Fatal(err)
	}
	for _, r := range w.Reports("h1", 1<<62, 0) {
		if !r.Read {
			t.Fatal("not all read")
		}
	}
	id := w.Reports("h1", 1<<62, 0)[0].ID
	if err := game.DeleteReport(w, "h1", id); err != nil || len(w.Reports("h1", 1<<62, 0)) != 1 {
		t.Fatal("delete one")
	}
	if got := w.Reports("h1", 1<<62, 1); len(got) != 1 {
		t.Fatal("limit")
	}
}

func TestStatsCounters(t *testing.T) {
	t.Run("stat counters: scouting vs attack, support", func(t *testing.T) {
		w, v := gametest.NewTestWorld(14)
		v.Units[game.USpear], v.Units[game.UScout], v.Buildings[game.BFarm] = 100, 10, 10
		x := v.X + 3
		for !game.TileHasVillage(w.Seed(), w.Config(), x, v.Y) {
			x++
		}
		mustSend(t, w, v, x, v.Y, game.UnitCounts{game.UScout: 2}, "attack")
		mustSend(t, w, v, x, v.Y, game.UnitCounts{game.USpear: 10}, "attack")
		if s := w.Human("h1").Stats; s.ScoutsSent != 1 || s.AttacksSent != 1 {
			t.Fatalf("stats %+v", s)
		}
	})

	t.Run("point sampling: ≥ 1 h apart, thinned above 1000 samples", func(t *testing.T) {
		w, _ := gametest.NewTestWorld(16)
		h := w.Human("h1")
		h.Stats.PointsHistory = [][2]int64{{0, int64(game.PlayerPoints(w, "h1"))}}
		game.SamplePoints(w, "h1")
		if len(h.Stats.PointsHistory) != 1 {
			t.Fatal("sampled too early")
		}
		w.SetNow(game.MsPerHour)
		game.SamplePoints(w, "h1")
		if len(h.Stats.PointsHistory) != 2 {
			t.Fatal("not sampled")
		}
		for range 1100 {
			w.SetNow(w.Now() + game.MsPerHour)
			game.SamplePoints(w, "h1")
		}
		if len(h.Stats.PointsHistory) > 1000 {
			t.Fatalf("%d samples", len(h.Stats.PointsHistory))
		}
		for i := 1; i < len(h.Stats.PointsHistory); i++ {
			if h.Stats.PointsHistory[i][0] <= h.Stats.PointsHistory[i-1][0] {
				t.Fatal("not increasing")
			}
		}
	})
}

func TestEarlyPace(t *testing.T) {
	nearest := func(w *gametest.MemWorld, v *game.Village) *game.Village {
		bx, by, best := 0, 0, 1e9
		for x := v.X - 12; x <= v.X+12; x++ {
			for y := v.Y - 12; y <= v.Y+12; y++ {
				d := game.Distance(v.X, v.Y, x, y)
				if d < 2 || !game.TileHasVillage(w.Seed(), w.Config(), x, y) || (w.Village(game.VillageIDFor(x, y)) != nil && w.Village(game.VillageIDFor(x, y)).OwnerID != "") {
					continue
				}
				if d < best {
					bx, by, best = x, y, d
				}
			}
		}
		return game.EnsureVillage(w, bx, by)
	}

	t.Run("default world: speed 8×, starting guard and an earlier auto-farm", func(t *testing.T) {
		if game.DefaultWorldConfig.WorldSpeed != 8 || game.FarmBalance.AutoFarmUnlockVillages > 3 {
			t.Fatal("defaults")
		}
		w, v := gametest.NewDefaultWorld(200)
		if w.Config().WorldSpeed != 8 || v.Units[game.USpear] != game.StartVillage.Units[game.USpear] || v.Points <= 0 {
			t.Fatal("start village")
		}
		b := nearest(w, v)
		if game.UnitCounts(b.Units).Total() != 0 || b.Buildings[game.BWall] != 0 {
			t.Fatal("barbarian guarded")
		}
	})

	t.Run("a first raid with a handful of spearmen on an empty barbarian village succeeds within the first hour", func(t *testing.T) {
		w, v := gametest.NewDefaultWorld(201)
		target := nearest(w, v)
		c := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.USpear: 10}, "attack")
		if c.ArrivesAt-w.Now() >= 2*game.MsPerHour {
			t.Fatal("too far")
		}
		advance(t, w, c.ArrivesAt+1000)
		looted := w.Human("h1").Stats.ResourcesLooted
		if lastReport(w, "h1").Outcome != "attackerWon" || game.SumResources(looted) <= 0 {
			t.Fatal("first raid failed")
		}
	})
}
