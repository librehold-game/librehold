package game

import (
	"math"
	"slices"
)

// Field battles: an army sent to meet another on the road. It marches to a meeting point fixed when it is sent
// and resolves there in a "field:" event, against whatever is left of the army it went after.

// InterceptCheck says whether an interception can go, and when and where it would meet its target.
type InterceptCheck struct {
	Blockers []Blocker
	MeetAt   int64
	Point    FieldPoint
}

// commandOwner is the player a command belongs to: the owner of its home village.
func commandOwner(w World, c *Command) PlayerID {
	if home := w.Village(CommandHome(c)); home != nil {
		return PlayerID(home.OwnerID)
	}
	return ""
}

func interceptable(c *Command) bool {
	return c.Type == "attack" || c.Type == "support" || c.Type == "return"
}

// CanSeeCommand says whether viewer knows this army is on the road: it is coming at one of the viewer's villages,
// or passing within sight of one.
func CanSeeCommand(w World, viewer PlayerID, c *Command) bool {
	if c.TargetID != "" {
		if v := w.Village(c.TargetID); v != nil && v.OwnerID == Ref(viewer) {
			return true
		}
	}
	cfg := w.Config()
	return FieldBattlesOn(cfg) && passingNear(w, viewer, c, w.Now(), ownVillagesOf(w, viewer), SightRadiusOf(cfg))
}

func CheckIntercept(w World, actor PlayerID, originID, commandID string, u UnitCounts, scout bool) InterceptCheck {
	if !FieldBattlesOn(w.Config()) {
		return InterceptCheck{Blockers: []Blocker{{"kind": "fieldBattlesOff"}}}
	}
	origin := w.Village(originID)
	if origin == nil || origin.OwnerID != Ref(actor) {
		return InterceptCheck{Blockers: []Blocker{{"kind": "notOwner"}}}
	}
	var blockers []Blocker
	n := u.Total()
	if n <= 0 {
		blockers = append(blockers, Blocker{"kind": "noUnits"})
	}
	for _, id := range UnitIDs {
		if want := u[id]; want < 0 || want > origin.Units[id] {
			blockers = append(blockers, Blocker{"kind": "notEnough", "unit": id, "have": origin.Units[id]})
		}
	}
	target := w.Command(commandID)
	var owner PlayerID
	if target != nil {
		owner = commandOwner(w, target)
	}
	if target == nil || !interceptable(target) || owner == "" || owner == actor {
		return InterceptCheck{Blockers: append(blockers, Blocker{"kind": "notInterceptable"})}
	}
	if !CanSeeCommand(w, actor, target) {
		blockers = append(blockers, Blocker{"kind": "notVisible"})
	}
	if IsProtected(w, actor) {
		blockers = append(blockers, Blocker{"kind": "selfProtected"})
	}
	if IsProtected(w, owner) {
		blockers = append(blockers, Blocker{"kind": "targetProtected", "until": w.Player(owner).ProtectedUntil})
	}
	if scout && u[UScout] != n {
		blockers = append(blockers, Blocker{"kind": "scoutsOnly"})
	}
	check := InterceptCheck{Blockers: blockers}
	if n > 0 {
		from, to, ok := LegEnds(target)
		home := FieldPoint{X: float64(origin.X), Y: float64(origin.Y)}
		var meet bool
		if ok {
			check.MeetAt, check.Point, meet = MeetingTime(from, to, target.DepartedAt, target.ArrivesAt, home, SlowestSpeed(u), w.Config().UnitSpeed, w.Now())
		}
		if !meet {
			check.Blockers = append(check.Blockers, Blocker{"kind": "tooLate"})
		}
	}
	return check
}

