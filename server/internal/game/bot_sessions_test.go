package game_test

import (
	"math"
	"slices"
	"sort"
	"testing"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// botWorld creates a world with human h1 and the given bot settings; refused bot orders fail the test.
func botWorld(t *testing.T, seed uint32, origin int64, mutate func(*game.WorldConfig)) (*gametest.MemWorld, *game.Village) {
	t.Helper()
	failOnBotErrors(t)
	cfg := game.DefaultWorldConfig
	if mutate != nil {
		mutate(&cfg)
	}
	w := gametest.NewMemWorld(seed, cfg, origin)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	v, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	return w, v
}

func botsOf(w *gametest.MemWorld) []*game.Player {
	var out []*game.Player
	for _, p := range w.Players() {
		if !p.IsHuman {
			out = append(out, p)
		}
	}
	return out
}

func bySkillDesc(w *gametest.MemWorld) []*game.Player {
	b := botsOf(w)
	sort.SliceStable(b, func(i, j int) bool { return w.Brain(b[i].ID).Skill > w.Brain(b[j].ID).Skill })
	return b
}

func TestBotSessions(t *testing.T) {
	sessionsAround := func(w *gametest.MemWorld, bot game.PlayerID, at int64) [][2]int64 {
		day := game.LocalDayIndex(w, at)
		var out [][2]int64
		for _, d := range []int64{day - 1, day} {
			for _, s := range game.DaySessions(w, bot, d) {
				out = append(out, [2]int64{game.WorldTimeOf(w, d, s.StartMin), game.WorldTimeOf(w, d, s.EndMin)})
			}
		}
		return out
	}

	t.Run("a bot thinks only in its sessions and sleeps 6–9 h every day", func(t *testing.T) {
		w, _ := botWorld(t, 21, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed = 1, 1 })
		var thinks []int64
		defer game.SpyThinks(func(id game.PlayerID, defenceOnly bool) {
			if id == "bot_0" && !defenceOnly {
				thinks = append(thinks, w.Now())
			}
		})()
		advance(t, w, 7*game.MsPerDay)
		if len(thinks) <= 20 {
			t.Fatalf("%d thinks", len(thinks))
		}
		for _, at := range thinks {
			inside := slices.ContainsFunc(sessionsAround(w, "bot_0", at), func(s [2]int64) bool { return s[0] <= at && at < s[1] })
			if !inside {
				t.Fatalf("think at %d outside sessions", at)
			}
		}
		byDay := map[int64]int64{}
		for i := 1; i < len(thinks); i++ {
			d := game.LocalDayIndex(w, thinks[i-1])
			byDay[d] = max(byDay[d], (thinks[i]-thinks[i-1])/game.MsPerMinute)
		}
		days := make([]int64, 0, len(byDay))
		for d := range byDay {
			days = append(days, d)
		}
		slices.Sort(days)
		for _, d := range days[:len(days)-1] {
			if byDay[d] < 360 {
				t.Fatalf("day %d longest gap %d min", d, byDay[d])
			}
		}
	})

	t.Run("the rhythm changes from day to day", func(t *testing.T) {
		w, _ := botWorld(t, 21, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed = 1, 1 })
		first := game.LocalDayIndex(w, 0)
		starts := map[float64]bool{}
		for i := range int64(14) {
			m := math.Inf(1)
			for _, s := range game.DaySessions(w, "bot_0", first+i) {
				m = math.Min(m, s.StartMin)
			}
			starts[m] = true
		}
		if len(starts) <= 7 {
			t.Fatalf("%d distinct first starts", len(starts))
		}
	})

	t.Run("phone: only offline and always before the hit", func(t *testing.T) {
		w, home := botWorld(t, 23, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays = 1, 1, 0 })
		bot := gametest.NearestBotVillage(w, home)
		brain := w.Brain(game.PlayerID(bot.OwnerID))
		home.Units[game.UAxe] = 500
		var attacks [][2]int64
		type phone struct {
			at     int64
			online bool
		}
		var phones []phone
		defer game.SpyEvents(func(key string) {
			if key == "phone:"+string(bot.OwnerID) {
				phones = append(phones, phone{w.Now(), brain.Online})
			}
		})()
		for at := int64(0); at < 7*game.MsPerDay; at += 50 * game.MsPerMinute {
			advance(t, w, at)
			game.Touch(w, home)
			c := mustSend(t, w, home, bot.X, bot.Y, game.UnitCounts{game.UAxe: 1}, "attack")
			attacks = append(attacks, [2]int64{c.DepartedAt, c.ArrivesAt})
		}
		advance(t, w, 8*game.MsPerDay)
		if len(phones) == 0 {
			t.Fatal("no phone checks")
		}
		for _, p := range phones {
			if p.online || !slices.ContainsFunc(attacks, func(a [2]int64) bool { return a[0] <= p.at && p.at <= a[1]-5*game.MsPerMinute }) {
				t.Fatalf("phone %+v", p)
			}
		}
	})

	t.Run("changing world speed does not shift bot sessions (wall clock)", func(t *testing.T) {
		log := func(speed float64) [][2]any {
			w, _ := botWorld(t, 22, monday, func(c *game.WorldConfig) { c.BotCount = 3 })
			w.Config().WorldSpeed = speed
			var out [][2]any
			restore := game.SpySessions(func(id game.PlayerID, at int64) { out = append(out, [2]any{id, at}) })
			advance(t, w, 3*game.MsPerDay)
			restore()
			return out
		}
		a, b := log(8), log(40)
		if len(a) <= 3 || !slices.Equal(a, b) {
			t.Fatalf("%d vs %d session starts", len(a), len(b))
		}
	})

	t.Run("after two empty days in a row the bot still comes back — it always has a next session", func(t *testing.T) {
		w, _ := botWorld(t, 2, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC).UnixMilli(), nil)
		for at := int64(0); at < 60*game.MsPerDay; at += game.MsPerHour {
			for _, b := range botsOf(w) {
				if _, ok := game.NextSessionStart(w, b.ID, at); !ok {
					t.Fatalf("%s stuck at day %d", b.ID, at/game.MsPerDay)
				}
			}
		}
	})

	t.Run("a weak player logs in less often than a strong one with the same daily rhythm", func(t *testing.T) {
		w, _ := botWorld(t, 3, monday, func(c *game.WorldConfig) { c.BotCount = 1 })
		count := func(skill float64) int {
			w.Brain("bot_0").Skill = skill
			n := 0
			for d := range int64(60) {
				n += len(game.DaySessions(w, "bot_0", d))
			}
			return n
		}
		if strong, weak := count(0.95), count(0.1); float64(weak) >= 0.85*float64(strong) {
			t.Fatalf("weak %d strong %d", weak, strong)
		}
	})

	t.Run("for the first days after joining a bot checks in every hour or two, then returns to its usual rhythm", func(t *testing.T) {
		w, _ := botWorld(t, 4, monday+6*game.MsPerHour, func(c *game.WorldConfig) { c.BotCount = 30 })
		day0 := game.LocalDayIndex(w, 0)
		count := func(d int64) int {
			n := 0
			for _, b := range botsOf(w) {
				n += len(game.DaySessions(w, b.ID, d))
			}
			return n
		}
		if float64(count(day0+1)) <= 2.5*float64(count(day0+10)) {
			t.Fatalf("day1 %d day10 %d", count(day0+1), count(day0+10))
		}
	})
}

