package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// styledBot: bot_0 with the skill and style set, levels past the mines opening, a big storage and farm.
func styledBot(t *testing.T, skill float64, style game.BotStyle, wood, clay, iron int) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w, v := oneBotWorld(t, 31, skill)
	w.Brain("bot_0").Style = style
	b := &v.Buildings
	*b = game.Buildings{}
	b[game.BWood], b[game.BClay], b[game.BIron], b[game.BMain], b[game.BStorage], b[game.BFarm] = wood, clay, iron, 4, 20, 15
	b[game.BBarracks], b[game.BHide], b[game.BStatue] = 1, 3, 1
	v.Units, v.Resources = game.Units{}, game.Resources{}
	return w, v
}

func cavalryReady(v *game.Village) {
	b := &v.Buildings
	b[game.BMain], b[game.BBarracks], b[game.BSmithy], b[game.BStable] = 10, 5, 5, 3
}

func TestBotFarmingStyles(t *testing.T) {
	military := []game.BuildingID{game.BMain, game.BBarracks, game.BSmithy, game.BStable}

	t.Run("a farming-style strong bot goes for the stable once wood and clay reach 8, an economy-style one keeps to the mines", func(t *testing.T) {
		w, v := styledBot(t, strongSkill, game.StyleFarmer, 8, 8, 6)
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); !slices.Contains(military, next) {
			t.Fatalf("farmer: next %s", next)
		}
		w, v = styledBot(t, strongSkill, game.StylePointBuilder, 8, 8, 6)
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BWood && next != game.BClay && next != game.BIron {
			t.Fatalf("point builder: next %s", next)
		}
		for range 9 {
			b, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1))
			v.Buildings[b]++
		}
		if l := v.Buildings; l[game.BStable] > 0 {
			t.Fatalf("point builder built a stable at mines %d/%d", l[game.BWood], l[game.BClay])
		}
	})

	t.Run("with the stable up, a farming bot keeps light cavalry's research price and researches it", func(t *testing.T) {
		w, v := styledBot(t, strongSkill, game.StyleFarmer, 12, 12, 9)
		cavalryReady(v)
		v.Research[game.UScout] = 1 // scouts come first
		price := game.ResearchCost(game.ULightCav, 1)
		v.Resources = game.Resources{Wood: price.Wood + 400, Clay: price.Clay + 400, Iron: price.Iron + 400}
		game.PlanBuilding(w, "bot_0", v)
		if !game.HasResources(v.Resources, price) {
			t.Fatalf("building left %v of %v", v.Resources, price)
		}
		game.PlanResearch(w, "bot_0", v)
		if len(v.ResearchQueue) != 1 || v.ResearchQueue[0].Unit != game.ULightCav {
			t.Fatalf("research %+v", v.ResearchQueue)
		}
	})

	t.Run("a farming bot recruits from half its purse while its army is under half the farm, an economy one does not", func(t *testing.T) {
		for _, style := range []game.BotStyle{game.StyleFarmer, game.StylePointBuilder} {
			w, v := styledBot(t, strongSkill, style, 12, 12, 9)
			cavalryReady(v)
			v.Research[game.ULightCav] = 1
			if style == game.StylePointBuilder {
				v.Units[game.USpear] = int(0.2 * float64(game.VillageFarmCapacity(v)))
			}
			next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1))
			v.Resources = game.BuildCost(next, v.Buildings[next]+1)
			before := game.UnitCounts(v.Units).Total()
			game.PlanRecruitment(w, "bot_0", v)
			queued := 0
			for _, b := range game.RecruitBuildings {
				for _, j := range *v.RecruitQueues.Q(b) {
					queued += j.Total
				}
			}
			if style == game.StyleFarmer && queued == 0 {
				t.Fatal("farmer: nothing recruited")
			}
			if style == game.StylePointBuilder && queued > 0 {
				t.Fatalf("point builder with %d units recruited %d", before, queued)
			}
		}
	})

	t.Run("a strong bot raids on the auto-farm tick once it has looted 5000, a weak one never", func(t *testing.T) {
		raids := func(skill, looted float64) int {
			w, v := oneBotWorld(t, 31, skill)
			farmingVillage(v)
			game.DiscoverIfStale(w, "bot_0")
			b := w.Brain("bot_0")
			b.Online, b.Looted = false, game.Resources{Wood: looted / 3, Clay: looted / 3, Iron: looted / 3}
			return game.BotAutoFarmTick(w, "bot_0", v.ID)
		}
		if n := raids(strongSkill, 4000); n != 0 {
			t.Fatalf("4000 looted: %d raids", n)
		}
		if n := raids(strongSkill, 6000); n == 0 || n > game.FarmBalance.AutoFarmMaxPerTick {
			t.Fatalf("6000 looted: %d raids", n)
		}
		if n := raids(weakSkill, 6000); n != 0 {
			t.Fatalf("weak bot: %d raids", n)
		}
	})

	t.Run("the auto-farm raids barbarians and abandoned villages, never players", func(t *testing.T) {
		failOnBotErrors(t)
		cfg := game.DefaultWorldConfig
		cfg.BotCount, cfg.BeginnerProtectionDays = 30, 0
		w, _ := gametest.NewWorldWithHuman(5, cfg)
		var from, weak *game.Village
		for _, a := range w.Players() {
			for _, b := range w.Players() {
				if from == nil && a.ID != b.ID && w.Brain(a.ID) != nil && w.Brain(b.ID) != nil {
					va, vb := w.Village(a.VillageIDs[0]), w.Village(b.VillageIDs[0])
					if game.Distance(va.X, va.Y, vb.X, vb.Y) <= 8 {
						from, weak = va, vb
					}
				}
			}
		}
		if from == nil {
			t.Fatal("setup: no two bots within 8 tiles")
		}
		bot := game.PlayerID(from.OwnerID)
		w.Brain(bot).Skill, w.Brain(bot).Looted = strongSkill, game.Resources{Wood: 6000}
		farmingVillage(from)
		weak.Buildings = game.Buildings{}
		weak.Buildings[game.BMain], weak.Units = 1, game.Units{}
		game.RecomputePoints(weak)
		w.Brain(bot).Memory.Known[weak.ID] = &game.KnownVillage{ID: weak.ID, X: weak.X, Y: weak.Y, OwnerID: weak.OwnerID,
			Points: weak.Points, Wall: ptr(0), Units: &game.UnitCounts{}, Hauls: []float64{}}
		if !slices.ContainsFunc(game.FarmCandidates(w, bot, from, 0), func(k *game.KnownVillage) bool { return k.ID == weak.ID }) {
			t.Fatal("setup: the weak bot is no farm for session raids")
		}
		if game.BotAutoFarmTick(w, bot, from.ID) == 0 {
			t.Fatal("setup: no raids at all")
		}
		for _, c := range w.Commands() {
			if c.TargetID == weak.ID {
				t.Fatalf("auto-farm raided the weak bot %s", weak.OwnerID)
			}
		}
	})

	t.Run("a bot that unlocked the auto-farm keeps its tick going", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		farmingVillage(v)
		w.Brain("bot_0").Looted = game.Resources{Wood: 6000}
		game.Think(w, "bot_0", false)
		at, ok := w.ScheduledAt("botfarm:" + v.ID)
		if !ok {
			t.Fatal("no tick scheduled")
		}
		if err := gametest.Advance(w, at+1); err != nil {
			t.Fatal(err)
		}
		if next, ok := w.ScheduledAt("botfarm:" + v.ID); !ok || next <= at {
			t.Fatalf("next tick %d (was %d)", next, at)
		}
	})
}
