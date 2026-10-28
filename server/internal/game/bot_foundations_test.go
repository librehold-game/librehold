package game_test

import (
	"math"
	"testing"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// 19.10.2026 09:00 local time (the origin already carries the zone offset, so it is computed in UTC).
var clockOrigin = time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC).UnixMilli()

// monday is 19.10.2026 00:00 local, the origin of the adaptation tests.
var monday = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).UnixMilli()

func TestClock(t *testing.T) {
	w := gametest.NewMemWorld(1, gametest.TestConfig(), clockOrigin)
	if game.WorldTimezone != "Europe/Warsaw" {
		t.Fatal("zone")
	}
	if got := game.LocalMinuteOfDay(w, 0); got != 9*60 {
		t.Fatalf("minute at world start %d", got)
	}
	if got := game.LocalMinuteOfDay(w, 15*game.MsPerHour+61_500); got != 1 {
		t.Fatalf("minute after midnight %d", got)
	}
	day := game.LocalDayIndex(w, 0)
	if game.LocalDayIndex(w, 15*game.MsPerHour) != day+1 {
		t.Fatal("day rolls at local midnight")
	}
	if got := game.WorldTimeOf(w, day+1, 8*60); got != 23*game.MsPerHour {
		t.Fatalf("08:00 next day = %d", got)
	}
	if game.IsWeekendDay(day) || !game.IsWeekendDay(day+5) || !game.IsWeekendDay(day+6) || game.IsWeekendDay(day+7) {
		t.Fatal("19.10.2026 is a Monday")
	}
	before := gametest.NewMemWorld(1, gametest.TestConfig(), -game.MsPerHour)
	if game.LocalMinuteOfDay(before, 0) != 23*60 || game.LocalDayIndex(before, 0) != -1 {
		t.Fatal("negative local time")
	}
}

func TestWeightedPick(t *testing.T) {
	t.Run("a strong bot usually picks the best option, a weak one often does not", func(t *testing.T) {
		opts := []game.Option[string]{{Value: "a", Score: 1}, {Value: "b", Score: 0.6}, {Value: "c", Score: 0.2}}
		rate := func(skill float64) float64 {
			rng := game.Mulberry32(9)
			n := 0
			for range 1000 {
				if game.WeightedPick(rng, opts, skill) == "a" {
					n++
				}
			}
			return float64(n) / 1000
		}
		if rate(1) <= 0.8 || rate(0) >= 0.6 {
			t.Fatalf("strong %v weak %v", rate(1), rate(0))
		}
	})
	t.Run("follows: strong by strong, weak by weak", func(t *testing.T) {
		rule := game.StrongWeak{Strong: 1, Weak: 0}
		if !game.Follows(game.Mulberry32(1), 1, rule) || game.Follows(game.Mulberry32(1), 0, rule) {
			t.Fatal("follows")
		}
	})
}

// stanceWorld has 3 bots, speed 1, no protection, and human h1.
func stanceWorld(t *testing.T, stance game.BotStance) (*gametest.MemWorld, []game.PlayerID) {
	t.Helper()
	cfg := gametest.TestConfig()
	cfg.BotCount, cfg.BeginnerProtectionDays, cfg.BotStance = 3, 0, stance
	w, _ := gametest.NewWorldWithHuman(61, cfg)
	var bots []game.PlayerID
	for _, p := range w.Players() {
		if !p.IsHuman && p.Status == game.StatusActive {
			bots = append(bots, p.ID)
		}
	}
	return w, bots
}