func TestNoAiWorld(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	advance(t, w, 30*game.MsPerDay)
	if len(w.Commands()) != 0 || len(w.Villages()) != 1 || w.Villages()[0].ID != v.ID {
		t.Fatalf("commands %d villages %d", len(w.Commands()), len(w.Villages()))
	}
}

// warWorld has two bots without protection; the attacker has an army and 5 nobles, the defender quit.
func warWorld(t *testing.T, skill float64, lastVillage string) (*gametest.MemWorld, game.PlayerID, *game.Village) {
	t.Helper()
	w, _ := botWorld(t, 81, 0, func(c *game.WorldConfig) {
		c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays, c.LastVillage = 2, 1, 0, lastVillage
	})
	a, d := "bot_0", "bot_1"
	w.Brain(a).Skill = skill
	w.Player(d).Status = game.StatusQuit
	v := game.PlayerVillages(w, a)[0]
	v.Units[game.UAxe], v.Units[game.ULightCav], v.Units[game.URam] = 6000, 500, 100
	v.Units[game.UCatapult], v.Units[game.UNoble], v.Units[game.UScout] = 100, 5, 20
	b := &v.Buildings
	b[game.BAcademy], b[game.BFarm], b[game.BWorkshop], b[game.BStable], b[game.BBarracks] = 3, 30, 10, 10, 20
	b[game.BSmithy], b[game.BMain], b[game.BMarket] = 20, 20, 10
	game.RecomputePoints(v)
	return w, a, game.PlayerVillages(w, d)[0]
}

func knownOf(w *gametest.MemWorld, bot game.PlayerID, v *game.Village) *game.KnownVillage {
	k := &game.KnownVillage{ID: v.ID, X: v.X, Y: v.Y, OwnerID: v.OwnerID, Points: v.Points, SeenAt: w.Now(), Hauls: []float64{},
		Units: &game.UnitCounts{}, Wall: ptr(0)}
	w.Brain(bot).Memory.Known[v.ID] = k
	return k
}

func scheduledAts(w *gametest.MemWorld, owner game.PlayerID) []int64 {
	var out []int64
	for _, s := range w.ScheduledSendsOf(owner) {
		out = append(out, s.At)
	}
	slices.Sort(out)
	return out
}

