package game

import "math"

// The 42 quests with their conditions as Go functions over one human; their
// titles and descriptions live in the client's catalogues.

type QuestReward struct {
	Resources      *Resources `json:"resources,omitempty"`
	BuildQueueSlot bool       `json:"buildQueueSlot,omitempty"` // +1 build queue slot (up to WorldLimits)
	AutoFarm       bool       `json:"autoFarm,omitempty"`
}

type QuestDef struct {
	ID        string
	Requires  []string // quests that must be completed for this one to become active
	Condition func(w World, h PlayerID) bool
	Progress  func(w World, h PlayerID) float64 // 0..1 for the UI bar; nil if none
	Reward    QuestReward
}

func res(wood, clay, iron float64) *Resources { return &Resources{wood, clay, iron} }

func maxBuildingOf(w World, h PlayerID, b BuildingID) int {
	m := 0
	for _, v := range PlayerVillages(w, h) {
		m = max(m, v.Buildings[b])
	}
	return m
}

// unitsOwned counts a human's units: present, queued, on the move and stationed as support.
func unitsOwned(w World, h PlayerID, u UnitID) int {
	n := 0
	for _, v := range PlayerVillages(w, h) {
		n += UnitCountIncludingQueued(v, u)
		n += UnitsAway(w, v.ID)[u]
	}
	return n
}

func maxResearchOf(w World, h PlayerID, u UnitID) int {
	m := 0
	for _, v := range PlayerVillages(w, h) {
		m = max(m, v.Research[u])
	}
	return m
}

func statsOf(w World, h PlayerID) PlayerStats {
	if hs := w.Human(h); hs != nil {
		return hs.Stats
	}
	return PlayerStats{}
}

func looted(w World, h PlayerID) float64 { return SumResources(statsOf(w, h).ResourcesLooted) }

func villageCount(w World, h PlayerID) int {
	if p := w.Player(h); p != nil {
		return len(p.VillageIDs)
	}
	return 0
}

func ratio(a, b float64) float64 { return math.Max(0, math.Min(1, a/b)) }

func buildingQuest(id string, b BuildingID, level int, requires []string, reward QuestReward) QuestDef {
	return QuestDef{ID: id, Requires: requires, Reward: reward,
		Condition: func(w World, h PlayerID) bool { return maxBuildingOf(w, h, b) >= level },
		Progress:  func(w World, h PlayerID) float64 { return ratio(float64(maxBuildingOf(w, h, b)), float64(level)) },
	}
}

func unitsQuest(id string, u UnitID, count int, requires []string, reward QuestReward) QuestDef {
	return QuestDef{ID: id, Requires: requires, Reward: reward,
		Condition: func(w World, h PlayerID) bool { return unitsOwned(w, h, u) >= count },
		Progress:  func(w World, h PlayerID) float64 { return ratio(float64(unitsOwned(w, h, u)), float64(count)) },
	}
}

func statQuest(id string, requires []string, reward QuestReward, ok func(w World, h PlayerID) bool) QuestDef {
	return QuestDef{ID: id, Requires: requires, Reward: reward, Condition: ok}
}

func countQuest(id string, requires []string, reward QuestReward, value func(w World, h PlayerID) float64, target float64) QuestDef {
	return QuestDef{ID: id, Requires: requires, Reward: reward,
		Condition: func(w World, h PlayerID) bool { return value(w, h) >= target },
		Progress:  func(w World, h PlayerID) float64 { return ratio(value(w, h), target) },
	}
}

func r(resources *Resources) QuestReward { return QuestReward{Resources: resources} }

func points(w World, h PlayerID) float64   { return float64(PlayerPoints(w, h)) }
func villages(w World, h PlayerID) float64 { return float64(villageCount(w, h)) }

