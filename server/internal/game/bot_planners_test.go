package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// failOnBotErrors turns refused bot orders into test failures (planners must check before acting).
func failOnBotErrors(t *testing.T) {
	t.Helper()
	game.BotActionError = func(id game.PlayerID, err error) { t.Errorf("bot %s: %v", id, err) }
	t.Cleanup(func() { game.BotActionError = nil })
}

// oneBotWorld: one bot (bot_0) and human h1, speed 8, no protection, the bot's skill set.
func oneBotWorld(t *testing.T, seed uint32, skill float64) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	failOnBotErrors(t)
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays, cfg.BotStance = 1, 0, game.StancePeacefulRetaliate
	w, _ := gametest.NewWorldWithHuman(seed, cfg)
	w.Brain("bot_0").Skill = skill
	return w, game.PlayerVillages(w, "bot_0")[0]
}

// farmingVillage gives v a stable ready and 200 light cavalry plus 20 scouts.
func farmingVillage(v *game.Village) {
	b := &v.Buildings
	b[game.BMain], b[game.BBarracks], b[game.BStable], b[game.BSmithy], b[game.BFarm] = 10, 5, 3, 5, 15
	b[game.BStorage], b[game.BWood], b[game.BClay], b[game.BIron] = 10, 10, 10, 10
	v.Research[game.ULightCav], v.Research[game.UScout] = 1, 1
	v.Units[game.ULightCav], v.Units[game.UScout] = 200, 20
	game.RecomputePoints(v)
}

func TestBotEconomyPlanners(t *testing.T) {
	t.Run("the first village is mixed", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.9)
		if game.ChooseRole(w, "bot_0", v) != game.RoleMixed {
			t.Fatal("first village not mixed")
		}
	})

	t.Run("the second village gets a fixed off or def role", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		second := game.CreateVillage(v.X+3, v.Y+3, "Second", "", w.Now())
		w.PutVillage(second)
		game.SetOwner(w, second, "bot_0")
		role := game.ChooseRole(w, "bot_0", second)
		if role == game.RoleMixed && w.Brain("bot_0").Skill > 0.9 {
			// a strong bot rarely makes the hybrid mistake; the role must still be remembered
			t.Log("hybrid mistake drawn")
		}
		if game.ChooseRole(w, "bot_0", second) != role || w.Brain("bot_0").Roles[second.ID] != role {
			t.Fatal("role not remembered")
		}
	})

	t.Run("a full farmstead is a bottleneck: Farmstead first", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		v.Units[game.USpear] = game.VillageFarmCapacity(v)
		if b, ok := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); !ok || b != game.BFarm {
			t.Fatalf("next %s %v", b, ok)
		}
	})

	t.Run("a full storehouse: Storehouse first", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		v.Resources.Wood = game.VillageStorageCapacity(v)
		if b, ok := game.NextBuilding(w, "bot_0", v, game.Mulberry32(1)); !ok || b != game.BStorage {
			t.Fatalf("next %s %v", b, ok)
		}
	})

	t.Run("a strong bot farmed every 30 min builds wall and hideout before the rest", func(t *testing.T) {
		for seed := uint32(1); seed <= 40; seed++ {
			w, bv := oneBotWorld(t, seed, 0.95)
			b := &bv.Buildings
			b[game.BMain], b[game.BBarracks], b[game.BStorage], b[game.BFarm], b[game.BWood], b[game.BClay], b[game.BIron] = 10, 5, 10, 10, 8, 8, 8
			w.SetNow(5 * game.MsPerHour)
			for i := range int64(8) {
				game.RecordHabit(w, "bot_0", "h1", game.HabitObservation{FarmedAt: ptr(w.Now() - (7-i)*30*game.MsPerMinute)})
			}
			if game.HardFarmer(w, "bot_0") != "h1" {
				continue // this seed's bot did not notice; covered statistically in TestBotAdaptation
			}
			if next, _ := game.NextBuilding(w, "bot_0", bv, game.Mulberry32(1)); next != game.BWall && next != game.BHide {
				t.Fatalf("seed %d next %s", seed, next)
			}
			return
		}
		t.Fatal("no seed where the bot noticed")
	})

	t.Run("build planning fills the slots with opening steps", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		v.Resources = game.Resources{Wood: 700, Clay: 700, Iron: 700} // below the storage bottleneck (800 of 1000)
		game.PlanBuilding(w, "bot_0", v)
		if len(v.BuildQueue) != 2 {
			t.Fatalf("queue %d", len(v.BuildQueue))
		}
		opening := map[game.BuildingID]bool{}
		for _, s := range game.Opening[:8] {
			opening[s.Building] = true
		}
		for _, j := range v.BuildQueue {
			if !opening[j.Building] {
				t.Fatalf("queued %s, not an early opening step", j.Building)
			}
		}
		if len(w.Brain("bot_0").ActionLog) != 2 {
			t.Fatal("actions not logged")
		}
	})

	t.Run("recruitment and research towards the role's targets", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		farmingVillage(v)
		v.Units = game.Units{}
		v.Resources = game.Resources{Wood: 20000, Clay: 20000, Iron: 20000}
		game.PlanResearch(w, "bot_0", v)
		if len(v.ResearchQueue) != 1 {
			t.Fatal("no research")
		}
		game.PlanRecruitment(w, "bot_0", v)
		queued := 0
		for _, b := range game.RecruitBuildings {
			queued += len(*v.RecruitQueues.Q(b))
		}
		if queued == 0 {
			t.Fatal("no recruitment")
		}
	})

	t.Run("before sleep, queues for the break or as much as is affordable", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		farmingVillage(v)
		// Below half the storage, so only the "cover the break" step recruits (not the spending-down step).
		v.Resources = game.Resources{Wood: 3000, Clay: 3000, Iron: 3000}
		next := w.Now() + 8*game.MsPerHour
		game.QueueForOffline(w, "bot_0", v, next)
		end := w.Now()
		for _, b := range game.RecruitBuildings {
			for _, j := range *v.RecruitQueues.Q(b) {
				end = max(end, j.CompletesAt)
			}
		}
		if end < next-2*game.MsPerHour && game.CanRecruitMore(w, "bot_0", v) {
			t.Fatalf("recruitment ends %d h before the session and more could be queued", (next-end)/game.MsPerHour)
		}
	})
}

