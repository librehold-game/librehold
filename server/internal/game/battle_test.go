package game_test

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"testing"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func academyVillage(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w, v := gametest.NewTestWorld(seed)
	v.Buildings[game.BMain], v.Buildings[game.BAcademy], v.Buildings[game.BFarm] = 20, 2, 25
	v.Buildings[game.BStorage], v.Buildings[game.BSmithy], v.Buildings[game.BMarket] = 30, 20, 10
	v.Resources = game.Resources{Wood: 200000, Clay: 200000, Iron: 200000}
	return w, v
}

// nearestBarbarianTile is the closest generated barbarian field within 12.
func nearestBarbarianTile(w *gametest.MemWorld, v *game.Village) (int, int) {
	bx, by, bestD := 0, 0, math.Inf(1)
	for x := v.X - 12; x <= v.X+12; x++ {
		for y := v.Y - 12; y <= v.Y+12; y++ {
			d := game.Distance(v.X, v.Y, x, y)
			if d < 1 || !game.TileHasVillage(w.Seed(), w.Config(), x, y) {
				continue
			}
			if t := w.Village(game.VillageIDFor(x, y)); t != nil && t.OwnerID != "" {
				continue
			}
			if d < bestD {
				bx, by, bestD = x, y, d
			}
		}
	}
	return bx, by
}

func ensureBarbarianTile(w *gametest.MemWorld, v *game.Village) *game.Village {
	x, y := nearestBarbarianTile(w, v)
	return game.EnsureVillage(w, x, y)
}

func ensureNearestBarbarian(w *gametest.MemWorld, v *game.Village) *game.Village {
	x, y := gametest.NearestBarbarianXY(w, v)
	return game.EnsureVillage(w, x, y)
}

func disarm(w *gametest.MemWorld, id string) {
	t := w.Village(id)
	t.Units = game.Units{}
	t.Buildings[game.BWall] = 0
	t.SupportFrom = map[string]game.UnitCounts{}
}

func lastReport(w *gametest.MemWorld, id game.PlayerID) *game.Report {
	r := w.ReportsOf(id)
	if len(r) == 0 {
		return nil
	}
	return r[len(r)-1]
}

func TestCoins(t *testing.T) {
	t.Run("coin cost is fixed; coins per noble = villages + nobles owned", func(t *testing.T) {
		w, v := academyVillage(t, 7)
		if game.CoinCost() != (game.Resources{Wood: 28000, Clay: 30000, Iron: 25000}) {
			t.Fatal("cost")
		}
		h := w.Player("h1")
		h.VillageIDs = append(h.VillageIDs, "x", "y")
		if game.CoinsForNextNoble(w, "h1", 0) != 3 {
			t.Fatal("3 villages")
		}
		h.VillageIDs = h.VillageIDs[:1]
		if game.CoinsNeededFor(w, "h1", 1) != 1 || game.CoinsNeededFor(w, "h1", 2) != 3 {
			t.Fatal("needed")
		}
		v.Units[game.UNoble] = 1
		if game.CoinsNeededFor(w, "h1", 1) != 2 {
			t.Fatal("owned noble")
		}
	})

	t.Run("minting needs an Academy and resources; coins are global", func(t *testing.T) {
		w, v := gametest.NewTestWorld(7)
		if k := blockerKinds(game.CheckMint(v).Blockers); !slices.Equal(k, []string{"academy", "resources"}) {
			t.Fatalf("blockers %v", k)
		}
		if game.MintCoin(w, "h1", v.ID) == nil {
			t.Fatal("minted without academy")
		}
		w2, v2 := academyVillage(t, 7)
		for range 2 {
			if err := game.MintCoin(w2, "h1", v2.ID); err != nil {
				t.Fatal(err)
			}
		}
		if w2.Player("h1").Coins != 2 || v2.Resources.Wood != 200000-56000 || w2.Human("h1").Stats.CoinsMinted != 2 {
			t.Fatal("mint")
		}
	})
}

func TestMintRejectsForeignVillage(t *testing.T) {
	w, v := academyVillage(t, 7)
	gametest.NewHumanPlayer(w, "h2")
	var be *game.BlockedError
	if err := game.MintCoin(w, "h2", v.ID); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("err %v", err)
	}
}

