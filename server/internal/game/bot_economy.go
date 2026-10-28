package game

import (
	"math"
	"slices"
	"sort"
	"strconv"
)

// Bot economy: what bots build and recruit. Records keyed by building or unit iterate in BuildingIDs / UnitIDs order.

var resourceBuildings = []BuildingID{BWood, BClay, BIron, BStorage, BFarm, BMain}

func mistakeRule(key string) StrongWeak {
	switch key {
	case "hybridVillage":
		return Mistakes.HybridVillage
	case "ecoTo30BeforeMilitary":
		return Mistakes.EcoTo30BeforeMilitary
	case "idleResources":
		return Mistakes.IdleResources
	case "shortQueues":
		return Mistakes.ShortQueues
	case "barracksDayOne":
		return Mistakes.BarracksDayOne
	}
	panic("unknown mistake " + key)
}

// makesMistake is a fixed draw per bot (and salt): the same bot makes the same mistake consistently.
func makesMistake(w World, botID PlayerID, key, salt string) bool {
	rng := Mulberry32(HiddenSeed(w, "mistake", botID, key, salt))
	return Follows(rng, brainOf(w, botID).Skill, mistakeRule(key))
}

// ChooseRole: the first village is mixed while the bot has one; later ones off or def by style and front line.
func ChooseRole(w World, botID PlayerID, v *Village) VillageRole {
	p := w.Player(botID)
	brain := brainOf(w, botID)
	if len(p.VillageIDs) < 2 {
		return RoleMixed
	}
	if known, ok := brain.Roles[v.ID]; ok {
		return known
	}
	role := RoleMixed
	if !makesMistake(w, botID, "hybridVillage", v.ID) {
		nearest := FrontlineDistance(w, botID, v)
		frontline := 0.0
		if nearest < 10 {
			frontline = 0.3 * (1 - nearest/10)
		}
		rng := Mulberry32(HiddenSeed(w, "role", botID, v.ID))
		if rng() < BotConfig.OffShare[brain.Style]-frontline {
			role = RoleOff
		} else {
			role = RoleDef
		}
	}
	brain.Roles[v.ID] = role
	return role
}

// openingFor: the opening, corrected for the "barracks on day one" mistake (weak players rush the military).
func openingFor(w World, botID PlayerID) []OpeningStep {
	if !makesMistake(w, botID, "barracksDayOne", "") {
		return Opening
	}
	early := []OpeningStep{{BMain, 2}, {BMain, 3}, {BBarracks, 1}, {BBarracks, 2}, {BBarracks, 3}}
	out := append([]OpeningStep{}, early...)
	for _, s := range Opening {
		if !slices.Contains(early, s) {
			out = append(out, s)
		}
	}
	return out
}

func roleTargetsInOrder(role VillageRole) []BuildingRequirement {
	var out []BuildingRequirement
	for _, b := range BuildingIDs {
		if lvl, ok := RoleTargets[role][b]; ok {
			out = append(out, BuildingRequirement{b, lvl})
		}
	}
	return out
}

func queuedBuilding(v *Village, b BuildingID) bool {
	for _, j := range v.BuildQueue {
		if j.Building == b {
			return true
		}
	}
	return false
}

// NextBuilding: a full farm comes first, a full storage next, then the opening, then the cheapest gap to the
// role's targets.
func NextBuilding(w World, botID PlayerID, v *Village, rng func() float64) (BuildingID, bool) {
	brain := brainOf(w, botID)
	levels := EffectiveBuildingLevels(v)
	capacity := float64(VillageFarmCapacity(v))
	if !queuedBuilding(v, BFarm) && levels[BFarm] < BuildingDefs[BFarm].MaxLevel &&
		capacity-float64(PopulationUsed(w, v)) < BotConfig.BottleneckShare*capacity {
		return BFarm, true
	}
	storageCap := VillageStorageCapacity(v)
	if !queuedBuilding(v, BStorage) && levels[BStorage] < BuildingDefs[BStorage].MaxLevel {
		for _, r := range ResourceIDs {
			if v.Resources.Get(r) >= (1-BotConfig.BottleneckShare)*storageCap {
				return BStorage, true
			}
		}
	}
	// Farmed non-stop: wall and hiding place before anything else.
	if HardFarmer(w, botID) != "" {
		for _, c := range []BuildingRequirement{{BWall, 20}, {BHide, 10}} {
			if !queuedBuilding(v, c.Building) && levels[c.Building] < c.Level && MeetsRequirements(levels, c.Building) {
				return c.Building, true
			}
		}
	}
	for _, s := range openingFor(w, botID) {
		if levels[s.Building] < s.Level && MeetsRequirements(levels, s.Building) {
			return s.Building, true
		}
	}
	role := ChooseRole(w, botID, v)
	ecoFirst := makesMistake(w, botID, "ecoTo30BeforeMilitary", v.ID) &&
		(levels[BWood] < 30 || levels[BClay] < 30 || levels[BIron] < 30)
	var candidates []Option[BuildingID]
	for _, t := range roleTargetsInOrder(role) {
		b := t.Building
		if levels[b] >= t.Level || levels[b] >= BuildingDefs[b].MaxLevel || !MeetsRequirements(levels, b) {
			continue
		}
		if ecoFirst && !slices.Contains(resourceBuildings, b) {
			continue
		}
		candidates = append(candidates, Option[BuildingID]{b, -SumResources(BuildCost(b, levels[b]+1))})
	}
	if len(candidates) == 0 {
		return 0, false
	}
	return WeightedPick(rng, candidates, brain.Skill), true
}

