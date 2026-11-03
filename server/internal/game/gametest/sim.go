package gametest

import (
	"slices"
	"sync"
	"time"

	"librehold/internal/game"
)

// A world of bots plus one idle human, for measuring how the bots play (`go run ./cmd/sim`, TestBotWorldStaysInBands).

type SimOptions struct {
	Seed       uint32
	Days       int
	Bots       int
	Difficulty game.Difficulty
	// OnCheckpoint is called every 5 days with the summary row.
	OnCheckpoint func(SimRow)
}

type SimRow struct {
	Day                int
	Active, Quit, Elim int
	MedianVillages     float64
	BarbConquests      int
	BotBotConquests    int
	StrongMedianPoints float64
	WeakMedianPoints   float64
	FarmRuns           int
}

type SimResult struct {
	SimRow
	MedianPoints   map[int]float64 // median points of an active bot at days 10, 20, 30, …
	FarmRunsPerDay float64
	QuitShare      float64
	World          *MemWorld
}

// The game's spies are global, so the simulations running at a time share one pair of them, which counts each
// event for the world it happened in.
var (
	simsMu       sync.RWMutex
	sims         = map[game.World]*simCounts{}
	restoreSpies func()
)

type simCounts struct{ farmRuns, barb, botBot int }

// trackSim counts w's bot farm runs and bot conquests until stop is called.
func trackSim(w *MemWorld) (c *simCounts, stop func()) {
	simsMu.Lock()
	defer simsMu.Unlock()
	if len(sims) == 0 {
		of := func(w game.World) *simCounts {
			simsMu.RLock()
			defer simsMu.RUnlock()
			return sims[w]
		}
		isBot := func(w game.World, id game.PlayerRef) bool { return id != "" && w.Brain(game.PlayerID(id)) != nil }
		r1 := game.SpyBotBattles(func(w game.World, b game.BotBattle) {
			if c := of(w); c != nil && b.TargetOwner == "" {
				c.farmRuns++
			}
		})
		r2 := game.SpyTransfers(func(w game.World, from, to game.PlayerRef, conquest bool) {
			c := of(w)
			if c == nil || !conquest || !isBot(w, to) {
				return
			}
			if from == "" {
				c.barb++
			} else if isBot(w, from) {
				c.botBot++
			}
		})
		restoreSpies = func() { r2(); r1() }
	}
	c = &simCounts{}
	sims[w] = c
	return c, func() {
		simsMu.Lock()
		defer simsMu.Unlock()
		delete(sims, w)
		if len(sims) == 0 {
			restoreSpies()
		}
	}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[len(s)/2]
}

func RunSimBots(o SimOptions) (SimResult, error) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.Difficulty = o.Bots, o.Difficulty
	w := NewMemWorld(o.Seed, cfg, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC).UnixMilli())
	if err := game.NewWorld(w); err != nil {
		return SimResult{}, err
	}
	if _, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom); err != nil {
		return SimResult{}, err
	}
	counts, stop := trackSim(w)
	defer stop()
	row := func(day int) SimRow {
		r := SimRow{Day: day, BarbConquests: counts.barb, BotBotConquests: counts.botBot, FarmRuns: counts.farmRuns}
		var villages, strong, weak []float64
		points := w.PointsByOwner()
		for _, p := range w.Players() {
			b := w.Brain(p.ID)
			if b == nil {
				continue
			}
			switch p.Status {
			case game.StatusQuit:
				r.Quit++
			case game.StatusEliminated:
				r.Elim++
			case game.StatusActive:
				r.Active++
				villages = append(villages, float64(len(p.VillageIDs)))
				if b.Skill >= game.BotConfig.StrongSkill {
					strong = append(strong, float64(points[p.ID]))
				} else {
					weak = append(weak, float64(points[p.ID]))
				}
			}
		}
		r.MedianVillages, r.StrongMedianPoints, r.WeakMedianPoints = median(villages), median(strong), median(weak)
		return r
	}
	res := SimResult{MedianPoints: map[int]float64{}, World: w}
	for day := 5; day <= o.Days; day += 5 {
		if err := Advance(w, int64(day)*game.MsPerDay); err != nil {
			return res, err
		}
		if o.OnCheckpoint != nil {
			o.OnCheckpoint(row(day))
		}
		if day%10 == 0 {
			var pts []float64
			points := w.PointsByOwner()
			for _, p := range w.Players() {
				if w.Brain(p.ID) != nil && p.Status == game.StatusActive {
					pts = append(pts, float64(points[p.ID]))
				}
			}
			res.MedianPoints[day] = median(pts)
		}
	}
	if o.Days%5 != 0 {
		if err := Advance(w, int64(o.Days)*game.MsPerDay); err != nil {
			return res, err
		}
	}
	res.SimRow = row(o.Days)
	res.FarmRunsPerDay = float64(counts.farmRuns) / float64(o.Days)
	res.QuitShare = float64(res.Quit) / float64(o.Bots)
	return res, nil
}
