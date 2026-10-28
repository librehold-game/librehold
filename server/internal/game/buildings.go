package game

import "math"

// Building formulas: cost, time, population, points, production and capacities by level.

func BuildCost(id BuildingID, level int) Resources {
	def := &BuildingDefs[id]
	l := float64(level - 1)
	return Resources{
		Wood: math.Round(def.BaseCost.Wood * math.Pow(def.CostFactor.Wood, l)),
		Clay: math.Round(def.BaseCost.Clay * math.Pow(def.CostFactor.Clay, l)),
		Iron: math.Round(def.BaseCost.Iron * math.Pow(def.CostFactor.Iron, l)),
	}
}

// BuildingPopulation is the total population of a building at a level.
func BuildingPopulation(id BuildingID, level int) int {
	if level <= 0 {
		return 0
	}
	def := &BuildingDefs[id]
	if def.BasePop == 0 {
		return 0
	}
	return int(math.Round(def.BasePop * math.Pow(def.PopFactor, float64(level-1))))
}

func BuildingPopulationDelta(id BuildingID, targetLevel int) int {
	return BuildingPopulation(id, targetLevel) - BuildingPopulation(id, targetLevel-1)
}

// BuildingPoints is the total points of a building at a level, not a sum over levels.
func BuildingPoints(id BuildingID, level int) int {
	if level <= 0 {
		return 0
	}
	return int(math.Round(BuildingDefs[id].BasePoints * math.Pow(BuildingBalance.PointsFactor, float64(level-1))))
}

func BuildingLevelPoints(id BuildingID, level int) int {
	return BuildingPoints(id, level) - BuildingPoints(id, level-1)
}

func VillagePoints(b Buildings) int {
	sum := 0
	for _, id := range BuildingIDs {
		sum += BuildingPoints(id, b[id])
	}
	return sum
}

// BuildTimeMs is the build time of a level in world ms.
func BuildTimeMs(id BuildingID, level, mainLevel int, worldSpeed float64) int64 {
	def := &BuildingDefs[id]
	b := &BuildingBalance
	exponent := b.TimeMinExponent
	if level > 1 {
		exponent = math.Max(b.TimeMinExponent, float64(level-1)-b.TimeShift/float64(level-1))
	}
	baseSec := def.BaseTimeSec * b.TimeConst * math.Pow(b.TimeFactor, exponent)
	mainFactor := math.Pow(b.MainTimeFactor, -math.Max(0, float64(mainLevel)))
	ms := (baseSec * mainFactor * 1000) / worldSpeed
	return max(b.MinBuildTimeMs, int64(math.Round(ms)))
}

func ProductionPerHour(level int) float64 {
	if level <= 0 {
		return 0
	}
	return math.Round(ResourceBalance.ProductionBase * math.Pow(ResourceBalance.ProductionFactor, float64(level-1)))
}

func StorageCapacity(level int, bonus *VillageBonus) float64 {
	base := ResourceBalance.StorageLevel0
	if level > 0 {
		base = math.Round(ResourceBalance.StorageBase * math.Pow(ResourceBalance.StorageFactor, float64(level-1)))
	}
	if bonus != nil && bonus.Kind == "storage" {
		return math.Round(base * BonusBalance.StorageCapacity)
	}
	return base
}

func FarmCapacity(level int, bonus *VillageBonus) float64 {
	base := ResourceBalance.FarmLevel0
	if level > 0 {
		base = math.Round(ResourceBalance.FarmBase * math.Pow(ResourceBalance.FarmFactor, float64(level-1)))
	}
	if bonus != nil && bonus.Kind == "farm" {
		return math.Round(base * BonusBalance.FarmCapacity)
	}
	return base
}

func HideCapacity(level int) float64 {
	if level <= 0 {
		return 0
	}
	l := min(level, BuildingDefs[BHide].MaxLevel)
	return math.Round(ResourceBalance.HideBase * math.Pow(ResourceBalance.HideFactor, float64(l-1)))
}

func BuildingsPopulation(b Buildings) int {
	sum := 0
	for _, id := range BuildingIDs {
		sum += BuildingPopulation(id, b[id])
	}
	return sum
}

func MeetsRequirements(b Buildings, id BuildingID) bool {
	for _, r := range BuildingDefs[id].Requirements {
		if b[r.Building] < r.Level {
			return false
		}
	}
	return true
}

func RecomputePoints(v *Village) {
	v.Points = VillagePoints(v.Buildings)
}