func SendIntercept(w World, actor PlayerID, originID, commandID string, u UnitCounts, scout bool) (*Command, error) {
	if origin := w.Village(originID); origin != nil && origin.OwnerID == Ref(actor) {
		Touch(w, origin)
	}
	check := CheckIntercept(w, actor, originID, commandID, u, scout)
	if len(check.Blockers) > 0 {
		return nil, Blocked("intercept", check.Blockers...)
	}
	origin, target := w.Village(originID), w.Command(commandID)
	var sent UnitCounts
	for _, id := range UnitIDs {
		if k := u[id]; k > 0 {
			origin.Units[id] -= k
			sent[id] = k
		}
	}
	point := check.Point
	c := &Command{ID: w.NewID("c"), Type: "intercept", OriginID: originID, Units: sent, Point: &point,
		Intercept: &InterceptOrder{CommandID: target.ID, Scout: scout, Leg: Leg{OriginID: target.OriginID, TargetID: target.TargetID,
			DepartedAt: target.DepartedAt, ArrivesAt: target.ArrivesAt}},
		DepartedAt: w.Now(), ArrivesAt: check.MeetAt}
	w.PutCommand(c)
	w.Schedule("field:"+c.ID, c.ArrivesAt)
	target.Threats = append(target.Threats, c.ID)
	countAttack(w, "attack")
	countHumanAttack(w, actor, sent)
	if owner := commandOwner(w, target); w.Human(actor) != nil && w.Brain(owner) != nil {
		OnHumanAggression(w, owner, actor)
	}
	return c, nil
}

func init() { registerHandler("field", onFieldEvent) }

func haulCapacity(u UnitCounts) float64 {
	c := 0.0
	for _, id := range UnitIDs {
		c += float64(u[id]) * UnitDefs[id].Haul
	}
	return c
}

func scaleResources(r Resources, k float64) Resources {
	return Resources{Wood: math.Floor(r.Wood * k), Clay: math.Floor(r.Clay * k), Iron: math.Floor(r.Iron * k)}
}

// SplitLoot divides an army's loot after a field battle: its survivors keep what they can carry, the winner
// takes what is left up to what it can carry, and the rest is lost on the road.
func SplitLoot(haul Resources, keeperCapacity, takerCapacity float64) (keep, take Resources) {
	total := SumResources(haul)
	if total <= 0 {
		return Resources{}, Resources{}
	}
	keep = scaleResources(haul, math.Min(total, math.Max(0, keeperCapacity))/total)
	rest := Resources{Wood: haul.Wood - keep.Wood, Clay: haul.Clay - keep.Clay, Iron: haul.Iron - keep.Iron}
	if restTotal := SumResources(rest); restTotal > 0 {
		take = scaleResources(rest, math.Min(restTotal, math.Max(0, takerCapacity))/restTotal)
	}
	return keep, take
}

// ReturnFromPoint sends troops home from a place in the field.
func ReturnFromPoint(w World, at FieldPoint, homeID string, units UnitCounts, haul *Resources) *Command {
	home, _ := pointOf(homeID)
	travel := TravelTimeMs(fieldDistance(at, home), SlowestSpeed(units), w.Config().UnitSpeed)
	p := at
	c := &Command{ID: w.NewID("c"), Type: "return", TargetID: homeID, Units: units, Point: &p,
		DepartedAt: w.Now(), ArrivesAt: w.Now() + max(1, travel)}
	if haul != nil && SumResources(*haul) > 0 {
		h := *haul
		c.Haul = &h
	}
	AddCommand(w, c)
	return c
}

func researchOf(w World, villageID string) UnitCounts {
	if v := w.Village(villageID); v != nil {
		return UnitCounts(v.Research)
	}
	return UnitCounts{}
}

func fieldReportType(scout bool) string {
	if scout {
		return "fieldScout"
	}
	return "field"
}

