package game

// Entities' JSON tags are the names the client reads, so the API can hand them over unchanged. Per-viewer
// knowledge of a village lives in Intel and a bot's brain in World.Brain, not on Village or Player.

type VillageBonusKind string

type VillageBonus struct {
	Kind VillageBonusKind `json:"kind"`
}

type BuildJob struct {
	ID          string     `json:"id"`
	Building    BuildingID `json:"building"`
	TargetLevel int        `json:"targetLevel"` // for demolish: current − 1
	Kind        string     `json:"kind"`        // build | demolish
	StartedAt   int64      `json:"startedAt"`
	CompletesAt int64      `json:"completesAt"`
}

type RecruitJob struct {
	ID          string `json:"id"`
	Unit        UnitID `json:"unit"`
	Total       int    `json:"total"`
	Done        int    `json:"done"`
	PerUnitMs   int64  `json:"perUnitMs"`
	StartedAt   int64  `json:"startedAt"`
	CompletesAt int64  `json:"completesAt"`
}

type ResearchJob struct {
	ID          string `json:"id"`
	Unit        UnitID `json:"unit"`
	TargetLevel int    `json:"targetLevel"`
	StartedAt   int64  `json:"startedAt"`
	CompletesAt int64  `json:"completesAt"`
}

// RecruitQueues holds one recruitment queue per recruit building.
type RecruitQueues struct {
	Barracks []RecruitJob `json:"barracks"`
	Stable   []RecruitJob `json:"stable"`
	Workshop []RecruitJob `json:"workshop"`
	Academy  []RecruitJob `json:"academy"`
	Statue   []RecruitJob `json:"statue"`
}

func EmptyRecruitQueues() RecruitQueues {
	return RecruitQueues{[]RecruitJob{}, []RecruitJob{}, []RecruitJob{}, []RecruitJob{}, []RecruitJob{}}
}

// Q returns the queue of a recruit building; nil for a building without one.
func (q *RecruitQueues) Q(b BuildingID) *[]RecruitJob {
	switch b {
	case BBarracks:
		return &q.Barracks
	case BStable:
		return &q.Stable
	case BWorkshop:
		return &q.Workshop
	case BAcademy:
		return &q.Academy
	case BStatue:
		return &q.Statue
	}
	return nil
}

type Village struct {
	ID                 string                `json:"id"`
	X                  int                   `json:"x"`
	Y                  int                   `json:"y"`
	Name               string                `json:"name"`
	OwnerID            PlayerRef             `json:"ownerId"`
	Points             int                   `json:"points"`
	Resources          Resources             `json:"resources"`
	ResourcesUpdatedAt int64                 `json:"resourcesUpdatedAt"`
	Buildings          Buildings             `json:"buildings"`
	Units              Units                 `json:"units"`
	SupportFrom        map[string]UnitCounts `json:"supportFrom"`
	Loyalty            float64               `json:"loyalty"`
	LoyaltyUpdatedAt   int64                 `json:"loyaltyUpdatedAt"`
	BuildQueue         []BuildJob            `json:"buildQueue"`
	RecruitQueues      RecruitQueues         `json:"recruitQueues"`
	ResearchQueue      []ResearchJob         `json:"researchQueue"`
	Research           Units                 `json:"research"`
	Bonus              *VillageBonus         `json:"bonus,omitempty"`
	BuildPlan          []BuildingID          `json:"buildPlan,omitempty"`
	ConqueredAt        *int64                `json:"conqueredAt,omitempty"`
}

type Command struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"` // attack | support | return | trade | intercept
	OriginID       string      `json:"originId"`
	TargetID       string      `json:"targetId"`
	Units          UnitCounts  `json:"units"`
	Haul           *Resources  `json:"haul,omitempty"`
	CatapultTarget *BuildingID `json:"catapultTarget,omitempty"`
	Merchants      int         `json:"merchants,omitempty"`
	Returning      bool        `json:"returning,omitempty"`
	DepartedAt     int64       `json:"departedAt"`
	ArrivesAt      int64       `json:"arrivesAt"`
	// Point is where an intercept marches to, or where a return with no OriginID starts.
	Point     *FieldPoint     `json:"point,omitempty"`
	Intercept *InterceptOrder `json:"intercept,omitempty"`
	// Threats are the intercepts on their way to this army.
	Threats []string `json:"threats,omitempty"`
	// FieldHit marks an army that survived a field battle; its owner may turn it back.
	FieldHit bool `json:"fieldHit,omitempty"`
}

// FieldPoint is a place on the map between tiles, in tile units.
type FieldPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Leg is a command's route as it was when an interception was sent at it.
type Leg struct {
	OriginID   string `json:"originId"`
	TargetID   string `json:"targetId"`
	DepartedAt int64  `json:"departedAt"`
	ArrivesAt  int64  `json:"arrivesAt"`
}

