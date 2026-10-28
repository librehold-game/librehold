package game

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

type battleOpts struct {
	attacker, defender UnitCounts
	wall               int
	luck               float64
	rng                func() float64
	lossMode           BattleLossMode
	catapultTarget     *BuildingID
}

func battleInput(o battleOpts) BattleInput {
	var b Buildings
	b[BMain], b[BWood], b[BClay], b[BIron], b[BStorage], b[BFarm] = 5, 5, 5, 5, 5, 5
	b[BWall] = o.wall
	if o.rng == nil {
		o.rng = Mulberry32(1)
	}
	if o.lossMode == "" {
		o.lossMode = "normal"
	}
	return BattleInput{
		Attacker: BattleSide{Units: o.attacker},
		Defender: BattleDefender{
			BattleSide: BattleSide{Units: o.defender},
			Wall:       o.wall, Buildings: b, Resources: Resources{1000, 1000, 1000},
		},
		Luck: o.luck, Morale: 1, Rng: o.rng, LossMode: o.lossMode, CatapultTarget: o.catapultTarget,
	}
}

func bptr(b BuildingID) *BuildingID { return &b }

func TestCombatRequired(t *testing.T) {
	t.Run("100 axemen on an empty village with no wall → 0 losses, full victory", func(t *testing.T) {
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 100}}))
		if r.Outcome != "attackerWon" || r.AttackerLosses.Total() != 0 || r.AttackerSurvivors[UAxe] != 100 || r.HaulCapacity != 1000 ||
			r.Haul != (Resources{334, 333, 333}) {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("1 axeman on 1000 spearmen → the axeman dies, the defender loses ~0", func(t *testing.T) {
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 1}, defender: UnitCounts{USpear: 1000}}))
		if r.Outcome != "defenderWon" || r.AttackerLosses[UAxe] != 1 || r.DefenderLosses.Total() != 0 || SumResources(r.Haul) != 0 {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("symmetric balance of forces → both sides lose ~100 %", func(t *testing.T) {
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 100}, defender: UnitCounts{USword: 80}}))
		if r.Outcome != "defenderWon" || r.AttackerLosses[UAxe] != 100 || r.DefenderLosses[USword] != 80 {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("losses are whole numbers and add up to round(total × ratio)", func(t *testing.T) {
		units := UnitCounts{USpear: 37, USword: 11, UAxe: 3, ULightCav: 1}
		for _, ratio := range []float64{0, 0.1, 0.333, 0.5, 0.777, 0.999, 1} {
			losses, survivors := DistributeLosses(units, ratio)
			for _, u := range UnitIDs {
				if losses[u]+survivors[u] != units[u] {
					t.Fatalf("ratio %v unit %s", ratio, u)
				}
			}
			if losses.Total()+survivors.Total() != 52 || float64(losses.Total()) != math.Round(52*ratio) {
				t.Fatalf("ratio %v losses %d", ratio, losses.Total())
			}
		}
	})

	t.Run("determinism: the same result over 100 repetitions", func(t *testing.T) {
		mk := func() []byte {
			r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 50, ULightCav: 10, URam: 5},
				defender: UnitCounts{USpear: 40, USword: 10}, wall: 5, luck: 0.1, rng: Mulberry32(42)}))
			b, _ := json.Marshal(r)
			return b
		}
		first := mk()
		for range 100 {
			if string(mk()) != string(first) {
				t.Fatal("nondeterministic")
			}
		}
	})

	t.Run("loot never exceeds capacity or available resources", func(t *testing.T) {
		for cap := 0.0; cap < 3000; cap += 97 {
			for _, hide := range []int{0, 3, 10} {
				h := ComputeHaul(cap, Resources{900.7, 120, 2500}, hide)
				hideAmt := HideCapacity(hide)
				if SumResources(h) > cap || h.Wood > math.Max(0, 900-hideAmt) || h.Clay > math.Max(0, 120-hideAmt) || h.Iron > math.Max(0, 2500-hideAmt) {
					t.Fatalf("cap %v hide %d haul %+v", cap, hide, h)
				}
				for _, x := range []float64{h.Wood, h.Clay, h.Iron} {
					if x != math.Trunc(x) {
						t.Fatal("fractional haul")
					}
				}
				avail := math.Max(0, 900-hideAmt) + math.Max(0, 120-hideAmt) + math.Max(0, 2500-hideAmt)
				if SumResources(h) != math.Min(cap, avail) {
					t.Fatalf("cap %v hide %d sum %v", cap, hide, SumResources(h))
				}
			}
		}
	})
}

