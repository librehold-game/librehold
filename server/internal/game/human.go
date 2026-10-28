package game

// NewHumanState is a new human's state: empty stats, quests and groups; default farm assistant and templates.
func NewHumanState() *HumanState {
	return &HumanState{
		Stats:  PlayerStats{PointsHistory: [][2]int64{}},
		Quests: QuestState{Completed: []string{}, Active: []string{}, Claimed: []string{}},
		FarmAssistant: FarmAssistantState{
			Templates:        map[string]FarmTemplate{"A": {}, "B": {}, "C": {}, "D": {}},
			AutoFarmVillages: []string{},
			Filters:          FarmBalance.DefaultFilters,
		},
		Trader:         TraderState{Offer: TraderOffer{Give: Wood, Get: Clay}},
		Groups:         []VillageGroup{},
		BuildTemplates: CloneBuildTemplates(),
	}
}

// humanOf returns the human state of a village owner, or nil for bots and barbarians.
func humanOf(w World, v *Village) (PlayerID, *HumanState) {
	if v.OwnerID == "" || !IsHumanID(PlayerID(v.OwnerID)) {
		return "", nil
	}
	id := PlayerID(v.OwnerID)
	return id, w.Human(id)
}
