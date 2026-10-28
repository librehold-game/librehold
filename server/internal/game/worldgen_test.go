package game_test

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestWorldGenerator(t *testing.T) {
	cfg := gametest.TestConfig()
	t.Run("is deterministic for the seed", func(t *testing.T) {
		same, diff := 0, 0
		for x := range 50 {
			for y := range 50 {
				if game.TileHasVillage(7, &cfg, x, y) != game.TileHasVillage(8, &cfg, x, y) {
					diff++
				} else {
					same++
				}
			}
		}
		if diff <= 100 || same <= 100 {
			t.Fatalf("same %d diff %d", same, diff)
		}
		if !reflect.DeepEqual(game.BarbarianTemplate(7, 120, 80), game.BarbarianTemplate(7, 120, 80)) {
			t.Fatal("template not deterministic")
		}
	})

	t.Run("density ≈ 28 % (normal), no villages off the map", func(t *testing.T) {
		n := 0
		for x := range 100 {
			for y := range 100 {
				if game.TileHasVillage(3, &cfg, x, y) {
					n++
				}
			}
		}
		if share := float64(n) / 10000; share <= 0.26 || share >= 0.30 {
			t.Fatalf("share %v", share)
		}
		if game.TileHasVillage(3, &cfg, -1, 5) || game.TileHasVillage(3, &cfg, 200, 5) {
			t.Fatal("village outside the map")
		}
	})

	t.Run("barbarian template: no wall, no troops, no military buildings, 20–160 pts", func(t *testing.T) {
		for i := range 300 {
			tp := game.BarbarianTemplate(3, 10+i%30, 10+i/30)
			b := tp.Buildings
			if b[game.BWall] != 0 || b[game.BBarracks]+b[game.BStable]+b[game.BWorkshop]+b[game.BSmithy] != 0 ||
				tp.Units.Total() != 0 || tp.Points < 20 || tp.Points > 160 {
				t.Fatalf("template %+v", tp)
			}
		}
	})

	t.Run("≈ 3 % bonus villages", func(t *testing.T) {
		villages, bonus := 0, 0
		for x := 0; x < 200; x += 2 {
			for y := 0; y < 200; y += 2 {
				if !game.TileHasVillage(5, &cfg, x, y) {
					continue
				}
				villages++
				if game.BarbarianTemplate(5, x, y).Bonus != nil {
					bonus++
				}
			}
		}
		if share := float64(bonus) / float64(villages); share <= 0.015 || share >= 0.05 {
			t.Fatalf("share %v", share)
		}
	})

	t.Run("tileInfo does not materialise, ensureVillage materialises once and is idempotent", func(t *testing.T) {
		w, _ := gametest.NewTestWorld(9)
		var fx, fy int
		found := false
		for x := 90; x < 110 && !found; x++ {
			for y := 90; y < 110 && !found; y++ {
				if game.TileHasVillage(9, w.Config(), x, y) && w.Village(game.VillageIDFor(x, y)) == nil {
					fx, fy, found = x, y, true
				}
			}
		}
		if !found {
			t.Fatal("no barbarian tile")
		}
		info := game.TileInfo(w, fx, fy)
		if info.Kind != "village" || len(w.Villages()) != 1 {
			t.Fatalf("info %+v villages %d", info, len(w.Villages()))
		}
		v := game.EnsureVillage(w, fx, fy)
		if len(w.Villages()) != 2 || game.EnsureVillage(w, fx, fy) != v || v.OwnerID != "" || v.Points != info.Points || v.Resources.Wood <= 0 {
			t.Fatalf("ensure %+v", v)
		}
		if info2 := game.TileInfo(w, fx, fy); info2.Kind != "village" || !info2.Materialized {
			t.Fatalf("info after %+v", info2)
		}
	})

	t.Run("the player's start tile stays the player's village", func(t *testing.T) {
		w, v := gametest.NewTestWorld(21)
		if info := game.TileInfo(w, v.X, v.Y); info.Kind != "village" || info.OwnerID != "h1" {
			t.Fatalf("info %+v", info)
		}
		if game.EnsureVillage(w, v.X, v.Y) != v {
			t.Fatal("ensure replaced the player's village")
		}
	})

	t.Run("villagesInRadius returns villages sorted by distance", func(t *testing.T) {
		w, v := gametest.NewTestWorld(4)
		list := game.VillagesInRadius(w, v.X, v.Y, 6)
		if len(list) <= 5 {
			t.Fatalf("%d villages", len(list))
		}
		for i := 1; i < len(list); i++ {
			if game.Distance(v.X, v.Y, list[i].X, list[i].Y) < game.Distance(v.X, v.Y, list[i-1].X, list[i-1].Y) {
				t.Fatal("not sorted")
			}
		}
	})
}