func TestNoble(t *testing.T) {
	t.Run("needs a coin, limit = Academy level (counting queued and sent), the coin is spent and refunded", func(t *testing.T) {
		w, v := academyVillage(t, 7)
		h := w.Player("h1")
		if !slices.Contains(blockerKinds(game.CheckRecruit(w, v, game.UNoble, 1).Blockers), "coins") || game.MaxRecruitable(w, v, game.UNoble) != 0 {
			t.Fatal("coins")
		}
		h.Coins = 5
		if game.MaxRecruitable(w, v, game.UNoble) != 2 {
			t.Fatal("max 2")
		}
		job := mustRecruit(t, w, v, game.UNoble, 1)
		if h.Coins != 4 || h.NoblesTrained != 1 {
			t.Fatal("coin spent")
		}
		mustRecruit(t, w, v, game.UNoble, 1)
		if !slices.Contains(blockerKinds(game.CheckRecruit(w, v, game.UNoble, 1).Blockers), "limit") {
			t.Fatal("limit")
		}
		if _, err := game.CancelRecruit(w, "h1", v.ID, job.ID); err != nil || h.Coins != 4 {
			t.Fatalf("refund: coins %d err %v", h.Coins, err)
		}
		advanceTouch(t, w, v, 10*game.MsPerHour)
		if v.Units[game.UNoble] != 1 {
			t.Fatalf("nobles %d", v.Units[game.UNoble])
		}
		v.Units[game.UNoble] = 0
		v.Units[game.UAxe] = 10
		tx, ty := nearestBarbarianTile(w, v)
		h.Coins = 1
		mustRecruit(t, w, v, game.UNoble, 1)
		advanceTouch(t, w, v, 20*game.MsPerHour)
		mustSend(t, w, v, tx, ty, game.UnitCounts{game.UNoble: 1, game.UAxe: 10}, "attack")
		if !slices.Contains(blockerKinds(game.CheckRecruit(w, v, game.UNoble, 2).Blockers), "limit") || game.MaxRecruitable(w, v, game.UNoble) != 0 {
			t.Fatal("noble away counts")
		}
	})
}

func TestLoyalty(t *testing.T) {
	t.Run("regenerates +1 per full hour, max 100, keeps the hour fraction", func(t *testing.T) {
		_, v := gametest.NewTestWorld(7)
		v.Loyalty, v.LoyaltyUpdatedAt = 40, 0
		game.SettleLoyalty(v, 5*game.MsPerHour/2, 1)
		if v.Loyalty != 42 || v.LoyaltyUpdatedAt != 2*game.MsPerHour {
			t.Fatalf("%v at %d", v.Loyalty, v.LoyaltyUpdatedAt)
		}
		game.SettleLoyalty(v, 3*game.MsPerHour, 1)
		if v.Loyalty != 43 || game.LoyaltyAt(v, 100*game.MsPerHour, 1) != 100 {
			t.Fatal("43 / at")
		}
		game.SettleLoyalty(v, 100*game.MsPerHour, 1)
		if v.Loyalty != 100 || game.NoblesNeeded(100) != 4 || game.NoblesNeeded(27) != 1 {
			t.Fatal("max / needed")
		}
	})

	t.Run("loyalty recovers faster on a faster world", func(t *testing.T) {
		_, v := gametest.NewTestWorld(5)
		v.Loyalty, v.LoyaltyUpdatedAt = 50, 0
		if game.LoyaltyAt(v, game.MsPerHour, 1) != 51 || game.LoyaltyAt(v, game.MsPerHour, 8) != 58 {
			t.Fatal("speed")
		}
	})

	t.Run("drop of 20–35 per noble, deterministic from the command id, conquest at ≤ 0", func(t *testing.T) {
		w, home := gametest.NewTestWorld(7)
		v := ensureBarbarianTile(w, home)
		c := &game.Command{ID: "c_test"}
		for i := range uint32(50) {
			wi := gametest.NewMemWorld(i, gametest.TestConfig(), 0)
			v.Loyalty = 100
			after, conquered := game.ApplyNobles(wi, v, c, 1)
			if drop := 100 - after; drop < 20 || drop > 35 || conquered {
				t.Fatalf("seed %d drop %v", i, drop)
			}
		}
		v.Loyalty = 100
		a, _ := game.ApplyNobles(w, v, c, 1)
		v.Loyalty = 100
		b, _ := game.ApplyNobles(w, v, c, 1)
		if a != b {
			t.Fatal("nondeterministic")
		}
		v.Loyalty = 30
		if _, conquered := game.ApplyNobles(w, v, c, 2); !conquered || v.Loyalty != 0 {
			t.Fatal("conquest")
		}
	})
}

func setupConquest(t *testing.T, seed uint32) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, v := academyVillage(t, seed)
	v.Units[game.UAxe], v.Units[game.UNoble] = 2000, 2
	target := ensureBarbarianTile(w, v)
	target.Units = game.Units{}
	target.Buildings[game.BWall] = 0
	target.Loyalty, target.LoyaltyUpdatedAt = 40, w.Now()
	return w, v, target
}