func TestBotAttacks(t *testing.T) {
	t.Run("a strong bot's train: a clear and 4 nobles within a second, in order", func(t *testing.T) {
		w, a, target := warWorld(t, 0.95, "canBeLost")
		game.LaunchConquest(w, a, knownOf(w, a, target))
		ats := scheduledAts(w, a)
		if len(ats) != 5 || ats[4]-ats[1] >= 1000 {
			t.Fatalf("ats %v", ats)
		}
		advance(t, w, ats[4]+1)
		if target.OwnerID != game.Ref(a) {
			t.Fatalf("owner %q", target.OwnerID)
		}
	})

	t.Run("fakes: four within a second", func(t *testing.T) {
		w, a, target := warWorld(t, 0.95, "canBeLost")
		game.LaunchHarass(w, a, knownOf(w, a, target))
		ats := scheduledAts(w, a)
		if len(ats) != 4 || ats[3]-ats[0] > 1000 {
			t.Fatalf("ats %v", ats)
		}
		for _, s := range w.ScheduledSendsOf(a) {
			if s.Units.Total() != 1 {
				t.Fatalf("fake %+v", s.Units)
			}
		}
	})

	t.Run("razing the farmstead: catapult count by level", func(t *testing.T) {
		w, a, target := warWorld(t, 0.95, "canBeLost")
		k := knownOf(w, a, target)
		k.Buildings = game.PartialBuildings{game.BFarm: 3}
		game.LaunchDemolish(w, a, k, game.BFarm)
		var s *game.ScheduledSend
		for _, x := range w.ScheduledSendsOf(a) {
			if x.Units[game.UCatapult] > 0 {
				s = x
			}
		}
		if s == nil || s.Units[game.UCatapult] < 6 || s.CatapultTarget == nil || *s.CatapultTarget != game.BFarm {
			t.Fatalf("demolish %+v", s)
		}
	})

	purposes := func(w *gametest.MemWorld, a game.PlayerID, k *game.KnownVillage) map[game.Purpose]bool {
		out := map[game.Purpose]bool{}
		for range 200 {
			w.SetNow(w.Now() + 1)
			out[game.ChoosePurpose(w, a, k)] = true
		}
		return out
	}

	t.Run("the last village of a player who left can be taken despite protection", func(t *testing.T) {
		w, a, target := warWorld(t, 0.95, "protected")
		if !purposes(w, a, knownOf(w, a, target))["conquer"] {
			t.Fatal("never conquer")
		}
	})

	t.Run("does not take a protected last village", func(t *testing.T) {
		w, a, target := warWorld(t, 0.95, "protected")
		k := knownOf(w, a, target)
		w.Player(game.PlayerID(k.OwnerID)).Status = game.StatusActive
		if purposes(w, a, k)["conquer"] {
			t.Fatal("conquer on a protected last village")
		}
	})

	t.Run("a human can lose a village when the settings allow it", func(t *testing.T) {
		w, home := botWorld(t, 82, 0, func(c *game.WorldConfig) {
			c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays, c.LastVillage, c.BotStance = 1, 1, 0, "canBeLost", game.StanceAllAgainstHumans
		})
		b := w.Brain("bot_0")
		b.Skill, b.Style = 0.95, game.StyleAggressor
		bv := game.PlayerVillages(w, "bot_0")[0]
		bv.Units[game.UAxe], bv.Units[game.ULightCav], bv.Units[game.UNoble], bv.Units[game.UScout] = 6000, 500, 5, 20
		bb := &bv.Buildings
		bb[game.BAcademy], bb[game.BFarm], bb[game.BBarracks], bb[game.BStable], bb[game.BSmithy], bb[game.BMain], bb[game.BMarket] = 3, 30, 20, 10, 20, 20, 10
		game.RecomputePoints(bv)
		empty := game.CreateVillage(bv.X+2, bv.Y+1, "Empty", "", w.Now())
		empty.Buildings = home.Buildings
		game.RecomputePoints(empty)
		w.PutVillage(empty)
		game.SetOwner(w, empty, "h1")
		advance(t, w, 10*game.MsPerDay)
		if empty.OwnerID != "bot_0" && home.OwnerID != "bot_0" {
			t.Fatal("bot took nothing")
		}
	})
}

// defenceWorld has a human and one strong online bot next to each other, no protection.
func defenceWorld(t *testing.T) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, home := botWorld(t, 91, 0, func(c *game.WorldConfig) {
		c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays, c.LastVillage, c.BotStance = 1, 1, 0, "canBeLost", game.StancePeacefulRetaliate
	})
	b := w.Brain("bot_0")
	b.Skill, b.Online, b.SessionEndsAt = 0.95, true, w.Now()+100*game.MsPerHour
	bv := game.PlayerVillages(w, "bot_0")[0]
	bv.Units[game.USpear] = 300
	return w, home, bv
}

func addBotVillage(w *gametest.MemWorld, near *game.Village, dx, dy int, units game.Units) *game.Village {
	v := game.CreateVillage(near.X+dx, near.Y+dy, "Bot's second", "", w.Now())
	v.Buildings = near.Buildings
	v.Buildings[game.BFarm] = 20
	game.RecomputePoints(v)
	w.PutVillage(v)
	game.SetOwner(w, v, near.OwnerID)
	v.Units = units
	return v
}

