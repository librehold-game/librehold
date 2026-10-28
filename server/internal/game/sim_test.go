package game_test

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"
	"time"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

type simBand struct {
	Seed           uint32             `json:"seed"`
	MedianPoints   map[string]float64 `json:"medianPoints"`
	MedianVillages float64            `json:"medianVillages"`
	FarmRunsPerDay float64            `json:"farmRunsPerDay"`
	QuitShare      float64            `json:"quitShare"`
}

func medianOf(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[len(s)/2]
}

// The bot world keeps playing as recorded: 30 bots for 30 days on seeds 1–3, medians across seeds within ±25 % of
// testdata/golden/simbands.json (villages ±1 and the quit share ±0.05 absolute, since those are small counts), and
// no handler errors. A deliberate bot change is expected to move the bands: re-record them with
// `go test ./internal/game/ -run Bands -update`.
func TestBotWorldStaysInBands(t *testing.T) {
	if testing.Short() {
		t.Skip("long simulation")
	}
	raw, err := os.ReadFile("testdata/golden/simbands.json")
	if err != nil {
		t.Fatal(err)
	}
	var ts []simBand
	if err := json.Unmarshal(raw, &ts); err != nil {
		t.Fatal(err)
	}
	failOnBotErrors(t)
	var goBands []gametest.SimResult
	for _, b := range ts {
		start := time.Now()
		r, err := gametest.RunSimBots(gametest.SimOptions{Seed: b.Seed, Days: 30, Bots: 30, Difficulty: game.Normal})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: points %v villages %v raids/day %.0f quit %.3f (%v)", b.Seed, r.MedianPoints, r.MedianVillages, r.FarmRunsPerDay, r.QuitShare, time.Since(start))
		goBands = append(goBands, r)
	}
	if game.Updating() {
		rec := make([]simBand, len(goBands))
		for i, r := range goBands {
			rec[i] = simBand{Seed: ts[i].Seed, MedianPoints: map[string]float64{}, MedianVillages: r.MedianVillages,
				FarmRunsPerDay: r.FarmRunsPerDay, QuitShare: r.QuitShare}
			for _, day := range []int{10, 20, 30} {
				rec[i].MedianPoints[itoa(day)] = r.MedianPoints[day]
			}
		}
		game.Golden(t, "simbands", rec)
		return
	}
	metric := func(name string, tsVals, goVals []float64, ok func(g, want float64) bool) {
		g, want := medianOf(goVals), medianOf(tsVals)
		if !ok(g, want) {
			t.Errorf("%s: median %v, recorded %v", name, g, want)
		}
	}
	rel := func(g, want float64) bool { return math.Abs(g-want) <= 0.25*want }
	for _, day := range []int{10, 20, 30} {
		var tv, gv []float64
		for i := range ts {
			tv = append(tv, ts[i].MedianPoints[itoa(day)])
			gv = append(gv, goBands[i].MedianPoints[day])
		}
		metric("points day "+itoa(day), tv, gv, rel)
	}
	var tVill, gVill, tRaids, gRaids, tQuit, gQuit []float64
	for i := range ts {
		tVill, gVill = append(tVill, ts[i].MedianVillages), append(gVill, goBands[i].MedianVillages)
		tRaids, gRaids = append(tRaids, ts[i].FarmRunsPerDay), append(gRaids, goBands[i].FarmRunsPerDay)
		tQuit, gQuit = append(tQuit, ts[i].QuitShare), append(gQuit, goBands[i].QuitShare)
	}
	metric("villages", tVill, gVill, func(g, want float64) bool { return math.Abs(g-want) <= math.Max(1, 0.25*want) })
	metric("raids per day", tRaids, gRaids, rel)
	metric("quit share", tQuit, gQuit, func(g, want float64) bool { return math.Abs(g-want) <= 0.05 })
}