func TestConquest(t *testing.T) {
	t.Run("a won attack with nobles takes the village", func(t *testing.T) {
		w, v, target := setupConquest(t, 7)
		before := target.Buildings
		c := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 2}, "attack")
		advance(t, w, c.ArrivesAt)
		if target.OwnerID != "h1" || target.Loyalty != 25 || !slices.Contains(w.Player("h1").VillageIDs, target.ID) {
			t.Fatalf("owner %q loyalty %v", target.OwnerID, target.Loyalty)
		}
		if target.Name != "" {
			t.Fatalf("name %q", target.Name)
		}
		for _, b := range game.BuildingIDs {
			if before[b] == 0 {
				if target.Buildings[b] != 0 {
					t.Fatalf("%s grew", b)
				}
				continue
			}
			minL := 0
			if b == game.BMain {
				minL = 1
			}
			if target.Buildings[b] > before[b] || target.Buildings[b] < max(minL, before[b]-3) {
				t.Fatalf("%s %d → %d", b, before[b], target.Buildings[b])
			}
		}
		if target.Research != (game.Units{}) {
			t.Fatal("research kept")
		}
		var rep *game.Report
		for _, r := range w.ReportsOf("h1") {
			if r.Type == "conquest" {
				rep = r
			}
		}
		if rep == nil || !rep.Conquered || *rep.LoyaltyBefore != 40 || *rep.LoyaltyAfter != 0 || rep.Haul != nil {
			t.Fatalf("report %+v", rep)
		}
		if w.Human("h1").Stats.VillagesConquered != 1 {
			t.Fatal("stats")
		}
		ret := w.Commands()[0]
		if ret.Type != "return" || ret.Units[game.UNoble] != 1 || ret.Units[game.UAxe] <= 0 {
			t.Fatalf("return %+v", ret)
		}
		wood := target.Resources.Wood
		capacity := game.VillageStorageCapacity(target)
		advanceTouch(t, w, target, w.Now()+game.MsPerHour)
		want := math.Min(capacity, wood+game.VillageProductionPerHour(target, game.Wood, w.Config()))
		if wood >= capacity {
			want = wood
		}
		if math.Abs(target.Resources.Wood-want) > 1e-3 {
			t.Fatalf("wood %v want %v", target.Resources.Wood, want)
		}
	})

	t.Run("one noble is not enough → loyalty drops, the village stays barbarian", func(t *testing.T) {
		w, v, target := setupConquest(t, 7)
		c := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 1}, "attack")
		advance(t, w, c.ArrivesAt)
		rep := w.ReportsOf("h1")[0]
		if target.OwnerID != "" || target.Loyalty >= 40 || target.Loyalty < 5 || w.Commands()[0].Units[game.UNoble] != 1 ||
			rep.Type != "attack" || *rep.LoyaltyAfter != target.Loyalty || w.Intel("h1", target.ID).Intel.Loyalty != target.Loyalty {
			t.Fatalf("loyalty %v report %+v", target.Loyalty, rep)
		}
	})

	t.Run("lost attack: the noble dies, loyalty unchanged", func(t *testing.T) {
		w, v, target := setupConquest(t, 7)
		target.Units[game.USpear] = 5000
		c := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 10, game.UNoble: 1}, "attack")
		advanceTouch(t, w, v, c.ArrivesAt)
		if target.Loyalty != 40 || len(w.Commands()) != 0 || v.Units[game.UNoble] != 1 {
			t.Fatal("lost attack")
		}
	})

	t.Run("a bonus village keeps its bonus after conquest", func(t *testing.T) {
		w, v, target := setupConquest(t, 7)
		target.Bonus = &game.VillageBonus{Kind: "wood"}
		c := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 2}, "attack")
		advance(t, w, c.ArrivesAt)
		if target.OwnerID != "h1" || target.Bonus == nil || target.Bonus.Kind != "wood" {
			t.Fatal("bonus lost")
		}
		target.Buildings[game.BClay] = target.Buildings[game.BWood]
		if math.Abs(game.VillageProductionPerHour(target, game.Wood, w.Config())-2*game.VillageProductionPerHour(target, game.Clay, w.Config())) > 1e-9 {
			t.Fatal("bonus production")
		}
	})

	t.Run("offline: one jump vs 1000 steps with a conquest gives identical state", func(t *testing.T) {
		build := func() *gametest.MemWorld {
			w, v, target := setupConquest(t, 9)
			mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 2}, "attack")
			return w
		}
		a, b := build(), build()
		advance(t, a, game.MsPerDay)
		for i := int64(1); i <= 1000; i++ {
			advance(t, b, game.MsPerDay*i/1000)
		}
		for _, w := range []*gametest.MemWorld{a, b} {
			for _, v := range w.Villages() {
				game.Touch(w, v)
			}
		}
		if len(a.Player("h1").VillageIDs) != 2 || !sameWorld(t, a, b) {
			t.Fatal("jump and steps differ")
		}
	})
}