// InterceptOrder is what an intercept command is after: the army it meets and how.
type InterceptOrder struct {
	CommandID string `json:"commandId"`
	Scout     bool   `json:"scout,omitempty"`
	Leg       Leg    `json:"leg"`
}

// FieldReport tells where a field battle happened and what it met.
type FieldReport struct {
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	TargetType      string  `json:"targetType"`
	Missed          bool    `json:"missed,omitempty"`
	TargetCommandID string  `json:"targetCommandId"`
}

type ReportSide struct {
	Units  UnitCounts `json:"units"`
	Losses UnitCounts `json:"losses"`
}

type ScoutIntel struct {
	Resources Resources  `json:"resources"`
	Buildings Buildings  `json:"buildings"`
	Units     UnitCounts `json:"units"`
	Loyalty   float64    `json:"loyalty"`
}

type BuildingDamage struct {
	Building BuildingID `json:"building"`
	From     int        `json:"from"`
	To       int        `json:"to"`
}

type Report struct {
	ID             string          `json:"id"`
	Time           int64           `json:"time"`
	Type           string          `json:"type"` // attack | support | scout | conquest | field | fieldScout
	OriginID       string          `json:"originId"`
	TargetID       string          `json:"targetId"`
	Origin         ReportVillage   `json:"origin"`
	Target         ReportVillage   `json:"target"`
	Attacker       ReportSide      `json:"attacker"`
	Defender       ReportSide      `json:"defender"`
	Scout          *ScoutIntel     `json:"scout,omitempty"`
	Luck           float64         `json:"luck"`
	Morale         float64         `json:"morale"`
	WallBefore     int             `json:"wallBefore"`
	WallAfter      int             `json:"wallAfter"`
	BuildingDamage *BuildingDamage `json:"buildingDamage,omitempty"`
	Haul           *Resources      `json:"haul,omitempty"`
	HaulCapacity   *float64        `json:"haulCapacity,omitempty"`
	LoyaltyBefore  *float64        `json:"loyaltyBefore,omitempty"`
	LoyaltyAfter   *float64        `json:"loyaltyAfter,omitempty"`
	Conquered      bool            `json:"conquered,omitempty"`
	Outcome        string          `json:"outcome"` // attackerWon | defenderWon
	Read           bool            `json:"read"`
	Field          *FieldReport    `json:"field,omitempty"`
}

type PlayerStatus string

const (
	StatusPending    PlayerStatus = "pending"
	StatusActive     PlayerStatus = "active"
	StatusQuit       PlayerStatus = "quit"
	StatusEliminated PlayerStatus = "eliminated"
)

type ODCounts struct {
	Att float64 `json:"att"`
	Def float64 `json:"def"`
	Sup float64 `json:"sup"`
}

// DailyCounts are one local day of a player's public counters.
type DailyCounts struct {
	Day  int64   `json:"day"`
	Att  float64 `json:"att"`
	Def  float64 `json:"def"`
	Sup  float64 `json:"sup"`
	Loot float64 `json:"loot"`
	Farm float64 `json:"farm"`
}

type PointsHistory struct {
	Hourly []int `json:"hourly"`
	Daily  []int `json:"daily"`
}

type Player struct {
	ID              PlayerID      `json:"id"`
	Name            string        `json:"name"`
	IsHuman         bool          `json:"isHuman"`
	Color           string        `json:"color"`
	VillageIDs      []string      `json:"villageIds"`
	Coins           int           `json:"coins"`
	NoblesTrained   int           `json:"noblesTrained"`
	BuildQueueSlots int           `json:"buildQueueSlots"`
	ProtectedUntil  int64         `json:"protectedUntil"`
	JoinAt          int64         `json:"joinAt"`
	Status          PlayerStatus  `json:"status"`
	Points          int           `json:"points"`
	QuitAt          *int64        `json:"quitAt,omitempty"`
	History         PointsHistory `json:"history"`
	OD              ODCounts      `json:"od,omitzero"`
	Daily           []DailyCounts `json:"daily,omitempty"`
	AccountID       int64         `json:"-"`
	Key             string        `json:"key,omitempty"` // a human's player key, its name in the action log
	IsGuest         bool          `json:"isGuest,omitempty"`
}

// ---- bot brain ----

type CheckIn struct {
	AtMin  float64 `json:"atMin"`
	LenMin float64 `json:"lenMin"`
}

