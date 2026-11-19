package game

import (
	"encoding/json"
	"fmt"
)

// The bots' playbook: openings, role and troop targets, mistakes and plays.

type VillageRole string

const (
	RoleMixed VillageRole = "mixed"
	RoleOff   VillageRole = "off"
	RoleDef   VillageRole = "def"
)

// OpeningStep is one opening build step; it marshals as a [building, level] tuple.
type OpeningStep struct {
	Building BuildingID
	Level    int
}

func (s OpeningStep) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{s.Building, s.Level})
}

func (s *OpeningStep) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) != 2 {
		return fmt.Errorf("opening step: want 2 elements, got %d", len(raw))
	}
	if err := json.Unmarshal(raw[0], &s.Building); err != nil {
		return err
	}
	return json.Unmarshal(raw[1], &s.Level)
}

var Opening = []OpeningStep{
	{BWood, 1}, {BClay, 1}, {BIron, 1}, {BWood, 2}, {BClay, 2}, {BMain, 2}, {BMain, 3}, {BBarracks, 1},
	{BWood, 3}, {BClay, 3}, {BBarracks, 2}, {BStorage, 2}, {BIron, 2}, {BStorage, 3}, {BBarracks, 3},
	{BStatue, 1}, {BFarm, 2}, {BIron, 3}, {BWood, 4}, {BClay, 4}, {BMain, 5}, {BSmithy, 1}, {BWall, 1},
	{BHide, 3}, {BWood, 5}, {BClay, 5}, {BMarket, 1}, {BMain, 10}, {BSmithy, 5}, {BStable, 3}, {BStorage, 7},
}

var offTargets = map[BuildingID]int{
	BMain: 20, BBarracks: 25, BStable: 20, BWorkshop: 10, BSmithy: 20, BMarket: 10, BFarm: 30, BStorage: 28,
	BWood: 30, BClay: 30, BIron: 30, BWall: 20, BHide: 10,
}

func defTargets() map[BuildingID]int {
	out := map[BuildingID]int{}
	for k, v := range offTargets {
		if k != BWorkshop {
			out[k] = v
		}
	}
	out[BStable] = 10
	return out
}

var RoleTargets = map[VillageRole]map[BuildingID]int{
	RoleOff: offTargets,
	RoleDef: defTargets(),
	RoleMixed: {BMain: 15, BBarracks: 10, BStable: 10, BSmithy: 10, BFarm: 25, BStorage: 25, BWood: 25, BClay: 25,
		BIron: 25, BWall: 15, BHide: 10},
}

// TroopTargets is the army each role builds towards, and the only place those numbers live. PlanRecruitment trains
// towards them and stops when they are met, so a target is a cap, not a floor: it has to be the army the village is
// supposed to have.
//
// Scouts sit in every plan because information is what lets a bot act at all, and a scouting mission that is caught
// reports nothing. The sizes follow from the scouting rule itself: a party gets through only when it beats
// ScoutDefenseFactor per defending scout, so an offence village's scout line has to clear twice a defence village's
// home guard, with something left over for the reserve that lets it look twice.
//
//   - off keeps 1400: it sends a 900 party and still holds more than half a party, so a caught mission is replaced
//     rather than ending its reconnaissance. With 900 committed it sees a village holding under 450.
//   - def keeps 400: its job is holding a wall, where a scout defends as little as it attacks, so a home guard.
//   - mixed keeps 250: a single village that has to do everything at once.
var TroopTargets = map[VillageRole]map[UnitID]int{
	RoleOff:   {UAxe: 6000, ULightCav: 3000, URam: 250, UCatapult: 50, UScout: 1400},
	RoleDef:   {USpear: 8000, UHeavyCav: 2000, UScout: 400},
	RoleMixed: {USpear: 300, USword: 100, ULightCav: 600, UScout: 250},
}

// scoutShare is the most of an army's population a role may put into scouts. It is not a separate design goal but the
// arithmetic of the scouting rule: a party must be bigger than twice the guard it means to beat, and the village has to
// keep more than half a party back to look again, so an offence village's line is about 12% of its plan and no less. A
// defence village needs no fleet at all, only a home guard.
func scoutShare(role VillageRole) float64 {
	switch role {
	case RoleOff:
		return 0.13
	case RoleDef:
		return 0.05
	}
	return 0.05
}

// Mistakes are the chances that a bot makes a given mistake (strong / weak; linear in skill between).
var Mistakes = struct {
	HybridVillage         StrongWeak `json:"hybridVillage"`
	EcoTo30BeforeMilitary StrongWeak `json:"ecoTo30BeforeMilitary"`
	IdleResources         StrongWeak `json:"idleResources"`
	ShortQueues           StrongWeak `json:"shortQueues"`
	BarracksDayOne        StrongWeak `json:"barracksDayOne"`
}{StrongWeak{0.02, 0.5}, StrongWeak{0, 0.6}, StrongWeak{0.05, 0.9}, StrongWeak{0.05, 0.9}, StrongWeak{0, 0.3}}

