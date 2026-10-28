package game

import (
	"fmt"
	"math"
	"sort"
)

// Where new players' villages are placed.

func init() { registerHandler("join", onJoinEvent) }

type Point struct{ X, Y int }

// SpawnSlots are the players' start fields on a sunflower spiral around the map centre: even gaps
// (~MapConfig.SpawnSpacing), no rings, no clusters. Ordered by distance from the centre.
func SpawnSlots(count, mapSize int, seed uint32) []Point {
	c := mapSize / 2
	taken := map[Point]bool{}
	type slot struct {
		p Point
		d float64
	}
	out := make([]slot, 0, count)
	clamp := func(n int) int { return max(0, min(mapSize-1, n)) }
	for i := range count {
		rng := Mulberry32(HashSeed(seed, "slot", i))
		r := MapConfig.SpawnSpacing * math.Sqrt(float64(i)) * 0.56
		a := (float64(i) * 137.508 * math.Pi) / 180
		x := clamp(int(math.Round(float64(c)+float64(r*math.Cos(a)))) + RandInt(rng, -1, 1))
		y := clamp(int(math.Round(float64(c)+float64(r*math.Sin(a)))) + RandInt(rng, -1, 1))
		for ring := 1; taken[Point{x, y}]; ring++ {
			dx, dy := RandInt(rng, -ring, ring), RandInt(rng, -ring, ring)
			x, y = clamp(x+dx), clamp(y+dy)
		}
		taken[Point{x, y}] = true
		out = append(out, slot{Point{x, y}, Distance(x, y, c, c)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].d < out[j].d })
	pts := make([]Point, len(out))
	for i, s := range out {
		pts[i] = s.p
	}
	return pts
}

type Quadrant string

const (
	QuadrantRandom Quadrant = "random"
	QuadrantNW     Quadrant = "nw"
	QuadrantNE     Quadrant = "ne"
	QuadrantSW     Quadrant = "sw"
	QuadrantSE     Quadrant = "se"
)

// quadrantStart is the start angle of a quadrant's 90° sector (x = c + r·cos a, y = c + r·sin a; north is
// smaller y).
var quadrantStart = map[Quadrant]float64{QuadrantSE: 0, QuadrantSW: math.Pi / 2, QuadrantNW: math.Pi, QuadrantNE: 1.5 * math.Pi}

func inQuadrant(x, y, c int, q Quadrant) bool {
	switch q {
	case QuadrantNW:
		return x < c && y < c
	case QuadrantNE:
		return x > c && y < c
	case QuadrantSW:
		return x < c && y > c
	case QuadrantSE:
		return x > c && y > c
	}
	return true
}

// EdgeSpawnTile is a free field at the edge of the settled area: the spawn spiral's radius for as many player
// villages as stand (spawnSpacing × 0.56 × √n), in the ring R…R+5, widened when there is no room. A quadrant
// other than random keeps the field strictly inside that sector.
func EdgeSpawnTile(w World, rng func() float64, q Quadrant) (int, int, error) {
	size := w.Config().MapSize
	c := size / 2
	n := 0
	for _, p := range w.Players() {
		if p.Status == StatusActive {
			n += len(p.VillageIDs)
		}
	}
	radius := MapConfig.SpawnSpacing * 0.56 * math.Sqrt(float64(n))
	start, sector := quadrantStart[q]
	for attempt := range 500 {
		widen := float64(attempt / 50)
		r := radius + widen + rng()*5
		var a float64
		if sector {
			a = start + rng()*(math.Pi/2)
		} else {
			a = rng() * 2 * math.Pi
		}
		x := int(math.Round(float64(c) + float64(r*math.Cos(a))))
		y := int(math.Round(float64(c) + float64(r*math.Sin(a))))
		if x < 0 || y < 0 || x >= size || y >= size || !inQuadrant(x, y, c, q) {
			continue
		}
		if w.Village(VillageIDFor(x, y)) == nil {
			return x, y, nil
		}
	}
	return 0, 0, Rule("noSpawnTile", nil)
}

// CreatePlayerVillage builds a player's start village (buildings and resources as a human's, no units) in
// place of any barbarian on the field, registers ownership and starts beginner protection.
func CreatePlayerVillage(w World, id PlayerID, x, y int) *Village {
	p := w.Player(id)
	if p == nil {
		panic(fmt.Sprintf("no player %s", id))
	}
	v := CreateVillage(x, y, "", "", w.Now())
	for b, lvl := range StartVillage.Buildings {
		v.Buildings[b] = lvl
	}
	v.Resources = StartVillage.Resources
	RecomputePoints(v)
	applyStartPreset(w, v)
	w.PutVillage(v)
	SetOwner(w, v, Ref(id))
	recordTimeline(w, TimelineEntry{Kind: "village", X: float64(x), Y: float64(y), Owner: Ref(id)})
	p.ProtectedUntil = w.Now() + int64(w.Config().BeginnerProtectionDays*float64(MsPerDay))
	if !p.IsHuman && len(p.VillageIDs) == 1 && scheduleFirstSession != nil {
		scheduleFirstSession(w, id)
	}
	return v
}

// scheduleFirstSession starts a bot's day once it has its first village (set by the bot sessions code).
var scheduleFirstSession func(w World, id PlayerID)

// RespawnPlayer gives a player who lost the last village (lastVillage: canBeLost) a fresh start at the edge,
// without coins.
func RespawnPlayer(w World, id PlayerID) error {
	x, y, err := EdgeSpawnTile(w, Mulberry32(HashSeed(w.Seed(), "respawn", w.Now())), QuadrantRandom)
	if err != nil {
		return err
	}
	v := CreatePlayerVillage(w, id, x, y)
	p := w.Player(id)
	p.Coins = 0
	p.NoblesTrained = 0
	AddNewsItem(w, "news.respawn", map[string]any{"x": x, "y": y}, v.ID, true, id)
	return nil
}

// onJoinEvent: a ramp-up bot settles at the edge of the world.
func onJoinEvent(w World, id PlayerID) error {
	p := w.Player(id)
	if p == nil || p.Status != StatusPending {
		return nil
	}
	p.Status = StatusActive
	x, y, err := EdgeSpawnTile(w, Mulberry32(HashSeed(w.Seed(), "join", id)), QuadrantRandom)
	if err != nil {
		return err
	}
	CreatePlayerVillage(w, id, x, y)
	AddNewsItem(w, "news.newPlayer", map[string]any{"player": p.Name, "isBot": !p.IsHuman, "x": x, "y": y}, VillageIDFor(x, y), false, "")
	return nil
}
