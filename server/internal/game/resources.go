package game

import "math"

// Resources: production, storage and lazy accrual (population with units away lives in units.go).

// MineOf is the building that produces a resource (it shares the resource's id).
func MineOf(r Resource) BuildingID {
	return [...]BuildingID{BWood, BClay, BIron}[r]
}

func BonusProductionMultiplier(bonus *VillageBonus, r Resource) float64 {
	if bonus == nil {
		return 1
	}
	if string(bonus.Kind) == r.String() {
		return BonusBalance.SingleResource
	}
	if bonus.Kind == "allResources" {
		return BonusBalance.AllResources
	}
	return 1
}

// VillageProductionPerHour includes the base 5/h, the bonus, the world speed and the side's multiplier.
func VillageProductionPerHour(v *Village, r Resource, cfg *WorldConfig) float64 {
	fromBuilding := ProductionPerHour(v.Buildings[MineOf(r)])
	rate := (ResourceBalance.BaseProductionPerHour + fromBuilding) * BonusProductionMultiplier(v.Bonus, r)
	return rate * cfg.WorldSpeed * sideModifiers(cfg, v.OwnerID).Resources
}

func VillageStorageCapacity(v *Village) float64 {
	return StorageCapacity(v.Buildings[BStorage], v.Bonus)
}

func VillageFarmCapacity(v *Village) int {
	return int(FarmCapacity(v.Buildings[BFarm], v.Bonus))
}

// Accrue moves a village's resources forward to now. It is linear in time and capped by storage, so one
// jump and many small steps give the same result.
func Accrue(v *Village, now int64, cfg *WorldConfig) {
	if now <= v.ResourcesUpdatedAt {
		return
	}
	hours := float64(now-v.ResourcesUpdatedAt) / float64(MsPerHour)
	capacity := VillageStorageCapacity(v)
	for _, r := range ResourceIDs {
		rate := VillageProductionPerHour(v, r, cfg)
		current := v.Resources.Get(r)
		// Resources above capacity (e.g. from loot) do not grow, and are not cut down here either.
		if current < capacity {
			v.Resources.Set(r, math.Min(capacity, current+rate*hours))
		}
	}
	v.ResourcesUpdatedAt = now
}

// MsUntilFull is the world ms until storage is full for a resource (+Inf with no production).
func MsUntilFull(v *Village, r Resource, cfg *WorldConfig) float64 {
	capacity := VillageStorageCapacity(v)
	rate := VillageProductionPerHour(v, r, cfg)
	if v.Resources.Get(r) >= capacity {
		return 0
	}
	if rate <= 0 {
		return math.Inf(1)
	}
	return ((capacity - v.Resources.Get(r)) / rate) * float64(MsPerHour)
}

// MsUntilAmount is the world ms until `amount` of a resource is available (0 if it already is).
func MsUntilAmount(v *Village, r Resource, amount float64, cfg *WorldConfig) float64 {
	if v.Resources.Get(r) >= amount {
		return 0
	}
	capacity := VillageStorageCapacity(v)
	if amount > capacity {
		return math.Inf(1)
	}
	rate := VillageProductionPerHour(v, r, cfg)
	if rate <= 0 {
		return math.Inf(1)
	}
	return ((amount - v.Resources.Get(r)) / rate) * float64(MsPerHour)
}

func HasResources(have, cost Resources) bool {
	return have.Wood >= cost.Wood && have.Clay >= cost.Clay && have.Iron >= cost.Iron
}

func SubtractResources(target *Resources, cost Resources) {
	target.Wood -= cost.Wood
	target.Clay -= cost.Clay
	target.Iron -= cost.Iron
}

// AddResourcesCapped adds resources up to storage capacity and returns what was accepted.
func AddResourcesCapped(v *Village, amount Resources) Resources {
	capacity := VillageStorageCapacity(v)
	var accepted Resources
	for _, r := range ResourceIDs {
		room := math.Max(0, capacity-v.Resources.Get(r))
		a := math.Min(room, amount.Get(r))
		accepted.Set(r, a)
		v.Resources.Set(r, v.Resources.Get(r)+a)
	}
	return accepted
}

func ScaleResources(set Resources, f float64) Resources {
	return Resources{set.Wood * f, set.Clay * f, set.Iron * f}
}

func SumResources(set Resources) float64 {
	return set.Wood + set.Clay + set.Iron
}
