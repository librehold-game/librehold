package game_test

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func questIDs(qs []game.QuestDef) []string {
	out := make([]string, len(qs))
	for i, q := range qs {
		out[i] = q.ID
	}
	return out
}

// maxedWorld is a world where every condition of the chain should hold on it.
func maxedWorld(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	w, v := gametest.NewTestWorld(500)
	for _, b := range game.BuildingIDs {
		v.Buildings[b] = game.BuildingDefs[b].MaxLevel
	}
	for _, u := range game.UnitIDs {
		v.Units[u], v.Research[u] = 5000, 3
	}
	game.RecomputePoints(v)
	v.Resources = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
	for i := 1; i < 12; i++ {
		extra := game.CreateVillage(v.X+i, v.Y+2, "Village", "", w.Now())
		for _, b := range game.BuildingIDs {
			extra.Buildings[b] = game.BuildingDefs[b].MaxLevel
		}
		game.RecomputePoints(extra)
		w.PutVillage(extra)
		game.SetOwner(w, extra, "h1")
	}
	h := w.Human("h1")
	s := &h.Stats
	s.AttacksSent, s.BattlesWon, s.ScoutsSent, s.SupportsSent, s.TradesSent = 1000, 500, 50, 20, 20
	s.CoinsMinted, s.ResearchCompleted, s.VillagesConquered, s.UnitsKilled, s.UnitsLost = 10, 20, 12, 500_000, 1000
	s.ResourcesLooted = game.Resources{Wood: 1e6, Clay: 1e6, Iron: 1e6}
	s.AttacksRepelled = 5
	w.Player("h1").Coins = 50
	h.Trader.Accepted = true
	h.FarmAssistant.Templates["A"] = game.FarmTemplate{Units: game.UnitCounts{game.UAxe: 20}}
	return w, v
}

func TestQuestDataGolden(t *testing.T) {
	type quest struct {
		Order       int              `json:"order"`
		ID          string           `json:"id"`
		Requires    []string         `json:"requires"`
		Reward      game.QuestReward `json:"reward"`
		HasProgress bool             `json:"hasProgress"`
	}
	got := make([]quest, len(game.Quests))
	for i, q := range game.Quests {
		requires := q.Requires
		if requires == nil {
			requires = []string{}
		}
		got[i] = quest{i, q.ID, requires, q.Reward, q.Progress != nil}
	}
	game.Golden(t, "quests", got)
}