func TestCombatDetails(t *testing.T) {
	t.Run("luck multiplies attack; the wall multiplies defence", func(t *testing.T) {
		neutral := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}}))
		lucky := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}, luck: 0.25}))
		if math.Abs(lucky.AttackPower-neutral.AttackPower*1.25) > 1e-9 {
			t.Fatal("luck")
		}
		walled := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}, defender: UnitCounts{USpear: 10}, wall: 10}))
		open := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}, defender: UnitCounts{USpear: 10}}))
		if math.Abs(walled.DefensePower-(150+200)*math.Pow(1.037, 10)) > 1e-6 || math.Abs(open.DefensePower-150) > 1e-9 {
			t.Fatalf("walled %v open %v", walled.DefensePower, open.DefensePower)
		}
	})

	t.Run("defence split: cavalry meets the defence against cavalry", func(t *testing.T) {
		vsInf := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 100}, defender: UnitCounts{USpear: 100}}))
		vsCav := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{ULightCav: 31}, defender: UnitCounts{USpear: 100}}))
		if math.Abs(vsInf.DefensePower-1500) > 1e-9 || math.Abs(vsCav.DefensePower-4500) > 1e-9 {
			t.Fatal("split")
		}
	})

	t.Run("scouting: succeeds when scouts > 2× defenders, otherwise they die; the defender takes no losses", func(t *testing.T) {
		ok := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UScout: 10}, defender: UnitCounts{UScout: 3, USpear: 500}}))
		if ok.Kind != "scout" || ok.Outcome != "attackerWon" || ok.DefenderLosses.Total() != 0 ||
			float64(ok.AttackerLosses[UScout]) != math.Round(10*math.Pow(0.6, 1.5)) {
			t.Fatalf("%+v", ok)
		}
		fail := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UScout: 5}, defender: UnitCounts{UScout: 3}}))
		if fail.Outcome != "defenderWon" || fail.AttackerLosses[UScout] != 5 || fail.DefenderLosses.Total() != 0 {
			t.Fatalf("%+v", fail)
		}
		none := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UScout: 1}}))
		if none.Outcome != "attackerWon" || none.AttackerLosses.Total() != 0 {
			t.Fatalf("%+v", none)
		}
	})

	t.Run("rams: ~50 against wall 20 knock off a few levels, 300 raze it", func(t *testing.T) {
		levels := 0.0
		for l := 1; l <= 20; l++ {
			levels += WallHp(l)
		}
		if levels <= 210 || levels >= 230 || BatterLevels(20, 225, WallHp, 0) != 0 || BatterLevels(20, 200, WallHp, 0) <= 0 {
			t.Fatalf("levels %v", levels)
		}
		after50 := BatterLevels(20, 50, WallHp, 0)
		if 20-after50 < 2 || 20-after50 > 3 {
			t.Fatalf("after50 %d", after50)
		}
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 500, URam: 50}, wall: 20}))
		if r.Outcome != "attackerWon" || r.WallBefore != 20 || r.WallAfter != after50 {
			t.Fatalf("%+v", r)
		}
		lost := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{URam: 50}, defender: UnitCounts{USpear: 1000}, wall: 20}))
		if lost.WallAfter != 20 {
			t.Fatal("lost battle battered the wall")
		}
	})

	t.Run("catapults: lower the chosen building, town hall not below 1, a random target when none is chosen", func(t *testing.T) {
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 200, UCatapult: 20}, catapultTarget: bptr(BWood)}))
		if r.BuildingDamage == nil || r.BuildingDamage.Building != BWood || r.BuildingDamage.To >= 5 {
			t.Fatalf("%+v", r.BuildingDamage)
		}
		main := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 200, UCatapult: 1000}, catapultTarget: bptr(BMain)}))
		if main.BuildingDamage == nil || *main.BuildingDamage != (BuildingDamage{BMain, 5, 1}) {
			t.Fatalf("%+v", main.BuildingDamage)
		}
		random := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 200, UCatapult: 20}, rng: Mulberry32(7)}))
		random2 := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 200, UCatapult: 20}, rng: Mulberry32(7)}))
		if random.BuildingDamage == nil || *random2.BuildingDamage != *random.BuildingDamage {
			t.Fatal("random target")
		}
	})

	t.Run("research raises attack and defence", func(t *testing.T) {
		base := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}}))
		in := battleInput(battleOpts{attacker: UnitCounts{UAxe: 10}})
		in.Attacker.Research = UnitCounts{UAxe: 3}
		if up := ResolveBattle(in); math.Abs(up.AttackPower-base.AttackPower*1.4) > 1e-9 {
			t.Fatal("research")
		}
	})

	t.Run("reduced-losses mode halves the winner's losses", func(t *testing.T) {
		n := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 100}, defender: UnitCounts{USpear: 50}}))
		r := ResolveBattle(battleInput(battleOpts{attacker: UnitCounts{UAxe: 100}, defender: UnitCounts{USpear: 50}, lossMode: "reduced"}))
		if r.AttackerLosses.Total() >= n.AttackerLosses.Total() || r.DefenderLosses.Total() != 50 {
			t.Fatal("reduced")
		}
	})
}

// jsonNear compares decoded JSON: integers exactly, other numbers to 1e-12 relative, everything else exactly.
// libm can differ across platforms in the last bit, and arm64 may fuse a*b+c.
func jsonNear(a, b any) bool {
	switch x := a.(type) {
	case float64:
		y, ok := b.(float64)
		if !ok {
			return false
		}
		if x == math.Trunc(x) || y == math.Trunc(y) {
			return x == y
		}
		return math.Abs(x-y) <= 1e-12*math.Max(math.Abs(x), math.Abs(y))
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			if w, ok := y[k]; !ok || !jsonNear(v, w) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !jsonNear(x[i], y[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a, b)
}

func TestCombatGolden(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden/combat.json")
	if err != nil {
		t.Fatal(err)
	}
	var battles []struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &battles); err != nil {
		t.Fatal(err)
	}
	if len(battles) != 60 {
		t.Fatalf("%d battles", len(battles))
	}
	type battle struct {
		Input  json.RawMessage `json:"input"`
		Result BattleResult    `json:"result"`
	}
	got := make([]battle, len(battles))
	for i, b := range battles {
		var in struct {
			BattleInput
			RngSeed uint32 `json:"rngSeed"`
		}
		if err := json.Unmarshal(b.Input, &in); err != nil {
			t.Fatal(err)
		}
		in.Rng = Mulberry32(in.RngSeed)
		got[i] = battle{b.Input, ResolveBattle(in.BattleInput)}
	}
	Golden(t, "combat", got)
}
