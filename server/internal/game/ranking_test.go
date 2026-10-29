package game_test

import (
	"math"
	"testing"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// statsWorld has two quitting bots (they never act), speed 1, no protection.
func statsWorld(t *testing.T) (*gametest.MemWorld, *game.Village, game.PlayerID, game.PlayerID) {
	t.Helper()
	cfg := gametest.TestConfig()
	cfg.BotCount, cfg.BeginnerProtectionDays = 2, 0
	w, home := gametest.NewWorldWithHuman(111, cfg)
	w.Player("bot_0").Status = game.StatusQuit
	w.Player("bot_1").Status = game.StatusQuit
	return w, home, "bot_0", "bot_1"
}

func statsAttack(t *testing.T, w *gametest.MemWorld, home *game.Village, targetID string, u game.UnitCounts) {
	t.Helper()
	for _, id := range game.UnitIDs {
		if u[id] > 0 {
			home.Units[id] = u[id]
		}
	}
	target := w.Village(targetID)
	c := mustSend(t, w, home, target.X, target.Y, u, "attack")
	advance(t, w, c.ArrivesAt+1)
}

func TestPublicStats(t *testing.T) {
	t.Run("defeated opponents are counted from the population of killed units", func(t *testing.T) {
		w, home, a, _ := statsWorld(t)
		v := game.PlayerVillages(w, a)[0]
		v.Units[game.USpear], v.Units[game.UHeavyCav] = 10, 2
		statsAttack(t, w, home, v.ID, game.UnitCounts{game.UAxe: 500})
		humanLost := lastReport(w, "h1").Attacker.Losses.Total()
		if w.Player("h1").OD.Att != 22 || w.Player(a).OD.Def != float64(humanLost) {
			t.Fatalf("att %v def %v lost %d", w.Player("h1").OD.Att, w.Player(a).OD.Def, humanLost)
		}
	})

	t.Run("support gets its share", func(t *testing.T) {
		w, home, a, b := statsWorld(t)
		va, vb := game.PlayerVillages(w, a)[0], game.PlayerVillages(w, b)[0]
		va.Units[game.USpear] = 50
		va.SupportFrom[vb.ID] = game.UnitCounts{game.USpear: 50}
		statsAttack(t, w, home, va.ID, game.UnitCounts{game.UAxe: 100})
		def, sup := w.Player(a).OD.Def, w.Player(b).OD.Sup
		if def <= 0 || math.Abs(def-sup) > 1 {
			t.Fatalf("def %v sup %v", def, sup)
		}
	})

	t.Run("farmer of the day: barbarian loot in the daily counter", func(t *testing.T) {
		w, home, _, _ := statsWorld(t)
		barb := ensureNearestBarbarian(w, home)
		statsAttack(t, w, home, barb.ID, game.UnitCounts{game.UAxe: 20})
		haul := lastReport(w, "h1").Haul
		day := game.LocalDayIndex(w, w.Now())
		var today *game.DailyCounts
		for i, d := range w.Player("h1").Daily {
			if d.Day == day {
				today = &w.Player("h1").Daily[i]
			}
		}
		if today == nil || today.Farm != game.SumResources(*haul) || today.Loot != today.Farm {
			t.Fatalf("today %+v haul %+v", today, haul)
		}
	})

	t.Run("a conquest goes on the public list", func(t *testing.T) {
		w, home, _, _ := statsWorld(t)
		barb := ensureNearestBarbarian(w, home)
		game.TransferVillage(w, barb, "h1", home.ID)
		list := w.ConquestsSince(0)
		last := list[len(list)-1]
		if last != (game.Conquest{T: w.Now(), VillageID: barb.ID, X: barb.X, Y: barb.Y, FromID: "", ToID: "h1"}) {
			t.Fatalf("conquest %+v", last)
		}
	})

	t.Run("daily counters keep the last 14 days", func(t *testing.T) {
		w, home, _, _ := statsWorld(t)
		for range 20 {
			advance(t, w, w.Now()+game.MsPerDay)
			barb := ensureNearestBarbarian(w, home)
			statsAttack(t, w, home, barb.ID, game.UnitCounts{game.UAxe: 20})
		}
		day := game.LocalDayIndex(w, w.Now())
		for _, d := range w.Player("h1").Daily {
			if d.Day <= day-14 {
				t.Fatalf("kept day %d (today %d)", d.Day, day)
			}
		}
	})
}

func TestRankingRefresh(t *testing.T) {
	origin := time.Date(2026, 10, 19, 8, 51, 0, 0, time.UTC).UnixMilli()
	cfg := game.DefaultWorldConfig
	cfg.BotCount = 20
	w := gametest.NewMemWorld(3, cfg, origin)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}

	t.Run("a new world shows points right away — not zeros for the first hour", func(t *testing.T) {
		settled := 0
		for _, p := range w.Players() {
			if len(p.VillageIDs) > 0 {
				settled++
				if p.Points != game.PlayerPoints(w, p.ID) {
					t.Fatalf("%s points %d", p.ID, p.Points)
				}
			}
		}
		if settled <= 1 {
			t.Fatal("no settled players")
		}
	})

	t.Run("recount on the full hour of the local clock (09:00), not an hour after the world was created", func(t *testing.T) {
		if at, ok := w.ScheduledAt("stats"); !ok || at != 9*game.MsPerMinute {
			t.Fatalf("stats at %d %v", at, ok)
		}
	})

	t.Run("hourly history every hour, daily at midnight", func(t *testing.T) {
		advance(t, w, 9*game.MsPerMinute+25*game.MsPerHour)
		p := w.Player("bot_0")
		if len(p.History.Hourly) != 26 || len(p.History.Daily) != 1 {
			t.Fatalf("hourly %d daily %d", len(p.History.Hourly), len(p.History.Daily))
		}
	})

	t.Run("a new human has points right away", func(t *testing.T) {
		if _, err := game.JoinHuman(w, "h9", "Nowy", 9, game.QuadrantRandom); err != nil {
			t.Fatal(err)
		}
		if p := w.Player("h9"); p.Points == 0 || p.Points != game.PlayerPoints(w, "h9") {
			t.Fatalf("points %d", p.Points)
		}
	})
}

func TestInstantRankingRefresh(t *testing.T) {
	origin := time.Date(2026, 10, 19, 8, 51, 30, 0, time.UTC).UnixMilli()
	cfg := game.InstantWorldConfig
	w := gametest.NewMemWorld(3, cfg, origin)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}

	t.Run("recount on the next full minute, not the next full hour", func(t *testing.T) {
		if at, ok := w.ScheduledAt("stats"); !ok || at != 30*game.MsPerSecond {
			t.Fatalf("stats at %d %v", at, ok)
		}
	})

	t.Run("new points show in the ranking within a minute", func(t *testing.T) {
		p := w.Players()[0]
		v := game.PlayerVillages(w, p.ID)[0]
		v.Buildings[game.BWall]++
		game.RecomputePoints(v)
		advance(t, w, 30*game.MsPerSecond+game.MsPerMinute)
		if p.Points != game.PlayerPoints(w, p.ID) {
			t.Fatalf("points %d, want %d", p.Points, game.PlayerPoints(w, p.ID))
		}
	})

	t.Run("history still once an hour", func(t *testing.T) {
		advance(t, w, 30*game.MsPerSecond+20*game.MsPerMinute)
		if h := len(w.Players()[0].History.Hourly); h != 1 {
			t.Fatalf("hourly %d", h)
		}
	})
}