func TestQuestChain(t *testing.T) {
	t.Run("ids are unique and requirements point at existing quests, with no cycles", func(t *testing.T) {
		seen := map[string]int{}
		for i, q := range game.Quests {
			if _, dup := seen[q.ID]; dup {
				t.Fatalf("duplicate %s", q.ID)
			}
			seen[q.ID] = i
		}
		for i, q := range game.Quests {
			for _, r := range q.Requires {
				if j, ok := seen[r]; !ok || j >= i {
					t.Fatalf("%s requires %s", q.ID, r)
				}
			}
			if q.Reward.Resources == nil && !q.Reward.BuildQueueSlot && !q.Reward.AutoFarm {
				t.Fatalf("%s has no reward", q.ID)
			}
		}
		if len(game.Quests) < 40 {
			t.Fatal("fewer than 40 quests")
		}
	})

	t.Run("conditions and progress bars do not blow up on a fresh game", func(t *testing.T) {
		w, _ := gametest.NewTestWorld(501)
		for _, q := range game.Quests {
			q.Condition(w, "h1")
			if q.Progress != nil {
				if p := q.Progress(w, "h1"); math.IsNaN(p) || p < 0 || p > 1 {
					t.Fatalf("%s progress %v", q.ID, p)
				}
			}
		}
	})

	t.Run("in an “everything achieved” state every condition is met (none is dead)", func(t *testing.T) {
		w, _ := maxedWorld(t)
		for _, q := range game.Quests {
			if !q.Condition(w, "h1") {
				t.Errorf("%s unmet", q.ID)
			}
		}
	})

	t.Run("settling completes the whole chain and every reward can be claimed", func(t *testing.T) {
		w, v := maxedWorld(t)
		game.SettleQuests(w, "h1")
		h := w.Human("h1")
		if len(h.Quests.Completed) != len(game.Quests) || len(game.ActiveQuests(w, "h1")) != 0 {
			t.Fatalf("completed %d", len(h.Quests.Completed))
		}
		for _, q := range game.Quests {
			if _, err := game.ClaimQuest(w, "h1", q.ID, v.ID); err != nil {
				t.Fatalf("%s: %v", q.ID, err)
			}
		}
		if _, err := game.ClaimQuest(w, "h1", game.Quests[0].ID, v.ID); err == nil || len(h.Quests.Claimed) != len(game.Quests) {
			t.Fatal("claimed twice")
		}
	})

	t.Run("quests unlock gradually: only the roots are active at the start", func(t *testing.T) {
		w, _ := gametest.NewTestWorld(502)
		game.SettleQuests(w, "h1")
		active := questIDs(game.ActiveQuests(w, "h1"))
		if len(active) == 0 || slices.Contains(active, "villages10") {
			t.Fatalf("active %v", active)
		}
	})

	t.Run("new game: only quests without requirements are active; completing one unlocks the next; reward on claim", func(t *testing.T) {
		w, v := gametest.NewTestWorld(11)
		game.SettleQuests(w, "h1")
		h := w.Human("h1")
		if len(h.Quests.Completed) != 0 || !slices.Equal(questIDs(game.ActiveQuests(w, "h1")), []string{"wood1"}) {
			t.Fatal("start")
		}
		mustBuild(t, w, v, game.BWood)
		advance(t, w, w.Now()+game.MsPerHour)
		if !slices.Contains(h.Quests.Completed, "wood1") || slices.Contains(h.Quests.Claimed, "wood1") ||
			!slices.Equal(questIDs(game.ActiveQuests(w, "h1")), []string{"clay1"}) ||
			!slices.Equal(questIDs(game.ClaimableQuests(w, "h1")), []string{"wood1"}) {
			t.Fatalf("after wood1: completed %v active %v", h.Quests.Completed, h.Quests.Active)
		}
		if !slices.Equal(h.Quests.Active, []string{"clay1"}) {
			t.Fatalf("active field %v", h.Quests.Active)
		}
		game.Touch(w, v)
		before := v.Resources
		res, err := game.ClaimQuest(w, "h1", "wood1", v.ID)
		reward := game.QuestByID["wood1"].Reward.Resources
		if err != nil || res.Lost || math.Abs(v.Resources.Wood-(before.Wood+reward.Wood)) > 1e-5 {
			t.Fatalf("claim %+v %v", res, err)
		}
		if _, err := game.ClaimQuest(w, "h1", "wood1", v.ID); err == nil {
			t.Fatal("claimed twice")
		}
		if _, err := game.ClaimQuest(w, "h1", "clay1", v.ID); err == nil {
			t.Fatal("claimed unfinished")
		}
	})

	t.Run("reward: +1 queue slot (max 5) and the auto-farm unlock", func(t *testing.T) {
		w, v := gametest.NewTestWorld(12)
		h := w.Human("h1")
		h.Quests.Completed = append(h.Quests.Completed, "conquest1", "loot5000")
		if _, err := game.ClaimQuest(w, "h1", "conquest1", v.ID); err != nil || w.Player("h1").BuildQueueSlots != 3 {
			t.Fatal("slot")
		}
		if h.FarmAssistant.AutoFarmUnlocked {
			t.Fatal("unlocked early")
		}
		if _, err := game.ClaimQuest(w, "h1", "loot5000", v.ID); err != nil || !h.FarmAssistant.AutoFarmUnlocked {
			t.Fatal("auto-farm")
		}
		w.Player("h1").BuildQueueSlots = 5
		h.Quests.Completed = append(h.Quests.Completed, "villages5")
		if _, err := game.ClaimQuest(w, "h1", "villages5", v.ID); err != nil || w.Player("h1").BuildQueueSlots != 5 {
			t.Fatal("slot cap")
		}
	})

	t.Run("settling quests does not change the simulation (jump vs steps identical)", func(t *testing.T) {
		setup := func() (*gametest.MemWorld, *game.Village) {
			w, v := gametest.NewTestWorld(13)
			v.Resources = game.Resources{Wood: 5000, Clay: 5000, Iron: 5000}
			mustBuild(t, w, v, game.BWood)
			mustBuild(t, w, v, game.BClay)
			return w, v
		}
		a, va := setup()
		b, vb := setup()
		advanceTouch(t, a, va, 2*game.MsPerDay)
		for i := int64(1); i <= 500; i++ {
			advanceTouch(t, b, vb, 2*game.MsPerDay*i/500)
		}
		qa, _ := json.Marshal(a.Human("h1"))
		qb, _ := json.Marshal(b.Human("h1"))
		if string(qa) != string(qb) || math.Abs(va.Resources.Wood-vb.Resources.Wood) > 1e-6 {
			t.Fatal("jump and steps differ")
		}
	})

	t.Run("a quest's queue slot goes to the player; loot5000 unlocks auto-farm", func(t *testing.T) {
		if !game.QuestByID["loot5000"].Reward.AutoFarm {
			t.Fatal("loot5000 reward")
		}
	})
}