// nobleSavings: in a short world a village with room in its academy keeps the next noble's price, so the bot goes
// on taking villages instead of spending the hour on buildings.
func nobleSavings(w World, v *Village) Resources {
	if !IsShort(w.Config()) || NobleLimit(w.Config(), v)-NobleCountOf(w, v) <= 0 {
		return Resources{}
	}
	return UnitCostIn(w.Config(), UNoble, 1)
}

// PlanBuilding fills free build queue slots. No resources for the chosen building = wait for the next session.
// Only the farm and the storage may dip into the noble's savings: the noble needs both.
func PlanBuilding(w World, botID PlayerID, v *Village) {
	rng := Mulberry32(HiddenSeed(w, "build", botID, v.ID, w.Now()))
	slots := BuildQueueSlotsFor(w, OwnerOf(w, v))
	savings := nobleSavings(w, v)
	for guard := 0; len(v.BuildQueue) < slots && guard < slots; guard++ {
		b, ok := NextBuilding(w, botID, v, rng)
		if !ok || len(CheckBuild(w, v, b).Blockers) > 0 {
			return
		}
		if b != BFarm && b != BStorage {
			left := v.Resources
			SubtractResources(&left, BuildCost(b, EffectiveBuildingLevels(v)[b]+1))
			if !HasResources(left, savings) {
				return
			}
		}
		act(w, botID, "build", villageArgs(v.ID, "building", b), func() error { _, err := EnqueueBuild(w, botID, v.ID, b); return err })
	}
}

func ownedUnits(w World, v *Village, u UnitID) int {
	return UnitCountIncludingQueued(v, u) + UnitsAway(w, v.ID)[u]
}

type recruitable struct {
	unit    UnitID
	deficit int
	fill    float64
}

// recruitableUnits are the role's units the village can already train, most lacking first. With beyondTargets
// the targets are soft: spending a surplus keeps training in the same proportions.
func recruitableUnits(w World, v *Village, role VillageRole, beyondTargets bool) []recruitable {
	var out []recruitable
	for _, u := range UnitIDs {
		target, ok := TroopTargets[role][u]
		if !ok || EffectiveResearchLevel(v, u) < 1 {
			continue
		}
		met := true
		for _, r := range UnitDefs[u].Requirements {
			if v.Buildings[r.Building] < r.Level {
				met = false
			}
		}
		if !met {
			continue
		}
		have := ownedUnits(w, v, u)
		fill := float64(have) / float64(target)
		if have < target {
			out = append(out, recruitable{u, target - have, fill})
		} else if beyondTargets {
			out = append(out, recruitable{u, math.MaxInt32, fill})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].fill < out[j].fill })
	return out
}

// popRoom is the farm room for troops. The reserve for buildings is BotConfig.FreePop, or near a maxed farm a
// share of capacity (buildings and nobles), since nothing frees room then and a full farm blocks building.
func popRoom(w World, v *Village) float64 {
	capacity := float64(VillageFarmCapacity(v))
	reserve := math.Min(float64(BotConfig.FreePop), 0.1*capacity)
	if v.Buildings[BFarm] >= BuildingDefs[BFarm].MaxLevel-5 {
		reserve = BotConfig.MaxedFarmReserve * capacity
	}
	return capacity - float64(PopulationUsed(w, v)) - reserve
}

