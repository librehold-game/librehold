package game

import "slices"

// Bots on the road: a bot meets a human attack coming at it when its cavalry and axes clearly outweigh what it
// expects, and a strong bot scouts human attacks passing by. Bots leave each other's armies alone, so a world of
// bots plays as before.

var fieldForceIDs = []UnitID{UAxe, ULightCav, UMountedArcher, UHeavyCav}

const (
	fieldWeakNotice   = 0.3
	fieldScouts       = 5
	fieldScoutedKeep  = 50
	fieldInterceptMin = 0.6 // the expected wave is under this share of what the bot can send
)

func fieldForce(v *Village) UnitCounts {
	var u UnitCounts
	for _, id := range fieldForceIDs {
		u[id] = v.Units[id]
	}
	return u
}

// threatenedFrom says whether one of the bot's villages already sent an interception at c.
func threatenedFrom(w World, c *Command, own map[string]bool) bool {
	return slices.ContainsFunc(c.Threats, func(id string) bool {
		in := w.Command(id)
		return in != nil && own[in.OriginID]
	})
}

// nearestTo is the bot's village closest to where c is now, among those that have something to send.
func nearestTo(w World, mine []*Village, c *Command, has func(v *Village) bool) *Village {
	at, ok := PositionAt(c, w.Now())
	if !ok {
		return nil
	}
	var best *Village
	bestD := 0.0
	for _, v := range mine {
		if !has(v) {
			continue
		}
		if d := fieldDistance(at, FieldPoint{X: float64(v.X), Y: float64(v.Y)}); best == nil || d < bestD {
			best, bestD = v, d
		}
	}
	return best
}

func PlanFieldBattles(w World, botID PlayerID) {
	b := w.Brain(botID)
	if b == nil || !FieldBattlesOn(w.Config()) {
		return
	}
	mine := PlayerVillages(w, botID)
	own := map[string]bool{}
	for _, v := range mine {
		own[v.ID] = true
	}
	strong := b.Skill >= BotConfig.StrongSkill
	notices := func(c *Command) bool {
		return strong || Mulberry32(HiddenSeed(w, "field", botID, c.ID))() < fieldWeakNotice
	}
	send := func(from *Village, c *Command, u UnitCounts, scout bool) bool {
		if len(CheckIntercept(w, botID, from.ID, c.ID, u, scout).Blockers) > 0 {
			return false
		}
		return actSoft(w, botID, "intercept", logArgs{"originId": from.ID, "commandId": c.ID, "units": u, "scout": scout}, func() error {
			_, err := SendIntercept(w, botID, from.ID, c.ID, u, scout)
			return err
		})
	}
	for _, v := range mine {
		for _, c := range w.CommandsTo(v.ID) {
			if c.Type != "attack" || own[c.OriginID] || w.Human(commandOwner(w, c)) == nil || threatenedFrom(w, c, own) || !notices(c) {
				continue
			}
			wave := estimateWave(w, botID, c)
			if wave == 0 {
				continue
			}
			from := nearestTo(w, mine, c, func(v *Village) bool { return unitsAttack(fieldForce(v)) > 0 })
			if from == nil {
				continue
			}
			if u := fieldForce(from); wave < fieldInterceptMin*unitsAttack(u) {
				send(from, c, u, false)
			}
		}
	}
	// Passing armies cost a scan of every command, so bots watch the road only in the small short worlds.
	if !strong || !IsShort(w.Config()) {
		return
	}
	for _, c := range Sighted(w, botID, w.Now()) {
		if c.Type != "attack" || slices.Contains(b.FieldScouted, c.ID) || w.Human(commandOwner(w, c)) == nil {
			continue
		}
		if t := w.Village(c.TargetID); t != nil && w.Human(PlayerID(t.OwnerID)) != nil {
			continue
		}
		from := nearestTo(w, mine, c, func(v *Village) bool { return v.Units[UScout] > 0 })
		if from == nil {
			continue
		}
		if send(from, c, UnitCounts{UScout: min(fieldScouts, from.Units[UScout])}, true) {
			b.FieldScouted = append(b.FieldScouted, c.ID)
			if over := len(b.FieldScouted) - fieldScoutedKeep; over > 0 {
				b.FieldScouted = b.FieldScouted[over:]
			}
		}
	}
}