// sameWorld compares villages, players, commands and h1's reports as JSON, floats within 1e-12.
func sameWorld(t *testing.T, a, b *gametest.MemWorld) bool {
	t.Helper()
	dump := func(w *gametest.MemWorld) any {
		raw, _ := json.Marshal(map[string]any{"villages": w.Villages(), "players": w.Players(), "commands": w.Commands(), "reports": w.ReportsOf("h1")})
		var out any
		_ = json.Unmarshal(raw, &out)
		return out
	}
	return game.JSONNear(dump(a), dump(b))
}

func TestTakenVillageTroopsAwayVanish(t *testing.T) {
	w, home := gametest.NewTestWorld(47)
	x := gametest.MakeBotVillage(w, home)
	y := gametest.MakeBotVillage(w, home)
	x.Units[game.UAxe] = 100
	bx, by := gametest.NearestBarbarianXY(w, home)
	if _, err := game.SendCommand(w, "bot_test", x.ID, bx, by, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{}); err != nil {
		t.Fatal(err)
	}
	y.SupportFrom[x.ID] = game.UnitCounts{game.USpear: 20}
	game.TransferVillage(w, x, "h1", home.ID)
	advanceTouch(t, w, x, w.Now()+game.MsPerDay)
	for _, c := range w.Commands() {
		if c.OriginID == x.ID || c.TargetID == x.ID {
			t.Fatalf("command %+v survived", c)
		}
	}
	if x.Units[game.UAxe] != 0 || slices.ContainsFunc(w.ReportsOf("h1"), func(r *game.Report) bool { return r.OriginID == x.ID }) {
		t.Fatal("troops came back")
	}
	if _, ok := y.SupportFrom[x.ID]; ok {
		t.Fatal("support kept")
	}
}

func TestTransferBarbarianToHuman(t *testing.T) {
	w, v := gametest.NewTestWorld(4)
	b := ensureNearestBarbarian(w, v)
	game.TransferVillage(w, b, "h1", v.ID)
	if b.OwnerID != "h1" || !slices.Contains(w.Player("h1").VillageIDs, b.ID) {
		t.Fatal("owner not rewritten in both places")
	}
}

func protectionWorld(t *testing.T, mutate func(*game.WorldConfig)) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	cfg := gametest.TestConfig()
	cfg.BotCount = 5
	if mutate != nil {
		mutate(&cfg)
	}
	return gametest.NewWorldWithHuman(11, cfg)
}

func TestProtection(t *testing.T) {
	t.Run("attack on a protected player: blocked with the protection end time", func(t *testing.T) {
		w, home := protectionWorld(t, nil)
		bot := gametest.NearestBotVillage(w, home)
		home.Units[game.UAxe] = 10
		found := false
		for _, b := range game.CheckSend(w, "h1", home.ID, bot.X, bot.Y, game.UnitCounts{game.UAxe: 10}, "attack").Blockers {
			if b.Kind() == "targetProtected" && b["until"] == w.Player(game.PlayerID(bot.OwnerID)).ProtectedUntil {
				found = true
			}
		}
		if !found {
			t.Fatal("no targetProtected")
		}
	})

	t.Run("a protected player attacks a barbarian but not a player", func(t *testing.T) {
		w, home := protectionWorld(t, nil)
		bot := gametest.NearestBotVillage(w, home)
		w.Player(game.PlayerID(bot.OwnerID)).ProtectedUntil = 0
		home.Units[game.UAxe] = 10
		if !slices.Contains(sendKinds(w, home, bot.X, bot.Y, game.UnitCounts{game.UAxe: 10}, "attack"), "selfProtected") {
			t.Fatal("selfProtected")
		}
		bx, by := gametest.NearestBarbarianXY(w, home)
		if k := sendKinds(w, home, bx, by, game.UnitCounts{game.UAxe: 10}, "attack"); len(k) != 0 {
			t.Fatalf("barbarian blocked: %v", k)
		}
	})

	t.Run("protection disabled in world settings", func(t *testing.T) {
		w, _ := protectionWorld(t, func(c *game.WorldConfig) { c.BeginnerProtectionDays = 0 })
		for _, p := range w.Players() {
			if p.ProtectedUntil > w.Now() {
				t.Fatalf("%s protected", p.ID)
			}
		}
	})

	t.Run("protected last village: nobles bring loyalty down to 1 at most", func(t *testing.T) {
		w, home := protectionWorld(t, func(c *game.WorldConfig) { c.BeginnerProtectionDays = 0 })
		bot := gametest.NearestBotVillage(w, home)
		owner := bot.OwnerID
		home.Units[game.UAxe], home.Units[game.UNoble] = 5000, 5
		for range 5 {
			c := mustSend(t, w, home, bot.X, bot.Y, game.UnitCounts{game.UAxe: 1000, game.UNoble: 1}, "attack")
			advance(t, w, c.ArrivesAt)
		}
		if bot.Loyalty != 1 || bot.OwnerID != owner {
			t.Fatalf("loyalty %v owner %q", bot.Loyalty, bot.OwnerID)
		}
	})

	t.Run("last village can be lost: a bot is out, a human starts over at the edge", func(t *testing.T) {
		w, home := protectionWorld(t, func(c *game.WorldConfig) { c.BeginnerProtectionDays, c.LastVillage = 0, "canBeLost" })
		victim := gametest.NearestBotVillage(w, home)
		victimID := game.PlayerID(victim.OwnerID)
		game.TransferVillage(w, victim, "h1", home.ID)
		if w.Player(victimID).Status != game.StatusEliminated {
			t.Fatal("bot not eliminated")
		}
		var conqueror game.PlayerID
		for _, p := range w.Players() {
			if !p.IsHuman && p.Status == game.StatusActive {
				conqueror = p.ID
				break
			}
		}
		for _, id := range slices.Clone(w.Player("h1").VillageIDs) {
			game.TransferVillage(w, w.Village(id), game.Ref(conqueror), "")
		}
		h := w.Player("h1")
		if len(h.VillageIDs) != 1 || h.Coins != 0 || h.VillageIDs[0] == home.ID || w.Village(h.VillageIDs[0]).OwnerID != "h1" {
			t.Fatalf("respawn %+v", h)
		}
		if !slices.ContainsFunc(w.News(0), func(n *game.NewsItem) bool { return n.For == "h1" && n.Alert }) {
			t.Fatal("no private respawn news")
		}
	})
}