func TestBotStance(t *testing.T) {
	for _, c := range []struct {
		stance   game.BotStance
		provoked bool
		want     bool
	}{
		{game.StanceNormal, false, true}, {game.StanceAllAgainstHumans, false, true},
		{game.StancePeacefulRetaliate, false, false}, {game.StancePeacefulRetaliate, true, true},
		{game.StancePeacefulReputation, false, false}, {game.StancePeacefulReputation, true, true},
	} {
		t.Run(string(c.stance), func(t *testing.T) {
			w, bots := stanceWorld(t, c.stance)
			if c.provoked {
				game.OnHumanAggression(w, bots[0], "h1")
			}
			if got := game.MayTarget(w, bots[0], "h1"); got != c.want {
				t.Fatalf("provoked=%v → %v, want %v", c.provoked, got, c.want)
			}
		})
	}

	t.Run("a grudge expires after 5 days without new attacks", func(t *testing.T) {
		w, bots := stanceWorld(t, game.StancePeacefulRetaliate)
		game.OnHumanAggression(w, bots[0], "h1")
		w.SetNow(w.Now() + 5*game.MsPerDay + game.MsPerMinute)
		if game.MayTarget(w, bots[0], "h1") {
			t.Fatal("grudge did not fade")
		}
	})

	t.Run("barbarians and other bots always; human weight 5 with all against humans", func(t *testing.T) {
		w, bots := stanceWorld(t, game.StancePeacefulRetaliate)
		if !game.MayTarget(w, bots[0], "") || !game.MayTarget(w, bots[0], game.Ref(bots[1])) {
			t.Fatal("non-humans")
		}
		if game.HumanTargetWeight(w, "h1") != 1 {
			t.Fatal("weight")
		}
		w.Config().BotStance = game.StanceAllAgainstHumans
		if game.HumanTargetWeight(w, "h1") != 5 || game.HumanTargetWeight(w, game.Ref(bots[1])) != 1 {
			t.Fatal("all against humans weight")
		}
	})
}

