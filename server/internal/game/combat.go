package game

import (
	"math"
	"sort"
)

// Battle resolution. ResolveBattle is pure: all randomness comes in through Luck and Rng.

type BattleSide struct {
	Units    UnitCounts `json:"units"`
	Research UnitCounts `json:"research"` // research levels 0–3
}

type BattleDefender struct {
	BattleSide
	Wall      int       `json:"wall"`
	Buildings Buildings `json:"buildings"`
	Resources Resources `json:"resources"`
	HideLevel int       `json:"hideLevel"`
}

type BattleInput struct {
	Attacker       BattleSide     `json:"attacker"`
	Defender       BattleDefender `json:"defender"`
	Luck           float64        `json:"luck"` // in [-0.25, 0.25]
	Morale         float64        `json:"morale"`
	CatapultTarget *BuildingID    `json:"catapultTarget,omitempty"`
	Rng            func() float64 `json:"-"` // random catapult target (deterministic per command)
	LossMode       BattleLossMode `json:"lossMode"`
}

type BattleResult struct {
	Kind              string          `json:"kind"`    // battle | scout
	Outcome           string          `json:"outcome"` // attackerWon | defenderWon
	AttackPower       float64         `json:"attackPower"`
	DefensePower      float64         `json:"defensePower"`
	Luck              float64         `json:"luck"`
	Morale            float64         `json:"morale"`
	AttackerLosses    UnitCounts      `json:"attackerLosses"`
	AttackerSurvivors UnitCounts      `json:"attackerSurvivors"`
	DefenderLosses    UnitCounts      `json:"defenderLosses"`
	DefenderSurvivors UnitCounts      `json:"defenderSurvivors"`
	WallBefore        int             `json:"wallBefore"`
	WallAfter         int             `json:"wallAfter"`
	BuildingDamage    *BuildingDamage `json:"buildingDamage,omitempty"`
	Haul              Resources       `json:"haul"`
	HaulCapacity      float64         `json:"haulCapacity"`
}

func IsScoutMission(u UnitCounts) bool {
	n := u.Total()
	return n > 0 && u[UScout] == n
}

func researchFactor(research UnitCounts, u UnitID) float64 {
	return 1 + ResearchBonus(research[u])
}