func TestBattleIntegration(t *testing.T) {
	t.Run("farming: report, loot returns home, target loses resources, stats", func(t *testing.T) {
		w, v := gametest.NewTestWorld(101)
		v.Units[game.UAxe], v.Units[game.ULightCav] = 200, 50
		v.Buildings[game.BStorage] = 15
		tx, ty := nearestBarbarianTile(w, v)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.UAxe: 200, game.ULightCav: 50}, "attack")
		target := w.Village(c.TargetID)
		before := target.Resources.Wood
		capacity := game.VillageStorageCapacity(target)
		expected := math.Min(capacity, before+game.VillageProductionPerHour(target, game.Wood, w.Config())*float64(c.ArrivesAt-w.Now())/float64(game.MsPerHour))
		advance(t, w, c.ArrivesAt)
		reps := w.ReportsOf("h1")
		if len(reps) != 1 {
			t.Fatalf("%d reports", len(reps))
		}
		rep := reps[0]
		if rep.Type != "attack" || rep.Outcome != "attackerWon" || rep.Haul == nil || rep.Read || game.SumResources(*rep.Haul) <= 0 {
			t.Fatalf("report %+v", rep)
		}
		if math.Abs(target.Resources.Wood-(expected-rep.Haul.Wood)) > 1e-3 {
			t.Fatalf("target wood %v want %v", target.Resources.Wood, expected-rep.Haul.Wood)
		}
		in := w.Intel("h1", target.ID)
		if in == nil || *in.LastFarmedAt != c.ArrivesAt || *in.Intel.LastHaul != *rep.Haul {
			t.Fatalf("intel %+v", in)
		}
		ret := w.Commands()[0]
		if ret.Type != "return" || *ret.Haul != *rep.Haul {
			t.Fatalf("return %+v", ret)
		}
		woodBefore := v.Resources.Wood
		advanceTouch(t, w, v, ret.ArrivesAt)
		if v.Resources.Wood < woodBefore+rep.Haul.Wood-1 || v.Units[game.UAxe]+v.Units[game.ULightCav] != 250-rep.Attacker.Losses.Total() {
			t.Fatal("haul / units home")
		}
		if w.Human("h1").Stats.ResourcesLooted.Wood != rep.Haul.Wood {
			t.Fatal("stats")
		}
	})

	t.Run("lost attack: no return, 💀 report, defender keeps resources", func(t *testing.T) {
		w, v := gametest.NewTestWorld(101)
		v.Units[game.USpear] = 1
		tx, ty := nearestBarbarianTile(w, v)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 1}, "attack")
		target := w.Village(c.TargetID)
		target.Units[game.USpear] = 500
		before := target.Resources.Wood
		advance(t, w, c.ArrivesAt)
		if w.ReportsOf("h1")[0].Outcome != "defenderWon" || len(w.Commands()) != 0 || target.Resources.Wood < before || w.Human("h1").Stats.UnitsLost != 1 {
			t.Fatal("lost attack")
		}
	})

	t.Run("scouting: report with resources and buildings; intel stored", func(t *testing.T) {
		w, v := gametest.NewTestWorld(101)
		v.Units[game.UScout] = 10
		tx, ty := nearestBarbarianTile(w, v)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.UScout: 10}, "attack")
		advance(t, w, c.ArrivesAt)
		rep := w.ReportsOf("h1")[0]
		in := w.Intel("h1", c.TargetID)
		if rep.Type != "scout" || rep.Scout == nil || in.Intel.At != c.ArrivesAt || *in.FarmTargetFlags.ScoutedAt != c.ArrivesAt ||
			w.Commands()[0].Units[game.UScout] != 10 {
			t.Fatalf("scout report %+v", rep)
		}
	})

	t.Run("offline: one jump vs 1000 steps with battles gives identical state", func(t *testing.T) {
		build := func() *gametest.MemWorld {
			w, v := gametest.NewTestWorld(202)
			v.Units[game.UAxe], v.Units[game.ULightCav], v.Units[game.URam], v.Units[game.UScout] = 2000, 300, 40, 20
			n := 0
			for x := v.X - 8; x <= v.X+8 && n < 10; x++ {
				for y := v.Y - 8; y <= v.Y+8 && n < 10; y++ {
					if !game.TileHasVillage(w.Seed(), w.Config(), x, y) || (x == v.X && y == v.Y) {
						continue
					}
					u := game.UnitCounts{game.UAxe: 150, game.ULightCav: 25, game.URam: 4}
					if n%3 == 0 {
						u = game.UnitCounts{game.UScout: 2}
					}
					mustSend(t, w, v, x, y, u, "attack")
					n++
				}
			}
			return w
		}
		a, b := build(), build()
		advance(t, a, 2*game.MsPerDay)
		for i := int64(1); i <= 1000; i++ {
			advance(t, b, 2*game.MsPerDay*i/1000)
		}
		for _, w := range []*gametest.MemWorld{a, b} {
			for _, v := range w.Villages() {
				game.Touch(w, v)
			}
		}
		if len(a.ReportsOf("h1")) != 10 || !sameWorld(t, a, b) {
			t.Fatal("jump and steps differ")
		}
	})

	t.Run("barbarians do not rebuild: a demolished level and a killed garrison remain after 200 h", func(t *testing.T) {
		w, home := gametest.NewTestWorld(101)
		v := ensureBarbarianTile(w, home)
		v.Buildings[game.BWood] = max(0, v.Buildings[game.BWood]-1)
		woodLevel := v.Buildings[game.BWood]
		v.Units[game.USpear] = 1
		home.Units[game.UScout] = 1
		c := mustSend(t, w, home, v.X, v.Y, game.UnitCounts{game.UScout: 1}, "attack")
		advanceTouch(t, w, v, c.ArrivesAt+200*game.MsPerHour)
		if v.Buildings[game.BWood] != woodLevel || v.Units[game.USpear] != 1 {
			t.Fatal("barbarian rebuilt")
		}
	})

	t.Run("a barbarian does not rebuild: a demolished building and killed troops remain", func(t *testing.T) {
		w, home := gametest.NewTestWorld(3)
		b := ensureNearestBarbarian(w, home)
		if err := gametest.ScoutOnce(w, home, b); err != nil {
			t.Fatal(err)
		}
		b.Units[game.USpear] = 2
		b.Buildings[game.BWood] = 0
		advance(t, w, 7*game.MsPerDay)
		if err := gametest.ScoutOnce(w, home, b); err != nil {
			t.Fatal(err)
		}
		if b.Units[game.USpear] != 2 || b.Buildings[game.BWood] != 0 {
			t.Fatal("barbarian rebuilt")
		}
	})
}

