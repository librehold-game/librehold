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

var TroopTargets = map[VillageRole]map[UnitID]int{
	RoleOff:   {UAxe: 6000, ULightCav: 3000, URam: 250, UCatapult: 50, UScout: 100},
	RoleDef:   {USpear: 8000, UHeavyCav: 2000, UScout: 100},
	RoleMixed: {USpear: 300, USword: 100, ULightCav: 600, UScout: 50},
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