func TestBotDefence(t *testing.T) {
	t.Run("a strong bot dodges an unstoppable wave", func(t *testing.T) {
		w, home, bv := defenceWorld(t)
		home.Units[game.UAxe] = 5000
		c := mustSend(t, w, home, bv.X, bv.Y, game.UnitCounts{game.UAxe: 5000}, "attack")
		game.Think(w, "bot_0", true)
		advanceTouch(t, w, bv, c.ArrivesAt+5000)
		if bv.Units[game.USpear] != 300 {
			t.Fatalf("spear %d", bv.Units[game.USpear])
		}
	})

	t.Run("a strong bot stacks against a wave it can stop", func(t *testing.T) {
		w, home, bv := defenceWorld(t)
		bv.Units[game.USpear] = 100
		def := addBotVillage(w, bv, 0, 2, game.Units{game.USpear: 3000})
		w.Brain("bot_0").Roles[def.ID] = game.RoleDef
		home.Units[game.UAxe] = 20
		c := mustSend(t, w, home, bv.X, bv.Y, game.UnitCounts{game.UAxe: 20}, "attack")
		game.Think(w, "bot_0", true)
		var support *game.Command
		for _, x := range w.Commands() {
			if x.Type == "support" && x.TargetID == bv.ID && x.OriginID == def.ID {
				support = x
			}
		}
		if support == nil || support.ArrivesAt >= c.ArrivesAt {
			t.Fatalf("support %+v", support)
		}
	})

	t.Run("a strong bot snipes a hetman train", func(t *testing.T) {
		w, home, bv := defenceWorld(t)
		addBotVillage(w, bv, 1, 1, game.Units{game.USpear: 3000})
		home.Units[game.UAxe], home.Units[game.UNoble] = 3000, 3
		home.Buildings[game.BAcademy] = 3
		w.Player("h1").Coins = 100
		T := w.Now() + 6*game.MsPerHour
		mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: home.ID, X: bv.X, Y: bv.Y, Units: game.UnitCounts{game.UAxe: 2700}, Type: "attack", Mode: "arriveAt", At: T})
		for i := int64(1); i <= 3; i++ {
			mustSchedule(t, w, "h1", game.ScheduleSendInput{OriginID: home.ID, X: bv.X, Y: bv.Y, Units: game.UnitCounts{game.UNoble: 1, game.UAxe: 100}, Type: "attack", Mode: "arriveAt", At: T + i*100})
		}
		last := int64(0)
		for _, s := range w.ScheduledSendsOf("h1") {
			last = max(last, s.SendAt)
		}
		advance(t, w, last+1)
		game.Think(w, "bot_0", true)
		advance(t, w, T+1000)
		if bv.OwnerID != "bot_0" || bv.Loyalty < 100-35 {
			t.Fatalf("owner %q loyalty %v", bv.OwnerID, bv.Loyalty)
		}
	})

	// The first seed from 92 on where the bot sleeps through the hit.
	t.Run("a sleeping bot without a phone does not react", func(t *testing.T) {
		for seed := uint32(92); seed < 160; seed++ {
			w, home := botWorld(t, seed, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays = 1, 1, 0 })
			w.Brain("bot_0").Caution = 0
			bv := game.PlayerVillages(w, "bot_0")[0]
			bv.Units[game.USpear] = 300
			home.Units[game.UAxe] = 5000
			c := mustSend(t, w, home, bv.X, bv.Y, game.UnitCounts{game.UAxe: 5000}, "attack")
			w.Unschedule("phone:bot_0")
			if next, ok := game.NextSessionStart(w, "bot_0", w.Now()); w.Brain("bot_0").Online || !ok || next <= c.ArrivesAt {
				continue
			}
			advance(t, w, c.ArrivesAt+1000)
			if len(w.ScheduledSendsOf("bot_0")) != 0 || lastReport(w, "h1").Defender.Units[game.USpear] != 300 {
				t.Fatal("sleeping bot reacted")
			}
			return
		}
		t.Fatal("no seed with the bot asleep through the hit")
	})

	t.Run("300 fakes on one village: defence planning < 50 ms", func(t *testing.T) {
		w, home, bv := defenceWorld(t)
		home.Units[game.UAxe] = 300
		for range 300 {
			mustSend(t, w, home, bv.X, bv.Y, game.UnitCounts{game.UAxe: 1}, "attack")
		}
		game.PlanDefence(w, "bot_0")
		start := time.Now()
		game.PlanDefence(w, "bot_0")
		if dt := time.Since(start); dt > 50*time.Millisecond {
			t.Fatalf("plan defence %v", dt)
		}
	})
}