func TestSpawnBotsOnMap(t *testing.T) {
	t.Run("default world: 100 bots, each with one village, villages do not overlap", func(t *testing.T) {
		w, _ := gametest.NewDefaultWorld(8)
		bots := 0
		for _, p := range w.Players() {
			if !p.IsHuman {
				bots++
				if len(p.VillageIDs) != 1 {
					t.Fatalf("%s has %d villages", p.ID, len(p.VillageIDs))
				}
			}
		}
		coords := map[[2]int]bool{}
		for _, v := range w.Villages() {
			coords[[2]int{v.X, v.Y}] = true
		}
		if bots != 100 || len(coords) != 101 {
			t.Fatalf("bots %d coords %d", bots, len(coords))
		}
	})

	t.Run("ramp-up: bots join gradually, at the edge of the occupied area", func(t *testing.T) {
		cfg := game.DefaultWorldConfig
		cfg.BotCount, cfg.BotArrival, cfg.RampUpDays = 20, "rampUp", 10
		w, _ := gametest.NewWorldWithHuman(8, cfg)
		active := func() int {
			n := 0
			for _, p := range w.Players() {
				if p.Status == game.StatusActive {
					n++
				}
			}
			return n
		}
		if active() != 1 {
			t.Fatalf("active at start %d", active())
		}
		advance(t, w, 5*game.MsPerDay)
		if mid := active(); mid <= 5 || mid >= 16 {
			t.Fatalf("active mid %d", mid)
		}
		advance(t, w, 12*game.MsPerDay)
		for _, p := range w.Players() {
			if p.Status == game.StatusPending {
				t.Fatalf("%s still pending", p.ID)
			}
		}
		if !slices.ContainsFunc(w.News(0), func(n *game.NewsItem) bool { return n.Key == "news.newPlayer" }) {
			t.Fatal("no join news")
		}
	})

	t.Run("a bot takes a barbarian tile: the map sees the bot", func(t *testing.T) {
		w, _ := gametest.NewDefaultWorld(8)
		var bv *game.Village
		for _, v := range w.Villages() {
			if v.OwnerID != "" && v.OwnerID != "h1" {
				bv = v
				break
			}
		}
		if info := game.TileInfo(w, bv.X, bv.Y); info.Kind != "village" || info.OwnerID != bv.OwnerID {
			t.Fatalf("info %+v", info)
		}
	})

	t.Run("the same seed gives the same player layout", func(t *testing.T) {
		a, _ := gametest.NewDefaultWorld(9)
		b, _ := gametest.NewDefaultWorld(9)
		layout := func(w *gametest.MemWorld) []string {
			var out []string
			for _, v := range w.Villages() {
				out = append(out, v.ID+"="+string(v.OwnerID))
			}
			return out
		}
		if !slices.Equal(layout(a), layout(b)) {
			t.Fatal("layouts differ")
		}
	})

	t.Run("ramp-up of 100 bots: all fit on the map, in the ring of the occupied area", func(t *testing.T) {
		cfg := game.DefaultWorldConfig
		cfg.BotCount, cfg.BotArrival, cfg.RampUpDays = 100, "rampUp", 14
		w, _ := gametest.NewWorldWithHuman(12, cfg)
		advance(t, w, 16*game.MsPerDay)
		c := float64(cfg.MapSize) / 2
		var dists []float64
		for _, v := range w.Villages() {
			if v.OwnerID != "" {
				dists = append(dists, math.Hypot(float64(v.X)-c, float64(v.Y)-c))
			}
		}
		if len(dists) != 101 || slices.Max(dists) >= 45 {
			t.Fatalf("%d villages, max dist %v", len(dists), slices.Max(dists))
		}
	})
}