// Quests run from the first clay pit to an empire. After the farm they split into lines that do not wait for each
// other: raiding (spearmen, battles, loot, auto-farm), defence (hiding place, wall), the smithy and the economy.
var Quests = []QuestDef{
	buildingQuest("wood1", BWood, 1, nil, r(res(60, 40, 20))),
	buildingQuest("clay1", BClay, 1, []string{"wood1"}, r(res(40, 60, 20))),
	buildingQuest("iron1", BIron, 1, []string{"clay1"}, r(res(40, 40, 60))),
	buildingQuest("main3", BMain, 3, []string{"iron1"}, r(res(100, 100, 50))),
	buildingQuest("storage3", BStorage, 3, []string{"main3"}, r(res(150, 150, 100))),
	buildingQuest("wood5", BWood, 5, []string{"storage3"}, r(res(200, 0, 0))),
	buildingQuest("clay5", BClay, 5, []string{"wood5"}, r(res(0, 200, 0))),
	buildingQuest("iron5", BIron, 5, []string{"clay5"}, r(res(0, 0, 200))),
	buildingQuest("farm5", BFarm, 5, []string{"iron5"}, r(res(100, 100, 100))),
	buildingQuest("barracks1", BBarracks, 1, []string{"farm5"}, r(res(50, 50, 50))),
	unitsQuest("spear10", USpear, 10, []string{"barracks1"}, r(res(100, 100, 100))),
	buildingQuest("hide3", BHide, 3, []string{"farm5"}, r(res(150, 150, 150))),
	buildingQuest("smithy1", BSmithy, 1, []string{"barracks1"}, r(res(100, 100, 200))),
	statQuest("research_sword", []string{"smithy1"}, r(res(200, 200, 200)),
		func(w World, h PlayerID) bool { return maxResearchOf(w, h, USword) >= 1 }),
	buildingQuest("wall1", BWall, 1, []string{"hide3"}, r(res(100, 100, 100))),
	statQuest("first_attack", []string{"spear10"}, r(res(100, 100, 100)),
		func(w World, h PlayerID) bool { return statsOf(w, h).AttacksSent >= 1 }),
	statQuest("first_battle", []string{"first_attack"}, r(res(500, 500, 500)),
		func(w World, h PlayerID) bool { return statsOf(w, h).BattlesWon >= 1 }),
	countQuest("loot5000", []string{"first_battle"},
		QuestReward{Resources: res(300, 300, 300), AutoFarm: true}, looted, 5000),
	statQuest("template_a", []string{"first_battle"}, r(res(200, 200, 200)),
		func(w World, h PlayerID) bool {
			hs := w.Human(h)
			return hs != nil && hs.FarmAssistant.Templates["A"].Units.Total() > 0
		}),
	buildingQuest("main10", BMain, 10, []string{"farm5"}, r(res(500, 500, 500))),
	buildingQuest("stable1", BStable, 1, []string{"main10"}, r(res(300, 300, 300))),
	statQuest("scout", []string{"stable1"}, r(res(200, 200, 100)),
		func(w World, h PlayerID) bool { return statsOf(w, h).ScoutsSent >= 1 }),
	unitsQuest("light_cav10", ULightCav, 10, []string{"stable1"}, r(res(500, 500, 500))),
	buildingQuest("market1", BMarket, 1, []string{"main10"}, r(res(300, 300, 300))),
	statQuest("trade", []string{"market1"}, r(res(400, 400, 400)),
		func(w World, h PlayerID) bool {
			hs := w.Human(h)
			return hs != nil && (hs.Stats.TradesSent >= 1 || hs.Trader.Accepted)
		}),
	{ID: "resources10", Requires: []string{"main10"},
		Reward: r(res(1000, 1000, 1000)),
		Condition: func(w World, h PlayerID) bool {
			return maxBuildingOf(w, h, BWood) >= 10 && maxBuildingOf(w, h, BClay) >= 10 && maxBuildingOf(w, h, BIron) >= 10
		},
		Progress: func(w World, h PlayerID) float64 {
			return ratio(float64(maxBuildingOf(w, h, BWood)+maxBuildingOf(w, h, BClay)+maxBuildingOf(w, h, BIron)), 30)
		}},
	countQuest("points1000", []string{"main10"}, r(res(1000, 1000, 1000)), points, 1000),
	buildingQuest("workshop1", BWorkshop, 1, []string{"resources10"}, r(res(500, 500, 500))),
	unitsQuest("rams5", URam, 5, []string{"workshop1"}, r(res(600, 600, 600))),
	buildingQuest("wall10", BWall, 10, []string{"rams5"}, r(res(800, 800, 800))),
	countQuest("loot100k", []string{"loot5000"}, r(res(2000, 2000, 2000)), looted, 100_000),
	buildingQuest("main20", BMain, 20, []string{"points1000"}, r(res(3000, 3000, 3000))),
	buildingQuest("smithy20", BSmithy, 20, []string{"main20"}, r(res(3000, 3000, 3000))),
	buildingQuest("academy1", BAcademy, 1, []string{"smithy20"}, r(res(5000, 5000, 5000))),
	statQuest("coin", []string{"academy1"}, r(res(3000, 3000, 3000)),
		func(w World, h PlayerID) bool { return statsOf(w, h).CoinsMinted >= 1 || w.Config().NoCoins }),
	unitsQuest("noble", UNoble, 1, []string{"coin"}, r(res(5000, 5000, 5000))),
	statQuest("conquest1", []string{"noble"}, QuestReward{BuildQueueSlot: true},
		func(w World, h PlayerID) bool { return statsOf(w, h).VillagesConquered >= 1 }),
	countQuest("villages3", []string{"conquest1"}, r(res(10_000, 10_000, 10_000)), villages, 3),
	countQuest("villages5", []string{"villages3"}, QuestReward{BuildQueueSlot: true}, villages, 5),
	countQuest("points10000", []string{"main20"}, r(res(10_000, 10_000, 10_000)), points, 10_000),
	countQuest("kills10000", []string{"loot100k"}, r(res(10_000, 10_000, 10_000)),
		func(w World, h PlayerID) float64 { return float64(statsOf(w, h).UnitsKilled) }, 10_000),
	countQuest("villages10", []string{"villages5"}, QuestReward{BuildQueueSlot: true}, villages, 10),
}

// QuestByID indexes Quests.
var QuestByID = func() map[string]*QuestDef {
	out := map[string]*QuestDef{}
	for i := range Quests {
		if Quests[i].Requires == nil {
			Quests[i].Requires = []string{}
		}
		out[Quests[i].ID] = &Quests[i]
	}
	return out
}()
