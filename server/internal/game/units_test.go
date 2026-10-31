package game_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func armedVillage(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w, v := gametest.NewTestWorld(12345)
	v.Buildings[game.BMain], v.Buildings[game.BBarracks], v.Buildings[game.BSmithy], v.Buildings[game.BStable] = 10, 5, 5, 3
	v.Buildings[game.BFarm], v.Buildings[game.BStorage], v.Buildings[game.BStatue] = 10, 15, 1
	v.Resources = game.Resources{Wood: 20000, Clay: 20000, Iron: 20000}
	return w, v
}

func speedCfg(s float64) *game.WorldConfig {
	cfg := game.DefaultWorldConfig
	cfg.WorldSpeed = s
	return &cfg
}

func mustRecruit(t *testing.T, w game.World, v *game.Village, u game.UnitID, n int) *game.RecruitJob {
	t.Helper()
	job, err := game.EnqueueRecruit(w, game.PlayerID(v.OwnerID), v.ID, u, n)
	if err != nil {
		t.Fatalf("EnqueueRecruit %s×%d: %v", u, n, err)
	}
	return job
}

// advanceTouch advances one village: play events, then settle the village to the new now.
func advanceTouch(t *testing.T, w *gametest.MemWorld, v *game.Village, until int64) {
	t.Helper()
	advance(t, w, until)
	game.Touch(w, v)
}

func TestRecruitTime(t *testing.T) {
	t.Run("formula and minimum", func(t *testing.T) {
		if got := game.RecruitTimeMs(game.USpear, 1, speedCfg(1), nil); got != 1_020_000 {
			t.Fatalf("spear 1: %d", got)
		}
		ratio := float64(game.RecruitTimeMs(game.USpear, 25, speedCfg(1), nil)) / float64(game.RecruitTimeMs(game.USpear, 1, speedCfg(1), nil))
		if math.Abs(ratio-math.Pow(0.9666, 24)) > 1e-3 || ratio <= 0.43 || ratio >= 0.45 {
			t.Fatalf("ratio %v", ratio)
		}
		if game.RecruitTimeMs(game.USpear, 1, speedCfg(2), nil) != 510_000 || game.RecruitTimeMs(game.USpear, 25, speedCfg(100), nil) < 1000 {
			t.Fatal("speed / minimum")
		}
	})

	t.Run("the village bonus shortens time in the matching building", func(t *testing.T) {
		_, v := armedVillage(t)
		v.Bonus = &game.VillageBonus{Kind: "infantry"}
		if game.RecruitTimeMs(game.USpear, 1, speedCfg(1), v) != 816_000 || game.RecruitTimeMs(game.UScout, 1, speedCfg(1), v) != 900_000 {
			t.Fatal("bonus")
		}
	})

	t.Run("a ×2 recruit multiplier halves recruitment only on its own side", func(t *testing.T) {
		w, human := gametest.NewTestWorld(5)
		w.Config().HumanModifiers.Recruitment = 2
		gametest.NewBotPlayer(w, "bot_test")
		bot := gametest.AddStartVillage(w, "bot_test", 102, 100)
		if game.RecruitTimeMs(game.USpear, 1, w.Config(), human) != int64(math.Round(float64(game.RecruitTimeMs(game.USpear, 1, w.Config(), bot))/2)) {
			t.Fatal("human side not halved")
		}
	})
}