func TestCommandsWithBattles(t *testing.T) {
	t.Run("return after arrival", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		v.Units[game.USpear], v.Units[game.UAxe] = 50, 10
		tx, ty := nearestUnstoredBarbarian(t, w, v, 1)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 20, game.UAxe: 5}, "attack")
		disarm(w, c.TargetID)
		travel := c.ArrivesAt - c.DepartedAt
		advanceTouch(t, w, v, c.ArrivesAt)
		cmds := w.Commands()
		if len(cmds) != 1 || cmds[0].Type != "return" || cmds[0].TargetID != v.ID || cmds[0].ArrivesAt-cmds[0].DepartedAt != travel || v.Units[game.USpear] != 30 {
			t.Fatalf("commands %+v", cmds)
		}
		advanceTouch(t, w, v, cmds[0].ArrivesAt)
		if len(w.Commands()) != 0 || v.Units[game.USpear] != 50 || v.Units[game.UAxe] != 10 {
			t.Fatal("not home")
		}
	})

	t.Run("offline: one jump vs 1000 steps with many commands (attack → return chain)", func(t *testing.T) {
		build := func() (*gametest.MemWorld, *game.Village) {
			w, v := gametest.NewTestWorld(33)
			v.Units[game.USpear], v.Units[game.ULightCav] = 500, 100
			var targets [][2]int
			for x := v.X - 10; x <= v.X+10 && len(targets) < 12; x++ {
				for y := v.Y - 10; y <= v.Y+10 && len(targets) < 12; y++ {
					if game.TileHasVillage(w.Seed(), w.Config(), x, y) && !(x == v.X && y == v.Y) {
						targets = append(targets, [2]int{x, y})
					}
				}
			}
			for i, tg := range targets {
				u := game.UnitCounts{game.ULightCav: 5, game.USpear: 3}
				if i%2 == 1 {
					u = game.UnitCounts{game.USpear: 40}
				}
				disarm(w, mustSend(t, w, v, tg[0], tg[1], u, "attack").TargetID)
			}
			return w, v
		}
		a, va := build()
		b, _ := build()
		advance(t, a, 3*game.MsPerDay)
		for i := int64(1); i <= 1000; i++ {
			advance(t, b, 3*game.MsPerDay*i/1000)
		}
		for _, w := range []*gametest.MemWorld{a, b} {
			for _, v := range w.Villages() {
				game.Touch(w, v)
			}
		}
		if !sameWorld(t, a, b) || va.Units[game.USpear] < 490 || va.Units[game.ULightCav] != 100 || a.NextIDCounter() != b.NextIDCounter() {
			t.Fatalf("spear %d lightCav %d", va.Units[game.USpear], va.Units[game.ULightCav])
		}
	})

	t.Run("performance: 500 commands and 30 days in < 2 s", func(t *testing.T) {
		w, v := gametest.NewTestWorld(44)
		v.Units[game.USpear] = 100_000
		sent := 0
		for x := v.X - 30; x <= v.X+30 && sent < 500; x++ {
			for y := v.Y - 30; y <= v.Y+30 && sent < 500; y++ {
				if !game.TileHasVillage(w.Seed(), w.Config(), x, y) || (x == v.X && y == v.Y) {
					continue
				}
				disarm(w, mustSend(t, w, v, x, y, game.UnitCounts{game.USpear: 50}, "attack").TargetID)
				sent++
			}
		}
		start := time.Now()
		advanceTouch(t, w, v, 30*game.MsPerDay)
		if dt := time.Since(start); sent != 500 || len(w.Commands()) != 0 || v.Units[game.USpear] < 99_000 || dt > 2*time.Second {
			t.Fatalf("sent %d commands %d spear %d in %v", sent, len(w.Commands()), v.Units[game.USpear], dt)
		}
	})

	t.Run("ensureVillage on send accrues the target's resources lazily (resourcesUpdatedAt = now)", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		v.Units[game.USpear] = 5
		advance(t, w, 5*game.MsPerMinute)
		tx, ty := nearestUnstoredBarbarian(t, w, v, 1)
		c := mustSend(t, w, v, tx, ty, game.UnitCounts{game.USpear: 5}, "attack")
		target := w.Village(c.TargetID)
		if target.ResourcesUpdatedAt != w.Now() {
			t.Fatal("not stored at now")
		}
		before := target.Resources.Wood
		advance(t, w, c.ArrivesAt)
		looted := 0.0
		if r := lastReport(w, "h1"); r.Haul != nil {
			looted = r.Haul.Wood
		}
		if target.ResourcesUpdatedAt != c.ArrivesAt || target.Resources.Wood+looted < before || game.EnsureVillage(w, tx, ty) != target {
			t.Fatal("lazy resources")
		}
	})
}

