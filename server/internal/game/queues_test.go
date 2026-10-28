package game_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func blockerKinds(bs []game.Blocker) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Kind()
	}
	return out
}

func mustBuild(t *testing.T, w game.World, v *game.Village, b game.BuildingID) *game.BuildJob {
	t.Helper()
	job, err := game.EnqueueBuild(w, game.PlayerID(v.OwnerID), v.ID, b)
	if err != nil {
		t.Fatalf("EnqueueBuild %s: %v", b, err)
	}
	return job
}

func advance(t *testing.T, w *gametest.MemWorld, until int64) {
	t.Helper()
	if err := gametest.Advance(w, until); err != nil {
		t.Fatal(err)
	}
}

func TestQueue(t *testing.T) {
	t.Run("an order takes resources and schedules the time", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		job := mustBuild(t, w, v, game.BWood)
		if v.Resources != (game.Resources{Wood: 450, Clay: 440, Iron: 460}) {
			t.Fatalf("resources %+v", v.Resources)
		}
		if job.TargetLevel != 1 || job.StartedAt != 0 || job.CompletesAt != game.BuildTimeMs(game.BWood, 1, 1, 1) {
			t.Fatalf("job %+v", job)
		}
	})

	t.Run("the second job starts after the first; the third is blocked (2 slots)", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		a := mustBuild(t, w, v, game.BWood)
		b := mustBuild(t, w, v, game.BWood)
		if b.TargetLevel != 2 || b.StartedAt != a.CompletesAt || b.CompletesAt != a.CompletesAt+game.BuildTimeMs(game.BWood, 2, 1, 1) {
			t.Fatalf("b %+v a %+v", b, a)
		}
		if !slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BClay).Blockers), "queueFull") {
			t.Fatal("no queueFull")
		}
		if _, err := game.EnqueueBuild(w, "h1", v.ID, game.BClay); err == nil {
			t.Fatal("third build accepted")
		}
	})

	t.Run("requirements, max level, resources, population", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		if !slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BBarracks).Blockers), "requirements") {
			t.Fatal("barracks requirements")
		}
		v.Buildings[game.BStatue] = 1
		if !slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BStatue).Blockers), "maxLevel") {
			t.Fatal("statue max")
		}
		v.Resources = game.Resources{}
		res := game.CheckBuild(w, v, game.BWood)
		var missing any
		for _, b := range res.Blockers {
			if b.Kind() == "resources" {
				missing = b["missing"]
			}
		}
		if missing != (game.Resources{Wood: 50, Clay: 60, Iron: 40}) {
			t.Fatalf("missing %v", missing)
		}
		v.Resources = game.Resources{Wood: 99999, Clay: 99999, Iron: 99999}
		v.Buildings[game.BFarm] = 1
		v.Buildings[game.BMain] = 30
		if !slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BMain).Blockers), "maxLevel") {
			t.Fatal("main max")
		}
		v.Buildings[game.BMain] = 3
		v.Buildings[game.BBarracks] = 25
		v.Buildings[game.BSmithy] = 20
		v.Buildings[game.BMarket] = 25
		if game.VillagePopulationUsed(v) <= 240 {
			t.Fatalf("pop %d", game.VillagePopulationUsed(v))
		}
		if !slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BWood).Blockers), "population") {
			t.Fatal("population")
		}
	})

	t.Run("completion via advance raises level, points and production", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		job := mustBuild(t, w, v, game.BWood)
		before := v.Points
		advance(t, w, job.CompletesAt-1)
		if v.Buildings[game.BWood] != 0 {
			t.Fatal("built early")
		}
		advance(t, w, job.CompletesAt)
		if v.Buildings[game.BWood] != 1 || len(v.BuildQueue) != 0 || v.Points != before+6 || v.Points != game.VillagePoints(v.Buildings) {
			t.Fatalf("after completion: wood %d queue %d points %d", v.Buildings[game.BWood], len(v.BuildQueue), v.Points)
		}
		woodAtCompletion := v.Resources.Wood
		advance(t, w, job.CompletesAt+game.MsPerHour)
		game.Touch(w, v)
		if math.Abs(v.Resources.Wood-(woodAtCompletion+35)) > 1e-5 {
			t.Fatalf("wood %v, want %v", v.Resources.Wood, woodAtCompletion+35)
		}
	})

	t.Run("queued jobs run one after another and unlock requirements", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		v.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
		mustBuild(t, w, v, game.BMain)
		mustBuild(t, w, v, game.BMain)
		advance(t, w, 10*game.MsPerHour)
		if v.Buildings[game.BMain] != 3 {
			t.Fatalf("main %d", v.Buildings[game.BMain])
		}
		if slices.Contains(blockerKinds(game.CheckBuild(w, v, game.BBarracks).Blockers), "requirements") {
			t.Fatal("barracks still blocked")
		}
	})

	t.Run("cancelling: 100 % for a waiting job, 90 % for a started one, later jobs' levels drop", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		v.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
		a := mustBuild(t, w, v, game.BWood)
		b := mustBuild(t, w, v, game.BWood)
		costA, costB := game.BuildCost(game.BWood, 1), game.BuildCost(game.BWood, 2)
		afterQueue := v.Resources
		refundB, err := game.CancelBuild(w, "h1", v.ID, b.ID)
		if err != nil || refundB != costB || v.Resources.Wood != afterQueue.Wood+costB.Wood {
			t.Fatalf("refund %+v err %v wood %v", refundB, err, v.Resources.Wood)
		}
		c := mustBuild(t, w, v, game.BWood)
		if c.TargetLevel != 2 {
			t.Fatalf("c level %d", c.TargetLevel)
		}
		advance(t, w, 1000)
		refundA, err := game.CancelBuild(w, "h1", v.ID, a.ID)
		if err != nil || refundA.Wood != math.Floor(costA.Wood*0.9) {
			t.Fatalf("refund %+v err %v", refundA, err)
		}
		if len(v.BuildQueue) != 1 || v.BuildQueue[0].TargetLevel != 1 || v.BuildQueue[0].StartedAt != 1000 {
			t.Fatalf("queue %+v", v.BuildQueue)
		}
	})

	t.Run("demolition: half the time, frees population, town hall min. 1", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		v.Buildings[game.BWood] = 5
		popBefore := game.VillagePopulationUsed(v)
		job, err := game.EnqueueDemolish(w, "h1", v.ID, game.BWood)
		if err != nil {
			t.Fatal(err)
		}
		if job.TargetLevel != 4 || job.CompletesAt-job.StartedAt != int64(math.Round(float64(game.BuildTimeMs(game.BWood, 5, 1, 1))/2)) {
			t.Fatalf("job %+v", job)
		}
		advance(t, w, job.CompletesAt)
		if v.Buildings[game.BWood] != 4 || game.VillagePopulationUsed(v) >= popBefore {
			t.Fatal("demolish not applied")
		}
		if _, err := game.EnqueueDemolish(w, "h1", v.ID, game.BMain); err == nil {
			t.Fatal("main demolished to 0")
		}
	})

	t.Run("offline: one jump vs 1000 steps with a build queue gives identical state", func(t *testing.T) {
		build := func() (*gametest.MemWorld, *game.Village) {
			w, v := gametest.NewTestWorld(12345)
			v.Resources = game.Resources{Wood: 900, Clay: 900, Iron: 900}
			mustBuild(t, w, v, game.BWood)
			mustBuild(t, w, v, game.BClay)
			return w, v
		}
		wa, va := build()
		wb, vb := build()
		total := 2 * game.MsPerDay
		advance(t, wa, total)
		for i := int64(1); i <= 1000; i++ {
			advance(t, wb, total*i/1000)
			game.Touch(wb, vb)
		}
		game.Touch(wa, va)
		if va.Buildings != vb.Buildings || va.Points != vb.Points {
			t.Fatal("buildings differ")
		}
		if math.Abs(va.Resources.Wood-vb.Resources.Wood) > 1e-4 || math.Abs(va.Resources.Clay-vb.Resources.Clay) > 1e-4 ||
			math.Abs(va.Resources.Iron-vb.Resources.Iron) > 1e-4 {
			t.Fatalf("jump %+v steps %+v", va.Resources, vb.Resources)
		}
	})

	t.Run("build time uses the town hall level after queued town hall upgrades", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12345)
		v.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
		m := mustBuild(t, w, v, game.BMain)
		wj := mustBuild(t, w, v, game.BWood)
		if wj.CompletesAt-wj.StartedAt != game.BuildTimeMs(game.BWood, 1, 2, 1) || m.CompletesAt-m.StartedAt != game.BuildTimeMs(game.BMain, 2, 1, 1) {
			t.Fatalf("m %+v w %+v", m, wj)
		}
	})

	t.Run("changing world speed mid-queue: the building completes exactly once", func(t *testing.T) {
		w, v := gametest.NewTestWorld(1)
		v.Resources = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
		before := v.Buildings
		mustBuild(t, w, v, game.BWood)
		mustBuild(t, w, v, game.BClay)
		advance(t, w, w.Now()+10*game.MsPerMinute)
		w.Config().WorldSpeed = 4
		game.RescheduleBuildQueue(v, w.Now(), w.Config())
		game.ScheduleBuildHead(w, v)
		advance(t, w, w.Now()+2*game.MsPerDay)
		if v.Buildings[game.BWood] != before[game.BWood]+1 || v.Buildings[game.BClay] != before[game.BClay]+1 || len(v.BuildQueue) != 0 {
			t.Fatalf("wood %d clay %d queue %d", v.Buildings[game.BWood], v.Buildings[game.BClay], len(v.BuildQueue))
		}
	})

	t.Run("a ×2 build multiplier halves build time only on its own side", func(t *testing.T) {
		w, human := gametest.NewTestWorld(5)
		w.Config().HumanModifiers.Building = 2
		gametest.NewBotPlayer(w, "bot_test")
		bot := gametest.AddStartVillage(w, "bot_test", 102, 100)
		bot.Buildings = human.Buildings
		for _, v := range []*game.Village{human, bot} {
			v.Resources = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
		}
		jh := mustBuild(t, w, human, game.BWood)
		jb := mustBuild(t, w, bot, game.BWood)
		if jh.CompletesAt-jh.StartedAt != int64(math.Round(float64(jb.CompletesAt-jb.StartedAt)/2)) {
			t.Fatalf("human %d bot %d", jh.CompletesAt-jh.StartedAt, jb.CompletesAt-jb.StartedAt)
		}
	})
}

func TestEnqueueBuildRejectsForeignVillage(t *testing.T) {
	w, v := gametest.NewTestWorld(12345)
	gametest.NewHumanPlayer(w, "h2")
	_, err := game.EnqueueBuild(w, "h2", v.ID, game.BWood)
	var be *game.BlockedError
	if !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("err %v", err)
	}
	if len(v.BuildQueue) != 0 || v.Resources.Wood != 500 {
		t.Fatalf("queue %d wood %v", len(v.BuildQueue), v.Resources.Wood)
	}
	for _, op := range []func() error{
		func() error { _, err := game.EnqueueDemolish(w, "h2", v.ID, game.BWood); return err },
		func() error { _, err := game.CancelBuild(w, "h2", v.ID, "b_1"); return err },
	} {
		if err := op(); !errors.As(err, &be) || !be.HasKind("notOwner") {
			t.Fatalf("err %v", err)
		}
	}
}