func TestRecruit(t *testing.T) {
	t.Run("an order takes resources, reserves population, units arrive one by one", func(t *testing.T) {
		w, v := armedVillage(t)
		popBefore := game.VillagePopulationUsed(v)
		job := mustRecruit(t, w, v, game.USpear, 10)
		if v.Resources != (game.Resources{Wood: 19500, Clay: 19700, Iron: 19900}) || game.VillagePopulationUsed(v) != popBefore+10 {
			t.Fatalf("resources %+v pop %d", v.Resources, game.VillagePopulationUsed(v))
		}
		per := job.PerUnitMs
		advanceTouch(t, w, v, per-1)
		if v.Units[game.USpear] != 0 {
			t.Fatal("early unit")
		}
		advanceTouch(t, w, v, per)
		if v.Units[game.USpear] != 1 {
			t.Fatal("first unit")
		}
		advanceTouch(t, w, v, per*7/2)
		if v.Units[game.USpear] != 3 || v.RecruitQueues.Barracks[0].Done != 3 {
			t.Fatalf("units %d", v.Units[game.USpear])
		}
		advanceTouch(t, w, v, per*10)
		if v.Units[game.USpear] != 10 || len(v.RecruitQueues.Barracks) != 0 || game.VillagePopulationUsed(v) != popBefore+10 {
			t.Fatalf("units %d queue %d", v.Units[game.USpear], len(v.RecruitQueues.Barracks))
		}
	})

	t.Run("building queues run in parallel, sequentially within a building", func(t *testing.T) {
		w, v := armedVillage(t)
		v.Research[game.ULightCav] = 1
		v.Research[game.USword] = 1
		a := mustRecruit(t, w, v, game.USpear, 2)
		b := mustRecruit(t, w, v, game.USword, 2)
		c := mustRecruit(t, w, v, game.ULightCav, 2)
		if b.StartedAt != a.CompletesAt || c.StartedAt != 0 {
			t.Fatalf("b %+v c %+v", b, c)
		}
		advanceTouch(t, w, v, a.CompletesAt)
		if v.Units[game.USpear] != 2 || v.Units[game.USword] != 0 {
			t.Fatal("after a")
		}
		advanceTouch(t, w, v, b.CompletesAt)
		if v.Units[game.USword] != 2 {
			t.Fatal("after b")
		}
		advanceTouch(t, w, v, 10*game.MsPerHour)
		if v.Units[game.ULightCav] != 2 {
			t.Fatal("after c")
		}
	})

	t.Run("blockers: requirements, research, resources, population, knight limit", func(t *testing.T) {
		w, v := armedVillage(t)
		kinds := func(u game.UnitID, n int) []string { return blockerKinds(game.CheckRecruit(w, v, u, n).Blockers) }
		if !slices.Contains(kinds(game.UHeavyCav, 1), "requirements") || !slices.Contains(kinds(game.USword, 1), "research") {
			t.Fatal("requirements / research")
		}
		if len(kinds(game.USpear, 1)) != 0 || !slices.Contains(kinds(game.USpear, 1000), "resources") {
			t.Fatal("spear")
		}
		v.Resources = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
		if !slices.Contains(kinds(game.USpear, 5000), "population") || len(kinds(game.UPaladin, 1)) != 0 {
			t.Fatal("population / paladin")
		}
		mustRecruit(t, w, v, game.UPaladin, 1)
		if !slices.Contains(kinds(game.UPaladin, 1), "limit") || game.CheckRecruit(w, v, game.UPaladin, 1).MaxAffordable != 0 {
			t.Fatal("paladin limit")
		}
	})

	t.Run("a paladin away from the village still counts toward its limit", func(t *testing.T) {
		w, v := armedVillage(t)
		v.Units[game.UPaladin] = 1
		x, y := nearestUnstoredBarbarian(t, w, v, 1)
		mustSend(t, w, v, x, y, game.UnitCounts{game.UPaladin: 1}, "attack")
		check := game.CheckRecruit(w, v, game.UPaladin, 1)
		if !slices.Contains(blockerKinds(check.Blockers), "limit") || check.MaxAffordable != 0 {
			t.Fatalf("blockers %v max %d", blockerKinds(check.Blockers), check.MaxAffordable)
		}
	})

	t.Run("cancelling refunds 100 % for units not yet produced", func(t *testing.T) {
		w, v := armedVillage(t)
		job := mustRecruit(t, w, v, game.USpear, 10)
		advance(t, w, job.PerUnitMs*4)
		refund, err := game.CancelRecruit(w, "h1", v.ID, job.ID)
		if err != nil || refund != (game.Resources{Wood: 300, Clay: 180, Iron: 60}) {
			t.Fatalf("refund %+v err %v", refund, err)
		}
		if v.Units[game.USpear] != 4 || len(v.RecruitQueues.Barracks) != 0 {
			t.Fatalf("units %d queue %d", v.Units[game.USpear], len(v.RecruitQueues.Barracks))
		}
	})

	t.Run("offline: one jump vs 1000 steps with recruit queues", func(t *testing.T) {
		setup := func() (*gametest.MemWorld, *game.Village) {
			w, v := armedVillage(t)
			v.Research[game.UAxe] = 1
			v.Research[game.UScout] = 1
			mustRecruit(t, w, v, game.USpear, 30)
			mustRecruit(t, w, v, game.UAxe, 20)
			mustRecruit(t, w, v, game.UScout, 5)
			return w, v
		}
		wa, va := setup()
		wb, vb := setup()
		total := 2 * game.MsPerDay
		advanceTouch(t, wa, va, total)
		for i := int64(1); i <= 1000; i++ {
			advanceTouch(t, wb, vb, total*i/1000)
		}
		ja, _ := json.Marshal(va.RecruitQueues)
		jb, _ := json.Marshal(vb.RecruitQueues)
		if va.Units != vb.Units || string(ja) != string(jb) || va.Units[game.USpear] != 30 || va.Units[game.UAxe] != 20 {
			t.Fatalf("jump %v steps %v", va.Units, vb.Units)
		}
	})

	t.Run("settle does not duplicate units when called repeatedly", func(t *testing.T) {
		w, v := armedVillage(t)
		job := mustRecruit(t, w, v, game.USpear, 3)
		game.SettleRecruitQueues(v, job.PerUnitMs*2)
		game.SettleRecruitQueues(v, job.PerUnitMs*2)
		game.SettleRecruitQueues(v, job.PerUnitMs)
		if v.Units[game.USpear] != 2 {
			t.Fatalf("units %d", v.Units[game.USpear])
		}
	})
}

