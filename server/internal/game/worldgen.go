package game

import (
	"math"
	"sort"
)

// The map is a pure function of the seed; barbarian villages are stored only once
// touched.

var bonusKinds = [...]VillageBonusKind{"wood", "clay", "iron", "allResources", "infantry", "cavalry", "farm", "storage"}

// barbarianLevelOrder is the order barbarian building levels are drawn in (BarbarianTemplateData.Levels is a
// map): the random draws follow it, and it is not BuildingIDs order.
var barbarianLevelOrder = [...]BuildingID{BMain, BWood, BClay, BIron, BStorage, BFarm, BHide}

func Distance(ax, ay, bx, by int) float64 {
	dx, dy := float64(ax-bx), float64(ay-by)
	return math.Sqrt(dx*dx + dy*dy)
}

func TileHasVillage(seed uint32, cfg *WorldConfig, x, y int) bool {
	if x < 0 || y < 0 || x >= cfg.MapSize || y >= cfg.MapSize {
		return false
	}
	return Mulberry32(HashSeed(seed, "tile", x, y))() < MapConfig.Density[cfg.BarbarianDensity]
}

func Terrain(seed uint32, x, y int) int {
	return int(HashSeed(seed, "terrain", x, y) % 7)
}

type BarbarianTemplateT struct {
	X           int           `json:"x"`
	Y           int           `json:"y"`
	Buildings   Buildings     `json:"buildings"`
	Units       UnitCounts    `json:"units"`
	Bonus       *VillageBonus `json:"bonus,omitempty"`
	Points      int           `json:"points"`
	InitialFill float64       `json:"initialFill"` // share of storage filled when materialised
}

func BarbarianTemplate(seed uint32, x, y int) BarbarianTemplateT {
	rng := Mulberry32(HashSeed(seed, "barb", x, y))
	var b Buildings
	for _, id := range barbarianLevelOrder {
		lv := BarbarianTemplateData.Levels[id]
		b[id] = RandInt(rng, lv[0], lv[1])
	}
	var bonus *VillageBonus
	if rng() < MapConfig.BonusShare {
		bonus = &VillageBonus{Kind: bonusKinds[RandInt(rng, 0, len(bonusKinds)-1)]}
	}
	fill := BarbarianTemplateData.InitialFill
	return BarbarianTemplateT{X: x, Y: y, Buildings: b, Bonus: bonus, Points: VillagePoints(b),
		InitialFill: fill[0] + rng()*(fill[1]-fill[0])}
}

// Tile is what the map shows of one field, without materialising a village.
type Tile struct {
	Kind         string        `json:"kind"` // empty | village
	X            int           `json:"x"`
	Y            int           `json:"y"`
	Terrain      int           `json:"terrain,omitempty"`
	ID           string        `json:"id,omitempty"`
	OwnerID      PlayerRef     `json:"ownerId"`
	OwnerName    string        `json:"ownerName,omitempty"`
	OwnerStatus  PlayerStatus  `json:"ownerStatus,omitempty"`
	OwnerColor   string        `json:"ownerColor,omitempty"`
	Name         string        `json:"name,omitempty"`
	Points       int           `json:"points,omitempty"`
	Bonus        *VillageBonus `json:"bonus,omitempty"`
	Materialized bool          `json:"materialized,omitempty"`
}

// TileInfo reads a tile: stored villages win over the generator.
func TileInfo(w World, x, y int) Tile {
	id := VillageIDFor(x, y)
	if v := w.Village(id); v != nil {
		t := Tile{Kind: "village", X: x, Y: y, ID: id, OwnerID: v.OwnerID, Name: v.Name, Points: v.Points,
			Bonus: v.Bonus, Materialized: true}
		if owner := OwnerOf(w, v); owner != nil {
			t.OwnerName, t.OwnerStatus, t.OwnerColor = owner.Name, owner.Status, owner.Color
		}
		return t
	}
	if !TileHasVillage(w.Seed(), w.Config(), x, y) {
		return Tile{Kind: "empty", X: x, Y: y, Terrain: Terrain(w.Seed(), x, y)}
	}
	tp := BarbarianTemplate(w.Seed(), x, y)
	return Tile{Kind: "village", X: x, Y: y, ID: id, Points: tp.Points, Bonus: tp.Bonus}
}

func VillageExistsAt(w World, x, y int) bool {
	return w.Village(VillageIDFor(x, y)) != nil || TileHasVillage(w.Seed(), w.Config(), x, y)
}

// EnsureVillage materialises a barbarian village on first touch (attack, scout, conquest); nil for no village.
func EnsureVillage(w World, x, y int) *Village {
	id := VillageIDFor(x, y)
	if v := w.Village(id); v != nil {
		return v
	}
	if !TileHasVillage(w.Seed(), w.Config(), x, y) {
		return nil
	}
	tp := BarbarianTemplate(w.Seed(), x, y)
	v := CreateVillage(x, y, "", "", w.Now())
	if l := w.Config().BarbarianLoyalty; l > 0 {
		v.Loyalty = l
	}
	v.Buildings = tp.Buildings
	for _, u := range UnitIDs {
		v.Units[u] = tp.Units[u]
	}
	v.Bonus = tp.Bonus
	capacity := StorageCapacity(v.Buildings[BStorage], v.Bonus)
	for _, r := range ResourceIDs {
		v.Resources.Set(r, math.Floor(capacity*tp.InitialFill))
	}
	RecomputePoints(v)
	w.PutVillage(v)
	return v
}

// VillagesInRadius lists villages within a radius (without materialising), sorted by distance.
func VillagesInRadius(w World, cx, cy int, radius float64) []Tile {
	var out []Tile
	r := int(math.Ceil(radius))
	size := w.Config().MapSize
	for x := max(0, cx-r); x <= min(size-1, cx+r); x++ {
		for y := max(0, cy-r); y <= min(size-1, cy+r); y++ {
			if Distance(cx, cy, x, y) > radius {
				continue
			}
			if t := TileInfo(w, x, y); t.Kind == "village" {
				out = append(out, t)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return Distance(cx, cy, out[i].X, out[i].Y) < Distance(cx, cy, out[j].X, out[j].Y)
	})
	return out
}
