// Command sim runs a world of bots and prints a summary table; with -instant it plays an instant world with
// scripted guests instead, and with -player it measures the game's pace with the reference player
// (gametest.RunReferencePlayer, the run pacing_test.go checks).
//
//	go run ./cmd/sim -bots 100 -days 60 -seed 7
//	go run ./cmd/sim -instant -seed 7 -guests 3
//	go run ./cmd/sim -player -seed 1 -days 24 [-speed 8]
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func main() {
	seed := flag.Uint("seed", 1, "world seed")
	days := flag.Int("days", 60, "days to simulate")
	bots := flag.Int("bots", 100, "number of bots")
	difficulty := flag.String("difficulty", "normal", "easy | normal | hard | brutal")
	instant := flag.Bool("instant", false, "simulate an instant world")
	guests := flag.Int("guests", 3, "scripted guests in an instant world")
	instantPlayer := flag.Bool("instant-player", false, "play an instant world with the active player; -guests idle guests")
	player := flag.Bool("player", false, "measure the pace with the reference player")
	speed := flag.Float64("speed", 0, "world speed for -player (0 = the default world settings)")
	flag.Parse()

	if *player {
		runPlayer(uint32(*seed), *days, *speed)
		return
	}

	if *instantPlayer {
		r, err := gametest.RunInstantPlayer(gametest.InstantPlayerOptions{Seed: uint32(*seed), IdleHumans: *guests})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("instant-player seed=%d guests=%d: villages %d, rank %d, top bot villages %d, owned changes %d, first conquest %.1f s, bot sends after end %d\n",
			*seed, *guests, r.Villages, r.Rank, r.TopBotVillages, r.OwnedChanges, float64(r.FirstConquestMs)/1000, r.BotSendsAfterEnd)
		return
	}
	if *instant {
		r, err := gametest.RunInstantSim(uint32(*seed), *guests)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("instant seed=%d guests=%d: first conquest %.1f s, bot conquests %d, bot attacks on newcomers %d, bot actions %d, guest villages %d\n",
			*seed, *guests, float64(r.FirstConquestMs)/1000, r.BotConquests, r.BotAttacksOnHumansEarly, r.BotActions, r.HumanVillages)
		return
	}

	fmt.Printf("bots seed=%d days=%d bots=%d difficulty=%s\n", *seed, *days, *bots, *difficulty)
	fmt.Println("day | active quit elim. | villages (med.) | conquests b→barb b→bot | pts strong / weak | raids")
	errors := 0
	start := time.Now()
	game.BotActionError = func(id game.PlayerID, err error) { errors++ }
	_, err := gametest.RunSimBots(gametest.SimOptions{
		Seed: uint32(*seed), Days: *days, Bots: *bots, Difficulty: game.Difficulty(*difficulty),
		OnCheckpoint: func(x gametest.SimRow) {
			fmt.Printf("%5d | %8d %7d %7d | %13.0f | %13d %5d | %7.0f / %-7.0f | %d\n", x.Day, x.Active, x.Quit, x.Elim,
				x.MedianVillages, x.BarbConquests, x.BotBotConquests, x.StrongMedianPoints, x.WeakMedianPoints, x.FarmRuns)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("time: %d ms, rejected bot orders: %d\n", time.Since(start).Milliseconds(), errors)
}

var playerTargets = map[string]string{
	"first attack": "~30 min", "first successful raid": "~30 min", "1,000 pts": "~2 days", "first hetman": "10–14 days",
	"2nd village": "—", "3rd village": "—", "5th village": "—", "10th village": "5–6 weeks",
}

func runPlayer(seed uint32, days int, speed float64) {
	fmt.Printf("player seed=%d days=%d\n", seed, days)
	start := time.Now()
	run, err := gametest.RunReferencePlayer(gametest.PlayerRunOptions{Seed: seed, Days: days, Speed: speed, OnDay: func(s gametest.DaySnapshot) {
		b := s.Buildings
		fmt.Printf("day %2d: %6d pts | villages %d | M%d B%d S%d Sm%d W%d/C%d/I%d F%d St%d A%d | army %d (home) | loot %.0fk | coins %d | nobles %d\n",
			s.Day, s.Points, s.Villages, b[game.BMain], b[game.BBarracks], b[game.BStable], b[game.BSmithy], b[game.BWood], b[game.BClay],
			b[game.BIron], b[game.BFarm], b[game.BStorage], b[game.BAcademy], s.Army, s.Looted/1000, s.Coins, s.Nobles)
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st := run.World.Human(run.Human).Stats
	fmt.Printf("simulation time: %d ms\n", time.Since(start).Milliseconds())
	fmt.Printf("attacks %d, won %d, lost %d, killed %d, reports %d\n", st.AttacksSent, st.BattlesWon, st.UnitsLost, st.UnitsKilled,
		len(run.World.ReportsOf(run.Human)))
	fmt.Printf("Milestones (world speed %v×):\n", run.World.Config().WorldSpeed)
	for _, m := range gametest.Milestones {
		at := "not reached"
		if ms := run.Milestones[m]; ms >= 0 {
			at = fmt.Sprintf("%.2f d", float64(ms)/float64(game.MsPerDay))
		}
		fmt.Printf("  %-22s %s  (target %s)\n", m, at, playerTargets[m])
	}
}
