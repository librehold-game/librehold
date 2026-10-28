package game

import (
	"math"
	"sort"
)

// Incoming attacks: labels read from the march time, noble trains, snipes and dodges.

// ThreatClass is the class of an incoming wave's slowest unit, read from the march time as players do.
type ThreatClass string

var threatClassOf = [NumUnits]ThreatClass{
	USpear: "infantry", USword: "infantry", UAxe: "infantry", UArcher: "infantry", UScout: "scout",
	ULightCav: "cavalry", UMountedArcher: "cavalry", UHeavyCav: "cavalry", URam: "siege", UCatapult: "siege",
	UPaladin: "cavalry", UNoble: "noble",
}

// ClassifySpeed is the class of the unit whose speed is closest to the measured minutes per tile.
func ClassifySpeed(minPerTile float64) ThreatClass {
	best, bestDiff := ThreatClass("infantry"), math.Inf(1)
	for _, u := range UnitIDs {
		if diff := math.Abs(UnitDefs[u].Speed - minPerTile); diff < bestDiff {
			best, bestDiff = threatClassOf[u], diff
		}
	}
	return best
}

func minPerTileOf(w World, c *Command, origin, target *Village) float64 {
	d := Distance(origin.X, origin.Y, target.X, target.Y)
	if d <= 0 {
		return -1
	}
	return float64(c.ArrivesAt-c.DepartedAt) / float64(MsPerMinute) * w.Config().UnitSpeed / d
}

// InferThreatClass is the class of an incoming command's slowest unit, inferred from its march time.
func InferThreatClass(w World, c *Command, origin, target *Village) ThreatClass {
	m := minPerTileOf(w, c, origin, target)
	if m < 0 {
		return "infantry"
	}
	return ClassifySpeed(m)
}

type IncomingLabel string

// labelSpeed are the base minutes per tile of each label.
var labelSpeed = []struct {
	label IncomingLabel
	speed float64
}{{"scout", 9}, {"cavalry", 10}, {"heavyCav", 11}, {"infantry", 18}, {"sword", 22}, {"siege", 30}, {"noble", 35}}

// LabelIncoming labels an incoming command by the slowest unit class read from its march time.
func LabelIncoming(w World, c *Command) IncomingLabel {
	origin, target := w.Village(c.OriginID), w.Village(c.TargetID)
	if origin == nil || target == nil {
		return "infantry"
	}
	m := minPerTileOf(w, c, origin, target)
	if m < 0 {
		return "infantry"
	}
	best, bestDiff := IncomingLabel("infantry"), math.Inf(1)
	for _, l := range labelSpeed {
		if diff := math.Abs(l.speed - m); diff < bestDiff {
			best, bestDiff = l.label, diff
		}
	}
	return best
}

type Train struct {
	Commands []*Command
	FirstAt  int64
	LastAt   int64
}

// IncomingTrains are hetman trains: attacks labelled "noble" on a village, landing at most 1 s apart.
func IncomingTrains(w World, villageID string) []Train {
	var nobles []*Command
	for _, c := range w.CommandsTo(villageID) {
		if c.Type == "attack" && LabelIncoming(w, c) == "noble" {
			nobles = append(nobles, c)
		}
	}
	sort.SliceStable(nobles, func(i, j int) bool { return nobles[i].ArrivesAt < nobles[j].ArrivesAt })
	var all []Train
	for _, c := range nobles {
		if n := len(all); n > 0 && c.ArrivesAt-all[n-1].LastAt <= 1000 {
			all[n-1].Commands = append(all[n-1].Commands, c)
			all[n-1].LastAt = c.ArrivesAt
		} else {
			all = append(all, Train{Commands: []*Command{c}, FirstAt: c.ArrivesAt, LastAt: c.ArrivesAt})
		}
	}
	var out []Train
	for _, t := range all {
		if len(t.Commands) >= 2 {
			out = append(out, t)
		}
	}
	return out
}

// PlanSnipe schedules support from fromVillageID landing exactly between the first and second noble of a train
// heading for one of the actor's villages.
func PlanSnipe(w World, actor PlayerID, trainCommandIDs []string, fromVillageID string, u UnitCounts) (*ScheduledSend, error) {
	var cmds []*Command
	for _, id := range trainCommandIDs {
		c := w.Command(id)
		if c == nil || c.Type != "attack" {
			return nil, Rule("noCommand", map[string]any{"id": id})
		}
		cmds = append(cmds, c)
	}
	sort.SliceStable(cmds, func(i, j int) bool { return cmds[i].ArrivesAt < cmds[j].ArrivesAt })
	if len(cmds) < 2 {
		return nil, Rule("smallGap", nil)
	}
	target := w.Village(cmds[0].TargetID)
	if target == nil || target.OwnerID != Ref(actor) || cmds[1].TargetID != target.ID {
		return nil, NotOwner("snipe")
	}
	n1, n2 := cmds[0], cmds[1]
	if n2.ArrivesAt-n1.ArrivesAt < 2 {
		return nil, Rule("smallGap", nil)
	}
	return ScheduleSend(w, actor, ScheduleSendInput{OriginID: fromVillageID, X: target.X, Y: target.Y, Units: u,
		Type: "support", Mode: "arriveAt", At: n1.ArrivesAt + (n2.ArrivesAt-n1.ArrivesAt)/2})
}

// PlanDodge: every unit leaves before the hit (support to the nearest own village, or else an attack on the
// nearest barbarian) and turns back to come home 1 s after it. The recall must fit the command's cancel window,
// so the march lasts the window minus half a second.
func PlanDodge(w World, actor PlayerID, commandID string) (*ScheduledSend, error) {
	c := w.Command(commandID)
	if c == nil {
		return nil, Rule("noCommand", map[string]any{"id": commandID})
	}
	v := w.Village(c.TargetID)
	if v == nil || v.OwnerID != Ref(actor) {
		return nil, NotOwner("dodge")
	}
	Touch(w, v)
	var units UnitCounts
	for _, u := range UnitIDs {
		units[u] = v.Units[u]
	}
	if units.Total() == 0 {
		return nil, Rule("noTroops", nil)
	}
	outward := CommandBalance.CancelWindowMs - 500
	sendAt := c.ArrivesAt + 1000 - 2*outward
	if sendAt < w.Now() {
		return nil, Rule("dodgeTooLate", nil)
	}
	reachable := func(x, y int) bool { return TravelTimeForUnits(w, v, x, y, units) > outward }
	var own []*Village
	for _, o := range PlayerVillages(w, actor) {
		if o.ID != v.ID && reachable(o.X, o.Y) {
			own = append(own, o)
		}
	}
	sort.SliceStable(own, func(i, j int) bool {
		return Distance(v.X, v.Y, own[i].X, own[i].Y) < Distance(v.X, v.Y, own[j].X, own[j].Y)
	})
	var dx, dy int
	typ := ""
	if len(own) > 0 {
		dx, dy, typ = own[0].X, own[0].Y, "support"
	} else {
		for _, t := range GeneratedVillagesNear(w.Seed(), w.Config(), v.X, v.Y, 15) {
			stored := w.Village(t.ID)
			if (stored == nil || stored.OwnerID == "") && reachable(t.X, t.Y) {
				dx, dy, typ = t.X, t.Y, "attack"
				break
			}
		}
	}
	if typ == "" {
		return nil, Rule("nowhere", nil)
	}
	cancelAt := sendAt + outward
	return ScheduleSend(w, actor, ScheduleSendInput{OriginID: v.ID, X: dx, Y: dy, Units: units, Type: typ,
		Mode: "sendAt", At: sendAt, CancelAt: &cancelAt})
}
