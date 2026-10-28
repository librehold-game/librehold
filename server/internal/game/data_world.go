package game

// World defaults and limits, the start village, map and command constants.

var DefaultWorldConfig = WorldConfig{
	WorldSpeed:             8,
	UnitSpeed:              1,
	MapSize:                200,
	BarbarianDensity:       "normal",
	BattleLosses:           "normal",
	HumanModifiers:         SideModifiers{1, 1, 1},
	BotModifiers:           SideModifiers{1, 1, 1},
	BotCount:               100,
	BotArrival:             "allAtOnce",
	RampUpDays:             14,
	BeginnerProtectionDays: 7,
	LastVillage:            "protected",
	Difficulty:             Normal,
	BotStance:              StanceNormal,
}

type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

var WorldLimits = struct {
	WorldSpeed             Range `json:"worldSpeed"`
	UnitSpeed              Range `json:"unitSpeed"`
	MapSizes               []int `json:"mapSizes"`
	BuildQueueSlots        Range `json:"buildQueueSlots"`
	Modifier               Range `json:"modifier"`
	BotCount               Range `json:"botCount"`
	RampUpDays             Range `json:"rampUpDays"`
	BeginnerProtectionDays Range `json:"beginnerProtectionDays"`
	SightRadius            Range `json:"sightRadius"`
}{Range{1, 100}, Range{0.5, 10}, []int{100, 200, 400}, Range{2, 5}, Range{0.25, 10}, Range{0, 300}, Range{1, 60}, Range{0, 30}, Range{0, 10}}

var StartVillage = struct {
	Buildings map[BuildingID]int `json:"buildings"`
	Resources Resources          `json:"resources"`
	Units     UnitCounts         `json:"units"`
}{
	Buildings: map[BuildingID]int{BMain: 1, BFarm: 1, BStorage: 1, BHide: 1},
	Resources: Resources{500, 500, 500},
	Units:     UnitCounts{USpear: 10},
}

var MapConfig = struct {
	Density           map[BarbarianDensity]float64 `json:"density"`
	BonusShare        float64                      `json:"bonusShare"`
	SpawnSpacing      float64                      `json:"spawnSpacing"`
	TileW             int                          `json:"tileW"`
	TileH             int                          `json:"tileH"`
	ViewCols          int                          `json:"viewCols"`
	ViewRows          int                          `json:"viewRows"`
	ZoomLevels        []float64                    `json:"zoomLevels"`
	DefaultZoomIdx    int                          `json:"defaultZoomIdx"`
	CrowdMinTileW     int                          `json:"crowdMinTileW"`
	VillageSizePoints []int                        `json:"villageSizePoints"`
}{
	Density:           map[BarbarianDensity]float64{"sparse": 0.18, "normal": 0.28, "dense": 0.38},
	BonusShare:        0.03,
	SpawnSpacing:      5,
	TileW:             53,
	TileH:             38,
	ViewCols:          15,
	ViewRows:          13,
	ZoomLevels:        []float64{2.5, 1.6, 1, 0.7, 0.5},
	DefaultZoomIdx:    2,
	CrowdMinTileW:     80,
	VillageSizePoints: []int{300, 1000, 3000, 9000, 11000},
}

// BarbarianTemplateData holds the ranges barbarian villages are drawn from (BarbarianTemplate is the generator).
var BarbarianTemplateData = struct {
	Levels      map[BuildingID][2]int `json:"levels"`
	InitialFill [2]float64            `json:"initialFill"`
}{
	Levels: map[BuildingID][2]int{BMain: {1, 3}, BWood: {1, 5}, BClay: {1, 5}, BIron: {1, 5}, BStorage: {1, 3},
		BFarm: {1, 3}, BHide: {0, 2}},
	InitialFill: [2]float64{0.4, 0.9},
}

var CommandBalance = struct {
	CancelWindowMs      int64   `json:"cancelWindowMs"`
	MerchantSpeed       float64 `json:"merchantSpeed"`
	MerchantCapacity    float64 `json:"merchantCapacity"`
	TradeCancelWindowMs int64   `json:"tradeCancelWindowMs"`
}{600_000, 6, 1000, 300_000}

var FarmBalance = struct {
	DefaultFilters         FarmFilters `json:"defaultFilters"`
	AutoFarmIntervalMs     int64       `json:"autoFarmIntervalMs"`
	AutoFarmUnlockVillages int         `json:"autoFarmUnlockVillages"`
	AutoFarmMaxPerTick     int         `json:"autoFarmMaxPerTick"`
}{FarmFilters{15, 30, 3, true, true}, 15 * MsPerMinute, 3, 20}

var TraderBalance = struct {
	IntervalMs int64     `json:"intervalMs"`
	Ratio      float64   `json:"ratio"`
	Amounts    []float64 `json:"amounts"`
}{8 * MsPerHour, 0.8, []float64{500, 1000, 2000, 4000}}

var PlanBalance = struct {
	FeedIntervalMs int64 `json:"feedIntervalMs"`
}{5 * MsPerMinute}

var NewsConfig = struct {
	Max int `json:"max"`
}{200}

var Sim = struct {
	MaxChunkMs                int64 `json:"maxChunkMs"`
	OfflineSummaryThresholdMs int64 `json:"offlineSummaryThresholdMs"`
	UITickMs                  int64 `json:"uiTickMs"`
	AutosaveMs                int64 `json:"autosaveMs"`
}{86_400_000, 60_000, 250, 30_000}
