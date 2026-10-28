package game_test

import (
	"fmt"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestBotsDoNotShareConquestTargets(t *testing.T) {
	failOnBotErrors(t)
	w, _ := newInstant(t, 5)
	seen := 0
	for now := int64(10_000); now <= 20*game.MsPerMinute; now += 10_000 {
		advance(t, w, now)
		bots := map[string]map[game.PlayerID]bool{}
		note := func(bot game.PlayerID, x, y int) {
			key := fmt.Sprintf("%d:%d", x, y)
			if bots[key] == nil {
				bots[key] = map[game.PlayerID]bool{}
			}
			bots[key][bot] = true
			seen++
		}
		for _, p := range botsOf(w) {
			for _, s := range w.ScheduledSendsOf(p.ID) {
				if s.Units[game.UNoble] > 0 {
					note(p.ID, s.X, s.Y)
				}
			}
		}
		for _, c := range w.Commands() {
			from := w.Village(c.OriginID)
			if c.Units[game.UNoble] > 0 && from != nil && !game.IsHumanID(game.PlayerID(from.OwnerID)) {
				if to := w.Village(c.TargetID); to != nil {
					note(game.PlayerID(from.OwnerID), to.X, to.Y)
				}
			}
		}
		for key, who := range bots {
			if len(who) > 1 {
				t.Fatalf("at %d s, %d bots send hetmans at %s", now/1000, len(who), key)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no bot sent a hetman in 20 minutes; the check proves nothing")
	}
}

func TestBotConquestNukeHasAxes(t *testing.T) {
	w, a, target := warWorld(t, 0.95, "canBeLost")
	v := game.PlayerVillages(w, a)[0]
	v.Units = game.Units{}
	v.Units[game.UAxe], v.Units[game.ULightCav], v.Units[game.UNoble] = 100, 30, 1
	game.LaunchConquest(w, a, knownOf(w, a, target))
	var nukeAxes, escortAxes int
	for _, s := range w.ScheduledSendsOf(a) {
		if s.Units[game.UNoble] > 0 {
			escortAxes += s.Units[game.UAxe]
		} else {
			nukeAxes += s.Units[game.UAxe]
		}
	}
	if nukeAxes == 0 || escortAxes == 0 {
		t.Fatalf("nuke axes %d, escort axes %d", nukeAxes, escortAxes)
	}
	if nukeAxes+escortAxes > 100 {
		t.Fatalf("sends %d axes out of 100", nukeAxes+escortAxes)
	}
}

func TestBotSkipsUnscoutedOwnedTarget(t *testing.T) {
	w, _ := botWorld(t, 34, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 2, 0 })
	bot, gone := game.PlayerID("bot_0"), game.PlayerID("bot_1")
	home := game.PlayerVillages(w, bot)[0]
	home.Buildings[game.BAcademy] = 3
	home.Units[game.UNoble], home.Units[game.UAxe] = 3, 3000
	var tile *game.Tile
	for _, t2 := range game.VillagesInRadius(w, home.X, home.Y, 3) {
		if !(t2.X == home.X && t2.Y == home.Y) && t2.OwnerID == "" {
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
	grey.Units[game.USpear] = 5000
	game.QuitPlayer(w, gone)
	home.Units[game.UScout] = 20
	game.PlanExpansion(w, bot)
	if slices.ContainsFunc(w.ScheduledSendsOf(bot), func(s *game.ScheduledSend) bool { return s.Units[game.UNoble] > 0 }) {
		t.Fatal("a hetman train went out at an unscouted village")
	}
	if !slices.ContainsFunc(w.ScheduledSendsOf(bot), func(s *game.ScheduledSend) bool { return s.Units[game.UScout] > 0 }) {
		t.Fatal("the bot did not send scouts first")
	}
}

func TestBotSendsNoblesFromSeveralVillagesAtAScoutedGreyVillage(t *testing.T) {
	w, _ := botWorld(t, 34, monday, func(c *game.WorldConfig) { c.BotCount, c.BeginnerProtectionDays = 2, 0 })
	bot, gone := game.PlayerID("bot_0"), game.PlayerID("bot_1")
	home := game.PlayerVillages(w, bot)[0]
	home.Buildings[game.BAcademy] = 3
	home.Units[game.UNoble], home.Units[game.UAxe] = 3, 3000
	var tile *game.Tile
	for _, t2 := range game.VillagesInRadius(w, home.X, home.Y, 3) {
		if !(t2.X == home.X && t2.Y == home.Y) && t2.OwnerID == "" {
			tile = &t2
			break
		}
	}
	if tile == nil {
		t.Fatal("no barbarian within 3")
	}
	second := addBotVillage(w, home, 1, 0, game.Units{})
	second.Buildings[game.BAcademy] = 1
	second.Units[game.UNoble] = 1
	grey := game.EnsureVillage(w, tile.X, tile.Y)
	game.TransferVillage(w, grey, game.Ref(gone), "")
	grey.Points = 5000
	game.QuitPlayer(w, gone)
	knownOf(w, bot, grey)
	game.PlanExpansion(w, bot)
	if !slices.ContainsFunc(w.ScheduledSendsOf(bot), func(s *game.ScheduledSend) bool { return s.Units[game.UNoble] > 0 }) {
		t.Fatal("four hetmans in two villages were not sent at a scouted, undefended grey village")
	}
}

func TestBotRecruitsNobleWithoutCoins(t *testing.T) {
	w, _ := botWorld(t, 31, monday, func(c *game.WorldConfig) {
		c.BotCount = 1
		c.NoCoins = true
		c.NobleCost = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	})
	home := game.PlayerVillages(w, "bot_0")[0]
	home.Buildings[game.BAcademy] = 1
	home.Buildings[game.BStorage] = 20
	home.Units[game.UNoble] = 0
	home.Resources = game.Resources{Wood: 50_000, Clay: 50_000, Iron: 50_000}
	game.PlanExpansion(w, "bot_0")
	if len(home.RecruitQueues.Academy) != 1 || home.RecruitQueues.Academy[0].Unit != game.UNoble {
		t.Fatalf("academy queue %+v", home.RecruitQueues.Academy)
	}
	if w.Player("bot_0").Coins != 0 {
		t.Fatalf("coins %d", w.Player("bot_0").Coins)
	}
}

func TestInstantBotLeavesConsolidate(t *testing.T) {
	w, _ := newInstant(t, 5)
	id := botsOf(w)[0].ID
	b := w.Brain(id)
	b.LastSessionStats.VillagesLost = 1
	game.UpdateMood(w, id)
	if b.Mood.Phase != "consolidate" {
		t.Fatalf("phase %q", b.Mood.Phase)
	}
	advance(t, w, b.Mood.Until+1)
	game.UpdateMood(w, id)
	if b.Mood.Phase == "consolidate" {
		t.Fatal("still consolidating after the mood ran out")
	}
}

func TestInstantBotSendsLandBeforeTheEnd(t *testing.T) {
	run, err := gametest.RunInstantPlayer(gametest.InstantPlayerOptions{Seed: 1, IdleHumans: 2})
	if err != nil {
		t.Fatal(err)
	}
	if run.BotSendsAfterEnd != 0 {
		t.Fatalf("%d bot sends are planned after the bell", run.BotSendsAfterEnd)
	}
}

func TestInstantQuitPlayerAbandonsWithinTheGame(t *testing.T) {
	w, _ := newInstant(t, 5)
	game.QuitPlayer(w, "bot_0")
	end := int64(game.InstantWorldConfig.DurationMin) * game.MsPerMinute
	advance(t, w, end)
	for _, v := range game.PlayerVillages(w, "bot_0") {
		t.Fatalf("village %s still belongs to the quit bot at the bell", v.ID)
	}
}

func TestInstantBotDurFitsTheGame(t *testing.T) {
	w, _ := newInstant(t, 5)
	quarter := int64(game.InstantWorldConfig.DurationMin) * game.MsPerMinute / 4
	if got := game.BotDur(w, 30*game.MsPerDay); got != quarter {
		t.Fatalf("BotDur of 30 days = %d, want %d", got, quarter)
	}
	if j := game.SendJitter(w, "bot_0", "x"); j > quarter {
		t.Fatalf("jitter %d", j)
	}
}

func TestInstantBotSavesForTheNextNoble(t *testing.T) {
	w, _ := newInstant(t, 5)
	id := botsOf(w)[0].ID
	home := game.PlayerVillages(w, id)[0]
	home.Units = game.Units{}
	home.BuildQueue = nil
	home.Resources = game.Resources{Wood: 15_000, Clay: 15_000, Iron: 15_000}
	game.PlanBuilding(w, id, home)
	for _, b := range home.BuildQueue {
		if b.Building != game.BStorage && b.Building != game.BFarm {
			t.Fatalf("spent the noble's savings on %s", b.Building)
		}
	}
	home.Resources = game.Resources{Wood: 30_000, Clay: 30_000, Iron: 30_000}
	game.PlanBuilding(w, id, home)
	if len(home.BuildQueue) == 0 {
		t.Fatal("builds nothing from the surplus above the noble's price")
	}
}

func TestInstantBotKeepsTheNobleSavingsWhenItLogsOff(t *testing.T) {
	w, _ := newInstant(t, 5)
	id := botsOf(w)[0].ID
	home := game.PlayerVillages(w, id)[0]
	home.Units = game.Units{}
	home.BuildQueue = nil
	home.Resources = game.Resources{Wood: 15_000, Clay: 15_000, Iron: 15_000}
	game.QueueForOffline(w, id, home, w.Now()+5*game.MsPerMinute)
	for _, b := range home.BuildQueue {
		if b.Building != game.BStorage && b.Building != game.BFarm {
			t.Fatalf("spent the noble's savings on %s", b.Building)
		}
	}
	if r := home.Resources; r.Wood < 14_000 || r.Clay < 14_000 || r.Iron < 14_000 {
		t.Fatalf("resources left %+v", r)
	}
}

// twoNobleBot: an unprotected instant world whose bot_0 holds two nobles and an army, next to bot_0's nearest
// neighbouring bot village.
func twoNobleBot(t *testing.T) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	failOnBotErrors(t)
	cfg := game.InstantWorldConfig
	cfg.BeginnerProtectionDays = 0
	w := gametest.NewMemWorld(5, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	home := game.PlayerVillages(w, "bot_0")[0]
	home.Units[game.UNoble], home.Units[game.UAxe], home.Units[game.UScout] = 2, 2000, 10
	var near *game.Village
	for _, p := range botsOf(w) {
		if p.ID == "bot_0" {
			continue
		}
		v := game.PlayerVillages(w, p.ID)[0]
		if near == nil || game.Distance(home.X, home.Y, v.X, v.Y) < game.Distance(home.X, home.Y, near.X, near.Y) {
			near = v
		}
	}
	return w, home, near
}

func nobleSendsAt(w *gametest.MemWorld, owner game.PlayerID, v *game.Village) int {
	n := 0
	for _, s := range w.ScheduledSendsOf(owner) {
		if s.Units[game.UNoble] > 0 && s.X == v.X && s.Y == v.Y {
			n += s.Units[game.UNoble]
		}
	}
	return n
}

func TestInstantBotWithTwoNoblesTakesAScoutedOwnedVillage(t *testing.T) {
	w, _, near := twoNobleBot(t)
	knownOf(w, "bot_0", near)
	game.PlanExpansion(w, "bot_0")
	if got := nobleSendsAt(w, "bot_0", near); got != 2 {
		t.Fatalf("%d nobles go at the scouted owned village", got)
	}
}

func TestInstantBotWithTwoNoblesScoutsAnOwnedVillageFirst(t *testing.T) {
	w, _, near := twoNobleBot(t)
	k := knownOf(w, "bot_0", near)
	k.Units, k.Wall = nil, nil
	game.PlanExpansion(w, "bot_0")
	for _, s := range w.ScheduledSendsOf("bot_0") {
		if s.Units[game.UNoble] > 0 {
			t.Fatalf("nobles went to %d:%d before the owned village was scouted", s.X, s.Y)
		}
	}
	scouted := slices.ContainsFunc(w.ScheduledSendsOf("bot_0"), func(s *game.ScheduledSend) bool { return s.Units[game.UScout] > 0 })
	if !scouted {
		t.Fatal("no scouts went out")
	}
}

func TestInstantBotWithThreeVillagesLeavesBarbariansAlone(t *testing.T) {
	w, home, _ := twoNobleBot(t)
	w.Brain("bot_0").Memory.Known = map[string]*game.KnownVillage{}
	for _, id := range []game.PlayerID{"bot_1", "bot_2"} {
		game.TransferVillage(w, game.PlayerVillages(w, id)[0], "bot_0", home.ID)
	}
	game.PlanExpansion(w, "bot_0")
	for _, s := range w.ScheduledSendsOf("bot_0") {
		if s.Units[game.UNoble] > 0 {
			t.Fatalf("a bot with three villages sent nobles at %d:%d", s.X, s.Y)
		}
	}
}
