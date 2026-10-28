package game

// VillagePopulationUsed is the population taken at home: buildings (including queued levels — population is
// reserved when ordered), units in the village and units in recruitment queues.
func VillagePopulationUsed(v *Village) int {
	levels := v.Buildings
	for _, job := range v.BuildQueue {
		levels[job.Building] = max(levels[job.Building], job.TargetLevel)
	}
	pop := BuildingsPopulation(levels)
	for _, u := range UnitIDs {
		pop += v.Units[u] * UnitDefs[u].Pop
	}
	for _, b := range RecruitBuildings {
		for _, job := range *v.RecruitQueues.Q(b) {
			pop += (job.Total - job.Done) * UnitDefs[job.Unit].Pop
		}
	}
	return pop
}

// PopulationUsed adds the village's units away from home to VillagePopulationUsed.
func PopulationUsed(w World, v *Village) int {
	pop := VillagePopulationUsed(v)
	if v.OwnerID != "" {
		away := UnitsAway(w, v.ID)
		for _, u := range UnitIDs {
			pop += away[u] * UnitDefs[u].Pop
		}
	}
	return pop
}

// UnitsAway counts a village's units outside it: on the move, and stationed as support in any village.
func UnitsAway(w World, villageID string) Units {
	var out Units
	for _, c := range w.CommandsHome(villageID) {
		for _, u := range UnitIDs {
			out[u] += c.Units[u]
		}
	}
	for _, host := range w.SupportHosts(villageID) {
		sup := host.SupportFrom[villageID]
		for _, u := range UnitIDs {
			out[u] += sup[u]
		}
	}
	return out
}