func TestStanceProvocation(t *testing.T) {
	hit := func(t *testing.T, w *gametest.MemWorld, targetID string, u game.UnitCounts) {
		home := w.Village(w.Player("h1").VillageIDs[0])
		for _, id := range game.UnitIDs {
			if u[id] > 0 {
				home.Units[id] = u[id]
			}
		}
		target := w.Village(targetID)
		c := mustSend(t, w, home, target.X, target.Y, u, "attack")
		advance(t, w, c.ArrivesAt+1)
	}
	firstVillage := func(w *gametest.MemWorld, id game.PlayerID) string { return w.Player(id).VillageIDs[0] }

	t.Run("reputation: attacking one bot marks the aggressor for all", func(t *testing.T) {
		w, bots := stanceWorld(t, game.StancePeacefulReputation)
		hit(t, w, firstVillage(w, bots[0]), game.UnitCounts{game.UAxe: 10})
		if !game.MayTarget(w, bots[1], "h1") {
			t.Fatal("reputation")
		}
	})

	t.Run("revenge: only the attacked bot holds a grudge", func(t *testing.T) {
		w, bots := stanceWorld(t, game.StancePeacefulRetaliate)
		hit(t, w, firstVillage(w, bots[0]), game.UnitCounts{game.UAxe: 10})
		if !game.MayTarget(w, bots[0], "h1") || game.MayTarget(w, bots[1], "h1") {
			t.Fatal("retaliate")
		}
	})

	t.Run("farming barbarians and inactive bots is not a provocation; scouting is", func(t *testing.T) {
		w, bots := stanceWorld(t, game.StancePeacefulRetaliate)
		home := w.Village(w.Player("h1").VillageIDs[0])
		barb := ensureNearestBarbarian(w, home)
		hit(t, w, barb.ID, game.UnitCounts{game.UAxe: 10})
		w.Player(bots[1]).Status = game.StatusQuit
		hit(t, w, firstVillage(w, bots[1]), game.UnitCounts{game.UAxe: 10})
		if w.Brain(bots[0]).Grudges["h1"] != nil || w.Brain(bots[1]).Grudges["h1"] != nil {
			t.Fatal("grudge without provocation")
		}
		hit(t, w, firstVillage(w, bots[0]), game.UnitCounts{game.UScout: 1})
		if !game.MayTarget(w, bots[0], "h1") {
			t.Fatal("scouting is provocation")
		}
	})
}

