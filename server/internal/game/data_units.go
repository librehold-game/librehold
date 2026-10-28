package game

// The units' table and the combat and unit constants.

type AttackClass string

const (
	ClassInfantry AttackClass = "infantry"
	ClassCavalry  AttackClass = "cavalry"
	ClassArcher   AttackClass = "archer"
)

type UnitDef struct {
	ID             UnitID                `json:"id"`
	Cost           Resources             `json:"cost"`
	Pop            int                   `json:"pop"`
	Attack         float64               `json:"attack"`
	Defense        float64               `json:"defense"`
	DefenseCavalry float64               `json:"defenseCavalry"`
	DefenseArcher  float64               `json:"defenseArcher"`
	Speed          float64               `json:"speed"` // minutes per tile
	Haul           float64               `json:"haul"`
	BaseRecruitSec float64               `json:"baseRecruitSec"`
	Building       BuildingID            `json:"building"`
	AttackClass    AttackClass           `json:"attackClass"`
	Requirements   []BuildingRequirement `json:"requirements"`
}

var UnitDefs = [NumUnits]UnitDef{
	USpear:         {USpear, Resources{50, 30, 10}, 1, 10, 15, 45, 20, 18, 25, 1020, BBarracks, ClassInfantry, []BuildingRequirement{req(BBarracks, 1)}},
	USword:         {USword, Resources{30, 30, 70}, 1, 25, 50, 15, 40, 22, 15, 1500, BBarracks, ClassInfantry, []BuildingRequirement{req(BBarracks, 1), req(BSmithy, 1)}},
	UAxe:           {UAxe, Resources{60, 30, 40}, 1, 40, 10, 5, 10, 18, 10, 1320, BBarracks, ClassInfantry, []BuildingRequirement{req(BBarracks, 1), req(BSmithy, 1)}},
	UArcher:        {UArcher, Resources{100, 30, 60}, 1, 15, 50, 40, 5, 18, 10, 1800, BBarracks, ClassArcher, []BuildingRequirement{req(BBarracks, 1), req(BSmithy, 1)}},
	UScout:         {UScout, Resources{50, 50, 20}, 2, 0, 2, 1, 2, 9, 0, 900, BStable, ClassCavalry, []BuildingRequirement{req(BStable, 1), req(BSmithy, 5)}},
	ULightCav:      {ULightCav, Resources{125, 100, 250}, 4, 130, 30, 40, 30, 10, 80, 1800, BStable, ClassCavalry, []BuildingRequirement{req(BStable, 3), req(BSmithy, 5)}},
	UMountedArcher: {UMountedArcher, Resources{250, 100, 150}, 5, 120, 40, 30, 50, 10, 50, 2700, BStable, ClassArcher, []BuildingRequirement{req(BStable, 5), req(BSmithy, 10)}},
	UHeavyCav:      {UHeavyCav, Resources{200, 150, 600}, 6, 150, 200, 80, 180, 11, 50, 3600, BStable, ClassCavalry, []BuildingRequirement{req(BStable, 10), req(BSmithy, 15)}},
	URam:           {URam, Resources{300, 200, 200}, 5, 2, 20, 50, 20, 30, 0, 4800, BWorkshop, ClassInfantry, []BuildingRequirement{req(BWorkshop, 1)}},
	UCatapult:      {UCatapult, Resources{320, 400, 100}, 8, 100, 100, 50, 100, 30, 0, 7200, BWorkshop, ClassInfantry, []BuildingRequirement{req(BWorkshop, 5)}},
	UPaladin:       {UPaladin, Resources{20, 20, 40}, 10, 150, 250, 400, 150, 10, 100, 21600, BStatue, ClassCavalry, []BuildingRequirement{req(BStatue, 1)}},
	UNoble:         {UNoble, Resources{40000, 50000, 50000}, 100, 30, 100, 50, 100, 35, 0, 18000, BAcademy, ClassInfantry, []BuildingRequirement{req(BAcademy, 1)}},
}

var CombatBalance = struct {
	LuckRange             float64 `json:"luckRange"`
	MoraleVsBarbarian     float64 `json:"moraleVsBarbarian"`
	VillageBaseDefense    float64 `json:"villageBaseDefense"`
	VillageDefensePerWall float64 `json:"villageDefensePerWall"`
	WallDefenseFactor     float64 `json:"wallDefenseFactor"`
	LossExponent          float64 `json:"lossExponent"`
	ReducedLossFactor     float64 `json:"reducedLossFactor"`
	WallHpA               float64 `json:"wallHpA"`
	WallHpB               float64 `json:"wallHpB"`
	BuildingHpA           float64 `json:"buildingHpA"`
	BuildingHpB           float64 `json:"buildingHpB"`
	CatapultFactor        float64 `json:"catapultFactor"`
	ScoutDefenseFactor    float64 `json:"scoutDefenseFactor"`
	MaxReports            int     `json:"maxReports"`
}{0.25, 1.0, 0, 20, 1.037, 1.5, 0.5, 0.27, 0.7, 0.39, 0.8, 0.8, 2, 500}

var UnitBalance = struct {
	RecruitTimeFactor          float64   `json:"recruitTimeFactor"`
	MinRecruitTimeMs           int64     `json:"minRecruitTimeMs"`
	ResearchBonus              []float64 `json:"researchBonus"`
	ResearchCostMultiplier     []float64 `json:"researchCostMultiplier"`
	ResearchTimePerResourceSec float64   `json:"researchTimePerResourceSec"`
}{0.9666, 1000, []float64{0, 0, 0.25, 0.4}, []float64{0, 1, 3, 9}, 0.3}
