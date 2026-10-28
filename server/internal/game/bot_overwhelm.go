package game

import (
	"math"
	"sort"
)

// Overwhelmed bots give up. QuitPlayer serves any player ("leave world").

func init() { registerHandler("abandon", onAbandonEvent) }

// neighbourMedianPoints is the median points of neighbours who started at a similar time (±BotConfig.PeerJoinDays):
// "falling behind" means behind those who started with me.
func neighbourMedianPoints(w World, botID PlayerID) (float64, bool) {
	mine := PlayerVillages(w, botID)
	if len(mine) == 0 {
		return 0, false
	}
	joinAt := w.Player(botID).JoinAt
	r := BotConfig.KnowRadius
	x0, y0, x1, y1 := mine[0].X, mine[0].Y, mine[0].X, mine[0].Y
	for _, m := range mine {
		x0, y0, x1, y1 = min(x0, m.X), min(y0, m.Y), max(x1, m.X), max(y1, m.Y)
	}
	ri := int(r)
	near := map[PlayerID]bool{}
	for _, v := range w.VillagesIn(x0-ri, y0-ri, x1+ri, y1+ri) {
		owner := PlayerID(v.OwnerID)
		if owner == "" || owner == botID || near[owner] {
			continue
		}
		p := w.Player(owner)
		if p == nil || math.Abs(float64(p.JoinAt-joinAt)) > BotConfig.PeerJoinDays*float64(MsPerDay) {
			continue
		}
		for _, m := range mine {
			if Distance(m.X, m.Y, v.X, v.Y) <= r {
				near[owner] = true
				break
			}
		}
	}
	if len(near) == 0 {
		return 0, false
	}
	points := w.PointsByOwner()
	pts := make([]int, 0, len(near))
	for id := range near {
		pts = append(pts, points[id])
	}
	sort.Ints(pts)
	return float64(pts[len(pts)/2]), true
}

// UpdateOverwhelm, at session start: overwhelm decays (×0.85) and grows with lost villages, attacks and falling
// behind neighbours. Attacks weigh less the better the player ("strong players rarely leave").
func UpdateOverwhelm(w World, botID PlayerID) {
	p, b := w.Player(botID), w.Brain(botID)
	if p == nil || b == nil || p.Status != StatusActive {
		return
	}
	st := b.LastSessionStats
	behind := 0.0
	// Falling behind counts only below half the peers' median (0 at 50 %, 1 at zero points).
	if median, ok := neighbourMedianPoints(w, botID); ok && median > 0 {
		behind = math.Max(0, 1-(2*float64(PlayerPoints(w, botID)))/median)
	}
	b.Overwhelm = 0.85*b.Overwhelm + 1.0*float64(st.VillagesLost) + 0.1*(1-0.9*b.Skill)*float64(st.IncomingAttacks) + 0.5*behind
	b.LastSessionStats = SessionStats{}
	if b.Overwhelm > BotConfig.QuitScale*b.Resilience {
		QuitPlayer(w, botID)
	}
}

// QuitPlayer: the player leaves — no more sessions or thinking; the villages stand until abandoned.
func QuitPlayer(w World, id PlayerID) {
	p := w.Player(id)
	if p == nil || p.Status != StatusActive {
		return
	}
	p.Status = StatusQuit
	p.QuitAt = ptrTo(w.Now())
	if b := w.Brain(id); b != nil {
		b.Online = false
	}
	for _, kind := range []string{"session", "think", "sessionEnd", "phone"} {
		w.Unschedule(kind + ":" + id)
	}
	w.Schedule("abandon:"+id, w.Now()+BotDur(w, int64(BotConfig.AbandonAfterDays*float64(MsPerDay))))
}

// onAbandonEvent: the villages of a player who left turn barbarian with the buildings they had.
func onAbandonEvent(w World, id PlayerID) error {
	p := w.Player(id)
	if p == nil || p.Status != StatusQuit {
		return nil
	}
	for _, v := range PlayerVillages(w, id) {
		TransferVillage(w, v, "", "")
	}
	return nil
}