// Routine is a bot's day in local minutes; SleepMin may exceed 1440 (after midnight).
type Routine struct {
	WakeMin             float64   `json:"wakeMin"`
	SleepMin            float64   `json:"sleepMin"`
	CheckIns            []CheckIn `json:"checkIns"`
	EveningStartMin     float64   `json:"eveningStartMin"`
	EveningLenMin       float64   `json:"eveningLenMin"`
	WeekendWakeShiftMin float64   `json:"weekendWakeShiftMin"`
}

type PartialBuildings map[BuildingID]int

// KnownVillage is what a bot knows of a village, only from its own observations and the map.
type KnownVillage struct {
	ID           string           `json:"id"`
	X            int              `json:"x"`
	Y            int              `json:"y"`
	OwnerID      PlayerRef        `json:"ownerId"`
	Points       int              `json:"points"`
	SeenAt       int64            `json:"seenAt"`
	Units        *UnitCounts      `json:"units,omitempty"`
	Wall         *int             `json:"wall,omitempty"`
	Hide         *float64         `json:"hide,omitempty"`
	Storage      *float64         `json:"storage,omitempty"`
	Buildings    PartialBuildings `json:"buildings,omitempty"`
	Hauls        []float64        `json:"hauls"`
	LastAttackAt *int64           `json:"lastAttackAt,omitempty"`
	Losses       *bool            `json:"losses,omitempty"`
	DeadSince    *int64           `json:"deadSince,omitempty"`
}

type BotMemory struct {
	Known        map[string]*KnownVillage `json:"known"`
	DiscoveredAt int64                    `json:"discoveredAt"`
}

type SessionStats struct {
	VillagesLost    int `json:"villagesLost"`
	IncomingAttacks int `json:"incomingAttacks"`
}

type Grudge struct {
	LastAt int64   `json:"lastAt"`
	Level  float64 `json:"level"`
}

type Habits struct {
	DodgeReturnMins  []float64 `json:"dodgeReturnMins"`
	FarmIntervalsMin []float64 `json:"farmIntervalsMin"`
	LastHitAt        *int64    `json:"lastHitAt,omitempty"`
}

type Mood struct {
	Phase         string   `json:"phase"` // build | campaign | consolidate
	Until         int64    `json:"until"`
	TargetPlayer  PlayerID `json:"targetPlayer,omitempty"`
	TargetVillage string   `json:"targetVillage,omitempty"`
	Failures      int      `json:"failures"`
	ProgressAt    *int64   `json:"progressAt,omitempty"`
	OpAt          *int64   `json:"opAt,omitempty"`
}

type BotBrain struct {
	Skill            float64                `json:"skill"`
	Style            BotStyle               `json:"style"`
	Resilience       float64                `json:"resilience"`
	Caution          float64                `json:"caution"`
	Routine          Routine                `json:"routine"`
	Online           bool                   `json:"online"`
	SessionEndsAt    int64                  `json:"sessionEndsAt"`
	ActionLog        []int64                `json:"actionLog"`
	Roles            map[string]VillageRole `json:"roles"`
	Memory           BotMemory              `json:"memory"`
	Overwhelm        float64                `json:"overwhelm"`
	LastSessionStats SessionStats           `json:"lastSessionStats"`
	ConquestTarget   string                 `json:"conquestTarget,omitempty"`
	Grudges          map[PlayerID]*Grudge   `json:"grudges"`
	Habits           map[PlayerID]*Habits   `json:"habits"`
	Mood             Mood                   `json:"mood"`
	// FieldScouted are the passing armies this bot already scouted on the road (the latest few).
	FieldScouted []string `json:"fieldScouted,omitempty"`
}

// ---- human-only state, per human player ----

type QuestState struct {
	Completed []string `json:"completed"`
	Active    []string `json:"active"`
	Claimed   []string `json:"claimed"`
	// Progress is view data for the quest bars, filled when the player's state is sent; never stored.
	Progress map[string]float64 `json:"progress,omitempty"`
}

type FarmTemplate struct {
	Units UnitCounts `json:"units"`
}

type FarmAssistantState struct {
	Templates        map[string]FarmTemplate `json:"templates"` // A, B, C, D
	AutoFarmUnlocked bool                    `json:"autoFarmUnlocked"`
	AutoFarmEnabled  bool                    `json:"autoFarmEnabled"`
	AutoFarmVillages []string                `json:"autoFarmVillages"`
	Filters          FarmFilters             `json:"filters"`
}

type TraderOffer struct {
	Give       Resource `json:"give"`
	GiveAmount float64  `json:"giveAmount"`
	Get        Resource `json:"get"`
	GetAmount  float64  `json:"getAmount"`
}

type TraderState struct {
	OfferIndex int         `json:"offerIndex"`
	Offer      TraderOffer `json:"offer"`
	ExpiresAt  int64       `json:"expiresAt"`
	Accepted   bool        `json:"accepted"`
}

