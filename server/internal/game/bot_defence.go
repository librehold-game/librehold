package game

import (
	"slices"
	"sort"
)

// Bot defence: reacting to incoming attacks with support, dodges and the wall.

var defensiveUnitIDs = []UnitID{USpear, USword, UArcher, UHeavyCav}

func defenceOf(u UnitCounts) float64 {
	d := 0.0
	for _, id := range UnitIDs {
		d += float64(u[id]) * UnitDefs[id].Defense
	}
	return d
}

// estimateWave: the sender's known offence from memory, else a full nuke for siege/nobles, 10 % for the rest.
func estimateWave(w World, botID PlayerID, c *Command) float64 {
	if k := brainOf(w, botID).Memory.Known[c.OriginID]; k != nil && k.Units != nil {
		if a := unitsAttack(*k.Units); a > 0 {
			return a
		}
	}
	switch LabelIncoming(w, c) {
	case "scout":
		return 0
	case "siege", "noble":
		return BotConfig.NukeAttack
	}
	return 0.1 * BotConfig.NukeAttack
}

func defensiveUnits(v *Village) UnitCounts {
	var out UnitCounts
	for _, u := range defensiveUnitIDs {
		out[u] = v.Units[u]
	}
	return out
}

func stationedDefence(v *Village) float64 {
	d := defenceOf(UnitCounts(v.Units))
	for _, sup := range v.SupportFrom {
		d += defenceOf(sup)
	}
	return d
}

type helperVillage struct {
	v     *Village
	units UnitCounts
}

// tryStack: support from defensive villages that makes it before the hit; sent when together they hold 1.5 × wave.
func tryStack(w World, botID PlayerID, v *Village, c *Command, need float64) bool {
	b := brainOf(w, botID)
	inbound := 0.0
	for _, x := range w.CommandsTo(v.ID) {
		if x.Type == "support" && x.ArrivesAt < c.ArrivesAt {
			inbound += defenceOf(x.Units)
		}
	}
	have := stationedDefence(v) + inbound
	if have >= need {
		return true
	}
	var helpers []helperVillage
	for _, o := range PlayerVillages(w, botID) {
		role, ok := b.Roles[o.ID]
		if !ok {
			role = RoleMixed
		}
		if o.ID == v.ID || role != RoleDef {
			continue
		}
		units := defensiveUnits(o)
		if units.Total() > 0 && w.Now()+TravelTimeForUnits(w, o, v.X, v.Y, units) < c.ArrivesAt {
			helpers = append(helpers, helperVillage{o, units})
		}
	}
	sort.SliceStable(helpers, func(i, j int) bool {
		return Distance(helpers[i].v.X, helpers[i].v.Y, v.X, v.Y) < Distance(helpers[j].v.X, helpers[j].v.Y, v.X, v.Y)
	})
	var plan []helperVillage
	for _, h := range helpers {
		if have >= need {
			break
		}
		plan = append(plan, h)
		have += defenceOf(h.units)
	}
	if have < need {
		return false
	}
	for _, h := range plan {
		if len(CheckSend(w, botID, h.v.ID, v.X, v.Y, h.units, "support").Blockers) == 0 {
			act(w, botID, "send", sendArgs(h.v.ID, v.X, v.Y, h.units, "support"), func() error {
				_, err := SendCommand(w, botID, h.v.ID, v.X, v.Y, h.units, "support", SendOpts{})
				return err
			})
		}
	}
	return true
}

// trySnipe snipes a hetman train with support from the nearest village that makes it in time.
func trySnipe(w World, botID PlayerID, v *Village, train Train) {
	cmds := slices.Clone(train.Commands)
	sort.SliceStable(cmds, func(i, j int) bool { return cmds[i].ArrivesAt < cmds[j].ArrivesAt })
	if len(cmds) < 2 {
		return
	}
	mid := cmds[0].ArrivesAt + (cmds[1].ArrivesAt-cmds[0].ArrivesAt)/2
	var best *helperVillage
	for _, o := range PlayerVillages(w, botID) {
		units := defensiveUnits(o)
		if o.ID == v.ID || units.Total() == 0 || w.Now()+TravelTimeForUnits(w, o, v.X, v.Y, units) > mid {
			continue
		}
		if best == nil || Distance(o.X, o.Y, v.X, v.Y) < Distance(best.v.X, best.v.Y, v.X, v.Y) {
			best = &helperVillage{o, units}
		}
	}
	if best != nil {
		ids := []string{cmds[0].ID, cmds[1].ID}
		args := logArgs{"commandIds": ids, "fromVillageId": best.v.ID, "units": best.units}
		actSoft(w, botID, "snipe", args, func() error {
			_, err := PlanSnipe(w, botID, ids, best.v.ID, best.units)
			return err
		})
	}
}

// PlanDefence, at every think (and phone glance): for each attacked village — stack when support makes it and
// holds, else dodge (weak: half the time it "does not notice"); a strong bot snipes hetman trains; before the hit
// it spends resources above 30 % of storage. One reaction per village at a time.
func PlanDefence(w World, botID PlayerID) {
	b := w.Brain(botID)
	if b == nil {
		return
	}
	mine := PlayerVillages(w, botID)
	own := map[string]bool{}
	for _, v := range mine {
		own[v.ID] = true
	}
	strong := b.Skill >= BotConfig.StrongSkill
	for _, v := range mine {
		var incoming []*Command
		for _, c := range w.CommandsTo(v.ID) {
			if c.Type == "attack" && !own[c.OriginID] {
				incoming = append(incoming, c)
			}
		}
		if len(incoming) == 0 {
			continue
		}
		storage := VillageStorageCapacity(v)
		if v.Resources.Wood > 0.3*storage || v.Resources.Clay > 0.3*storage || v.Resources.Iron > 0.3*storage {
			PlanBuilding(w, botID, v)
			PlanRecruitment(w, botID, v)
		}
		if strong {
			for _, train := range IncomingTrains(w, v.ID) {
				sniping := slices.ContainsFunc(w.ScheduledSendsOf(botID), func(s *ScheduledSend) bool {
					return s.Type == "support" && s.X == v.X && s.Y == v.Y
				})
				if !sniping {
					trySnipe(w, botID, v, train)
				}
			}
		}
		var worst *Command
		worstPower := 0.0
		for _, c := range incoming {
			p := estimateWave(w, botID, c)
			if p > worstPower || (p == worstPower && worst != nil && c.ArrivesAt < worst.ArrivesAt) {
				worst, worstPower = c, p
			}
		}
		if worst == nil || worstPower == 0 {
			continue
		}
		if tryStack(w, botID, v, worst, 1.5*worstPower) {
			continue
		}
		dodging := slices.ContainsFunc(w.ScheduledSendsOf(botID), func(s *ScheduledSend) bool {
			return s.OriginID == v.ID && s.CancelAt != nil
		})
		if dodging {
			continue
		}
		// A weak bot "does not notice" half the attacks — fixed per wave, not drawn at every think.
		if strong || Mulberry32(HiddenSeed(w, "notice", botID, worst.ID))() < 0.5 {
			c := worst
			actSoft(w, botID, "dodge", logArgs{"commandId": c.ID}, func() error { _, err := PlanDodge(w, botID, c.ID); return err })
		}
	}
}
