package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	MsPerSecond int64 = 1000
	MsPerMinute int64 = 60_000
	MsPerHour   int64 = 3_600_000
	MsPerDay    int64 = 86_400_000
)

type PlayerID = string

// PlayerRef is an optional player id: "" means none (a barbarian village) and marshals as JSON null.
type PlayerRef string

func (r PlayerRef) MarshalJSON() ([]byte, error) {
	if r == "" {
		return []byte("null"), nil
	}
	return json.Marshal(string(r))
}

func (r *PlayerRef) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		*r = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*r = PlayerRef(s)
	return nil
}

func Ref(id PlayerID) PlayerRef { return PlayerRef(id) }

// ---- resources ----

type Resource int

const (
	Wood Resource = iota
	Clay
	Iron
)

var ResourceIDs = [...]Resource{Wood, Clay, Iron}
var resourceNames = [...]string{"wood", "clay", "iron"}

func (r Resource) String() string { return resourceNames[r] }

func (r Resource) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

func (r *Resource) UnmarshalText(b []byte) error {
	for i, n := range resourceNames {
		if n == string(b) {
			*r = Resource(i)
			return nil
		}
	}
	return fmt.Errorf("unknown resource %q", b)
}

type Resources struct {
	Wood float64 `json:"wood"`
	Clay float64 `json:"clay"`
	Iron float64 `json:"iron"`
}

func (r Resources) Get(k Resource) float64 {
	switch k {
	case Wood:
		return r.Wood
	case Clay:
		return r.Clay
	default:
		return r.Iron
	}
}

func (r *Resources) Set(k Resource, v float64) {
	switch k {
	case Wood:
		r.Wood = v
	case Clay:
		r.Clay = v
	default:
		r.Iron = v
	}
}

// ---- buildings ----

type BuildingID int

const (
	BMain BuildingID = iota
	BBarracks
	BStable
	BWorkshop
	BAcademy
	BSmithy
	BStatue
	BMarket
	BWood
	BClay
	BIron
	BFarm
	BStorage
	BHide
	BWall
	NumBuildings = iota
)

// BuildingIDs is the iteration order of every record keyed by building.
var BuildingIDs = [NumBuildings]BuildingID{BMain, BBarracks, BStable, BWorkshop, BAcademy, BSmithy, BStatue,
	BMarket, BWood, BClay, BIron, BFarm, BStorage, BHide, BWall}

var buildingNames = [NumBuildings]string{"main", "barracks", "stable", "workshop", "academy", "smithy", "statue",
	"market", "wood", "clay", "iron", "farm", "storage", "hide", "wall"}

func (b BuildingID) String() string {
	if b < 0 || int(b) >= NumBuildings {
		return "building(" + strconv.Itoa(int(b)) + ")"
	}
	return buildingNames[b]
}

func (b BuildingID) MarshalText() ([]byte, error) { return []byte(b.String()), nil }

func (b *BuildingID) UnmarshalText(t []byte) error {
	id, ok := ParseBuilding(string(t))
	if !ok {
		return fmt.Errorf("unknown building %q", t)
	}
	*b = id
	return nil
}

func ParseBuilding(s string) (BuildingID, bool) {
	for i, n := range buildingNames {
		if n == s {
			return BuildingID(i), true
		}
	}
	return 0, false
}

// RecruitBuildings are the buildings with their own parallel recruitment queue.
var RecruitBuildings = [...]BuildingID{BBarracks, BStable, BWorkshop, BAcademy, BStatue}

// Buildings holds a level per building; it marshals every key, in BuildingIDs order.
type Buildings [NumBuildings]int

