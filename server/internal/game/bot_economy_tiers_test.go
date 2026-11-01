package game_test

import (
	"encoding/json"
	"slices"
	"sort"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

const (
	strongSkill = 0.95
	mediumSkill = 0.55
	weakSkill   = 0.2
)

var economyBuildings = []game.BuildingID{game.BWood, game.BClay, game.BIron, game.BStorage, game.BFarm, game.BMain}

// openingPicks are the bot's next n buildings on an empty purse, each one built at once.
func openingPicks(w *gametest.MemWorld, v *game.Village, n int) []game.BuildingID {
	var out []game.BuildingID
	for i := range n {
		b, ok := game.NextBuilding(w, "bot_0", v, game.Mulberry32(uint32(i)))
		if !ok {
			break
		}
		out = append(out, b)
		v.Buildings[b]++
	}
	return out
}

// afterOpening gives v levels past the mines opening (not the classic one: smithy 1, no stable), mines and main set.
func afterOpening(v *game.Village, wood, clay, iron, main int) {
	b := &v.Buildings
	b[game.BWood], b[game.BClay], b[game.BIron], b[game.BMain] = wood, clay, iron, main
	b[game.BStorage], b[game.BFarm], b[game.BBarracks], b[game.BSmithy], b[game.BStable] = 20, 15, 3, 1, 0
	b[game.BStatue], b[game.BMarket], b[game.BWall], b[game.BHide] = 1, 1, 5, 3
	v.Resources = game.Resources{}
}

func distinctPicks(w *gametest.MemWorld, v *game.Village, tries int) map[game.BuildingID]bool {
	out := map[game.BuildingID]bool{}
	for i := range tries {
		if b, ok := game.NextBuilding(w, "bot_0", v, game.Mulberry32(uint32(i))); ok {
			out[b] = true
		}
	}
	return out
}

func TestBotEconomyTiers(t *testing.T) {
	t.Run("strong and medium bots take the mines to 3–4 before main and barracks", func(t *testing.T) {
		for _, skill := range []float64{strongSkill, mediumSkill} {
			w, v := oneBotWorld(t, 31, skill)
			for i := range 30 {
				b, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(uint32(i)))
				l := &v.Buildings
				if (b == game.BMain || b == game.BBarracks) && min(l[game.BWood], l[game.BClay], l[game.BIron]) < 3 {
					t.Fatalf("skill %.2f: %s at mines %d/%d/%d", skill, b, l[game.BWood], l[game.BClay], l[game.BIron])
				}
				l[b]++
			}
			if l := v.Buildings; l[game.BWood] < 6 || l[game.BMain] > 3 {
				t.Fatalf("skill %.2f: after 30 builds wood %d, main %d", skill, l[game.BWood], l[game.BMain])
			}
		}
	})

	t.Run("weak bots keep the classic opening", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, weakSkill)
		if picks := openingPicks(w, v, 8); !slices.Contains(picks, game.BMain) {
			t.Fatalf("no main in %v", picks)
		}
	})

	t.Run("strong bots in a short world keep the classic opening", func(t *testing.T) {
		failOnBotErrors(t)
		cfg := game.InstantWorldConfig
		cfg.BotCount = 1
		w, _ := gametest.NewWorldWithHuman(31, cfg)
		w.Brain("bot_0").Skill = strongSkill
		v := game.PlayerVillages(w, "bot_0")[0]
		v.Buildings, v.Units, v.Resources = game.Buildings{}, game.Units{}, game.Resources{}
		v.Buildings[game.BMain], v.Buildings[game.BFarm], v.Buildings[game.BStorage], v.Buildings[game.BHide] = 1, 1, 1, 1
		if picks := openingPicks(w, v, 8); !slices.Contains(picks, game.BMain) {
			t.Fatalf("no main in %v", picks)
		}
	})

	t.Run("after the opening an economy-style strong bot builds the economy until its mines reach 18, iron 3 behind", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		w.Brain("bot_0").Style = game.StylePointBuilder
		afterOpening(v, 12, 12, 10, 10)
		for b := range distinctPicks(w, v, 40) {
			if !slices.Contains(economyBuildings, b) || b == game.BIron {
				t.Fatalf("picked %s", b)
			}
		}
	})

	t.Run("a medium bot builds the economy to 15, then the military too", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, mediumSkill)
		afterOpening(v, 11, 11, 8, 10)
		for b := range distinctPicks(w, v, 40) {
			if !slices.Contains(economyBuildings, b) {
				t.Fatalf("mines 11: picked %s", b)
			}
		}
		afterOpening(v, 16, 16, 13, 12)
		military := false
		for b := range distinctPicks(w, v, 40) {
			military = military || !slices.Contains(economyBuildings, b)
		}
		if !military {
			t.Fatal("mines 16: economy only")
		}
	})

	t.Run("a strong bot expands its farm when a quarter of it is left, and not much sooner", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		afterOpening(v, 12, 12, 9, 10)
		fill := func(share float64) {
			v.Units = game.Units{}
			v.Units[game.USpear] = int(share*float64(game.VillageFarmCapacity(v))) - game.PopulationUsed(w, v)
		}
		fill(0.78)
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BFarm {
			t.Fatalf("22 %% free: next %s", next)
		}
		fill(0.5)
		if picks := distinctPicks(w, v, 40); picks[game.BFarm] {
			t.Fatalf("half free: picked the farm, %v", picks)
		}
	})

	t.Run("a strong bot grows its storage when it holds less than 12 hours of production, and not beyond", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		hours := func() float64 {
			top := 0.0
			for _, r := range game.ResourceIDs {
				top = max(top, game.VillageProductionPerHour(v, r, w.Config()))
			}
			return game.VillageStorageCapacity(v) / top
		}
		afterOpening(v, 16, 16, 13, 12)
		v.Buildings[game.BStorage] = 17
		if h := hours(); h >= 12 || h < 11 {
			t.Fatalf("setup: storage 17 holds %.1f h", h)
		}
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BStorage {
			t.Fatalf("storage for %.1f h: next %s", hours(), next)
		}
		v.Buildings[game.BStorage] = 18
		if picks := distinctPicks(w, v, 40); picks[game.BStorage] {
			t.Fatalf("storage for %.1f h: picked it, %v", hours(), picks)
		}
	})

	t.Run("a bot that loots counts its loot in the storage it needs", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		afterOpening(v, 16, 16, 13, 12)
		v.Buildings[game.BStorage] = 18
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next == game.BStorage {
			t.Fatal("setup: storage 18 is short without loot")
		}
		hours := float64(w.Now()-w.Player("bot_0").JoinAt)/float64(game.MsPerHour) + 1
		perHour := game.VillageProductionPerHour(v, game.Wood, w.Config())
		w.Brain("bot_0").Looted = game.Resources{Wood: perHour * hours, Clay: perHour * hours, Iron: perHour * hours}
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BStorage {
			t.Fatalf("looting as much as it produces: next %s", next)
		}
	})

	t.Run("the storage counts the recent loot, not the average since joining", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		afterOpening(v, 16, 16, 13, 12)
		v.Buildings[game.BStorage] = 18
		w.Player("bot_0").JoinAt = w.Now() - 1000*game.MsPerHour
		perHour := game.VillageProductionPerHour(v, game.Wood, w.Config())
		w.Brain("bot_0").Looted = game.Resources{Wood: perHour}
		w.Brain("bot_0").RecentLoot = game.LootWindow{Since: w.Now(), PerHour: 3 * perHour}
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BStorage {
			t.Fatalf("looting as much as it produces lately: next %s", next)
		}
	})

	t.Run("a raid coming home rolls the recent loot rate every 12 hours", func(t *testing.T) {
		var lw game.LootWindow
		lw.Add(0, 600)
		lw.Add(6*game.MsPerHour, 600)
		if lw.PerHour != 0 {
			t.Fatalf("rate %v before the window closes", lw.PerHour)
		}
		lw.Add(12*game.MsPerHour, 600)
		if lw.PerHour != 100 || lw.Loot != 600 {
			t.Fatalf("window %+v", lw)
		}
	})

	t.Run("a second farm level goes in when the first one queued still leaves less than a quarter", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		afterOpening(v, 12, 12, 9, 10)
		v.Units[game.USpear] = int(0.98*float64(game.VillageFarmCapacity(v))) - game.PopulationUsed(w, v)
		v.Resources = game.BuildCost(game.BFarm, v.Buildings[game.BFarm]+1)
		if _, err := game.EnqueueBuild(w, "bot_0", v.ID, game.BFarm); err != nil {
			t.Fatal(err)
		}
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BFarm {
			t.Fatalf("next %s", next)
		}
	})

	t.Run("a strong bot not farmed keeps its hiding place at 3 and its wall at 5", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		afterOpening(v, 20, 20, 20, 20)
		v.Buildings[game.BSmithy], v.Buildings[game.BStable], v.Buildings[game.BBarracks] = 5, 3, 5
		picks := distinctPicks(w, v, 60)
		if picks[game.BHide] || picks[game.BWall] {
			t.Fatalf("picked %v", picks)
		}
	})

	t.Run("an unaffordable farm does not leave a strong bot's queue empty", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, strongSkill)
		b := &v.Buildings
		b[game.BFarm], b[game.BStorage], b[game.BWood], b[game.BClay], b[game.BIron] = 8, 8, 3, 6, 6
		v.Units[game.USpear] += int(0.9*float64(game.VillageFarmCapacity(v))) - game.PopulationUsed(w, v)
		v.Resources = game.BuildCost(game.BWood, 4)
		if next, _ := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); next != game.BFarm {
			t.Fatalf("setup: next %s, want the farm", next)
		}
		if len(game.CheckBuild(w, v, game.BFarm).Blockers) == 0 || len(game.CheckBuild(w, v, game.BWood).Blockers) > 0 {
			t.Fatal("setup: want the farm unaffordable and the wood affordable")
		}
		game.PlanBuilding(w, "bot_0", v)
		if len(v.BuildQueue) == 0 {
			t.Fatal("queue empty")
		}
	})

	t.Run("a strong bot leaves a build plan of economy steps for its break, a weak one does not", func(t *testing.T) {
		for _, skill := range []float64{strongSkill, weakSkill} {
			w, v := oneBotWorld(t, 31, skill)
			w.Brain("bot_0").Style = game.StylePointBuilder
			afterOpening(v, 12, 12, 9, 8)
			game.QueueForOffline(w, "bot_0", v, w.Now()+8*game.MsPerHour)
			if skill == weakSkill {
				if len(v.BuildPlan) > 0 {
					t.Fatalf("weak bot plan %v", v.BuildPlan)
				}
				continue
			}
			if len(v.BuildPlan) < 10 {
				t.Fatalf("plan %v", v.BuildPlan)
			}
			for _, b := range v.BuildPlan[:6] {
				if !slices.Contains(economyBuildings, b) {
					t.Fatalf("plan %v: step %s before the mines reach 14", v.BuildPlan, b)
				}
			}
		}
	})

	t.Run("start check-ins last 4 days for strong bots, 2 for medium ones", func(t *testing.T) {
		count := func(skill float64, day int64) int {
			w, _ := oneBotWorld(t, 31, skill)
			return len(game.DaySessions(w, "bot_0", day))
		}
		if n := count(strongSkill, 3); n < 6 {
			t.Fatalf("strong bot, day 4: %d sessions", n)
		}
		if n := count(mediumSkill, 1); n < 5 {
			t.Fatalf("medium bot, day 2: %d sessions", n)
		}
		if n := count(mediumSkill, 3); n > 5 {
			t.Fatalf("medium bot, day 4: %d sessions", n)
		}
	})
}