func TestBotStrategy(t *testing.T) {
	strategyWorld := func(t *testing.T, stance game.BotStance) (*gametest.MemWorld, game.PlayerID, []game.PlayerID) {
		w, _ := botWorld(t, 71, 0, func(c *game.WorldConfig) {
			c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays, c.BotStance = 3, 1, 0, stance
		})
		game.PlayerVillages(w, "bot_0")[0].Units[game.UAxe] = 1000
		return w, "bot_0", []game.PlayerID{"bot_1", "bot_2"}
	}

	t.Run("never picks a target whose known defence it cannot break", func(t *testing.T) {
		w, bot, others := strategyWorld(t, game.StanceNormal)
		a := w.Village(w.Player(others[0]).VillageIDs[0])
		b := w.Village(w.Player(others[1]).VillageIDs[0])
		ka := knownOf(w, bot, a)
		ka.Units = &game.UnitCounts{game.USpear: 5000}
		knownOf(w, bot, b)
		brain := w.Brain(bot)
		picks := map[string]bool{}
		for range 200 {
			brain.Mood = game.Mood{Phase: "campaign", Until: w.Now() + game.MsPerDay}
			w.SetNow(w.Now() + 1)
			game.PickCampaignTarget(w, bot)
			picks[brain.Mood.TargetVillage] = true
		}
		if picks[a.ID] || !picks[b.ID] {
			t.Fatalf("picks %v", picks)
		}
	})

	t.Run("prefers a big inactive village to a small active one", func(t *testing.T) {
		w, bot, others := strategyWorld(t, game.StanceNormal)
		v := game.PlayerVillages(w, bot)[0]
		a, b := w.Player(others[0]), w.Player(others[1])
		a.History.Hourly = slices.Repeat([]int{900}, 48)
		b.History.Hourly = nil
		for i := range 48 {
			b.History.Hourly = append(b.History.Hourly, 100+i)
		}
		big := &game.KnownVillage{ID: "v:1:1", X: v.X + 5, Y: v.Y, OwnerID: game.Ref(a.ID), Points: 900, Units: &game.UnitCounts{}, Hauls: []float64{}}
		small := &game.KnownVillage{ID: "v:1:2", X: v.X - 5, Y: v.Y, OwnerID: game.Ref(b.ID), Points: 150, Units: &game.UnitCounts{}, Hauls: []float64{}}
		if game.ScoreTarget(w, bot, big) <= game.ScoreTarget(w, bot, small) {
			t.Fatal("small active preferred")
		}
	})

	t.Run("protected targets and those botStance forbids score 0", func(t *testing.T) {
		w, bot, others := strategyWorld(t, game.StancePeacefulRetaliate)
		a := w.Player(others[0])
		a.ProtectedUntil = w.Now() + game.MsPerDay
		prot := knownOf(w, bot, w.Village(a.VillageIDs[0]))
		human := knownOf(w, bot, w.Village(w.Player("h1").VillageIDs[0]))
		if game.ScoreTarget(w, bot, prot) != 0 || game.ScoreTarget(w, bot, human) != 0 {
			t.Fatal("non-zero score")
		}
	})

	t.Run("moods change over time", func(t *testing.T) {
		w, _ := botWorld(t, 72, 0, func(c *game.WorldConfig) { c.BotCount = 1 })
		w.Brain("bot_0").Style = game.StyleAggressor
		var phases []string
		for at := int64(0); at <= 20*game.MsPerDay; at += 6 * game.MsPerHour {
			advance(t, w, at)
			game.UpdateMood(w, "bot_0")
			if p := w.Brain("bot_0").Mood.Phase; len(phases) == 0 || phases[len(phases)-1] != p {
				phases = append(phases, p)
			}
		}
		if len(phases) < 4 || !slices.Contains(phases, "build") || !slices.Contains(phases, "campaign") {
			t.Fatalf("phases %v", phases)
		}
	})
}

func TestBotEconomySessions(t *testing.T) {
	twinWorld := func(t *testing.T, skillA, skillB float64, seed uint32) *gametest.MemWorld {
		w, _ := botWorld(t, seed, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays = 2, 8, 0 })
		a, b := w.Brain("bot_0"), w.Brain("bot_1")
		a.Skill, b.Skill = skillA, skillB
		a.Style, b.Style = game.StyleFarmer, game.StyleFarmer
		b.Routine = a.Routine
		b.Routine.CheckIns = slices.Clone(a.Routine.CheckIns)
		b.Caution = a.Caution
		return w
	}

	t.Run("a strong bot grows faster than a weak one under the same conditions", func(t *testing.T) {
		var ratios []float64
		for seed := uint32(31); seed <= 35; seed++ {
			w := twinWorld(t, 0.95, 0.1, seed)
			advance(t, w, 10*game.MsPerDay)
			r := bySkillDesc(w)
			ratios = append(ratios, float64(game.PlayerPoints(w, r[0].ID))/float64(game.PlayerPoints(w, r[1].ID)))
		}
		slices.Sort(ratios)
		if ratios[2] <= 1.3 {
			t.Fatalf("median ratio %v (%v)", ratios[2], ratios)
		}
	})

	t.Run("before sleep a strong bot has queues for the whole night (or as much as is affordable) and few resources in the storehouse", func(t *testing.T) {
		w := twinWorld(t, 0.95, 0.95, 31)
		total, ok := 0, 0
		restore := game.SpyOffline(func(botID game.PlayerID, nextSessionAt int64) {
			end, fill, exhausted := w.Now(), 0.0, true
			for _, v := range game.PlayerVillages(w, botID) {
				if game.CanSpendMore(w, botID, v) {
					exhausted = false
				}
				for _, j := range v.BuildQueue {
					end = max(end, j.CompletesAt)
				}
				for _, b := range game.RecruitBuildings {
					for _, j := range *v.RecruitQueues.Q(b) {
						end = max(end, j.CompletesAt)
					}
				}
				capacity := game.VillageStorageCapacity(v)
				fill = math.Max(fill, math.Max(v.Resources.Wood, math.Max(v.Resources.Clay, v.Resources.Iron))/capacity)
			}
			total++
			if (float64(end-w.Now()) >= 0.8*float64(nextSessionAt-w.Now()) || exhausted) && (fill <= 0.5 || exhausted) {
				ok++
			}
		})
		advance(t, w, 7*game.MsPerDay)
		restore()
		if total <= 10 || float64(ok)/float64(total) <= 0.8 {
			t.Fatalf("%d/%d session ends prepared", ok, total)
		}
	})
}

