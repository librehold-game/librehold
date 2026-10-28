package game_test

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func testConfig() game.WorldConfig {
	cfg := game.DefaultWorldConfig
	cfg.WorldSpeed = 1
	cfg.BotCount = 0
	return cfg
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestResources(t *testing.T) {
	t.Run("production checkpoints", func(t *testing.T) {
		for level, want := range map[int]float64{0: 0, 1: 30, 10: 117, 20: 530, 30: 2400} {
			if got := game.ProductionPerHour(level); got != want {
				t.Errorf("ProductionPerHour(%d) = %v, want %v", level, got, want)
			}
		}
	})

	t.Run("base 5/h at level 0 and the world speed multiplier", func(t *testing.T) {
		cfg := testConfig()
		v := game.CreateVillage(1, 1, "", "h1", 0)
		if got := game.VillageProductionPerHour(v, game.Wood, &cfg); got != 5 {
			t.Fatalf("got %v", got)
		}
		cfg.WorldSpeed = 3
		if got := game.VillageProductionPerHour(v, game.Wood, &cfg); got != 15 {
			t.Fatalf("got %v", got)
		}
		v.Buildings[game.BWood] = 10
		if got := game.VillageProductionPerHour(v, game.Wood, &cfg); got != (5+117)*3 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("bonus village bonuses", func(t *testing.T) {
		cfg := testConfig()
		v := game.CreateVillage(1, 1, "", "h1", 0)
		v.Buildings[game.BWood] = 1
		v.Bonus = &game.VillageBonus{Kind: "wood"}
		if got := game.VillageProductionPerHour(v, game.Wood, &cfg); got != 70 {
			t.Fatalf("wood %v", got)
		}
		if got := game.VillageProductionPerHour(v, game.Clay, &cfg); got != 5 {
			t.Fatalf("clay %v", got)
		}
		v.Bonus = &game.VillageBonus{Kind: "allResources"}
		if got := game.VillageProductionPerHour(v, game.Wood, &cfg); !near(got, 35*1.3, 1e-9) {
			t.Fatalf("all %v", got)
		}
	})
}

func TestStorageFarmHide(t *testing.T) {
	t.Run("storehouse capacity", func(t *testing.T) {
		if game.StorageCapacity(1, nil) != 1000 {
			t.Fatal("L1")
		}
		if c := game.StorageCapacity(30, nil); c <= 390_000 || c >= 410_000 {
			t.Fatalf("L30 %v", c)
		}
		if c := game.StorageCapacity(15, nil); c <= 17_000 || c >= 19_000 {
			t.Fatalf("L15 %v", c)
		}
		if game.StorageCapacity(1, &game.VillageBonus{Kind: "storage"}) != 1500 {
			t.Fatal("bonus")
		}
	})
	t.Run("population limit", func(t *testing.T) {
		if game.FarmCapacity(1, nil) != 240 {
			t.Fatal("L1")
		}
		if c := game.FarmCapacity(30, nil); c <= 23_500 || c >= 24_500 {
			t.Fatalf("L30 %v", c)
		}
		if game.FarmCapacity(1, &game.VillageBonus{Kind: "farm"}) != 288 {
			t.Fatal("bonus")
		}
	})
	t.Run("hideout from the table", func(t *testing.T) {
		for level, want := range map[int]float64{0: 0, 1: 150, 2: 200, 10: 2000, 11: 2000} {
			if got := game.HideCapacity(level); got != want {
				t.Errorf("HideCapacity(%d) = %v, want %v", level, got, want)
			}
		}
	})
}

func TestAccrue(t *testing.T) {
	t.Run("accrues linearly and clamps to capacity", func(t *testing.T) {
		cfg := testConfig()
		v := game.CreateVillage(1, 1, "", "h1", 0)
		v.Buildings[game.BWood] = 1
		v.Buildings[game.BStorage] = 1
		game.Accrue(v, game.MsPerHour, &cfg)
		if !near(v.Resources.Wood, 35, 1e-9) || !near(v.Resources.Clay, 5, 1e-9) {
			t.Fatalf("after 1 h: %+v", v.Resources)
		}
		game.Accrue(v, 1000*game.MsPerHour, &cfg)
		if v.Resources.Wood != 1000 || v.ResourcesUpdatedAt != 1000*game.MsPerHour {
			t.Fatalf("capped: %+v at %d", v.Resources, v.ResourcesUpdatedAt)
		}
	})

	t.Run("does not move time or resources backwards", func(t *testing.T) {
		cfg := testConfig()
		v := game.CreateVillage(1, 1, "", "h1", 0)
		v.Resources = game.Resources{Wood: 100, Clay: 100, Iron: 100}
		v.ResourcesUpdatedAt = 5000
		game.Accrue(v, 1000, &cfg)
		if v.Resources.Wood != 100 || v.ResourcesUpdatedAt != 5000 {
			t.Fatalf("%+v at %d", v.Resources, v.ResourcesUpdatedAt)
		}
	})

	t.Run("an 8 h jump == 1000 small steps", func(t *testing.T) {
		cfg := testConfig()
		a := game.CreateVillage(1, 1, "", "h1", 0)
		a.Buildings[game.BWood], a.Buildings[game.BClay], a.Buildings[game.BIron], a.Buildings[game.BStorage] = 7, 5, 3, 5
		a.Resources = game.Resources{Wood: 10, Clay: 20, Iron: 30}
		b := *a
		total := 8 * game.MsPerHour
		game.Accrue(a, total, &cfg)
		for i := int64(1); i <= 1000; i++ {
			game.Accrue(&b, total*i/1000, &cfg)
		}
		if !near(a.Resources.Wood, b.Resources.Wood, 1e-6) || !near(a.Resources.Clay, b.Resources.Clay, 1e-6) ||
			!near(a.Resources.Iron, b.Resources.Iron, 1e-6) {
			t.Fatalf("jump %+v steps %+v", a.Resources, b.Resources)
		}
	})

	t.Run("msUntilFull", func(t *testing.T) {
		cfg := testConfig()
		v := game.CreateVillage(1, 1, "", "h1", 0)
		v.Buildings[game.BWood] = 1
		v.Buildings[game.BStorage] = 1
		v.Resources.Wood = 650
		if got := game.MsUntilFull(v, game.Wood, &cfg); !near(got, float64(10*game.MsPerHour), 1e-3) {
			t.Fatalf("got %v", got)
		}
		v.Resources.Wood = 1000
		if got := game.MsUntilFull(v, game.Wood, &cfg); got != 0 {
			t.Fatalf("got %v", got)
		}
	})
}

func TestBuildingsCostsPopPointsTime(t *testing.T) {
	t.Run("level 1 cost = base cost, level 2 = × multiplier", func(t *testing.T) {
		if c := game.BuildCost(game.BMain, 1); c != (game.Resources{Wood: 90, Clay: 80, Iron: 70}) {
			t.Fatalf("main 1 %+v", c)
		}
		if c := game.BuildCost(game.BMain, 2); c != (game.Resources{Wood: 113, Clay: 102, Iron: 88}) {
			t.Fatalf("main 2 %+v", c)
		}
		if c := game.BuildCost(game.BStatue, 1); c != (game.Resources{Wood: 220, Clay: 220, Iron: 220}) {
			t.Fatalf("statue %+v", c)
		}
	})

	t.Run("maxed buildings' population fits in farmstead 30", func(t *testing.T) {
		sum := 0
		for _, id := range game.BuildingIDs {
			sum += game.BuildingPopulation(id, game.BuildingDefs[id].MaxLevel)
		}
		if sum <= 3000 || float64(sum) >= game.FarmCapacity(30, nil) {
			t.Fatalf("sum %d", sum)
		}
	})

	t.Run("a fully built village has 11 000–13 000 points", func(t *testing.T) {
		var maxed game.Buildings
		for _, id := range game.BuildingIDs {
			maxed[id] = game.BuildingDefs[id].MaxLevel
		}
		if p := game.VillagePoints(maxed); p < 11_000 || p > 13_000 {
			t.Fatalf("points %d", p)
		}
	})

	t.Run("build time: level formula, town hall shortens, worldSpeed divides, minimum 1 s", func(t *testing.T) {
		t1 := game.BuildTimeMs(game.BWood, 1, 1, 1)
		if want := int64(math.Round(900 * 1.18 * math.Pow(1.2, -13) * math.Pow(1.05, -1) * 1000)); t1 != want {
			t.Fatalf("t1 %d want %d", t1, want)
		}
		t30 := game.BuildTimeMs(game.BWood, 1, 30, 1)
		if !near(float64(t30)/float64(t1), math.Pow(1.05, -29), 1e-5) {
			t.Fatalf("main 30 ratio %v", float64(t30)/float64(t1))
		}
		if got := game.BuildTimeMs(game.BWood, 1, 1, 2); got != int64(math.Round(float64(t1)/2)) {
			t.Fatalf("speed 2 %d", got)
		}
		if got := game.BuildTimeMs(game.BWood, 1, 30, 100); got != 1000 {
			t.Fatalf("minimum %d", got)
		}
		main30 := float64(game.BuildTimeMs(game.BMain, 30, 29, 1)) / 3_600_000
		if main30 <= 10 || main30 >= 20 {
			t.Fatalf("main30 %v h", main30)
		}
		main10 := float64(game.BuildTimeMs(game.BMain, 10, 9, 1)) / 60_000
		if main10 <= 30 || main10 >= 60 {
			t.Fatalf("main10 %v min", main10)
		}
	})

	t.Run("village points and hideout capacity", func(t *testing.T) {
		var b game.Buildings
		b[game.BMain] = 1
		if game.VillagePoints(b) != 10 {
			t.Fatal("main 1 points")
		}
		if game.HideCapacity(3) != 267 || game.HideCapacity(5) != 474 || game.HideCapacity(10) != 2000 {
			t.Fatal("hide")
		}
	})
}

func TestModifiers(t *testing.T) {
	t.Run("side multipliers: human ×3 resources, bot unchanged, barbarian unchanged", func(t *testing.T) {
		cfg := testConfig()
		cfg.HumanModifiers.Resources = 3
		w := gametest.NewMemWorld(5, cfg, 0)
		human := game.CreateVillage(10, 10, "", "h1", 0)
		bot := game.CreateVillage(12, 10, "", "bot_test", 0)
		barb := game.CreateVillage(14, 10, "", "", 0)
		for _, v := range []*game.Village{human, bot, barb} {
			v.Buildings[game.BWood] = 1
			w.PutVillage(v)
		}
		ph := game.VillageProductionPerHour(human, game.Wood, w.Config())
		pb := game.VillageProductionPerHour(bot, game.Wood, w.Config())
		pz := game.VillageProductionPerHour(barb, game.Wood, w.Config())
		if ph != 3*pb || pz != pb {
			t.Fatalf("human %v bot %v barbarian %v", ph, pb, pz)
		}
		if game.ModifiersFor(w, human).Resources != 3 || game.ModifiersFor(w, bot).Resources != 1 {
			t.Fatal("ModifiersFor sides")
		}
	})
}

func TestTouchAccruesAtWorldNow(t *testing.T) {
	cfg := testConfig()
	w := gametest.NewMemWorld(1, cfg, 0)
	v := game.CreateVillage(1, 1, "", "h1", 0)
	w.PutVillage(v)
	w.SetNow(game.MsPerHour)
	game.Touch(w, v)
	if v.ResourcesUpdatedAt != game.MsPerHour || !near(v.Resources.Wood, 5, 1e-9) {
		t.Fatalf("%+v at %d", v.Resources, v.ResourcesUpdatedAt)
	}
}

type productionGolden struct {
	Production []struct {
		Speed float64 `json:"speed"`
		Bonus *string `json:"bonus"`
		Level int     `json:"level"`
		Wood  float64 `json:"wood"`
		Clay  float64 `json:"clay"`
		Iron  float64 `json:"iron"`
	} `json:"production"`
	Capacity []struct {
		Level        int     `json:"level"`
		Storage      float64 `json:"storage"`
		StorageBonus float64 `json:"storageBonus"`
		Farm         float64 `json:"farm"`
		FarmBonus    float64 `json:"farmBonus"`
		Hide         float64 `json:"hide"`
	} `json:"capacity"`
	Buildings []struct {
		ID     game.BuildingID  `json:"id"`
		Level  int              `json:"level"`
		Cost   game.Resources   `json:"cost"`
		Points int              `json:"points"`
		Pop    int              `json:"pop"`
		Times  map[string]int64 `json:"times"`
	} `json:"buildings"`
	MaxedVillagePoints int `json:"maxedVillagePoints"`
}

func loadProductionGolden(t *testing.T) productionGolden {
	t.Helper()
	raw, err := os.ReadFile("testdata/golden/production.json")
	if err != nil {
		t.Fatal(err)
	}
	var g productionGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// TestProductionGolden records production, capacities, building costs, times, points and population.
func TestProductionGolden(t *testing.T) {
	g := loadProductionGolden(t)
	if len(g.Production) == 0 || len(g.Capacity) != 31 || len(g.Buildings) == 0 {
		t.Fatal("empty fixture")
	}
	for i, c := range g.Production {
		cfg := game.DefaultWorldConfig
		cfg.WorldSpeed = c.Speed
		v := game.CreateVillage(0, 0, "", "", 0)
		v.Buildings[game.BWood], v.Buildings[game.BClay], v.Buildings[game.BIron] = c.Level, c.Level, c.Level
		if c.Bonus != nil {
			v.Bonus = &game.VillageBonus{Kind: game.VillageBonusKind(*c.Bonus)}
		}
		p := &g.Production[i]
		p.Wood = game.VillageProductionPerHour(v, game.Wood, &cfg)
		p.Clay = game.VillageProductionPerHour(v, game.Clay, &cfg)
		p.Iron = game.VillageProductionPerHour(v, game.Iron, &cfg)
	}
	for i, c := range g.Capacity {
		p := &g.Capacity[i]
		p.Storage, p.StorageBonus = game.StorageCapacity(c.Level, nil), game.StorageCapacity(c.Level, &game.VillageBonus{Kind: "storage"})
		p.Farm, p.FarmBonus = game.FarmCapacity(c.Level, nil), game.FarmCapacity(c.Level, &game.VillageBonus{Kind: "farm"})
		p.Hide = game.HideCapacity(c.Level)
	}
	for i, c := range g.Buildings {
		p := &g.Buildings[i]
		p.Cost = game.BuildCost(c.ID, c.Level)
		p.Points = game.BuildingPoints(c.ID, c.Level)
		p.Pop = game.BuildingPopulation(c.ID, c.Level)
		p.Times = map[string]int64{}
		for _, speed := range []float64{1, 8} {
			for _, main := range []int{1, 10, 20} {
				key := strconv.FormatFloat(speed, 'f', -1, 64) + ":" + strconv.Itoa(main)
				p.Times[key] = game.BuildTimeMs(c.ID, c.Level, main, speed)
			}
		}
	}
	var maxed game.Buildings
	for _, id := range game.BuildingIDs {
		maxed[id] = game.BuildingDefs[id].MaxLevel
	}
	g.MaxedVillagePoints = game.VillagePoints(maxed)
	game.Golden(t, "production", g)
}