type VillageGroup struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	VillageIDs []string `json:"villageIds"`
}

type DayCount struct {
	Day   int64 `json:"day"`
	Count int   `json:"count"`
}

type DayTotal struct {
	Day   int64   `json:"day"`
	Total float64 `json:"total"`
}

type PlayerStats struct {
	AttacksSent       int        `json:"attacksSent"`
	AttacksToday      DayCount   `json:"attacksToday"`
	LootedToday       DayTotal   `json:"lootedToday"`
	ResourcesLooted   Resources  `json:"resourcesLooted"`
	UnitsKilled       int        `json:"unitsKilled"`
	UnitsLost         int        `json:"unitsLost"`
	VillagesConquered int        `json:"villagesConquered"`
	PointsHistory     [][2]int64 `json:"pointsHistory"` // [world time, points]
	BattlesWon        int        `json:"battlesWon"`
	ScoutsSent        int        `json:"scoutsSent"`
	SupportsSent      int        `json:"supportsSent"`
	TradesSent        int        `json:"tradesSent"`
	CoinsMinted       int        `json:"coinsMinted"`
	ResearchCompleted int        `json:"researchCompleted"`
	AttacksRepelled   int        `json:"attacksRepelled"`
}

// HumanState holds what only a human player has; one per human.
type HumanState struct {
	Stats          PlayerStats        `json:"stats"`
	Quests         QuestState         `json:"quests"`
	FarmAssistant  FarmAssistantState `json:"farmAssistant"`
	Trader         TraderState        `json:"trader"`
	Groups         []VillageGroup     `json:"villageGroups"`
	BuildTemplates []BuildTemplate    `json:"buildTemplates"`
	AggressorUntil int64              `json:"aggressorUntil"`
}

// ---- per-player records ----

type VillageIntel struct {
	At               int64      `json:"at"`
	Units            UnitCounts `json:"units"`
	Wall             int        `json:"wall"`
	Loyalty          float64    `json:"loyalty"`
	LastHaul         *Resources `json:"lastHaul,omitempty"`
	LastHaulCapacity *float64   `json:"lastHaulCapacity,omitempty"`
	LastLosses       *int       `json:"lastLosses,omitempty"`
	LastOutcome      string     `json:"lastOutcome,omitempty"`
}

type FarmTargetFlags struct {
	HasWall   bool   `json:"hasWall"`
	ScoutedAt *int64 `json:"scoutedAt,omitempty"`
}

// Intel is one player's knowledge of one village: scouting intel, farm target flags, last farmed time.
type Intel struct {
	Intel           *VillageIntel    `json:"intel,omitempty"`
	FarmTargetFlags *FarmTargetFlags `json:"farmTargetFlags,omitempty"`
	LastFarmedAt    *int64           `json:"lastFarmedAt,omitempty"`
}

// ScheduledSend is a send planned to the millisecond: "send at" or "arrive at" (then SendAt is derived).
type ScheduledSend struct {
	ID             string      `json:"id"`
	OwnerID        PlayerID    `json:"ownerId"`
	OriginID       string      `json:"originId"`
	X              int         `json:"x"`
	Y              int         `json:"y"`
	Units          UnitCounts  `json:"units"`
	Type           string      `json:"type"` // attack | support
	CatapultTarget *BuildingID `json:"catapultTarget,omitempty"`
	Mode           string      `json:"mode"` // sendAt | arriveAt
	At             int64       `json:"at"`
	SendAt         int64       `json:"sendAt"`
	CancelAt       *int64      `json:"cancelAt,omitempty"`
}

type NewsItem struct {
	ID        string         `json:"id"`
	Time      int64          `json:"time"`
	Kind      string         `json:"kind"`
	Key       string         `json:"key"`
	Params    map[string]any `json:"params,omitempty"`
	Alert     bool           `json:"alert,omitempty"`
	VillageID string         `json:"villageId,omitempty"`
	// For is the one player a private item is for ("" = the whole world), such as "your last villages fell".
	For PlayerID `json:"for,omitempty"`
}

// Conquest is one entry of the public list of village takeovers.
type Conquest struct {
	T         int64     `json:"t"`
	VillageID string    `json:"villageId"`
	X         int       `json:"x"`
	Y         int       `json:"y"`
	FromID    PlayerRef `json:"fromId"`
	ToID      PlayerID  `json:"toId"`
}

// VillageRef is the light projection of a stored village that area queries return.
type VillageRef struct {
	ID      string    `json:"id"`
	X       int       `json:"x"`
	Y       int       `json:"y"`
	Name    string    `json:"name"`
	OwnerID PlayerRef `json:"ownerId"`
	Points  int       `json:"points"`
}