func TestBotFarmingSessions(t *testing.T) {
	farmingWorld := func(t *testing.T, skill float64) *gametest.MemWorld {
		w, _ := botWorld(t, 41, monday, func(c *game.WorldConfig) {
			c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays, c.BotStance = 1, 8, 0, game.StancePeacefulRetaliate
		})
		w.Brain("bot_0").Skill = skill
		farmingVillage(game.PlayerVillages(w, "bot_0")[0])
		return w
	}

	t.Run("a strong bot farms barbarians with a known wall of 0 without losses", func(t *testing.T) {
		w := farmingWorld(t, 0.95)
		var log []game.BotBattle
		restore := game.SpyBotBattles(func(b game.BotBattle) { log = append(log, b) })
		advance(t, w, 3*game.MsPerDay)
		restore()
		if len(log) <= 50 {
			t.Fatalf("%d battles", len(log))
		}
		for _, b := range log {
			if b.TargetOwner == "" && b.WallKnown != nil && *b.WallKnown == 0 && b.AttackerLost != 0 {
				t.Fatalf("lost %d units on a known open barbarian", b.AttackerLost)
			}
		}
	})

	t.Run("wall > 0 without rams is skipped; a poor farm is struck off (in sessions)", func(t *testing.T) {
		w := farmingWorld(t, 0.95)
		home := game.PlayerVillages(w, "bot_0")[0]
		near := game.GeneratedVillagesNear(w.Seed(), w.Config(), home.X, home.Y, 6)
		walled := game.EnsureVillage(w, near[0].X, near[0].Y)
		walled.Buildings[game.BWall] = 2
		game.ObserveBattle(w, "bot_0", walled, game.BattleSeen{Units: &game.UnitCounts{}, Wall: ptr(2)})
		dry := game.EnsureVillage(w, near[1].X, near[1].Y)
		for range 3 {
			game.ObserveBattle(w, "bot_0", dry, game.BattleSeen{Units: &game.UnitCounts{}, Wall: ptr(0), HaulRatio: ptr(0.1)})
		}
		var targets []string
		restore := game.SpyRaids(func(id game.PlayerID, c *game.Command, night bool, next int64) { targets = append(targets, c.TargetID) })
		advance(t, w, 2*game.MsPerDay)
		restore()
		if len(targets) <= 20 || slices.Contains(targets, walled.ID) || slices.Contains(targets, dry.ID) {
			t.Fatalf("%d raids", len(targets))
		}
	})

	t.Run("night raids return around the next session", func(t *testing.T) {
		w := farmingWorld(t, 0.95)
		total, onTime := 0, 0
		restore := game.SpyRaids(func(id game.PlayerID, c *game.Command, night bool, next int64) {
			if night && next != 0 {
				total++
				if abs64(c.ArrivesAt+(c.ArrivesAt-c.DepartedAt)-next) <= 60*game.MsPerMinute {
					onTime++
				}
			}
		})
		advance(t, w, 5*game.MsPerDay)
		restore()
		if total <= 5 || float64(onTime)/float64(total) < 0.8 {
			t.Fatalf("%d/%d on time", onTime, total)
		}
	})

	t.Run("bot fights with barbarians create no player reports", func(t *testing.T) {
		w := farmingWorld(t, 0.9)
		n := 0
		restore := game.SpyBotBattles(func(game.BotBattle) { n++ })
		advance(t, w, game.MsPerDay)
		restore()
		if n == 0 || len(w.ReportsOf("h1")) != 0 {
			t.Fatalf("battles %d reports %d", n, len(w.ReportsOf("h1")))
		}
	})
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func TestBotExpansionAndQuitting(t *testing.T) {
	t.Run("a bot with a solid base builds an Academy, mints coins and takes a barbarian", func(t *testing.T) {
		w, _ := botWorld(t, 31, monday, func(c *game.WorldConfig) {
			c.BotCount = 1
			c.BotModifiers = game.SideModifiers{Resources: 10, Recruitment: 10, Building: 10}
		})
		w.Brain("bot_0").Skill = 0.95
		advance(t, w, 20*game.MsPerDay)
		p := w.Player("bot_0")
		if len(p.VillageIDs) < 2 || p.Coins+p.NoblesTrained == 0 {
			t.Fatalf("villages %d coins %d nobles %d", len(p.VillageIDs), p.Coins, p.NoblesTrained)
		}
	})

	t.Run("nobles also go for scouted grey villages of players who left — not just barbarians", func(t *testing.T) {
		w, _ := botWorld(t, 34, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 2, 0 })
		bot, gone := game.PlayerID("bot_0"), game.PlayerID("bot_1")
		home := game.PlayerVillages(w, bot)[0]
		home.Buildings[game.BAcademy] = 3
		home.Units[game.UNoble], home.Units[game.UAxe] = 4, 3000
		var tile *game.Tile
		for _, t2 := range game.VillagesInRadius(w, home.X, home.Y, 3) {
			if !(t2.X == home.X && t2.Y == home.Y) && game.Distance(home.X, home.Y, t2.X, t2.Y) <= 3 && t2.OwnerID == "" {
				tile = &t2
				break
			}
		}
		if tile == nil {
			t.Fatal("no barbarian within 3")
		}
		grey := game.EnsureVillage(w, tile.X, tile.Y)
		game.TransferVillage(w, grey, game.Ref(gone), "")
		grey.Points = 5000
		game.QuitPlayer(w, gone)
		knownOf(w, bot, grey)
		game.PlanExpansion(w, bot)
		if !slices.ContainsFunc(w.ScheduledSendsOf(bot), func(s *game.ScheduledSend) bool {
			return s.Units[game.UNoble] > 0 && s.X == grey.X && s.Y == grey.Y
		}) {
			t.Fatal("no hetman train on the grey village")
		}
	})

	t.Run("a weak bot under constant pressure quits, a strong one stays", func(t *testing.T) {
		w, home := botWorld(t, 32, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 2, 0 })
		weak, strong := w.Brain("bot_0"), w.Brain("bot_1")
		weak.Skill, weak.Resilience = 0.1, 0.3
		strong.Skill, strong.Resilience = 0.95, 0.85
		home.Units[game.UAxe] = 2000
		for at := int64(0); at < 10*game.MsPerDay; at += 30 * game.MsPerMinute {
			advance(t, w, at)
			game.Touch(w, home)
			for _, id := range []game.PlayerID{"bot_0", "bot_1"} {
				p := w.Player(id)
				if len(p.VillageIDs) == 0 {
					continue
				}
				if v := w.Village(p.VillageIDs[0]); v != nil && v.OwnerID == game.Ref(id) && home.Units[game.UAxe] > 0 {
					mustSend(t, w, home, v.X, v.Y, game.UnitCounts{game.UAxe: 1}, "attack")
				}
			}
		}
		if w.Player("bot_0").Status != game.StatusQuit || w.Player("bot_1").Status != game.StatusActive {
			t.Fatalf("weak %s strong %s", w.Player("bot_0").Status, w.Player("bot_1").Status)
		}
	})

	t.Run("abandoned villages turn barbarian after BOT.abandonAfterDays days and the bot does nothing more", func(t *testing.T) {
		w, _ := botWorld(t, 33, monday, func(c *game.WorldConfig) { c.BotCount = 1 })
		advance(t, w, game.MsPerDay)
		villages := game.PlayerVillages(w, "bot_0")
		game.QuitPlayer(w, "bot_0")
		brain := w.Brain("bot_0")
		actions := len(brain.ActionLog)
		advance(t, w, w.Now()+int64(game.BotConfig.AbandonAfterDays-1)*game.MsPerDay)
		for _, v := range villages {
			if v.OwnerID != "bot_0" {
				t.Fatal("abandoned too early")
			}
		}
		var buildings []game.Buildings
		for _, v := range villages {
			buildings = append(buildings, v.Buildings)
		}
		advance(t, w, w.Now()+2*game.MsPerDay)
		if w.Player("bot_0").Status != game.StatusQuit || len(brain.ActionLog) != actions {
			t.Fatal("quit bot acted")
		}
		for i, v := range villages {
			if v.OwnerID != "" || v.Buildings != buildings[i] {
				t.Fatalf("village %s owner %q", v.ID, v.OwnerID)
			}
		}
	})
}