// affordableUnits is how many units the budget (resources above the reserve) and the farm room allow.
func affordableUnits(w World, v *Village, u UnitID, reserve Resources) int {
	c := UnitDefs[u].Cost
	n := int(math.Floor(popRoom(w, v) / float64(UnitDefs[u].Pop)))
	for _, r := range ResourceIDs {
		if c.Get(r) > 0 {
			n = min(n, int(math.Floor((v.Resources.Get(r)-reserve.Get(r))/c.Get(r))))
		}
	}
	return max(0, n)
}

func CanRecruitMore(w World, botID PlayerID, v *Village) bool {
	for _, r := range recruitableUnits(w, v, ChooseRole(w, botID, v), true) {
		if affordableUnits(w, v, r.unit, Resources{}) > 0 {
			return true
		}
	}
	return false
}

func researchWanted(role VillageRole) []UnitID {
	out := []UnitID{UScout, ULightCav}
	for _, u := range UnitIDs {
		if _, ok := TroopTargets[role][u]; ok {
			out = append(out, u)
		}
	}
	return out
}

// CanSpendMore: a free slot with an affordable build, recruitment, or a research.
func CanSpendMore(w World, botID PlayerID, v *Village) bool {
	if len(v.BuildQueue) < BuildQueueSlotsFor(w, OwnerOf(w, v)) && len(affordableBuilds(w, botID, v)) > 0 {
		return true
	}
	if CanRecruitMore(w, botID, v) {
		return true
	}
	if len(v.ResearchQueue) > 0 {
		return false
	}
	for _, u := range researchWanted(ChooseRole(w, botID, v)) {
		if EffectiveResearchLevel(v, u) < 1 && len(CheckResearch(w, v, u).Blockers) == 0 {
			return true
		}
	}
	return false
}

func botRecruit(w World, botID PlayerID, v *Village, u UnitID, n int) bool {
	if n <= 0 || len(CheckRecruit(w, v, u, n).Blockers) > 0 {
		return false
	}
	return act(w, botID, "recruit", villageArgs(v.ID, "unit", u, "count", n), func() error {
		_, err := EnqueueRecruit(w, botID, v.ID, u, n)
		return err
	})
}

// IdlesNow: a weak player sometimes simply does nothing with resources at a given moment.
func IdlesNow(w World, botID PlayerID) bool {
	return makesMistake(w, botID, "idleResources", strconv.FormatInt(w.Now(), 10))
}

// PlanRecruitment trains towards the role's targets from the surplus above the next building's cost.
func PlanRecruitment(w World, botID PlayerID, v *Village) {
	role := ChooseRole(w, botID, v)
	var reserve Resources
	if next, ok := NextBuilding(w, botID, v, Mulberry32(HiddenSeed(w, "reserve", botID, v.ID, w.Now()))); ok {
		reserve = BuildCost(next, EffectiveBuildingLevels(v)[next]+1)
	}
	savings := nobleSavings(w, v)
	reserve = Resources{Wood: max(reserve.Wood, savings.Wood), Clay: max(reserve.Clay, savings.Clay), Iron: max(reserve.Iron, savings.Iron)}
	for _, r := range recruitableUnits(w, v, role, false) {
		botRecruit(w, botID, v, r.unit, min(r.deficit, affordableUnits(w, v, r.unit, reserve)))
	}
}

// PlanResearch: scouts and light cavalry first, then the role's units; never out of the noble's savings.
func PlanResearch(w World, botID PlayerID, v *Village) {
	if len(v.ResearchQueue) > 0 {
		return
	}
	savings := nobleSavings(w, v)
	for _, u := range researchWanted(ChooseRole(w, botID, v)) {
		check := CheckResearch(w, v, u)
		if EffectiveResearchLevel(v, u) >= 1 || len(check.Blockers) > 0 {
			continue
		}
		left := v.Resources
		SubtractResources(&left, check.Cost)
		if !HasResources(left, savings) {
			return
		}
		act(w, botID, "research", villageArgs(v.ID, "unit", u), func() error { _, err := EnqueueResearch(w, botID, v.ID, u); return err })
		return
	}
}

func recruitQueueEnd(v *Village, u UnitID, now int64) int64 {
	end := now
	for _, j := range *v.RecruitQueues.Q(UnitDefs[u].Building) {
		end = max(end, j.CompletesAt)
	}
	return end
}

type buildOption struct {
	b          BuildingID
	durationMs int64
}