const longWorldConfig = `{"mapSize": 200, "botCount": 100, "botStance": "normal", "nobleCost": {"clay": 0, "iron": 0, "wood": 0},
 "unitSpeed": 1, "botArrival": "allAtOnce", "difficulty": "normal", "rampUpDays": 14, "worldSpeed": 4,
 "lastVillage": "canBeLost", "sightRadius": 1, "battleLosses": "normal",
 "botModifiers": {"building": 1, "resources": 1, "recruitment": 1},
 "humanModifiers": {"building": 1, "resources": 1, "recruitment": 1}, "barbarianDensity": "normal",
 "beginnerProtectionDays": 5}`

// On a long world the classic play left strong bots at about 240 points 66 hours in, with mines at 5; the tier plays
// must get them well past that.
func TestStrongBotsGrowFastOnALongWorld(t *testing.T) {
	cfg := game.DefaultWorldConfig
	if err := json.Unmarshal([]byte(longWorldConfig), &cfg); err != nil {
		t.Fatal(err)
	}
	w := gametest.NewMemWorld(3, cfg, 1793209655024)
	raids := map[game.PlayerID]int{}
	defer game.SpyRaids(func(id game.PlayerID, _ *game.Command, _ bool, _ int64) { raids[id]++ })()
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, 66*game.MsPerHour); err != nil {
		t.Fatal(err)
	}
	points := w.PointsByOwner()
	tiers := map[string][]float64{}
	mines := map[string][]float64{}
	tierRaids := map[string][]float64{}
	for _, p := range w.Players() {
		b := w.Brain(p.ID)
		if b == nil || p.Status != game.StatusActive {
			continue
		}
		tier := string(game.TierOf(b.Skill))
		v := w.Village(p.VillageIDs[0])
		tiers[tier] = append(tiers[tier], float64(points[p.ID]))
		mines[tier] = append(mines[tier], float64(v.Buildings[game.BWood]+v.Buildings[game.BClay]+v.Buildings[game.BIron])/3)
		tierRaids[tier] = append(tierRaids[tier], float64(raids[p.ID]))
	}
	strong := tiers[string(game.TierStrong)]
	sort.Sort(sort.Reverse(sort.Float64Slice(strong)))
	if len(strong) < 5 || (strong[0]+strong[1]+strong[2]+strong[3]+strong[4])/5 < 350 {
		t.Errorf("strong bots' top 5: %v", strong[:min(5, len(strong))])
	}
	if m := medianOf(mines[string(game.TierStrong)]); m < 11 {
		t.Errorf("strong bots' median mines %.1f", m)
	}
	if m := medianOf(mines[string(game.TierMedium)]); m < 8 {
		t.Errorf("medium bots' median mines %.1f", m)
	}
	if r := medianOf(tierRaids[string(game.TierMedium)]); r < 5 {
		t.Errorf("medium bots' median raids %.0f", r)
	}
}