func TestStancePerHuman(t *testing.T) {
	w, bots := stanceWorld(t, game.StancePeacefulReputation)
	if _, err := game.JoinHuman(w, "h2", "Friend", 2, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	game.OnHumanAggression(w, bots[0], "h1")
	if !game.MayTarget(w, bots[1], "h1") || game.MayTarget(w, bots[1], "h2") {
		t.Fatal("reputation must be per human")
	}
	if w.Brain(bots[0]).Grudges["h1"] == nil || w.Brain(bots[0]).Grudges["h2"] != nil {
		t.Fatal("grudge per human")
	}
}

func TestBotAdaptation(t *testing.T) {
	newBotWorld := func(t *testing.T) (*gametest.MemWorld, game.PlayerID) {
		cfg := gametest.TestConfig()
		cfg.BotCount, cfg.BeginnerProtectionDays = 1, 0
		w := gametest.NewMemWorld(103, cfg, monday)
		if err := game.NewWorld(w); err != nil {
			t.Fatal(err)
		}
		w.Brain("bot_0").Skill = 0.95
		return w, "bot_0"
	}

	// Noticing is a seeded draw (strong 0.9, weak 0.1), so the rule is checked over many worlds rather than on one
	// seed.
	t.Run("a strong bot farmed every 30 min spots the farmer, a weak one rarely", func(t *testing.T) {
		recognised := func(skill float64) int {
			n := 0
			for seed := uint32(1); seed <= 100; seed++ {
				cfg := gametest.TestConfig()
				cfg.BotCount = 1
				w := gametest.NewMemWorld(seed, cfg, monday)
				if err := game.NewWorld(w); err != nil {
					t.Fatal(err)
				}
				w.Brain("bot_0").Skill = skill
				w.SetNow(5 * game.MsPerHour)
				for i := range int64(8) {
					game.RecordHabit(w, "bot_0", "h1", game.HabitObservation{FarmedAt: ptr(w.Now() - (7-i)*30*game.MsPerMinute)})
				}
				if game.HardFarmer(w, "bot_0") == "h1" {
					n++
				}
			}
			return n
		}
		if strong, weak := recognised(0.95), recognised(0); strong < 75 || weak > 25 {
			t.Fatalf("strong %d/100, weak %d/100", strong, weak)
		}
	})

	t.Run("rare attacks are not farming", func(t *testing.T) {
		w, bot := newBotWorld(t)
		w.SetNow(20 * game.MsPerHour)
		for i := range int64(8) {
			game.RecordHabit(w, bot, "h1", game.HabitObservation{FarmedAt: ptr(w.Now() - (7-i)*2*game.MsPerHour)})
		}
		if got := game.HardFarmer(w, bot); got != "" {
			t.Fatalf("hard farmer %q", got)
		}
	})

	t.Run("a steady dodge rhythm → the return minute and a landing right after it", func(t *testing.T) {
		w, bot := newBotWorld(t)
		for _, dm := range [][2]int64{{0, 6*60 + 50}, {1, 7 * 60}, {2, 7*60 + 10}} {
			game.RecordHabit(w, bot, "h1", game.HabitObservation{DodgeReturnAt: ptr(dm[0]*game.MsPerDay + dm[1]*game.MsPerMinute)})
		}
		m, ok := game.DodgeMinute(w, bot, "h1")
		if !ok || m != 7*60 {
			t.Fatalf("dodge minute %d %v", m, ok)
		}
		w.SetNow(3*game.MsPerDay + 12*game.MsPerHour)
		at, ok := game.LandAfterDodge(w, bot, "h1", w.Now(), game.Mulberry32(1))
		lm := game.LocalMinuteOfDay(w, at)
		if !ok || at < w.Now() || lm < 7*60+2 || lm > 7*60+10 {
			t.Fatalf("land at minute %d", lm)
		}
	})

	t.Run("send delay 5–90 min, deterministic", func(t *testing.T) {
		w, bot := newBotWorld(t)
		a, b := game.SendJitter(w, bot, "x"), game.SendJitter(w, bot, "x")
		if a != b || a < 5*game.MsPerMinute || a > 90*game.MsPerMinute {
			t.Fatalf("jitter %d %d", a, b)
		}
	})
}

func TestBotMemory(t *testing.T) {
	cfg := gametest.TestConfig()
	cfg.BotCount = 3
	w, human := gametest.NewWorldWithHuman(61, cfg)
	bot := game.PlayerID("bot_0")
	game.DiscoverNearby(w, bot)
	brain := w.Brain(bot)
	if brain.Memory.DiscoveredAt != w.Now() {
		t.Fatal("discoveredAt")
	}
	mine := game.PlayerVillages(w, bot)[0]
	if game.Distance(mine.X, mine.Y, human.X, human.Y) <= game.BotConfig.KnowRadius && brain.Memory.Known[human.ID] == nil {
		t.Fatal("near human village not known")
	}
	for id, k := range brain.Memory.Known {
		if k.OwnerID == "" || k.OwnerID == game.Ref(bot) {
			t.Fatalf("%s: barbarians and own villages are not copied", id)
		}
	}

	t.Run("a barbarian that taught no lesson drops from memory after 3 days", func(t *testing.T) {
		bx, by := gametest.NearestBarbarianXY(w, mine)
		barb := game.EnsureVillage(w, bx, by)
		game.ObserveBattle(w, bot, barb, game.BattleSeen{Losses: false, HaulRatio: ptr(1.0)})
		if brain.Memory.Known[barb.ID] == nil {
			t.Fatal("observed barbarian not known")
		}
		w.SetNow(w.Now() + 3*game.MsPerDay + 1)
		game.DiscoverNearby(w, bot)
		if brain.Memory.Known[barb.ID] != nil {
			t.Fatal("barbarian not forgotten")
		}
	})

	t.Run("three poor hauls: the farm runs dry", func(t *testing.T) {
		bx, by := gametest.NearestBarbarianXY(w, mine)
		barb := game.EnsureVillage(w, bx, by)
		for range 3 {
			game.ObserveBattle(w, bot, barb, game.BattleSeen{HaulRatio: ptr(0.1)})
		}
		k := brain.Memory.Known[barb.ID]
		if k.DeadSince == nil || *k.DeadSince != w.Now() || len(k.Hauls) != 0 {
			t.Fatalf("known %+v", k)
		}
	})

	t.Run("generatedVillagesNear: generator tiles nearest first", func(t *testing.T) {
		near := game.GeneratedVillagesNear(w.Seed(), w.Config(), mine.X, mine.Y, 6)
		if len(near) == 0 {
			t.Fatal("none")
		}
		for i := 1; i < len(near); i++ {
			if near[i].D < near[i-1].D || near[i].D > 6 || near[i].D == 0 {
				t.Fatal("order / radius")
			}
		}
		if math.IsNaN(near[0].D) {
			t.Fatal("nan")
		}
	})
}

func ptr[T any](v T) *T { return &v }