// DistributeLosses spreads losses in proportion to numbers: floors first, the remainder from the largest
// unit type, so the losses sum to round(total × ratio).
func DistributeLosses(units UnitCounts, ratio float64) (losses, survivors UnitCounts) {
	r := math.Max(0, math.Min(1, ratio))
	target := int(math.Round(float64(units.Total()) * r))
	assigned := 0
	var present []UnitID
	for _, u := range UnitIDs {
		if units[u] > 0 {
			present = append(present, u)
		}
	}
	for _, u := range present {
		l := int(math.Floor(float64(units[u]) * r))
		losses[u] = l
		assigned += l
	}
	order := append([]UnitID(nil), present...)
	sort.SliceStable(order, func(i, j int) bool { return units[order[i]] > units[order[j]] })
	remainder := target - assigned
	for guard := 0; remainder > 0 && guard < 100_000; guard++ {
		progressed := false
		for _, u := range order {
			if remainder <= 0 {
				break
			}
			if losses[u] < units[u] {
				losses[u]++
				remainder--
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	for _, u := range present {
		survivors[u] = units[u] - losses[u]
	}
	return losses, survivors
}

func WallHp(level int) float64 {
	return CombatBalance.WallHpA*math.Pow(float64(level), 1.5) + CombatBalance.WallHpB
}

func BuildingHp(level int) float64 {
	return CombatBalance.BuildingHpA*math.Pow(float64(level), 1.5) + CombatBalance.BuildingHpB
}

// BatterLevels takes levels off from the top while the power lasts.
func BatterLevels(level int, power float64, hp func(int) float64, minLevel int) int {
	l, p := level, power
	for l > minLevel {
		cost := hp(l)
		if p < cost {
			break
		}
		p -= cost
		l--
	}
	return l
}

// ComputeHaul takes loot in proportion to the resources above the hiding place, up to capacity, in integers.
func ComputeHaul(capacity float64, resources Resources, hideLevel int) Resources {
	hide := HideCapacity(hideLevel)
	var avail Resources
	for _, r := range ResourceIDs {
		avail.Set(r, math.Max(0, math.Floor(resources.Get(r))-hide))
	}
	total := SumResources(avail)
	c := math.Max(0, math.Floor(capacity))
	var haul Resources
	if total == 0 || c == 0 {
		return haul
	}
	if total <= c {
		return avail
	}
	taken := 0.0
	for _, r := range ResourceIDs {
		h := math.Floor((c * avail.Get(r)) / total)
		haul.Set(r, h)
		taken += h
	}
	// The rounding remainder goes where something is still left.
	rest := c - taken
	for guard := 0; rest > 0 && guard < 10; guard++ {
		for _, r := range ResourceIDs {
			if rest <= 0 {
				break
			}
			if haul.Get(r) < avail.Get(r) {
				haul.Set(r, haul.Get(r)+1)
				rest--
			}
		}
	}
	return haul
}

type attackPower struct{ total, infantry, cavalry, archer float64 }

func attackPowerOf(side BattleSide) attackPower {
	var ap attackPower
	for _, u := range UnitIDs {
		n := side.Units[u]
		if n <= 0 {
			continue
		}
		p := float64(n) * UnitDefs[u].Attack * researchFactor(side.Research, u)
		switch UnitDefs[u].AttackClass {
		case ClassCavalry:
			ap.cavalry += p
		case ClassArcher:
			ap.archer += p
		default:
			ap.infantry += p
		}
	}
	ap.total = ap.infantry + ap.cavalry + ap.archer
	return ap
}

func defensePowerOf(side BattleSide) (general, cavalry, archer float64) {
	for _, u := range UnitIDs {
		n := side.Units[u]
		if n <= 0 {
			continue
		}
		b := researchFactor(side.Research, u)
		general += float64(n) * UnitDefs[u].Defense * b
		cavalry += float64(n) * UnitDefs[u].DefenseCavalry * b
		archer += float64(n) * UnitDefs[u].DefenseArcher * b
	}
	return
}

func resolveScout(in BattleInput) BattleResult {
	att := float64(in.Attacker.Units[UScout])
	def := float64(in.Defender.Units[UScout]) * CombatBalance.ScoutDefenseFactor
	res := BattleResult{
		Kind: "scout", AttackPower: att, DefensePower: def, Morale: in.Morale,
		DefenderSurvivors: in.Defender.Units, WallBefore: in.Defender.Wall, WallAfter: in.Defender.Wall,
	}
	if att > def {
		ratio := 0.0
		if def > 0 {
			ratio = math.Pow(def/att, CombatBalance.LossExponent)
		}
		res.Outcome = "attackerWon"
		res.AttackerLosses, res.AttackerSurvivors = DistributeLosses(in.Attacker.Units, ratio)
		return res
	}
	res.Outcome = "defenderWon"
	res.AttackerLosses = in.Attacker.Units
	return res
}

func ResolveBattle(in BattleInput) BattleResult {
	if IsScoutMission(in.Attacker.Units) {
		return resolveScout(in)
	}
	ap := attackPowerOf(in.Attacker)
	attackEff := ap.total * (1 + in.Luck) * in.Morale
	shareInf, shareCav, shareArch := 1.0, 0.0, 0.0
	if ap.total > 0 {
		shareInf, shareCav, shareArch = ap.infantry/ap.total, ap.cavalry/ap.total, ap.archer/ap.total
	}
	general, cavalry, archer := defensePowerOf(in.Defender.BattleSide)
	weighted := general*shareInf + cavalry*shareCav + archer*shareArch
	wall := in.Defender.Wall
	villageBase := CombatBalance.VillageBaseDefense + CombatBalance.VillageDefensePerWall*float64(wall)
	defenseEff := (weighted + villageBase) * math.Pow(CombatBalance.WallDefenseFactor, float64(wall))

	reduced := 1.0
	if in.LossMode == "reduced" {
		reduced = CombatBalance.ReducedLossFactor
	}
	res := BattleResult{Kind: "battle", AttackPower: attackEff, DefensePower: defenseEff, Luck: in.Luck,
		Morale: in.Morale, WallBefore: wall, WallAfter: wall}
	if attackEff > defenseEff {
		res.Outcome = "attackerWon"
		ratio := math.Pow(defenseEff/attackEff, CombatBalance.LossExponent) * reduced
		res.AttackerLosses, res.AttackerSurvivors = DistributeLosses(in.Attacker.Units, ratio)
		res.DefenderLosses = in.Defender.Units
	} else {
		res.Outcome = "defenderWon"
		ratio := 0.0
		if defenseEff > 0 {
			ratio = math.Pow(attackEff/defenseEff, CombatBalance.LossExponent) * reduced
		}
		res.DefenderLosses, res.DefenderSurvivors = DistributeLosses(in.Defender.Units, ratio)
		res.AttackerLosses = in.Attacker.Units
	}

	if res.Outcome == "attackerWon" {
		if rams := res.AttackerSurvivors[URam]; rams > 0 && wall > 0 {
			power := float64(rams) * researchFactor(in.Attacker.Research, URam) * (1 + in.Luck)
			res.WallAfter = BatterLevels(wall, power, WallHp, 0)
		}
		if cats := res.AttackerSurvivors[UCatapult]; cats > 0 {
			minOf := func(b BuildingID) int {
				if b == BMain {
					return 1
				}
				return 0
			}
			var target *BuildingID
			if in.CatapultTarget != nil && in.Defender.Buildings[*in.CatapultTarget] > minOf(*in.CatapultTarget) {
				target = in.CatapultTarget
			} else {
				var candidates []BuildingID
				for _, b := range BuildingOrder {
					if in.Defender.Buildings[b] > minOf(b) {
						candidates = append(candidates, b)
					}
				}
				if len(candidates) > 0 {
					t := candidates[int(math.Floor(in.Rng()*float64(len(candidates))))%len(candidates)]
					target = &t
				}
			}
			if target != nil {
				from := in.Defender.Buildings[*target]
				power := float64(cats) * researchFactor(in.Attacker.Research, UCatapult) * (1 + in.Luck) * CombatBalance.CatapultFactor
				if to := BatterLevels(from, power, BuildingHp, minOf(*target)); to != from {
					res.BuildingDamage = &BuildingDamage{Building: *target, From: from, To: to}
				}
			}
		}
		for _, u := range UnitIDs {
			res.HaulCapacity += float64(res.AttackerSurvivors[u]) * UnitDefs[u].Haul
		}
		res.Haul = ComputeHaul(res.HaulCapacity, in.Defender.Resources, in.Defender.HideLevel)
	}
	return res
}
