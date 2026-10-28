package game

import "math"

// Where armies are between villages. Villages stand at their integer coordinates, as Distance measures them;
// a command whose OriginID or TargetID is empty starts or ends at its Point instead.

func pointOf(id string) (FieldPoint, bool) {
	x, y, ok := parseVillageID(id)
	return FieldPoint{X: float64(x), Y: float64(y)}, ok
}

// LegEnds is where a command starts and where it is going.
func LegEnds(c *Command) (from, to FieldPoint, ok bool) {
	var okFrom, okTo bool
	if c.OriginID == "" && c.Point != nil {
		from, okFrom = *c.Point, true
	} else {
		from, okFrom = pointOf(c.OriginID)
	}
	if c.TargetID == "" && c.Point != nil || c.Type == "intercept" && c.Point != nil {
		to, okTo = *c.Point, true
	} else {
		to, okTo = pointOf(c.TargetID)
	}
	return from, to, okFrom && okTo
}

// PositionAt is where a command's army is at time t: at its start before it leaves, at its end once it arrives.
func PositionAt(c *Command, t int64) (FieldPoint, bool) {
	from, to, ok := LegEnds(c)
	if !ok {
		return FieldPoint{}, false
	}
	return lerp(from, to, progress(c.DepartedAt, c.ArrivesAt, t)), true
}

func progress(departedAt, arrivesAt, t int64) float64 {
	if arrivesAt <= departedAt {
		return 1
	}
	return math.Max(0, math.Min(1, float64(t-departedAt)/float64(arrivesAt-departedAt)))
}

func lerp(a, b FieldPoint, p float64) FieldPoint {
	return FieldPoint{X: a.X + (b.X-a.X)*p, Y: a.Y + (b.Y-a.Y)*p}
}

func fieldDistance(a, b FieldPoint) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// MeetingTime is the first moment from now at which an army leaving home at minPerTile can stand where the
// target is, and that place. The time is the game's own travel time to the place, so the event lands exactly
// when the army gets there; ok is false when that is not before the target arrives.
func MeetingTime(from, to FieldPoint, departedAt, arrivesAt int64, home FieldPoint, minPerTile, unitSpeed float64, now int64) (int64, FieldPoint, bool) {
	if now >= arrivesAt || minPerTile <= 0 || unitSpeed <= 0 {
		return 0, FieldPoint{}, false
	}
	v := unitSpeed / (minPerTile * float64(MsPerMinute))
	var vx, vy float64
	start := lerp(from, to, progress(departedAt, arrivesAt, now))
	if now >= departedAt && arrivesAt > departedAt {
		span := float64(arrivesAt - departedAt)
		vx, vy = (to.X-from.X)/span, (to.Y-from.Y)/span
	}
	dx, dy := start.X-home.X, start.Y-home.Y
	// |D + V·s| = v·s for the smallest s ≥ 0.
	a := vx*vx + vy*vy - v*v
	b := 2 * (dx*vx + dy*vy)
	c := dx*dx + dy*dy
	s, found := smallestRoot(a, b, c)
	if !found {
		return 0, FieldPoint{}, false
	}
	at := FieldPoint{X: start.X + vx*s, Y: start.Y + vy*s}
	t := now + TravelTimeMs(fieldDistance(home, at), minPerTile, unitSpeed)
	if t >= arrivesAt {
		return 0, FieldPoint{}, false
	}
	return t, at, true
}

// smallestRoot is the smallest s ≥ 0 with a·s² + b·s + c = 0.
func smallestRoot(a, b, c float64) (float64, bool) {
	if c == 0 {
		return 0, true
	}
	if math.Abs(a) < 1e-18 {
		if b >= 0 {
			return 0, false
		}
		return -c / b, true
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	sq := math.Sqrt(disc)
	r1, r2 := (-b-sq)/(2*a), (-b+sq)/(2*a)
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	switch {
	case r1 >= 0:
		return r1, true
	case r2 >= 0:
		return r2, true
	}
	return 0, false
}