func TestQuitPlayerHuman(t *testing.T) {
	w, v := gametest.NewTestWorld(5)
	game.QuitPlayer(w, "h1")
	if w.Player("h1").Status != game.StatusQuit || w.Player("h1").QuitAt == nil {
		t.Fatal("not quit")
	}
	advance(t, w, w.Now()+21*game.MsPerDay+1)
	if v.OwnerID != "" || v.Name != "" {
		t.Fatalf("owner %q name %q", v.OwnerID, v.Name)
	}
}

func TestBotAdaptationSessions(t *testing.T) {
	t.Run("a steady dodge rhythm → the attack lands right after the return", func(t *testing.T) {
		for seed := uint32(103); seed < 140; seed++ {
			w, home := botWorld(t, seed, monday, func(c *game.WorldConfig) { c.BotCount, c.WorldSpeed, c.BeginnerProtectionDays = 1, 1, 0 })
			w.Brain("bot_0").Skill = 0.95
			game.PlayerVillages(w, "bot_0")[0].Units[game.UAxe] = 100
			for _, dm := range [][2]int64{{0, 6*60 + 50}, {1, 7 * 60}, {2, 7*60 + 10}} {
				game.RecordHabit(w, "bot_0", "h1", game.HabitObservation{DodgeReturnAt: ptr(dm[0]*game.MsPerDay + dm[1]*game.MsPerMinute)})
			}
			if _, learnt := game.DodgeMinute(w, "bot_0", "h1"); !learnt {
				continue // seeded draw: this bot did not notice
			}
			w.SetNow(3*game.MsPerDay + 12*game.MsPerHour)
			game.LaunchHarass(w, "bot_0", &game.KnownVillage{ID: home.ID, X: home.X, Y: home.Y, OwnerID: "h1", Points: home.Points, SeenAt: w.Now(), Hauls: []float64{}})
			ats := scheduledAts(w, "bot_0")
			m := game.LocalMinuteOfDay(w, ats[0])
			if m < 7*60+2 || m > 7*60+20 {
				t.Fatalf("lands at %d:%02d", m/60, m%60)
			}
			return
		}
		t.Fatal("no seed where the bot noticed the dodge rhythm")
	})

	// With equal hauls the farm order is best haul per distance, so the day's first targets need not vary; the
	// send hour is what players can see.
	t.Run("one bot over three weeks: the day's first attack at varying hours", func(t *testing.T) {
		w, _ := botWorld(t, 211, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 20, 0 })
		bot := bySkillDesc(w)[0].ID
		seen := map[string]bool{}
		firstHour := map[int64]int{}
		for at := 10 * game.MsPerMinute; at <= 21*game.MsPerDay; at += 10 * game.MsPerMinute {
			advance(t, w, at)
			for _, c := range w.Commands() {
				if c.Type != "attack" || seen[c.ID] || !slices.Contains(w.Player(bot).VillageIDs, c.OriginID) {
					continue
				}
				seen[c.ID] = true
				d := game.LocalDayIndex(w, c.DepartedAt)
				h := game.LocalMinuteOfDay(w, c.DepartedAt) / 60
				if cur, ok := firstHour[d]; !ok || h < cur {
					firstHour[d] = h
				}
			}
		}
		hours := map[int]bool{}
		for _, h := range firstHour {
			hours[h] = true
		}
		if len(firstHour) < 10 || len(hours) < 4 {
			t.Fatalf("days %d hours %d", len(firstHour), len(hours))
		}
	})

	t.Run("the same bot over 20 seeds has no fixed send hour", func(t *testing.T) {
		var hours []int
		for seed := uint32(201); seed <= 220; seed++ {
			w, _ := botWorld(t, seed, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 3, 0 })
			best := bySkillDesc(w)[0].ID
			seen := map[string]bool{}
			var mine []int
			for at := game.MsPerHour; at <= 6*game.MsPerDay && len(mine) < 10; at += game.MsPerHour {
				advance(t, w, at)
				for _, c := range w.Commands() {
					if c.Type != "attack" || seen[c.ID] || !slices.Contains(w.Player(best).VillageIDs, c.OriginID) {
						continue
					}
					seen[c.ID] = true
					if len(mine) < 10 {
						mine = append(mine, game.LocalMinuteOfDay(w, c.DepartedAt)/60)
					}
				}
			}
			hours = append(hours, mine...)
		}
		counts := map[int]int{}
		top := 0
		for _, h := range hours {
			counts[h]++
			top = max(top, counts[h])
		}
		if len(hours) <= 100 || len(counts) < 6 || float64(top)/float64(len(hours)) >= 0.35 {
			t.Fatalf("%d sends over %d hours, top share %v", len(hours), len(counts), float64(top)/float64(len(hours)))
		}
	})

	t.Run("the same seed gives the same decisions", func(t *testing.T) {
		run := func() []string {
			w, _ := botWorld(t, 103, monday, func(c *game.WorldConfig) { c.BotCount = 10 })
			advance(t, w, 10*game.MsPerDay)
			var out []string
			for _, c := range w.Commands() {
				out = append(out, c.ID+c.OriginID+c.TargetID+itoa(int(c.ArrivesAt)))
			}
			slices.Sort(out)
			return out
		}
		if a, b := run(), run(); !slices.Equal(a, b) || len(a) == 0 {
			t.Fatalf("%d vs %d commands", len(a), len(b))
		}
	})
}