// Falling behind the neighbours is no reason to quit: weak bots keep playing next to strong ones.
func TestWeakBotFarBehindKeepsPlaying(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 3, 0
	w, _ := gametest.NewWorldWithHuman(22, cfg)
	b := w.Brain("bot_0")
	b.Skill, b.Resilience = weakSkill, 0.2
	for _, id := range []game.PlayerID{"bot_1", "bot_2"} {
		v := game.PlayerVillages(w, id)[0]
		afterOpening(v, 25, 25, 25, 20)
		game.RecomputePoints(v)
	}
	for range 30 {
		game.UpdateOverwhelm(w, "bot_0")
	}
	if p := w.Player("bot_0"); p.Status != game.StatusActive {
		t.Fatalf("status %s, overwhelm %.2f", p.Status, b.Overwhelm)
	}
}

// Weak bots always play, however hard they are hit; stronger ones may still give up.
func TestWeakBotsNeverQuit(t *testing.T) {
	for _, skill := range []float64{weakSkill, mediumSkill} {
		cfg := game.DefaultWorldConfig
		cfg.BotCount = 1
		w, _ := gametest.NewWorldWithHuman(22, cfg)
		b := w.Brain("bot_0")
		b.Skill, b.Resilience = skill, 0.2
		for range 10 {
			b.LastSessionStats = game.SessionStats{VillagesLost: 1, IncomingAttacks: 40}
			game.UpdateOverwhelm(w, "bot_0")
		}
		status := w.Player("bot_0").Status
		if skill == weakSkill && status != game.StatusActive {
			t.Fatalf("weak bot: %s", status)
		}
		if skill == mediumSkill && status != game.StatusQuit {
			t.Fatalf("medium bot: %s", status)
		}
	}
}