// affordableBuilds are all buildings that can be queued now (opening, role targets, bottlenecks), with times.
func affordableBuilds(w World, botID PlayerID, v *Village) []buildOption {
	levels := EffectiveBuildingLevels(v)
	wanted := map[BuildingID]bool{BFarm: true, BStorage: true}
	for _, s := range Opening {
		if levels[s.Building] < s.Level {
			wanted[s.Building] = true
		}
	}
	for _, t := range roleTargetsInOrder(ChooseRole(w, botID, v)) {
		if levels[t.Building] < t.Level {
			wanted[t.Building] = true
		}
	}
	var out []buildOption
	for _, b := range BuildingIDs {
		if !wanted[b] {
			continue
		}
		if check := CheckBuild(w, v, b); len(check.Blockers) == 0 {
			out = append(out, buildOption{b, check.DurationMs})
		}
	}
	return out
}

// planNightBuilding: before sleep a strong player queues the longest builds it can afford, so the queue works
// through the break.
func planNightBuilding(w World, botID PlayerID, v *Village, gap int64) {
	brain := brainOf(w, botID)
	rng := Mulberry32(HiddenSeed(w, "nightBuild", botID, v.ID, w.Now()))
	if !Follows(rng, brain.Skill, Plays.LongBuildBeforeSleep) {
		PlanBuilding(w, botID, v)
		return
	}
	slots := BuildQueueSlotsFor(w, OwnerOf(w, v))
	for guard := 0; len(v.BuildQueue) < slots && guard < slots; guard++ {
		options := affordableBuilds(w, botID, v)
		if len(options) == 0 {
			break
		}
		opts := make([]Option[BuildingID], len(options))
		for i, o := range options {
			opts[i] = Option[BuildingID]{o.b, float64(min(o.durationMs, gap))}
		}
		b := WeightedPick(rng, opts, brain.Skill)
		act(w, botID, "build", villageArgs(v.ID, "building", b), func() error { _, err := EnqueueBuild(w, botID, v.ID, b); return err })
	}
}

// QueueForOffline, before leaving the game: the longest builds for the night, a recruitment queue for the whole
// break (the "short queues" mistake: 40 % of it) and spending down to half the storage (the "idle resources"
// mistake: not that). No farm room or resources ends the queue sooner — as for a person.
// In a short world a village saving for its academy or the next noble only spends the surplus, as when online.
func QueueForOffline(w World, botID PlayerID, v *Village, nextSessionAt int64) {
	if nobleSavings(w, v) != (Resources{}) {
		PlanBuilding(w, botID, v)
		PlanRecruitment(w, botID, v)
		PlanResearch(w, botID, v)
		return
	}
	role := ChooseRole(w, botID, v)
	gap := max(0, nextSessionAt-w.Now())
	planNightBuilding(w, botID, v, gap)
	share := 1.0
	if makesMistake(w, botID, "shortQueues", strconv.FormatInt(w.Now(), 10)) {
		share = 0.4
	}
	coverUntil := float64(w.Now()) + float64(gap)*share
	units := recruitableUnits(w, v, role, true)
	coverEnd := func() int64 {
		end := w.Now()
		for _, u := range units {
			end = max(end, recruitQueueEnd(v, u.unit, w.Now()))
		}
		return end
	}
	for _, r := range units {
		missing := coverUntil - float64(coverEnd())
		if missing <= 0 {
			break
		}
		perUnit := CheckRecruit(w, v, r.unit, 1).PerUnitMs
		botRecruit(w, botID, v, r.unit, min(r.deficit, int(math.Ceil(missing/float64(perUnit))), affordableUnits(w, v, r.unit, Resources{})))
	}
	if IdlesNow(w, botID) {
		return
	}
	half := VillageStorageCapacity(v) * 0.5
	for _, r := range recruitableUnits(w, v, role, true) {
		if v.Resources.Wood <= half && v.Resources.Clay <= half && v.Resources.Iron <= half {
			break
		}
		c := UnitDefs[r.unit].Cost
		over := math.MinInt32
		for _, res := range ResourceIDs {
			n := 0
			if c.Get(res) > 0 {
				n = int(math.Ceil((v.Resources.Get(res) - half) / c.Get(res)))
			}
			over = max(over, n)
		}
		botRecruit(w, botID, v, r.unit, min(r.deficit, over, affordableUnits(w, v, r.unit, Resources{})))
	}
	PlanResearch(w, botID, v)
	PlanBuilding(w, botID, v)
}
