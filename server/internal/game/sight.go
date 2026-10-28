package game

import (
	"maps"
	"math"
	"slices"
	"strings"
)

// Bystanders: a player sees another player's army while it passes within the world's sight radius of one of
// their villages. Entering and leaving sight are events, so the bystander's state refreshes on time; what is
// visible is always worked out from the army's position at that moment.

func init() { registerHandler("sight", onSightEvent) }

type SightWindow struct {
	Player PlayerID
	From   int64
	To     int64
}

// segmentCircle is the part [s1, s2] of the segment a→b (s from 0 to 1) within r of p.
func segmentCircle(a, b, p FieldPoint, r float64) (float64, float64, bool) {
	dx, dy := b.X-a.X, b.Y-a.Y
	fx, fy := a.X-p.X, a.Y-p.Y
	qa := dx*dx + dy*dy
	qb := 2 * (fx*dx + fy*dy)
	qc := fx*fx + fy*fy - r*r
	if qa == 0 {
		return 0, 1, qc <= 0
	}
	disc := qb*qb - 4*qa*qc
	if disc < 0 {
		return 0, 0, false
	}
	sq := math.Sqrt(disc)
	s1, s2 := math.Max(0, (-qb-sq)/(2*qa)), math.Min(1, (-qb+sq)/(2*qa))
	return s1, s2, s1 <= s2
}

// watchers are the two players who never watch an army pass: its owner and the owner of the village it is going to.
func watchers(w World, c *Command) (owner, targetOwner PlayerRef) {
	if v := w.Village(CommandHome(c)); v != nil {
		owner = v.OwnerID
	}
	if v := w.Village(c.TargetID); v != nil {
		targetOwner = v.OwnerID
	}
	return owner, targetOwner
}

// SightWindows are the times at which humans other than its owner and its target's owner can see c pass.
func SightWindows(w World, c *Command) []SightWindow {
	cfg := w.Config()
	if !FieldBattlesOn(cfg) || cfg.NoSight || !interceptable(c) || c.ArrivesAt <= c.DepartedAt {
		return nil
	}
	from, to, ok := LegEnds(c)
	if !ok {
		return nil
	}
	r := SightRadiusOf(cfg)
	owner, targetOwner := watchers(w, c)
	last := cfg.MapSize - 1
	x0, x1 := max(0, int(math.Floor(math.Min(from.X, to.X)-r))), min(last, int(math.Ceil(math.Max(from.X, to.X)+r)))
	y0, y1 := max(0, int(math.Floor(math.Min(from.Y, to.Y)-r))), min(last, int(math.Ceil(math.Max(from.Y, to.Y)+r)))
	span := float64(c.ArrivesAt - c.DepartedAt)
	per := map[PlayerID][]SightWindow{}
	for _, ref := range w.VillagesIn(x0, y0, x1, y1) {
		p := PlayerID(ref.OwnerID)
		if ref.OwnerID == "" || ref.OwnerID == owner || ref.OwnerID == targetOwner || !IsHumanID(p) {
			continue
		}
		s1, s2, ok := segmentCircle(from, to, FieldPoint{X: float64(ref.X), Y: float64(ref.Y)}, r)
		if !ok {
			continue
		}
		// Rounded up, so a refresh at the event already finds the army inside (or outside) the radius.
		per[p] = append(per[p], SightWindow{Player: p, From: c.DepartedAt + int64(math.Ceil(s1*span)), To: c.DepartedAt + int64(math.Ceil(s2*span))})
	}
	var out []SightWindow
	for _, p := range slices.Sorted(maps.Keys(per)) {
		ws := per[p]
		slices.SortFunc(ws, func(a, b SightWindow) int { return int(a.From - b.From) })
		merged := []SightWindow{ws[0]}
		for _, win := range ws[1:] {
			if top := &merged[len(merged)-1]; win.From <= top.To {
				top.To = max(top.To, win.To)
			} else {
				merged = append(merged, win)
			}
		}
		out = append(out, merged...)
	}
	return out
}

// scheduleSight queues the moments humans start and stop seeing c pass.
func scheduleSight(w World, c *Command) {
	for _, win := range SightWindows(w, c) {
		key := "sight:" + c.ID + ":" + string(win.Player)
		w.Schedule(key+":in", win.From)
		w.Schedule(key+":out", win.To)
	}
}

func onSightEvent(w World, ref string) error {
	parts := strings.Split(ref, ":")
	if len(parts) == 3 {
		w.Notify(PlayerID(parts[1]))
	}
	return nil
}

// IsArmy says whether c is troops on the road that may be watched and met: an attack, support or return.
func IsArmy(c *Command) bool { return interceptable(c) }

// ownVillages are the viewer's villages, by ID and as points on the map.
type ownVillages struct {
	ids    map[string]bool
	points []FieldPoint
}

func ownVillagesOf(w World, viewer PlayerID) ownVillages {
	o := ownVillages{ids: map[string]bool{}}
	if p := w.Player(viewer); p != nil {
		for _, id := range p.VillageIDs {
			o.ids[id] = true
			if pt, ok := pointOf(id); ok {
				o.points = append(o.points, pt)
			}
		}
	}
	return o
}

// passingNear says whether c is a foreign army within sight of one of viewer's villages at time now.
func passingNear(w World, viewer PlayerID, c *Command, now int64, mine ownVillages, r float64) bool {
	if !interceptable(c) || now < c.DepartedAt || now >= c.ArrivesAt || mine.ids[CommandHome(c)] || mine.ids[c.TargetID] {
		return false
	}
	p, ok := PositionAt(c, now)
	if !ok {
		return false
	}
	for _, v := range mine.points {
		if fieldDistance(p, v) <= r+1e-9 {
			owner, _ := watchers(w, c)
			return owner != "" && owner != Ref(viewer)
		}
	}
	return false
}

// Sighted are the other players' armies passing within sight of viewer's villages at time now, sorted by ID.
func Sighted(w World, viewer PlayerID, now int64) []*Command {
	cfg := w.Config()
	if !FieldBattlesOn(cfg) || cfg.NoSight {
		return nil
	}
	mine := ownVillagesOf(w, viewer)
	if len(mine.points) == 0 {
		return nil
	}
	r := SightRadiusOf(cfg)
	var out []*Command
	for _, c := range w.ArmiesMarching(now) {
		if passingNear(w, viewer, c, now, mine, r) {
			out = append(out, c)
		}
	}
	return out
}