func TestReportsToEveryHumanInvolved(t *testing.T) {
	cfg := gametest.TestConfig()
	cfg.BeginnerProtectionDays = 0
	w, v1 := gametest.NewWorldWithHuman(12, cfg)
	v2, err := game.JoinHuman(w, "h2", "Defender", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := game.JoinHuman(w, "h3", "Ally", 3, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	v3.Units[game.USpear] = 7
	sup := mustSend(t, w, v3, v2.X, v2.Y, game.UnitCounts{game.USpear: 7}, "support")
	advance(t, w, sup.ArrivesAt)
	v1.Units[game.UAxe] = 300
	atk := mustSend(t, w, v1, v2.X, v2.Y, game.UnitCounts{game.UAxe: 300}, "attack")
	advance(t, w, atk.ArrivesAt)

	r1, r2, r3 := w.ReportsOf("h1"), w.ReportsOf("h2"), w.ReportsOf("h3")
	if len(r1) != 1 || len(r2) != 1 || len(r3) != 1 {
		t.Fatalf("reports %d %d %d", len(r1), len(r2), len(r3))
	}
	for _, r := range []*game.Report{r2[0], r3[0]} {
		if r.Time != r1[0].Time || r.Outcome != r1[0].Outcome || r.Attacker.Losses != r1[0].Attacker.Losses || r.Defender.Losses != r1[0].Defender.Losses {
			t.Fatalf("report differs: %+v vs %+v", r, r1[0])
		}
	}
	if r3[0].Defender.Units[game.USpear] < 7 {
		t.Fatalf("h3 does not see its support: %+v", r3[0].Defender.Units)
	}
	if r1[0].ID == r2[0].ID {
		t.Fatal("reports share an id")
	}
}

func TestBotAttackerGetsNoReport(t *testing.T) {
	w, home := gametest.NewTestWorld(12)
	bv := gametest.MakeBotVillage(w, home)
	bv.Units[game.UAxe] = 50
	bx, by := nearestBarbarianTile(w, bv)
	c, err := game.SendCommand(w, "bot_test", bv.ID, bx, by, game.UnitCounts{game.UAxe: 50}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	advance(t, w, c.ArrivesAt)
	if len(w.ReportsOf("bot_test")) != 0 || len(w.ReportsOf("h1")) != 0 {
		t.Fatal("bot battle produced a report")
	}
}

func TestIntelIsPerAttacker(t *testing.T) {
	w, v1 := gametest.NewTestWorld(12)
	if _, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	v1.Units[game.UScout] = 3
	tx, ty := nearestUnstoredBarbarian(t, w, v1, 1)
	c := mustSend(t, w, v1, tx, ty, game.UnitCounts{game.UScout: 3}, "attack")
	advance(t, w, c.ArrivesAt)
	if w.Intel("h1", c.TargetID) == nil || w.Intel("h2", c.TargetID) != nil {
		t.Fatal("intel not per attacker")
	}
}
