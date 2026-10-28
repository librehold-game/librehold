package app

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"librehold/internal/game"
)

// The site's guides and FAQ quote these numbers; each one must be what the server actually does.
func TestSiteFactsMatchTheServer(t *testing.T) {
	raw, err := os.ReadFile("../../../src/account/content/facts.json")
	if err != nil {
		t.Fatal(err)
	}
	var facts map[string]float64
	if err := json.Unmarshal(raw, &facts); err != nil {
		t.Fatal(err)
	}
	inst, long := game.InstantWorldConfig, game.DefaultWorldConfig
	iv, lv := game.InstantStartVillage, game.StartVillage
	days := func(d time.Duration) float64 { return d.Hours() / 24 }
	want := map[string]float64{
		"instantDurationMin":      float64(inst.DurationMin),
		"instantJoinWindowMin":    float64(inst.JoinWindowMin),
		"instantSeats":            float64(inst.HumanSeats),
		"instantBots":             float64(inst.BotCount),
		"instantMapSize":          float64(inst.MapSize),
		"instantSpeed":            inst.WorldSpeed,
		"instantProtectionMin":    math.Round(inst.BeginnerProtectionDays * 24 * 60),
		"instantDominationPct":    math.Round(inst.DominationShare * 100),
		"instantBarbarianLoyalty": inst.BarbarianLoyalty,
		"instantStartMain":        float64(iv.Buildings[game.BMain]),
		"instantStartBarracks":    float64(iv.Buildings[game.BBarracks]),
		"instantStartStable":      float64(iv.Buildings[game.BStable]),
		"instantStartSmithy":      float64(iv.Buildings[game.BSmithy]),
		"instantStartWall":        float64(iv.Buildings[game.BWall]),
		"instantStartResources":   iv.Resources.Wood,
		"instantStartSpear":       float64(iv.Units[game.USpear]),
		"instantStartAxe":         float64(iv.Units[game.UAxe]),
		"instantStartLightCav":    float64(iv.Units[game.ULightCav]),
		"instantStartScout":       float64(iv.Units[game.UScout]),
		"instantStartNoble":       float64(iv.Units[game.UNoble]),
		"longDefaultSpeed":        long.WorldSpeed,
		"longProtectionDays":      long.BeginnerProtectionDays,
		"protectionMaxDays":       game.WorldLimits.BeginnerProtectionDays.Max,
		"longStartResources":      lv.Resources.Wood,
		"longStartSpear":          float64(lv.Units[game.USpear]),
		"cancelWindowMin":         float64(game.CommandBalance.CancelWindowMs) / float64(game.MsPerMinute),
		"sightRadiusLong":         game.SightRadiusOf(&long),
		"sightRadiusInstant":      game.SightRadiusOf(&inst),
		"guestDeleteDays":         days(guestInactivity),
		"instantKeepHours":        instantKeep.Hours(),
		"autoFarmUnlockVillages":  float64(game.FarmBalance.AutoFarmUnlockVillages),
		"autoFarmIntervalMin":     float64(game.FarmBalance.AutoFarmIntervalMs) / float64(game.MsPerMinute),
		"reportsPerHour":          reportsPerHour,
		"reportTextMin":           reportTextMin,
		"reportTextMax":           reportTextMax,
		"reportKeepDays":          days(reportKeep),
	}
	// Each resource starts at the same amount, so one number stands for all three.
	for _, r := range []game.Resources{iv.Resources, lv.Resources} {
		if r.Clay != r.Wood || r.Iron != r.Wood {
			t.Errorf("start resources differ between kinds: %+v", r)
		}
	}
	var keys []string
	for k := range facts {
		keys = append(keys, k)
	}
	for k := range want {
		if _, ok := facts[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		got, inFile := facts[k]
		w, known := want[k]
		switch {
		case !inFile:
			t.Errorf("facts.json misses %s (the server says %v)", k, w)
		case !known:
			t.Errorf("facts.json has %s, which no server constant backs", k)
		case got != w:
			t.Errorf("%s: facts.json says %v, the server %v", k, got, w)
		}
	}
}