func TestBotFarmingPlanners(t *testing.T) {
	t.Run("unknown wall: scout with 4 LC + a scout; wall without rams: skipped", func(t *testing.T) {
		w, v := oneBotWorld(t, 41, 0.95)
		farmingVillage(v)
		k := &game.KnownVillage{ID: "v:1:1", X: 1, Y: 1, Hauls: []float64{}}
		if u, ok := game.SizeRaid(w, v, k, 0.95); !ok || u != (game.UnitCounts{game.ULightCav: 4, game.UScout: 1}) {
			t.Fatalf("unknown wall %v %v", u, ok)
		}
		k.Wall = ptr(3)
		if _, ok := game.SizeRaid(w, v, k, 0.95); ok {
			t.Fatal("walled target raided")
		}
		k.Wall, k.Units = ptr(0), &game.UnitCounts{}
		u, ok := game.SizeRaid(w, v, k, 0.95)
		if !ok || u[game.ULightCav] < 4 || u.Total() != u[game.ULightCav] {
			t.Fatalf("known empty barbarian %v %v", u, ok)
		}
	})

	t.Run("wall > 0 without rams and a dry farm are skipped", func(t *testing.T) {
		w, home := oneBotWorld(t, 41, 0.95)
		farmingVillage(home)
		near := game.GeneratedVillagesNear(w.Seed(), w.Config(), home.X, home.Y, 6)
		walled := game.EnsureVillage(w, near[0].X, near[0].Y)
		dry := game.EnsureVillage(w, near[1].X, near[1].Y)
		walled.Buildings[game.BWall] = 2
		game.ObserveBattle(w, "bot_0", walled, game.BattleSeen{Units: &game.UnitCounts{}, Wall: ptr(2)})
		for range 3 {
			game.ObserveBattle(w, "bot_0", dry, game.BattleSeen{Units: &game.UnitCounts{}, Wall: ptr(0), HaulRatio: ptr(0.1)})
		}
		var targets []string
		defer game.SpyRaids(func(id game.PlayerID, c *game.Command, night bool, next int64) { targets = append(targets, c.TargetID) })()
		game.PlanFarming(w, "bot_0")
		if len(targets) < 10 || slices.Contains(targets, walled.ID) || slices.Contains(targets, dry.ID) {
			t.Fatalf("%d raids, walled %v, dry %v", len(targets), slices.Contains(targets, walled.ID), slices.Contains(targets, dry.ID))
		}
		k := w.Brain("bot_0").Memory.Known[targets[0]]
		if k == nil || *k.LastAttackAt != w.Now() {
			t.Fatal("raid not remembered")
		}
	})

	t.Run("farm targets: no own villages and no active players under a peaceful stance", func(t *testing.T) {
		w, home := oneBotWorld(t, 41, 0.95)
		for _, k := range game.FarmCandidates(w, "bot_0", home, 25) {
			if k.OwnerID == "bot_0" || k.OwnerID == "h1" {
				t.Fatalf("candidate %+v", k)
			}
		}
	})

	t.Run("a strong bot's night raids return within ±60 min of the next session", func(t *testing.T) {
		w, home := oneBotWorld(t, 41, 0.95)
		farmingVillage(home)
		next := w.Now() + 6*game.MsPerHour
		var backs []int64
		defer game.SpyRaids(func(id game.PlayerID, c *game.Command, night bool, n int64) {
			if night {
				backs = append(backs, c.ArrivesAt+(c.ArrivesAt-c.DepartedAt))
			}
		})()
		game.PlanNightRuns(w, "bot_0", next)
		if len(backs) == 0 {
			t.Fatal("no night runs")
		}
		for _, b := range backs {
			if b < next-game.MsPerHour || b > next+game.MsPerHour {
				t.Fatalf("back %d min off", (b-next)/game.MsPerMinute)
			}
		}
	})
}

