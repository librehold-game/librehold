package game

// The buildings' table and the building, resource and bonus constants.

type BuildingRequirement struct {
	Building BuildingID `json:"building"`
	Level    int        `json:"level"`
}

type BuildingDef struct {
	ID           BuildingID            `json:"id"`
	MaxLevel     int                   `json:"maxLevel"`
	BaseCost     Resources             `json:"baseCost"`
	CostFactor   Resources             `json:"costFactor"`
	BasePop      float64               `json:"basePop"`
	PopFactor    float64               `json:"popFactor"`
	BaseTimeSec  float64               `json:"baseTimeSec"`
	BasePoints   float64               `json:"basePoints"`
	Requirements []BuildingRequirement `json:"requirements"`
}

func req(b BuildingID, level int) BuildingRequirement { return BuildingRequirement{b, level} }

var BuildingDefs = [NumBuildings]BuildingDef{
	BMain:     {BMain, 30, Resources{90, 80, 70}, Resources{1.26, 1.275, 1.26}, 5, 1.17, 900, 10, []BuildingRequirement{}},
	BBarracks: {BBarracks, 25, Resources{200, 170, 90}, Resources{1.26, 1.28, 1.26}, 7, 1.17, 1800, 16, []BuildingRequirement{req(BMain, 3)}},
	BStable:   {BStable, 20, Resources{270, 240, 260}, Resources{1.26, 1.28, 1.26}, 8, 1.17, 6000, 20, []BuildingRequirement{req(BBarracks, 5), req(BSmithy, 5)}},
	BWorkshop: {BWorkshop, 15, Resources{300, 240, 260}, Resources{1.26, 1.28, 1.26}, 8, 1.17, 6000, 24, []BuildingRequirement{req(BMain, 10), req(BSmithy, 10)}},
	BAcademy:  {BAcademy, 3, Resources{15000, 25000, 10000}, Resources{2, 2, 2}, 80, 1.17, 586800, 512, []BuildingRequirement{req(BMain, 20), req(BMarket, 10), req(BSmithy, 20)}},
	BSmithy:   {BSmithy, 20, Resources{220, 180, 240}, Resources{1.26, 1.275, 1.26}, 20, 1.17, 6000, 19, []BuildingRequirement{req(BMain, 5), req(BBarracks, 1)}},
	BStatue:   {BStatue, 1, Resources{220, 220, 220}, Resources{1.26, 1.275, 1.26}, 10, 1.17, 1500, 24, []BuildingRequirement{req(BMain, 1)}},
	BMarket:   {BMarket, 25, Resources{100, 100, 100}, Resources{1.26, 1.275, 1.26}, 20, 1.17, 2700, 10, []BuildingRequirement{req(BMain, 3), req(BStorage, 2)}},
	BWood:     {BWood, 30, Resources{50, 60, 40}, Resources{1.25, 1.275, 1.245}, 5, 1.155, 900, 6, []BuildingRequirement{}},
	BClay:     {BClay, 30, Resources{65, 50, 40}, Resources{1.27, 1.265, 1.24}, 10, 1.14, 900, 6, []BuildingRequirement{}},
	BIron:     {BIron, 30, Resources{75, 65, 70}, Resources{1.252, 1.275, 1.24}, 10, 1.17, 1080, 6, []BuildingRequirement{}},
	BFarm:     {BFarm, 30, Resources{45, 40, 30}, Resources{1.3, 1.32, 1.29}, 0, 1, 1200, 5, []BuildingRequirement{}},
	BStorage:  {BStorage, 30, Resources{60, 50, 40}, Resources{1.265, 1.27, 1.245}, 0, 1.15, 1020, 6, []BuildingRequirement{}},
	BHide:     {BHide, 10, Resources{50, 60, 50}, Resources{1.25, 1.25, 1.25}, 2, 1.17, 1800, 5, []BuildingRequirement{}},
	BWall:     {BWall, 20, Resources{50, 100, 20}, Resources{1.26, 1.275, 1.26}, 5, 1.17, 3600, 8, []BuildingRequirement{req(BBarracks, 1)}},
}

// BuildingOrder is the display order of the buildings table.
var BuildingOrder = []BuildingID{BMain, BBarracks, BStable, BWorkshop, BAcademy, BSmithy, BStatue,
	BMarket, BWood, BClay, BIron, BFarm, BStorage, BHide, BWall}

var BuildingBalance = struct {
	PointsFactor        float64 `json:"pointsFactor"`
	TimeConst           float64 `json:"timeConst"`
	TimeFactor          float64 `json:"timeFactor"`
	TimeShift           float64 `json:"timeShift"`
	TimeMinExponent     float64 `json:"timeMinExponent"`
	MainTimeFactor      float64 `json:"mainTimeFactor"`
	MinBuildTimeMs      int64   `json:"minBuildTimeMs"`
	CancelRefundStarted float64 `json:"cancelRefundStarted"`
	CancelRefundQueued  float64 `json:"cancelRefundQueued"`
}{1.2, 1.18, 1.2, 14, -13, 1.05, 1000, 0.9, 1.0}

var ResourceBalance = struct {
	BaseProductionPerHour float64 `json:"baseProductionPerHour"`
	ProductionBase        float64 `json:"productionBase"`
	ProductionFactor      float64 `json:"productionFactor"`
	StorageBase           float64 `json:"storageBase"`
	StorageFactor         float64 `json:"storageFactor"`
	StorageLevel0         float64 `json:"storageLevel0"`
	FarmBase              float64 `json:"farmBase"`
	FarmFactor            float64 `json:"farmFactor"`
	FarmLevel0            float64 `json:"farmLevel0"`
	HideBase              float64 `json:"hideBase"`
	HideFactor            float64 `json:"hideFactor"`
	StorageWarningRatio   float64 `json:"storageWarningRatio"`
}{5, 30, 1.163118, 1000, 1.2294, 500, 240, 1.1722, 0, 150, 1.3335, 0.9}

var BonusBalance = struct {
	SingleResource    float64 `json:"singleResource"`
	AllResources      float64 `json:"allResources"`
	RecruitTimeFactor float64 `json:"recruitTimeFactor"`
	FarmCapacity      float64 `json:"farmCapacity"`
	StorageCapacity   float64 `json:"storageCapacity"`
}{2.0, 1.3, 0.8, 1.2, 1.5}
