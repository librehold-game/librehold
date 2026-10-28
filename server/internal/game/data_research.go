package game

// Research unlock costs. Level 2 costs 3×, level 3 9× (UnitBalance.ResearchCostMultiplier).
var ResearchUnlockCost = [NumUnits]Resources{
	USpear:         {500, 300, 100},
	USword:         {300, 300, 700},
	UAxe:           {600, 300, 400},
	UArcher:        {1000, 300, 600},
	UScout:         {500, 500, 200},
	ULightCav:      {1250, 1000, 2500},
	UMountedArcher: {2500, 1000, 1500},
	UHeavyCav:      {2000, 1500, 6000},
	URam:           {3000, 2000, 2000},
	UCatapult:      {3200, 4000, 1000},
	UPaladin:       {200, 200, 400},
	UNoble:         {0, 0, 0},
}

var ResearchBalance = struct {
	MaxLevel          int   `json:"maxLevel"`
	MinResearchTimeMs int64 `json:"minResearchTimeMs"`
}{3, 1000}