func TestQuestsArePerHuman(t *testing.T) {
	w, v1 := gametest.NewTestWorld(11)
	v2, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	mustBuild(t, w, v1, game.BWood)
	advance(t, w, w.Now()+game.MsPerHour)
	if !slices.Contains(w.Human("h1").Quests.Completed, "wood1") || slices.Contains(w.Human("h2").Quests.Completed, "wood1") {
		t.Fatal("quest progress shared")
	}
	w.Human("h1").Quests.Completed = append(w.Human("h1").Quests.Completed, "conquest1")
	if _, err := game.ClaimQuest(w, "h1", "conquest1", v2.ID); err == nil {
		t.Fatal("reward into another player's village")
	}
}

func gossipWorld(t *testing.T) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	return botWorld(t, 141, monday, func(c *game.WorldConfig) { c.BotCount = 3 })
}

func nextMorningAt(w *gametest.MemWorld) int64 {
	m := game.LocalMinuteOfDay(w, w.Now())
	d := (8*60 - m + 1440) % 1440
	if d == 0 {
		d = 1440
	}
	return w.Now() + int64(d)*game.MsPerMinute + game.MsPerMinute
}

func TestGossip(t *testing.T) {
	t.Run("a conquest from the last day makes the rumours", func(t *testing.T) {
		w, home := gossipWorld(t)
		advance(t, w, 2*game.MsPerHour)
		bot := w.Player("bot_0")
		v := ensureNearestBarbarian(w, home)
		game.RecordConquest(w, v, "", bot.ID)
		advance(t, w, nextMorningAt(w))
		if !slices.ContainsFunc(w.News(0), func(n *game.NewsItem) bool {
			return strings.HasPrefix(n.Key, "gossip.conquest.") && n.Params["a"] == bot.Name && n.Params["x"] == v.X && n.Params["y"] == v.Y
		}) {
			t.Fatal("no conquest rumour")
		}
	})

	t.Run("a player leaving starts a rumour", func(t *testing.T) {
		w, _ := gossipWorld(t)
		advance(t, w, 3*game.MsPerDay+game.MsPerHour)
		bot := w.Player("bot_0")
		bot.Status = game.StatusQuit
		bot.QuitAt = ptr(nextMorningAt(w) - 5*game.MsPerDay/2)
		advance(t, w, nextMorningAt(w))
		if !slices.ContainsFunc(w.News(0), func(n *game.NewsItem) bool {
			return strings.HasPrefix(n.Key, "gossip.inactive.") && n.Params["a"] == bot.Name
		}) {
			t.Fatal("no departure rumour")
		}
	})

	t.Run("the same day and seed give the same texts", func(t *testing.T) {
		run := func() []string {
			w, _ := gossipWorld(t)
			advance(t, w, game.MsPerHour)
			advance(t, w, nextMorningAt(w))
			var out []string
			for _, n := range w.News(0) {
				out = append(out, n.Key+fmt.Sprint(n.Params))
			}
			return out
		}
		if a, b := run(), run(); !slices.Equal(a, b) {
			t.Fatal("different texts")
		}
	})

	t.Run("rumours at 08:00 Warsaw time (the world's local clock)", func(t *testing.T) {
		w, _ := gossipWorld(t)
		at, ok := w.ScheduledAt("gossip")
		if !ok || game.LocalMinuteOfDay(w, at) != 8*60 {
			t.Fatalf("gossip at %d", at)
		}
	})
}

func TestQuestProgressCoversActiveQuestsWithBars(t *testing.T) {
	w, _ := gametest.NewTestWorld(1)
	p := game.QuestProgress(w, "h1")
	active := game.ActiveQuests(w, "h1")
	if len(active) == 0 {
		t.Fatal("no active quests")
	}
	for _, q := range active {
		if _, has := p[q.ID]; has != (q.Progress != nil) {
			t.Errorf("%s: progress present %v, has bar %v", q.ID, has, q.Progress != nil)
		}
	}
	if v, ok := p["wood1"]; !ok || v != 0 {
		t.Fatal(p)
	}
	if game.QuestProgress(w, "nobody") != nil {
		t.Fatal("unknown human")
	}
}