func (b Buildings) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, id := range BuildingIDs {
		if i > 0 {
			buf.WriteByte(',')
		}
		fmt.Fprintf(&buf, "%q:%d", id.String(), b[id])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (b *Buildings) UnmarshalJSON(data []byte) error {
	var m map[BuildingID]float64
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*b = Buildings{}
	for k, v := range m {
		b[k] = int(v)
	}
	return nil
}

// ---- units ----

type UnitID int

const (
	USpear UnitID = iota
	USword
	UAxe
	UArcher
	UScout
	ULightCav
	UMountedArcher
	UHeavyCav
	URam
	UCatapult
	UPaladin
	UNoble
	NumUnits = iota
)

// UnitIDs is the iteration order of every record keyed by unit.
var UnitIDs = [NumUnits]UnitID{USpear, USword, UAxe, UArcher, UScout, ULightCav, UMountedArcher, UHeavyCav,
	URam, UCatapult, UPaladin, UNoble}

var unitNames = [NumUnits]string{"spear", "sword", "axe", "archer", "scout", "lightCav", "mountedArcher", "heavyCav",
	"ram", "catapult", "paladin", "noble"}

func (u UnitID) String() string {
	if u < 0 || int(u) >= NumUnits {
		return "unit(" + strconv.Itoa(int(u)) + ")"
	}
	return unitNames[u]
}

func (u UnitID) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

func (u *UnitID) UnmarshalText(t []byte) error {
	id, ok := ParseUnit(string(t))
	if !ok {
		return fmt.Errorf("unknown unit %q", t)
	}
	*u = id
	return nil
}

func ParseUnit(s string) (UnitID, bool) {
	for i, n := range unitNames {
		if n == s {
			return UnitID(i), true
		}
	}
	return 0, false
}

// Units is a full unit record; it marshals every key.
type Units [NumUnits]int

func (u Units) MarshalJSON() ([]byte, error) { return marshalUnits(u, false), nil }

func (u *Units) UnmarshalJSON(data []byte) error { return unmarshalUnits((*[NumUnits]int)(u), data) }

// UnitCounts is a partial unit record; it marshals non-zero keys only.
type UnitCounts [NumUnits]int

func (u UnitCounts) MarshalJSON() ([]byte, error) { return marshalUnits(u, true), nil }

func (u *UnitCounts) UnmarshalJSON(data []byte) error {
	return unmarshalUnits((*[NumUnits]int)(u), data)
}

func (u UnitCounts) Total() int {
	n := 0
	for _, c := range u {
		n += c
	}
	return n
}

func (u UnitCounts) IsEmpty() bool { return u.Total() == 0 }

func marshalUnits(u [NumUnits]int, skipZero bool) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, id := range UnitIDs {
		if skipZero && u[id] == 0 {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false
		fmt.Fprintf(&buf, "%q:%d", id.String(), u[id])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

func unmarshalUnits(u *[NumUnits]int, data []byte) error {
	var m map[UnitID]float64
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*u = [NumUnits]int{}
	for k, v := range m {
		u[k] = int(v)
	}
	return nil
}

// ---- world config ----

type Difficulty string

const (
	Easy   Difficulty = "easy"
	Normal Difficulty = "normal"
	Hard   Difficulty = "hard"
	Brutal Difficulty = "brutal"
)

type BotStyle string

const (
	StyleFarmer       BotStyle = "farmer"
	StyleTurtle       BotStyle = "turtle"
	StyleAggressor    BotStyle = "aggressor"
	StylePointBuilder BotStyle = "pointBuilder"
	StyleOpportunist  BotStyle = "opportunist"
)

type BarbarianDensity string
type BattleLossMode string

type BotStance string

const (
	StancePeacefulRetaliate  BotStance = "peacefulRetaliate"
	StancePeacefulReputation BotStance = "peacefulReputation"
	StanceNormal             BotStance = "normal"
	StanceAllAgainstHumans   BotStance = "allAgainstHumans"
)

// SideModifiers are the multipliers of one side of the world (humans or bots); 1 = unchanged.
type SideModifiers struct {
	Resources   float64 `json:"resources"`
	Recruitment float64 `json:"recruitment"`
	Building    float64 `json:"building"`
}

// WorldConfig is a world's rule settings (pausing lives on the world row, not here).
type WorldConfig struct {
	WorldSpeed             float64          `json:"worldSpeed"`
	UnitSpeed              float64          `json:"unitSpeed"`
	MapSize                int              `json:"mapSize"`
	BarbarianDensity       BarbarianDensity `json:"barbarianDensity"`
	BattleLosses           BattleLossMode   `json:"battleLosses"`
	HumanModifiers         SideModifiers    `json:"humanModifiers"`
	BotModifiers           SideModifiers    `json:"botModifiers"`
	BotCount               int              `json:"botCount"`
	BotArrival             string           `json:"botArrival"`
	RampUpDays             float64          `json:"rampUpDays"`
	BeginnerProtectionDays float64          `json:"beginnerProtectionDays"`
	LastVillage            string           `json:"lastVillage"`
	Difficulty             Difficulty       `json:"difficulty"`
	BotStance              BotStance        `json:"botStance"`

	// Instant worlds only (see instant.go); zero values mean a long world.
	Kind             WorldKind `json:"kind,omitempty"`
	DurationMin      int       `json:"durationMin,omitempty"`
	JoinWindowMin    int       `json:"joinWindowMin,omitempty"`
	HumanSeats       int       `json:"humanSeats,omitempty"`
	BotThinkEverySec float64   `json:"botThinkEverySec,omitempty"`
	BotTimeScale     float64   `json:"botTimeScale,omitempty"`
	BarbarianLoyalty float64   `json:"barbarianLoyalty,omitempty"`
	// NoCoins: a nobleman needs no coins and coins cannot be minted.
	NoCoins bool `json:"noCoins,omitempty"`
	// NobleCost replaces the nobleman's price when set.
	NobleCost Resources `json:"nobleCost,omitempty"`
	// OwnedNobleDropMin/Max replace the loyalty a nobleman takes from a village with an owner when set.
	OwnedNobleDropMin int `json:"ownedNobleDropMin,omitempty"`
	OwnedNobleDropMax int `json:"ownedNobleDropMax,omitempty"`
	// AcademyExtraNobles: an academy holds this many nobles more than its level.
	AcademyExtraNobles int     `json:"academyExtraNobles,omitempty"`
	DominationShare    float64 `json:"dominationShare,omitempty"`
	StartPreset        string  `json:"startPreset,omitempty"`
	// SnapshotEveryMin is how often humans' snapshots go to the action log; 0 means every 10 hours.
	SnapshotEveryMin int `json:"snapshotEveryMin,omitempty"`
	// VillageTotal is computed when the world is created: how many villages the map can hold.
	VillageTotal int `json:"villageTotal,omitempty"`

	// Field battles (meeting armies on the road) are on unless switched off.
	NoFieldBattles bool `json:"noFieldBattles,omitempty"`
	// SightRadius is how close, in tiles, a passing army must come to a village to be seen; 0 means 1.
	SightRadius float64 `json:"sightRadius,omitempty"`
	// NoSight: nobody sees other players' armies pass (a player world's host may switch it off).
	NoSight bool `json:"noSight,omitempty"`
}

func FieldBattlesOn(cfg *WorldConfig) bool { return !cfg.NoFieldBattles }

func SightRadiusOf(cfg *WorldConfig) float64 {
	if cfg.SightRadius <= 0 {
		return 1
	}
	return cfg.SightRadius
}

// ---- small shared records used by the balance tables ----

type FarmFilters struct {
	Radius                float64 `json:"radius"`
	MinMinutesSinceAttack float64 `json:"minMinutesSinceAttack"`
	MaxWall               int     `json:"maxWall"`
	HideLosses            bool    `json:"hideLosses"`
	HideInbound           bool    `json:"hideInbound"`
}

type BuildTemplate struct {
	ID    string       `json:"id"`
	Name  string       `json:"name"`
	Steps []BuildingID `json:"steps"`
}
