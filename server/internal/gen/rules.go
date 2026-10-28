package gen

import (
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// The browser keeps a few formulas to draw screens between server replies (travel times, production, costs,
// loyalty). tests/fixtures/rules.json holds Go's answers for a grid of inputs; tests/rules.test.ts runs the
// TypeScript helpers on the same inputs and expects the same outputs.

type travelCase struct {
	Unit            game.UnitID `json:"unit"`
	Dist            float64     `json:"dist"`
	SpeedMinPerTile float64     `json:"speedMinPerTile"`
	UnitSpeed       float64     `json:"unitSpeed"`
	Ms              int64       `json:"ms"`
}

type productionCase struct {
	Resource   string  `json:"resource"`
	Level      int     `json:"level"`
	WorldSpeed float64 `json:"worldSpeed"`
	Modifier   float64 `json:"modifier"`
	Bonus      string  `json:"bonus"`
	PerHour    float64 `json:"perHour"`
}

type capacityCase struct {
	Level      int     `json:"level"`
	Value      float64 `json:"value"`
	BonusValue float64 `json:"bonusValue"`
}

type levelCase struct {
	Level int     `json:"level"`
	Value float64 `json:"value"`
}

type buildCostCase struct {
	Building game.BuildingID `json:"building"`
	Level    int             `json:"level"`
	game.Resources
}

type buildTimeCase struct {
	Building   game.BuildingID `json:"building"`
	Level      int             `json:"level"`
	MainLevel  int             `json:"mainLevel"`
	WorldSpeed float64         `json:"worldSpeed"`
	Modifier   float64         `json:"modifier"`
	Ms         int64           `json:"ms"`
}

type recruitTimeCase struct {
	Unit          game.UnitID `json:"unit"`
	BuildingLevel int         `json:"buildingLevel"`
	WorldSpeed    float64     `json:"worldSpeed"`
	Modifier      float64     `json:"modifier"`
	Ms            int64       `json:"ms"`
}

type researchCostCase struct {
	Unit  game.UnitID `json:"unit"`
	Level int         `json:"level"`
	game.Resources
}

type loyaltyCase struct {
	Loyalty    float64 `json:"loyalty"`
	ElapsedMs  int64   `json:"elapsedMs"`
	WorldSpeed float64 `json:"worldSpeed"`
	Value      float64 `json:"value"`
}

var resourceNames = []struct {
	name     string
	resource game.Resource
	building game.BuildingID
}{{"wood", game.Wood, game.BWood}, {"clay", game.Clay, game.BClay}, {"iron", game.Iron, game.BIron}}

// humanVillage is a village of human h1 in a world with the given speed and side modifier for every side.
func humanVillage(worldSpeed, modifier float64) (*gametest.MemWorld, *game.Village) {
	cfg := game.DefaultWorldConfig
	cfg.WorldSpeed = worldSpeed
	cfg.HumanModifiers = game.SideModifiers{Resources: modifier, Recruitment: modifier, Building: modifier}
	w := gametest.NewMemWorld(1, cfg, 0)
	gametest.NewHumanPlayer(w, "h1")
	return w, gametest.AddStartVillage(w, "h1", 0, 0)
}

// RulesJSON is tests/fixtures/rules.json.
func RulesJSON() ([]byte, error) {
	out := map[string]any{}

	var travel []travelCase
	for _, dist := range []float64{1, 2.5, 7, 15.3, 40, 70} {
		for _, u := range game.UnitIDs {
			for _, us := range []float64{0.5, 1, 2, game.InstantWorldConfig.UnitSpeed} {
				speed := game.UnitDefs[u].Speed
				travel = append(travel, travelCase{u, dist, speed, us, game.TravelTimeMs(dist, speed, us)})
			}
		}
	}
	out["travel"] = travel

	var production []productionCase
	for _, ws := range []float64{1, 8} {
		for _, mod := range []float64{1, 3} {
			_, v := humanVillage(ws, mod)
			cfg := game.DefaultWorldConfig
			cfg.WorldSpeed, cfg.HumanModifiers = ws, game.SideModifiers{Resources: mod, Recruitment: mod, Building: mod}
			for _, r := range resourceNames {
				for _, bonus := range []string{"", r.name, "allResources"} {
					v.Bonus = nil
					if bonus != "" {
						v.Bonus = &game.VillageBonus{Kind: game.VillageBonusKind(bonus)}
					}
					for level := 0; level <= game.BuildingDefs[r.building].MaxLevel; level++ {
						v.Buildings[r.building] = level
						production = append(production, productionCase{r.name, level, ws, mod, bonus, game.VillageProductionPerHour(v, r.resource, &cfg)})
					}
				}
			}
		}
	}
	out["production"] = production

	var storage, farm []capacityCase
	var hide []levelCase
	for level := 0; level <= 30; level++ {
		storage = append(storage, capacityCase{level, game.StorageCapacity(level, nil), game.StorageCapacity(level, &game.VillageBonus{Kind: "storage"})})
		farm = append(farm, capacityCase{level, game.FarmCapacity(level, nil), game.FarmCapacity(level, &game.VillageBonus{Kind: "farm"})})
	}
	for level := 0; level <= game.BuildingDefs[game.BHide].MaxLevel; level++ {
		hide = append(hide, levelCase{level, game.HideCapacity(level)})
	}
	out["storage"], out["farm"], out["hide"] = storage, farm, hide

	var costs []buildCostCase
	var times []buildTimeCase
	for _, b := range game.BuildingIDs {
		for level := 1; level <= game.BuildingDefs[b].MaxLevel; level++ {
			costs = append(costs, buildCostCase{b, level, game.BuildCost(b, level)})
		}
	}
	for _, ws := range []float64{1, 8} {
		for _, mod := range []float64{1, 2} {
			w, v := humanVillage(ws, mod)
			for _, b := range game.BuildingIDs {
				for level := 1; level <= game.BuildingDefs[b].MaxLevel; level++ {
					for _, main := range []int{1, 20} {
						v.Buildings = game.Buildings{}
						v.Buildings[game.BMain] = main
						v.Buildings[b] = level - 1
						c := game.CheckBuild(w, v, b)
						times = append(times, buildTimeCase{b, level, v.Buildings[game.BMain], ws, mod, c.DurationMs})
					}
				}
			}
		}
	}
	out["buildCost"], out["buildTime"] = costs, times

	var recruit []recruitTimeCase
	for _, ws := range []float64{1, 8} {
		for _, mod := range []float64{1, 2} {
			w, v := humanVillage(ws, mod)
			for _, u := range game.UnitIDs {
				for _, level := range []int{1, 10, 25} {
					recruit = append(recruit, recruitTimeCase{u, level, ws, mod, game.RecruitTimeMs(u, level, w.Config(), v)})
				}
			}
		}
	}
	out["recruitTime"] = recruit

	var research []researchCostCase
	for _, u := range game.UnitIDs {
		for level := 1; level <= 3; level++ {
			research = append(research, researchCostCase{u, level, game.ResearchCost(u, level)})
		}
	}
	out["researchCost"] = research

	var loyalty []loyaltyCase
	for _, l := range []float64{25, 60, 99} {
		for _, elapsed := range []int64{0, game.MsPerHour, 10 * game.MsPerHour, 100 * game.MsPerHour} {
			for _, ws := range []float64{1, 8} {
				v := &game.Village{Loyalty: l}
				loyalty = append(loyalty, loyaltyCase{l, elapsed, ws, game.LoyaltyAt(v, elapsed, ws)})
			}
		}
	}
	out["loyalty"] = loyalty

	type nobles struct {
		Loyalty float64 `json:"loyalty"`
		N       int     `json:"n"`
	}
	var needed []nobles
	for l := 0; l <= 100; l += 5 {
		needed = append(needed, nobles{float64(l), game.NoblesNeeded(float64(l))})
	}
	out["noblesNeeded"] = needed

	type limit struct {
		Academy int `json:"academy"`
		N       int `json:"n"`
	}
	type bonus struct {
		Level int     `json:"level"`
		Bonus float64 `json:"bonus"`
	}
	var limits []limit
	var bonuses []bonus
	long := game.DefaultWorldConfig
	for i := 0; i <= 3; i++ {
		v := &game.Village{}
		v.Buildings[game.BAcademy] = i
		limits = append(limits, limit{i, game.NobleLimit(&long, v)})
		bonuses = append(bonuses, bonus{i, game.ResearchBonus(i)})
	}
	out["nobleLimit"], out["researchBonus"] = limits, bonuses

	return canonical(out)
}