func TestNewWorldHasNoHuman(t *testing.T) {
	w := gametest.NewMemWorld(8, game.DefaultWorldConfig, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	for _, p := range w.Players() {
		if p.IsHuman || w.Human(p.ID) != nil || w.Brain(p.ID) == nil {
			t.Fatalf("player %+v", p)
		}
	}
	if len(w.Players()) != 100 || len(w.Villages()) != 100 {
		t.Fatalf("%d players %d villages", len(w.Players()), len(w.Villages()))
	}
}

func TestJoinHumanQuadrant(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount = 30
	w := gametest.NewMemWorld(11, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	c := cfg.MapSize / 2
	for i, q := range []game.Quadrant{game.QuadrantNW, game.QuadrantNE, game.QuadrantSW, game.QuadrantSE} {
		settled := 0
		for _, p := range w.Players() {
			if p.Status == game.StatusActive {
				settled += len(p.VillageIDs)
			}
		}
		radius := game.MapConfig.SpawnSpacing * 0.56 * math.Sqrt(float64(settled))
		id := game.PlayerID("h" + string(rune('1'+i)))
		v, err := game.JoinHuman(w, id, "Player", int64(i+1), q)
		if err != nil {
			t.Fatal(err)
		}
		west, north := v.X <= c, v.Y <= c
		inSector := map[game.Quadrant]bool{
			game.QuadrantNW: west && north, game.QuadrantNE: !west && north,
			game.QuadrantSW: west && !north, game.QuadrantSE: !west && !north,
		}[q]
		if !inSector {
			t.Errorf("%s spawned at %d|%d", q, v.X, v.Y)
		}
		if d := game.Distance(v.X, v.Y, c, c); d < radius-1 {
			t.Errorf("%s at distance %v inside settled radius %v", q, d, radius)
		}
		p := w.Player(id)
		if p.ProtectedUntil != w.Now()+7*game.MsPerDay || !p.IsHuman || p.AccountID != int64(i+1) || w.Human(id) == nil {
			t.Errorf("player %+v", p)
		}
		if v.Units[game.USpear] != 10 {
			t.Errorf("start guard %d", v.Units[game.USpear])
		}
	}
	if _, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom); err == nil {
		t.Fatal("joined twice")
	}
}

func TestOwnership(t *testing.T) {
	t.Run("ownership helpers match a new game", func(t *testing.T) {
		w, v := gametest.NewTestWorld(4)
		h := w.Player("h1")
		if !h.IsHuman || !slices.Equal(h.VillageIDs, []string{v.ID}) || v.OwnerID != "h1" {
			t.Fatalf("player %+v", h)
		}
		bx, by := gametest.NearestBarbarianXY(w, v)
		b := game.EnsureVillage(w, bx, by)
		if !game.IsBarbarianVillage(b) || game.PlayerPoints(w, "h1") != v.Points {
			t.Fatal("barbarian / points")
		}
	})

	t.Run("a player with 2 slots does not get a third build in the queue", func(t *testing.T) {
		w, v := gametest.NewTestWorld(4)
		v.Resources = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
		mustBuild(t, w, v, game.BWood)
		mustBuild(t, w, v, game.BClay)
		found := false
		for _, b := range game.CheckBuild(w, v, game.BIron).Blockers {
			if b.Kind() == "queueFull" && b["slots"] == 2 {
				found = true
			}
		}
		if !found {
			t.Fatal("no queueFull with 2 slots")
		}
	})
}

func TestBotIdentity(t *testing.T) {
	share := func(d game.Difficulty) float64 {
		strong := 0
		for i := range 2000 {
			if game.CreateBotBrain(1, "b"+itoa(i), d).Skill >= game.BotConfig.StrongSkill {
				strong++
			}
		}
		return float64(strong) / 2000
	}
	t.Run("difficulty sets the minimum share of strong bots, and the weaker ones stay", func(t *testing.T) {
		for _, d := range []game.Difficulty{game.Easy, game.Normal, game.Hard, game.Brutal} {
			if share(d) < game.DifficultyFloor[d]-0.03 {
				t.Errorf("%s share %v", d, share(d))
			}
		}
		if share(game.Hard) >= 0.9 || share(game.Easy) <= 0.05 {
			t.Fatal("extremes")
		}
	})
	t.Run("sleep lasts 6–9 h, and the brain depends only on the seed and id", func(t *testing.T) {
		for i := range 500 {
			r := game.CreateBotBrain(2, "b"+itoa(i), game.Normal).Routine
			sleep := math.Mod(1440+r.WakeMin-math.Mod(r.SleepMin, 1440), 1440)
			if sleep < 360 || sleep > 540 {
				t.Fatalf("sleep %v", sleep)
			}
		}
		if !reflect.DeepEqual(game.CreateBotBrain(3, "x", game.Hard), game.CreateBotBrain(3, "x", game.Hard)) {
			t.Fatal("nondeterministic brain")
		}
	})
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestWorldgenGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/worldgen.json")
	if err != nil {
		t.Fatal(err)
	}
	type tile struct {
		X        int             `json:"x"`
		Y        int             `json:"y"`
		Has      bool            `json:"has"`
		Template json.RawMessage `json:"template,omitempty"`
		Terrain  *int            `json:"terrain"`
	}
	type brain struct {
		ID         game.PlayerID   `json:"id"`
		Difficulty game.Difficulty `json:"difficulty"`
		Brain      json.RawMessage `json:"brain"`
	}
	var g struct {
		Tiles []tile `json:"tiles"`
		Slots []struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"slots"`
		Brains []brain `json:"brains"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Tiles) == 0 || len(g.Slots) == 0 || len(g.Brains) == 0 {
		t.Fatal("empty fixture")
	}
	cfg := game.DefaultWorldConfig
	for i, tl := range g.Tiles {
		p := &g.Tiles[i]
		p.Has, p.Template, p.Terrain = game.TileHasVillage(7, &cfg, tl.X, tl.Y), nil, nil
		if p.Has {
			p.Template, _ = json.Marshal(game.BarbarianTemplate(7, tl.X, tl.Y))
		} else {
			terrain := game.Terrain(7, tl.X, tl.Y)
			p.Terrain = &terrain
		}
	}
	slots := game.SpawnSlots(len(g.Slots), 200, 7)
	for i := range g.Slots {
		g.Slots[i].X, g.Slots[i].Y = slots[i].X, slots[i].Y
	}
	for i, b := range g.Brains {
		g.Brains[i].Brain, _ = json.Marshal(game.CreateBotBrain(7, b.ID, b.Difficulty))
	}
	game.Golden(t, "worldgen", g)
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	return game.JSONNear(x, y)
}