func TestResearch(t *testing.T) {
	t.Run("cost and time by level", func(t *testing.T) {
		if game.ResearchCost(game.UAxe, 1) != (game.Resources{Wood: 600, Clay: 300, Iron: 400}) ||
			game.ResearchCost(game.UAxe, 2) != (game.Resources{Wood: 1800, Clay: 900, Iron: 1200}) ||
			game.ResearchCost(game.UAxe, 3) != (game.Resources{Wood: 5400, Clay: 2700, Iron: 3600}) {
			t.Fatal("cost")
		}
		if game.ResearchTimeMs(game.UAxe, 1, 1) != 1300*300 || game.ResearchTimeMs(game.UAxe, 1, 2) != 1300*150 {
			t.Fatal("time")
		}
	})

	t.Run("spearman unlocked without a smithy, axeman needs research", func(t *testing.T) {
		_, v := gametest.NewTestWorld(12345)
		if game.EffectiveResearchLevel(v, game.USpear) != 1 || game.EffectiveResearchLevel(v, game.UAxe) != 0 {
			t.Fatal("levels")
		}
	})

	t.Run("one research at a time, completion via advance, max 3", func(t *testing.T) {
		w, v := armedVillage(t)
		mustResearch := func(u game.UnitID) *game.ResearchJob {
			j, err := game.EnqueueResearch(w, "h1", v.ID, u)
			if err != nil {
				t.Fatal(err)
			}
			return j
		}
		j := mustResearch(game.UAxe)
		if j.TargetLevel != 1 || !slices.Contains(blockerKinds(game.CheckResearch(w, v, game.USword).Blockers), "busy") {
			t.Fatal("first")
		}
		advance(t, w, j.CompletesAt)
		if v.Research[game.UAxe] != 1 || len(v.ResearchQueue) != 0 {
			t.Fatal("completion")
		}
		j2 := mustResearch(game.UAxe)
		if j2.TargetLevel != 2 {
			t.Fatal("second level")
		}
		advance(t, w, j2.CompletesAt)
		j3 := mustResearch(game.UAxe)
		advance(t, w, j3.CompletesAt)
		if v.Research[game.UAxe] != 3 || !slices.Contains(blockerKinds(game.CheckResearch(w, v, game.UAxe).Blockers), "maxLevel") {
			t.Fatal("max")
		}
		if w.Human("h1").Stats.ResearchCompleted != 3 {
			t.Fatalf("stats %d", w.Human("h1").Stats.ResearchCompleted)
		}
	})

	t.Run("cancelling research refunds the cost", func(t *testing.T) {
		w, v := armedVillage(t)
		before := v.Resources
		j, err := game.EnqueueResearch(w, "h1", v.ID, game.UAxe)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := game.CancelResearch(w, "h1", v.ID, j.ID); err != nil || v.Resources != before {
			t.Fatalf("err %v resources %+v", err, v.Resources)
		}
	})
}

func TestRecruitRejectsForeignVillage(t *testing.T) {
	w, v := armedVillage(t)
	gametest.NewHumanPlayer(w, "h2")
	var be *game.BlockedError
	for name, op := range map[string]func() error{
		"recruit":         func() error { _, err := game.EnqueueRecruit(w, "h2", v.ID, game.USpear, 1); return err },
		"cancel-recruit":  func() error { _, err := game.CancelRecruit(w, "h2", v.ID, "q_1"); return err },
		"research":        func() error { _, err := game.EnqueueResearch(w, "h2", v.ID, game.UAxe); return err },
		"cancel-research": func() error { _, err := game.CancelResearch(w, "h2", v.ID, "r_1"); return err },
	} {
		if err := op(); !errors.As(err, &be) || !be.HasKind("notOwner") {
			t.Errorf("%s: err %v", name, err)
		}
	}
	if len(v.RecruitQueues.Barracks) != 0 || len(v.ResearchQueue) != 0 || v.Resources.Wood != 20000 {
		t.Fatal("foreign order changed the village")
	}
}

func TestRecruitResearchGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/units.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Recruit []struct {
			Speed float64     `json:"speed"`
			Bonus *string     `json:"bonus"`
			Unit  game.UnitID `json:"unit"`
			Level int         `json:"level"`
			Ms    int64       `json:"ms"`
		} `json:"recruit"`
		Research []struct {
			Unit  game.UnitID    `json:"unit"`
			Level int            `json:"level"`
			Cost  game.Resources `json:"cost"`
			Ms1   int64          `json:"ms1"`
			Ms8   int64          `json:"ms8"`
		} `json:"research"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Recruit) == 0 || len(g.Research) == 0 {
		t.Fatal("empty fixture")
	}
	for i, c := range g.Recruit {
		v := game.CreateVillage(0, 0, "", "", 0)
		if c.Bonus != nil {
			v.Bonus = &game.VillageBonus{Kind: game.VillageBonusKind(*c.Bonus)}
		}
		g.Recruit[i].Ms = game.RecruitTimeMs(c.Unit, c.Level, speedCfg(c.Speed), v)
	}
	for i, c := range g.Research {
		p := &g.Research[i]
		p.Cost = game.ResearchCost(c.Unit, c.Level)
		p.Ms1, p.Ms8 = game.ResearchTimeMs(c.Unit, c.Level, 1), game.ResearchTimeMs(c.Unit, c.Level, 8)
	}
	game.Golden(t, "units", g)
}