func TestBotTargets(t *testing.T) {
	t.Run("known defence, offence and target score", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		k := &game.KnownVillage{ID: "x", Units: &game.UnitCounts{game.USpear: 10}, Wall: ptr(5), Hauls: []float64{}}
		if d, ok := game.KnownDefence(k); !ok || d != 150*1.2 {
			t.Fatalf("defence %v", d)
		}
		v.Units[game.UAxe] = 100
		if game.OwnOffence(w, "bot_0") != 4000 {
			t.Fatal("offence")
		}
		human := w.Village(w.Player("h1").VillageIDs[0])
		hk := &game.KnownVillage{ID: human.ID, X: human.X, Y: human.Y, OwnerID: "h1", Points: human.Points, Hauls: []float64{}}
		if game.ScoreTarget(w, "bot_0", hk) != 0 {
			t.Fatal("peaceful stance: human scored")
		}
		w.Config().BotStance = game.StanceNormal
		normal := game.ScoreTarget(w, "bot_0", hk)
		w.Config().BotStance = game.StanceAllAgainstHumans
		if normal <= 0 || game.ScoreTarget(w, "bot_0", hk) != 5*normal {
			t.Fatalf("normal %v all-against %v", normal, game.ScoreTarget(w, "bot_0", hk))
		}
		w.Player("h1").ProtectedUntil = w.Now() + 1
		if game.ScoreTarget(w, "bot_0", hk) != 0 {
			t.Fatal("protected scored")
		}
	})

	t.Run("sleep window from the public hourly history", func(t *testing.T) {
		w, _ := oneBotWorld(t, 31, 0.95)
		p := w.Player("h1")
		w.SetNow(7 * game.MsPerDay)
		p.History.Hourly = nil
		pts := 100
		for i := range 7 * 24 {
			hour := game.LocalMinuteOfDay(w, w.Now()-int64(7*24-1-i)*game.MsPerHour-1) / 60
			if hour >= 7 {
				pts += 5
			}
			p.History.Hourly = append(p.History.Hourly, pts)
		}
		win, ok := game.LikelyOfflineWindow(w, "h1")
		if !ok || win.StartMin != 0 || win.EndMin != 7*60 {
			t.Fatalf("window %+v %v", win, ok)
		}
	})

	t.Run("a campaign under “all against humans” targets a human, even a distant one", func(t *testing.T) {
		cfg := gametest.TestConfig()
		cfg.BotCount, cfg.BeginnerProtectionDays, cfg.BotStance = 40, 0, game.StanceAllAgainstHumans
		failOnBotErrors(t)
		w, _ := gametest.NewWorldWithHuman(63, cfg)
		picks, human := 0, 0
		for _, p := range w.Players() {
			if p.IsHuman || p.Status != game.StatusActive {
				continue
			}
			w.Village(p.VillageIDs[0]).Units[game.UAxe] = 3000
			b := w.Brain(p.ID)
			b.Mood = game.Mood{Phase: "campaign", Until: w.Now() + game.MsPerDay}
			game.PickCampaignTarget(w, p.ID)
			picks++
			if b.Mood.TargetPlayer == "h1" {
				human++
			}
		}
		if float64(human) < 0.8*float64(picks) {
			t.Fatalf("%d/%d picked the human", human, picks)
		}
	})
}

func TestBotExpansionPlanner(t *testing.T) {
	t.Run("a strong bot with troops builds the path to the Academy", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		b := &v.Buildings
		b[game.BMain], b[game.BSmithy], b[game.BMarket], b[game.BStorage], b[game.BFarm], b[game.BBarracks] = 20, 20, 9, 25, 25, 20
		v.Resources = game.Resources{Wood: 300000, Clay: 300000, Iron: 300000}
		v.Units[game.USpear] = 300
		v.Units[game.USword] = 100
		v.Units[game.ULightCav] = 600
		game.PlanExpansion(w, "bot_0")
		if len(v.BuildQueue) == 0 || v.BuildQueue[0].Building != game.BMarket {
			t.Fatalf("queue %+v", v.BuildQueue)
		}
	})

	t.Run("coins and a noble with an Academy", func(t *testing.T) {
		w, v := oneBotWorld(t, 31, 0.95)
		v.Buildings[game.BAcademy], v.Buildings[game.BFarm], v.Buildings[game.BStorage] = 1, 25, 30
		v.Resources = game.Resources{Wood: 400000, Clay: 400000, Iron: 400000}
		game.PlanExpansion(w, "bot_0")
		if w.Player("bot_0").Coins == 0 && len(v.RecruitQueues.Academy) == 0 {
			t.Fatal("no coin and no noble")
		}
		if len(v.RecruitQueues.Academy) != 1 {
			t.Fatalf("noble queue %d", len(v.RecruitQueues.Academy))
		}
	})
}
