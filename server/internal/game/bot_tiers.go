package game

import "slices"

// BotTier groups bots by skill: weak, medium and strong players build up differently.
type BotTier string

const (
	TierWeak   BotTier = "weak"
	TierMedium BotTier = "medium"
	TierStrong BotTier = "strong"
)

func TierOf(skill float64) BotTier {
	switch {
	case skill >= BotConfig.StrongSkill:
		return TierStrong
	case skill >= BotConfig.MediumSkill:
		return TierMedium
	}
	return TierWeak
}

// economyPlay is the bot's play for its tier and style on a normal world; short worlds play the classic one.
func economyPlay(w World, botID PlayerID) EconomyPlay {
	if IsShort(w.Config()) {
		return EconomyPlay{}
	}
	b := brainOf(w, botID)
	plays := EconomyPlays[TierOf(b.Skill)]
	if farmingStyle(b.Style) {
		return plays.Farming
	}
	return plays.Economy
}

func farmingStyle(s BotStyle) bool {
	return s == StyleFarmer || s == StyleAggressor || s == StyleOpportunist
}

// cavalryStep is the next CavalryPath step once wood and clay reach CavalryAt.
func (p EconomyPlay) cavalryStep(levels Buildings) (BuildingID, bool) {
	if p.CavalryAt == 0 || min(levels[BWood], levels[BClay]) < p.CavalryAt {
		return 0, false
	}
	for _, s := range CavalryPath {
		if levels[s.Building] < s.Level && MeetsRequirements(levels, s.Building) {
			return s.Building, true
		}
	}
	return 0, false
}

// cavalrySavings: with the stable up and light cavalry not yet researched, its price is kept from building.
func (p EconomyPlay) cavalrySavings(w World, v *Village) Resources {
	if p.CavalryAt == 0 || EffectiveResearchLevel(v, ULightCav) >= 1 || len(v.ResearchQueue) > 0 {
		return Resources{}
	}
	for _, b := range CheckResearch(w, v, ULightCav).Blockers {
		if b.Kind() != "resources" {
			return Resources{}
		}
	}
	return ResearchCost(ULightCav, 1)
}

// armyShort: the troops (home, away and in training) fill less than ArmyShare of the farm.
func (p EconomyPlay) armyShort(w World, v *Village) bool {
	troops := PopulationUsed(w, v) - BuildingsPopulation(EffectiveBuildingLevels(v))
	return p.ArmyShare > 0 && float64(troops) < p.ArmyShare*float64(VillageFarmCapacity(v))
}

func maxResources(a, b Resources) Resources {
	return Resources{Wood: max(a.Wood, b.Wood), Clay: max(a.Clay, b.Clay), Iron: max(a.Iron, b.Iron)}
}

// minesFirst: after the opening the play builds only the economy while wood or clay is below MinesFirstTo, or iron
// below it less IronBehind.
func (p EconomyPlay) minesFirst(levels Buildings) bool {
	return levels[BWood] < p.MinesFirstTo || levels[BClay] < p.MinesFirstTo || levels[BIron] < p.MinesFirstTo-p.IronBehind
}

// farmShort: less than FarmAt of the farm is free, with the queued farm levels counted.
func (p EconomyPlay) farmShort(w World, v *Village, levels Buildings) bool {
	capacity := FarmCapacity(levels[BFarm], v.Bonus)
	return levels[BFarm] < BuildingDefs[BFarm].MaxLevel && float64(PopulationUsed(w, v)) > (1-p.FarmAt)*capacity
}

// storageShort: the storage, queued levels counted, holds less than StorageHours of the village's top income —
// production plus the owner's loot per hour and village (the recent rate or the average, whichever is more), as
// farming fills a storage much sooner.
func (p EconomyPlay) storageShort(w World, v *Village, levels Buildings) bool {
	return levels[BStorage] < BuildingDefs[BStorage].MaxLevel &&
		StorageCapacity(levels[BStorage], v.Bonus) < p.StorageHours*topIncomePerHour(w, v)
}

func topIncomePerHour(w World, v *Village) float64 {
	var loot Resources
	owner := w.Player(PlayerID(v.OwnerID))
	if b := w.Brain(PlayerID(v.OwnerID)); b != nil && owner != nil && len(owner.VillageIDs) > 0 {
		hours := max(1, float64(w.Now()-owner.JoinAt)/float64(MsPerHour))
		villages := float64(len(owner.VillageIDs))
		for _, r := range ResourceIDs {
			loot.Set(r, max(b.Looted.Get(r)/hours, b.RecentLoot.PerHour/float64(len(ResourceIDs)))/villages)
		}
	}
	top := 0.0
	for _, r := range ResourceIDs {
		top = max(top, VillageProductionPerHour(v, r, w.Config())+loot.Get(r))
	}
	return top
}

// ironWaits: iron is not yet IronBehind levels behind the lower of wood and clay.
func (p EconomyPlay) ironWaits(levels Buildings) bool {
	lower := min(levels[BWood], levels[BClay])
	return p.IronBehind > 0 && lower < BuildingDefs[BWood].MaxLevel && levels[BIron] > lower-p.IronBehind
}

// target caps the hiding place and the wall of a bot that is not farmed hard.
func (p EconomyPlay) target(w World, botID PlayerID, b BuildingID, lvl int) int {
	switch {
	case b == BHide && p.HideCap > 0 && HardFarmer(w, botID) == "":
		return min(lvl, p.HideCap)
	case b == BWall && p.WallCap > 0 && HardFarmer(w, botID) == "":
		return min(lvl, p.WallCap)
	}
	return lvl
}

// fallbackBuild: the lowest affordable mine that is wanted, else any wanted affordable building.
func fallbackBuild(w World, botID PlayerID, v *Village) (BuildingID, bool) {
	options := affordableBuilds(w, botID, v)
	if len(options) == 0 {
		return 0, false
	}
	levels := EffectiveBuildingLevels(v)
	best, mine := options[0].b, false
	for _, o := range options {
		if (o.b == BWood || o.b == BClay || o.b == BIron) && (!mine || levels[o.b] < levels[best]) {
			best, mine = o.b, true
		}
	}
	return best, true
}

// setOfflinePlan leaves the next steps the bot would pick as the village's build plan for the break. They are
// picked on a copy of the village whose queue grows by each step, with an empty purse so that a full storage does
// not repeat.
func setOfflinePlan(w World, botID PlayerID, v *Village, n int) {
	sim := *v
	sim.BuildQueue = slices.Clone(v.BuildQueue)
	sim.Resources = Resources{}
	rng := Mulberry32(HiddenSeed(w, "offlinePlan", botID, v.ID, w.Now()))
	var steps []BuildingID
	for range n {
		b, ok := NextBuilding(w, botID, &sim, rng)
		if !ok {
			break
		}
		steps = append(steps, b)
		sim.BuildQueue = append(sim.BuildQueue, BuildJob{Building: b, TargetLevel: EffectiveBuildingLevels(&sim)[b] + 1, Kind: "build"})
	}
	if len(steps) == 0 || economyPlay(w, botID).cavalrySavings(w, v) != (Resources{}) {
		return
	}
	v.BuildPlan = steps
	schedulePlan(w, v.ID)
}