// onFieldEvent: an interceptor reaches its meeting point and fights what it finds there, if anything.
func onFieldEvent(w World, id string) error {
	c := w.Command(id)
	if c == nil || c.Intercept == nil || c.Point == nil {
		return nil
	}
	w.DeleteCommand(id)
	at, homeID := *c.Point, c.OriginID
	A := humanOwner(w, w.Village(homeID))
	target := w.Command(c.Intercept.CommandID)
	if target != nil {
		target.Threats = slices.DeleteFunc(target.Threats, func(t string) bool { return t == id })
	}
	leg := c.Intercept.Leg
	field := &FieldReport{X: at.X, Y: at.Y, TargetCommandID: c.Intercept.CommandID}
	if target == nil || target.OriginID != leg.OriginID || target.TargetID != leg.TargetID || target.DepartedAt != leg.DepartedAt ||
		target.ArrivesAt != leg.ArrivesAt || w.Now() >= target.ArrivesAt {
		field.Missed = true
		if target != nil {
			field.TargetType = target.Type
		}
		if A != "" {
			sendReports(w, []PlayerID{A}, Report{Time: w.Now(), Type: fieldReportType(c.Intercept.Scout), OriginID: homeID,
				Origin: ReportVillageOf(w, homeID), Attacker: ReportSide{Units: c.Units}, Outcome: "defenderWon", Field: field})
		}
		ReturnFromPoint(w, at, homeID, c.Units, nil)
		return nil
	}
	field.TargetType = target.Type
	targetHome := CommandHome(target)
	D := humanOwner(w, w.Village(targetHome))
	rng := Mulberry32(HiddenSeed(w, "field", id, target.ID))
	luck := -CombatBalance.LuckRange + rng()*2*CombatBalance.LuckRange
	before := target.Units
	in := BattleInput{
		Attacker: BattleSide{Units: c.Units, Research: researchOf(w, homeID)},
		Defender: BattleDefender{BattleSide: BattleSide{Units: before, Research: researchOf(w, targetHome)}},
		Luck:     luck, Morale: CombatBalance.MoraleVsBarbarian, Rng: rng, LossMode: w.Config().BattleLosses,
	}
	var result BattleResult
	if c.Intercept.Scout || IsScoutMission(c.Units) {
		result = resolveScout(in)
	} else {
		result = ResolveBattle(in)
	}
	r := Report{Time: w.Now(), Type: fieldReportType(result.Kind == "scout"), OriginID: homeID, TargetID: targetHome,
		Origin: ReportVillageOf(w, homeID), Target: ReportVillageOf(w, targetHome),
		Attacker: ReportSide{Units: c.Units, Losses: result.AttackerLosses},
		Luck:     result.Luck, Morale: result.Morale, Outcome: result.Outcome, Field: field}
	var captured *Resources
	if result.Kind == "scout" {
		if result.Outcome == "attackerWon" {
			r.Defender.Units = before
			r.Scout = &ScoutIntel{Units: before}
		}
	} else {
		r.Defender = ReportSide{Units: before, Losses: result.DefenderLosses}
		survivors := result.DefenderSurvivors
		if target.Haul != nil {
			keep, take := SplitLoot(*target.Haul, haulCapacity(survivors), haulCapacity(result.AttackerSurvivors))
			target.Haul = &keep
			if SumResources(take) > 0 {
				captured = &take
				r.Haul = &take
			}
		}
		if survivors.Total() == 0 {
			w.DeleteCommand(target.ID)
			w.Unschedule("cmd:" + target.ID)
		} else {
			target.Units = survivors
			target.FieldHit = true
		}
		lostA, lostD := result.AttackerLosses.Total(), result.DefenderLosses.Total()
		if A != "" {
			s := &w.Human(A).Stats
			s.UnitsLost += lostA
			s.UnitsKilled += lostD
			if result.Outcome == "attackerWon" {
				s.BattlesWon++
			}
		}
		if D != "" {
			s := &w.Human(D).Stats
			s.UnitsLost += lostD
			s.UnitsKilled += lostA
		}
		countBattle(w, c.Units.Total()+before.Total(), "")
		recordPublicEvent(w, "battle", ownerOf(w, homeID), ownerOf(w, targetHome), c.Units.Total()+before.Total(),
			int(math.Round(at.X)), int(math.Round(at.Y)))
		battle := TimelineEntry{Kind: "battle", X: at.X, Y: at.Y, Attacker: ownerOf(w, homeID), Defender: ownerOf(w, targetHome),
			Troops: c.Units.Total() + before.Total()}
		if home := w.Village(homeID); home != nil {
			battle.FromX, battle.FromY = home.X, home.Y
		}
		recordTimeline(w, battle)
	}
	var to []PlayerID
	for _, p := range []PlayerID{A, D} {
		if p != "" && !slices.Contains(to, p) {
			to = append(to, p)
		}
	}
	sendReports(w, to, r)
	if result.AttackerSurvivors.Total() > 0 {
		ReturnFromPoint(w, at, homeID, result.AttackerSurvivors, captured)
	}
	return nil
}
