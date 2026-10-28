package game

// QuestExport is a quest as the browser lists it: its conditions and progress stay on the server.
type QuestExport struct {
	ID       string      `json:"id"`
	Requires []string    `json:"requires"`
	Reward   QuestReward `json:"reward"`
}

// BalanceExport returns the world-invariant tables the browser reads, under their TypeScript export names;
// cmd/gen writes them to src/data/balance.json.
func BalanceExport() map[string]any {
	buildings := map[string]BuildingDef{}
	for _, id := range BuildingIDs {
		buildings[id.String()] = BuildingDefs[id]
	}
	units := map[string]UnitDef{}
	unlock := map[string]Resources{}
	for _, id := range UnitIDs {
		units[id.String()] = UnitDefs[id]
		unlock[id.String()] = ResearchUnlockCost[id]
	}
	quests := make([]QuestExport, len(Quests))
	for i, q := range Quests {
		requires := q.Requires
		if requires == nil {
			requires = []string{}
		}
		quests[i] = QuestExport{ID: q.ID, Requires: requires, Reward: q.Reward}
	}
	return map[string]any{
		"BUILDINGS":            buildings,
		"BUILDING_ORDER":       BuildingOrder,
		"BUILDING_BALANCE":     BuildingBalance,
		"RESOURCE_BALANCE":     ResourceBalance,
		"BONUS_BALANCE":        BonusBalance,
		"UNITS":                units,
		"COMBAT_BALANCE":       CombatBalance,
		"UNIT_BALANCE":         UnitBalance,
		"RESEARCH_UNLOCK_COST": unlock,
		"RESEARCH_BALANCE":     ResearchBalance,
		"CONQUEST_BALANCE":     ConquestBalance,
		"DEFAULT_WORLD_CONFIG": DefaultWorldConfig,
		"WORLD_LIMITS":         WorldLimits,
		"MAP":                  MapConfig,
		"COMMAND_BALANCE":      CommandBalance,
		"FARM_BALANCE":         FarmBalance,
		"QUESTS":               quests,
	}
}