var Plays = struct {
	LongBuildBeforeSleep StrongWeak `json:"longBuildBeforeSleep"`
}{StrongWeak{0.9, 0.05}}

// CatapultsPerLevel is the catapults needed to knock one level off a building (index = level).
var CatapultsPerLevel = []int{
	0, 2, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16,
}

// EconomyPlay is how a bot tier builds up on a normal world: the mines first, iron behind wood and clay, no idle
// build queue, a build plan for the break, a farming army and the auto-farm. The zero play is the classic one,
// which short worlds always play.
type EconomyPlay struct {
	MinesOpening bool    `json:"minesOpening"` // MinesOpening instead of Opening
	MinesFirstTo int     `json:"minesFirstTo"` // after the opening only economy buildings until the mines reach it
	IronBehind   int     `json:"ironBehind"`   // iron waits until it is this many levels behind wood and clay
	FillQueue    bool    `json:"fillQueue"`    // an unaffordable pick gives way to an affordable wanted building
	PlanSteps    int     `json:"planSteps"`    // the build plan left for the break
	FarmAt       float64 `json:"farmAt"`       // the farm grows when less than this share is free, queued levels counted
	StorageHours float64 `json:"storageHours"` // the storage grows while it holds less than this much top income
	HideCap      int     `json:"hideCap"`      // unless farmed hard
	WallCap      int     `json:"wallCap"`      // unless farmed hard
	RushDays     float64 `json:"rushDays"`     // the start check-ins (BotConfig.StartRush) last at least this long
	CavalryAt    int     `json:"cavalryAt"`    // CavalryPath once wood and clay reach this level; 0 = never
	ArmyShare    float64 `json:"armyShare"`    // troops recruited first, from half the purse, up to this share of the farm
	AutoFarm     bool    `json:"autoFarm"`     // raids on the auto-farm tick once unlocked, as humans' auto-farm
}

// TierPlays: within a tier, farming styles keep an army from the start (light cavalry early, over half the farm in
// troops) and the other styles put the mines first with a small army.
type TierPlays struct {
	Economy EconomyPlay `json:"economy"`
	Farming EconomyPlay `json:"farming"`
}

// CavalryPath: barracks 5, smithy 5 and stable 3 for light cavalry to farm with.
var CavalryPath = []OpeningStep{{BMain, 5}, {BBarracks, 3}, {BSmithy, 1}, {BBarracks, 5}, {BSmithy, 3}, {BSmithy, 5},
	{BStable, 1}, {BStable, 2}, {BStable, 3}}

// EconomyPlays: strong bots keep the economy first to mines 18, the farming ones with the stable from mines 8; medium
// ones turn to troops at 15 and farm a little.
var EconomyPlays = map[BotTier]TierPlays{
	TierStrong: {
		Economy: strongPlay(14, 0.1),
		Farming: strongPlay(8, 0.55),
	},
	TierMedium: {Economy: mediumPlay, Farming: mediumPlay},
}

func strongPlay(cavalryAt int, armyShare float64) EconomyPlay {
	return EconomyPlay{MinesOpening: true, MinesFirstTo: 18, IronBehind: 3, FillQueue: true, PlanSteps: 24, FarmAt: 0.25,
		StorageHours: 12, HideCap: 3, WallCap: 5, RushDays: 4, CavalryAt: cavalryAt, ArmyShare: armyShare, AutoFarm: true}
}

var mediumPlay = EconomyPlay{MinesOpening: true, MinesFirstTo: 15, IronBehind: 3, FillQueue: true, PlanSteps: 12,
	FarmAt: 0.25, StorageHours: 12, HideCap: 3, WallCap: 5, RushDays: 2, CavalryAt: 12, ArmyShare: 0.3, AutoFarm: true}

// MinesOpening takes wood and clay to 10 and iron to 8, with storage and main alongside, then a barracks. The farm
// follows EconomyPlay.FarmAt.
var MinesOpening = minesOpening()

func minesOpening() []OpeningStep {
	var out []OpeningStep
	for lvl := 1; lvl <= 10; lvl++ {
		out = append(out, OpeningStep{BWood, lvl}, OpeningStep{BClay, lvl})
		if lvl <= 3 {
			out = append(out, OpeningStep{BIron, lvl})
		} else if lvl%4 != 0 {
			out = append(out, OpeningStep{BIron, lvl - lvl/4})
		}
		switch {
		case lvl == 2:
			out = append(out, OpeningStep{BStorage, 2})
		case lvl == 4:
			out = append(out, OpeningStep{BStorage, 3}, OpeningStep{BMain, 2}, OpeningStep{BBarracks, 1})
		case lvl >= 5 && lvl%2 == 1:
			out = append(out, OpeningStep{BStorage, lvl - 1}, OpeningStep{BMain, lvl / 2})
		}
	}
	return append(out, OpeningStep{BStorage, 10}, OpeningStep{BMain, 7}, OpeningStep{BBarracks, 3}, OpeningStep{BSmithy, 1})
}
