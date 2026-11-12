package game

import (
	"math"
	"sort"
)

// Attack arrivals, for many humans: every human involved gets a report, intel is the attacker's.

func init() { attackArrival = ResolveAttackArrival }

// DefenderUnitsOf is the garrison plus all stationed support.
func DefenderUnitsOf(v *Village) UnitCounts {
	var all UnitCounts
	for _, u := range UnitIDs {
		all[u] = v.Units[u]
	}
	for _, sup := range v.SupportFrom {
		for _, u := range UnitIDs {
			all[u] += sup[u]
		}
	}
	return all
}

func sortedSupportHomes(v *Village) []string {
	homes := make([]string, 0, len(v.SupportFrom))
	for h := range v.SupportFrom {
		homes = append(homes, h)
	}
	sort.Strings(homes)
	return homes
}

// applyDefenderLosses splits the defender's losses between garrison and support in proportion (the remainder
// from the largest share).
func applyDefenderLosses(v *Village, losses UnitCounts) {
	homes := sortedSupportHomes(v)
	for _, u := range UnitIDs {
		loss := losses[u]
		if loss <= 0 {
			continue
		}
		have := make([]int, 1+len(homes))
		have[0] = v.Units[u]
		total := have[0]
		for i, h := range homes {
			have[i+1] = v.SupportFrom[h][u]
			total += have[i+1]
		}
		if total <= 0 {
			continue
		}
		take := make([]int, len(have))
		rest := loss
		for i, n := range have {
			take[i] = int(math.Floor(float64(loss) * float64(n) / float64(total)))
			rest -= take[i]
		}
		order := make([]int, len(have))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool { return have[order[a]] > have[order[b]] })
		for guard := 0; rest > 0 && guard < 1000; guard++ {
			for _, i := range order {
				if rest <= 0 {
					break
				}
				if take[i] < have[i] {
					take[i]++
					rest--
				}
			}
		}
		v.Units[u] -= take[0]
		for i, h := range homes {
			sup := v.SupportFrom[h]
			sup[u] -= take[i+1]
			v.SupportFrom[h] = sup
		}
	}
	for _, h := range homes {
		if v.SupportFrom[h].Total() == 0 {
			delete(v.SupportFrom, h)
		}
	}
}

// humanID returns the player as a human id, or "" for bots and barbarians.
func humanID(w World, id PlayerID) PlayerID {
	if id == "" || w.Human(id) == nil {
		return ""
	}
	return id
}

// humanOwner returns the owner as a human id, or "" for bots and barbarians.
func humanOwner(w World, v *Village) PlayerID {
	if v == nil {
		return ""
	}
	return humanID(w, PlayerID(v.OwnerID))
}

// reportRecipients: the attacking human, the defending human and every human with support in the target.
func reportRecipients(w World, attacker, defender PlayerID, target *Village) []PlayerID {
	var out []PlayerID
	add := func(id PlayerID) {
		if id != "" && w.Human(id) != nil {
			for _, x := range out {
				if x == id {
					return
				}
			}
			out = append(out, id)
		}
	}
	add(attacker)
	add(defender)
	for _, h := range sortedSupportHomes(target) {
		if home := w.Village(h); home != nil {
			add(PlayerID(home.OwnerID))
		}
	}
	return out
}

// defendingHumans are the human players whose troops stood in the target: its owner and everyone with support
// there. They fight the battle together, so each of them sees his own side of it.
func defendingHumans(w World, target *Village) map[PlayerID]bool {
	out := map[PlayerID]bool{}
	add := func(id PlayerID) {
		if id != "" && w.Human(id) != nil {
			out[id] = true
		}
	}
	if target.OwnerID != "" {
		add(PlayerID(target.OwnerID))
	}
	for _, h := range sortedSupportHomes(target) {
		if home := w.Village(h); home != nil {
			add(PlayerID(home.OwnerID))
		}
	}
	return out
}

func sendReports(w World, to []PlayerID, r Report) {
	for _, id := range to {
		copy := r
		// Only the sender learns the auto-farm sent the troops.
		copy.AutoFarm = r.AutoFarm && id == r.Origin.OwnerID
		w.AddReport(id, &copy)
	}
}

// wipedOut says whether one side of a battle came home with nobody at all.
func wipedOut(side ReportSide) bool {
	total := side.Units.Total()
	return total > 0 && side.Losses.Total() >= total
}

// blindSide clears the numbers of one side, keeping only the flag that says it is unknown. A reader who cannot
// know the enemy's army must not be able to read it off the wire either, so the troop counts leave the report
// rather than only being flagged for the view to skip.
func blindSide(side *ReportSide) {
	*side = ReportSide{}
}

// blindedFor is the report one reader gets, which depends on whether *that reader* was wiped out.
//
// Nobody comes home to count the enemy: whoever died cannot tell his lord what killed him. So a side that lost every
// man reads only what its own lord already knew — the troops he sent and that they were all lost — with the enemy's
// numbers withheld. The surviving side saw the fight and reads it in full.
//
// A village is the exception, for one side: the wall it was fought over is still standing, and the defender who lost
// his army there is standing on it. He watched the attack come in and break, so he reads the whole battle even when
// nothing of his survived. What he cannot learn that way is an attack that never reached the wall at all — and that
// is a field battle, not this one.
//
// Supporters fight on the defending side, so they are blinded no more and no less than the garrison: a supporter
// whose troops died on somebody else's wall reads his own side, his own losses included, but does not learn the
// attacker's army. The wall is the village owner's alone, which is why the exception above does not reach a supporter.
//
// The decision reads the losses, not who was nominally attacking, because both roles can be wiped out: an interceptor
// is the attacker in his own report, and a noble train dying on the wall leaves the attacker just as blind as one
// destroyed on the road.
//
// Every reader always sees his own side, even when both armies are gone: each copy of the report is blinded from
// scratch, and only the enemy's side is cleared from it.
func blindedFor(r Report, reader PlayerID, defenders map[PlayerID]bool) Report {
	switch r.Type {
	case "attack", "conquest", "field":
	case "scout", "fieldScout":
		// A scouting mission reports by whether it got through, not by who happened to survive: a scout that
		// slipped past its counterparts came back able to draw the garrison, so his lord reads it in full, and one
		// caught short brings nothing home at all. That is also why this reads the outcome rather than the losses —
		// the scouts can all die on a mission that still got its look at the village.
		//
		// The village's own lord is never blinded here: what he was not told is inside his own walls.
		if r.Outcome != "attackerWon" && r.Origin.OwnerID == reader {
			blindSide(&r.Defender)
			r.DefenderHidden = true
		}
		return r
	default:
		// Support and market reports carry no battle.
		return r
	}
	readAttacker := r.Origin.OwnerID == reader
	onDefendingSide := r.Target.OwnerID == reader || defenders[reader]
	// A defender cannot see the attacker, so the report has to say whether the attacker saw him. Nothing came home
	// from this fight, so if every attacker died nobody rode away to describe the garrison either. A field battle is
	// fought on the road with no wall to stand behind, so this only speaks for an attack on a village. It is set
	// before the wall exception below, which returns early for the village's own lord.
	if onDefendingSide && (r.Type == "attack" || r.Type == "conquest") {
		seen := !wipedOut(r.Attacker) && r.Attacker.Units.Total() > 0
		r.DefenderTroopsSeen = &seen
	}
	// The owner of the village sees the attack even if his garrison died: he was behind the wall it broke on.
	if r.Target.OwnerID == reader && (r.Type == "attack" || r.Type == "conquest") {
		return r
	}
	// A wiped-out attacker learns nothing of what beat it; likewise a wiped-out defence learns nothing of what
	// broke it. Only the enemy's side is ever cleared, so the reader keeps his own troops either way.
	if wipedOut(r.Attacker) && readAttacker {
		blindSide(&r.Defender)
		r.DefenderHidden = true
	}
	if wipedOut(r.Defender) && onDefendingSide {
		blindSide(&r.Attacker)
		r.AttackerHidden = true
	}
	return r
}

// sendBattleReports delivers a battle to every side, each seeing only what it could know. defenders are the human
// players fighting on the defending side, so a supporter is never blinded against his own troops.
func sendBattleReports(w World, to []PlayerID, r Report, defenders map[PlayerID]bool) {
	for _, id := range to {
		copy := blindedFor(r, id, defenders)
		// Only the sender learns the auto-farm sent the troops.
		copy.AutoFarm = r.AutoFarm && id == r.Origin.OwnerID
		w.AddReport(id, &copy)
	}
}

// ResolveAttackArrival: the full battle, reports, loot, return, loyalty and conquest.
func ResolveAttackArrival(w World, c *Command, target *Village) error {
	origin := w.Village(c.OriginID)
	attackerOwner, loose := Ref(commandOwner(w, c)), c.OwnerID != ""
	// The attack reached a village that became the attacker's own meanwhile (e.g. after the first noble of a
	// train): the troops stay there as support instead of fighting their own defenders. An army cut loose from its
	// village has no home to stay for, and is lost.
	if attackerOwner != "" && attackerOwner == target.OwnerID {
		if !loose {
			slot := target.SupportFrom[c.OriginID]
			for _, u := range UnitIDs {
				slot[u] += c.Units[u]
			}
			target.SupportFrom[c.OriginID] = slot
		}
		return nil
	}
	Touch(w, target)

	rng := Mulberry32(HiddenSeed(w, c.ID))
	luck := -CombatBalance.LuckRange + rng()*2*CombatBalance.LuckRange
	defenderUnits := DefenderUnitsOf(target)
	shares := DefenderShares(w, target)
	loyaltyBefore := target.Loyalty
	result := ResolveBattle(BattleInput{
		Attacker: BattleSide{Units: c.Units, Research: commandResearch(w, c)},
		Defender: BattleDefender{
			BattleSide: BattleSide{Units: defenderUnits, Research: UnitCounts(target.Research)},
			Wall:       target.Buildings[BWall], Buildings: target.Buildings, Resources: target.Resources,
			HideLevel: target.Buildings[BHide],
		},
		Luck: luck, Morale: CombatBalance.MoraleVsBarbarian, CatapultTarget: c.CatapultTarget, Rng: rng,
		LossMode: w.Config().BattleLosses,
	})

	attackerLost, defenderLost := result.AttackerLosses.Total(), result.DefenderLosses.Total()
	targetOwner := target.OwnerID
	A, D := humanID(w, PlayerID(attackerOwner)), humanOwner(w, target)
	if A != "" {
		s := &w.Human(A).Stats
		s.UnitsLost += attackerLost
		s.UnitsKilled += defenderLost
	}
	if D != "" {
		s := &w.Human(D).Stats
		s.UnitsLost += defenderLost
		s.UnitsKilled += attackerLost
	}
	var botAttacker PlayerID
	if attackerOwner != "" && A == "" {
		botAttacker = PlayerID(attackerOwner)
	}
	if A != "" && targetOwner != "" && w.Brain(PlayerID(targetOwner)) != nil {
		OnHumanAggression(w, PlayerID(targetOwner), A)
	}
	if targetOwner != "" && attackerOwner != "" && attackerOwner != targetOwner && result.Kind != "scout" {
		now := w.Now()
		RecordHabit(w, PlayerID(targetOwner), PlayerID(attackerOwner), HabitObservation{FarmedAt: &now})
	}
	recipients := reportRecipients(w, A, D, target)
	// Read the defending side now, while everybody's troops are still standing in the village: a supporter wiped
	// out to the last man loses his slot with the losses, and he must still be recognized as one of the defenders.
	defenders := defendingHumans(w, target)

	if result.Kind == "scout" {
		scoutOK := result.Outcome == "attackerWon"
		if botAttacker != "" {
			seen := BattleSeen{Losses: attackerLost > 0, AttackerLost: &attackerLost}
			if scoutOK {
				u, b := defenderUnits, PartialBuildings{}
				for _, id := range BuildingIDs {
					b[id] = target.Buildings[id]
				}
				seen.Units, seen.Buildings = &u, b
				seen.Wall = ptrTo(target.Buildings[BWall])
				seen.Hide = ptrTo(HideCapacity(target.Buildings[BHide]))
				seen.Storage = ptrTo(VillageStorageCapacity(target))
			}
			ObserveBattle(w, botAttacker, target, seen)
		}
		if result.AttackerSurvivors.Total() > 0 && origin != nil && !loose {
			markFarm(w, CreateReturn(w, target.ID, c.OriginID, result.AttackerSurvivors, nil), c.Farm)
		}
		RecordBattle(w, BattleRecord{AttackerID: attackerOwner, DefenderID: targetOwner, Shares: shares,
			AttackerLost: result.AttackerLosses, DefenderLost: result.DefenderLosses})
		countBattle(w, c.Units.Total()+defenderUnits.Total(), "")
		if len(recipients) == 0 {
			return nil
		}
		if A != "" {
			in := intelCopy(w.Intel(A, target.ID))
			prev := in.Intel
			vi := &VillageIntel{At: w.Now(), Wall: 0, Loyalty: 100}
			if prev != nil {
				*vi = *prev
				vi.At = w.Now()
			}
			if scoutOK {
				vi.Units, vi.Wall, vi.Loyalty = defenderUnits, target.Buildings[BWall], target.Loyalty
				vi.Scouted = &Scouted{At: w.Now(), Buildings: target.Buildings, Resources: Resources{math.Floor(target.Resources.Wood),
					math.Floor(target.Resources.Clay), math.Floor(target.Resources.Iron)}}
				in.FarmTargetFlags = &FarmTargetFlags{HasWall: target.Buildings[BWall] > 0, ScoutedAt: ptrTo(w.Now())}
			}
			in.Intel = vi
			w.PutIntel(A, target.ID, in)
		}
		r := Report{
			Time: w.Now(), Type: "scout", OriginID: c.OriginID, TargetID: target.ID,
			Origin: armyVillage(w, c, c.OriginID), Target: ReportVillageOf(w, target.ID),
			Attacker: ReportSide{Units: c.Units, Losses: result.AttackerLosses},
			Defender: ReportSide{Units: defenderUnits, Losses: result.DefenderLosses},
			Morale:   result.Morale, WallBefore: target.Buildings[BWall], WallAfter: target.Buildings[BWall],
			Outcome: result.Outcome, AutoFarm: c.AutoFarm,
		}
		if scoutOK {
			r.Scout = &ScoutIntel{
				Resources: Resources{math.Floor(target.Resources.Wood), math.Floor(target.Resources.Clay), math.Floor(target.Resources.Iron)},
				Buildings: target.Buildings, Units: defenderUnits, Loyalty: target.Loyalty,
			}
		}
		sendBattleReports(w, recipients, r, defenders)
		return nil
	}

	applyDefenderLosses(target, result.DefenderLosses)
	target.Buildings[BWall] = result.WallAfter
	if result.BuildingDamage != nil {
		target.Buildings[result.BuildingDamage.Building] = result.BuildingDamage.To
	}
	if result.WallAfter != result.WallBefore || result.BuildingDamage != nil {
		RecomputePoints(target)
	}

	survivors := result.AttackerSurvivors
	haul := result.Haul
	conquered := false
	loyaltyAfter := target.Loyalty
	if result.Outcome == "attackerWon" && A != "" {
		w.Human(A).Stats.BattlesWon++
	}
	if result.Outcome == "attackerWon" {
		if nobles := survivors[UNoble]; nobles > 0 && attackerOwner != "" && attackerOwner != targetOwner {
			loyaltyAfter, conquered = ApplyNobles(w, target, c, nobles)
			if conquered {
				survivors[UNoble] = nobles - 1
				haul = Resources{}
			}
		}
		if !conquered && !loose {
			for _, r := range ResourceIDs {
				target.Resources.Set(r, math.Max(0, target.Resources.Get(r)-haul.Get(r)))
			}
			if A != "" {
				s := &w.Human(A).Stats
				s.ResourcesLooted.Wood += haul.Wood
				s.ResourcesLooted.Clay += haul.Clay
				s.ResourcesLooted.Iron += haul.Iron
				day := LocalDayIndex(w, w.Now())
				if s.LootedToday.Day != day {
					s.LootedToday = DayTotal{Day: day}
				}
				s.LootedToday.Total += SumResources(haul)
			}
		}
	}

	looted := 0.0
	if result.Outcome == "attackerWon" && !conquered && !loose {
		looted = SumResources(haul)
	}
	RecordBattle(w, BattleRecord{AttackerID: attackerOwner, DefenderID: targetOwner, Shares: shares,
		AttackerLost: result.AttackerLosses, DefenderLost: result.DefenderLosses, Loot: looted})
	countBattle(w, c.Units.Total()+defenderUnits.Total(), "")
	kind := "battle"
	if conquered {
		kind = "conquest"
	}
	recordPublicEvent(w, kind, attackerOwner, targetOwner, target.X, target.Y)
	battle := TimelineEntry{Kind: "battle", X: float64(target.X), Y: float64(target.Y), Attacker: attackerOwner, Defender: targetOwner,
		Troops: c.Units.Total() + defenderUnits.Total()}
	if origin != nil {
		battle.FromX, battle.FromY = origin.X, origin.Y
	}
	recordTimeline(w, battle)
	// A force that lost every man has nobody to ride home and count the enemy, so this is the one thing an attack
	// cannot teach: what stood against it. Nothing of the garrison goes into the attacker's intel or a bot's memory
	// — otherwise a single spear would still buy the whole army.
	attackerWiped := result.AttackerSurvivors.Total() == 0
	if botAttacker != "" {
		won := result.Outcome == "attackerWon"
		seen := BattleSeen{Wall: ptrTo(result.WallAfter), Losses: attackerLost > 0, AttackerLost: &attackerLost, Won: &won}
		if !attackerWiped {
			u := defenderUnits
			seen.Units = &u
		}
		if won && !conquered && !loose && result.HaulCapacity > 0 {
			seen.HaulRatio = ptrTo(SumResources(haul) / result.HaulCapacity)
		}
		ObserveBattle(w, botAttacker, target, seen)
	}

	if A != "" {
		in := intelCopy(w.Intel(A, target.ID))
		var scouted *Scouted
		var lastHaul *Resources
		var lastHaulCapacity *float64
		known := UnitCounts{}
		if in.Intel != nil {
			scouted, lastHaul, lastHaulCapacity = in.Intel.Scouted, in.Intel.LastHaul, in.Intel.LastHaulCapacity
			// Without a survivor the garrison stays whatever the last witness reported, not what died today.
			known = in.Intel.Units
		}
		if !attackerWiped {
			known = defenderUnits
		}
		// An army cut loose from its village took nothing, so the last haul stays the one before it.
		if !loose {
			h := haul
			lastHaul, lastHaulCapacity = &h, ptrTo(result.HaulCapacity)
		}
		in.Intel = &VillageIntel{At: w.Now(), Units: known, Wall: result.WallAfter, Loyalty: loyaltyAfter,
			LastHaul: lastHaul, LastHaulCapacity: lastHaulCapacity, LastLosses: ptrTo(attackerLost), LastOutcome: result.Outcome,
			Scouted: scouted}
		var scoutedAt *int64
		if in.FarmTargetFlags != nil {
			scoutedAt = in.FarmTargetFlags.ScoutedAt
		}
		in.FarmTargetFlags = &FarmTargetFlags{HasWall: result.WallAfter > 0, ScoutedAt: scoutedAt}
		if result.Outcome == "attackerWon" && !conquered && !loose {
			in.LastFarmedAt = ptrTo(w.Now())
		}
		w.PutIntel(A, target.ID, in)
	}
	if len(recipients) > 0 {
		r := Report{
			Time: w.Now(), Type: "attack", OriginID: c.OriginID, TargetID: target.ID,
			Origin: armyVillage(w, c, c.OriginID), Target: ReportVillageOf(w, target.ID),
			Attacker: ReportSide{Units: c.Units, Losses: result.AttackerLosses},
			Defender: ReportSide{Units: defenderUnits, Losses: result.DefenderLosses},
			Luck:     result.Luck, Morale: result.Morale, WallBefore: result.WallBefore, WallAfter: result.WallAfter,
			BuildingDamage: result.BuildingDamage, Conquered: conquered, Outcome: result.Outcome, AutoFarm: c.AutoFarm,
		}
		if conquered {
			r.Type = "conquest"
		}
		if result.Outcome == "attackerWon" && !conquered {
			if loose {
				r.NoHome = true
			} else {
				h := haul
				r.Haul, r.HaulCapacity = &h, ptrTo(result.HaulCapacity)
			}
		}
		if c.Units[UNoble] > 0 {
			r.LoyaltyBefore, r.LoyaltyAfter = ptrTo(loyaltyBefore), ptrTo(loyaltyAfter)
		}
		sendBattleReports(w, recipients, r, defenders)
	}

	if D != "" && D != A && result.Outcome == "defenderWon" {
		w.Human(D).Stats.AttacksRepelled++
	}
	if conquered {
		TransferVillage(w, target, attackerOwner, c.OriginID)
	}
	if survivors.Total() > 0 && origin != nil && !loose {
		var h *Resources
		if SumResources(haul) > 0 {
			h = &haul
		}
		markFarm(w, CreateReturn(w, target.ID, c.OriginID, survivors, h), c.Farm)
	}
	return nil
}

// markFarm carries a farm raid's mark onto its way home.
func markFarm(w World, ret *Command, farm bool) {
	if farm {
		ret.Farm = true
		w.PutCommand(ret)
	}
}

func intelCopy(in *Intel) *Intel {
	if in == nil {
		return &Intel{}
	}
	cp := *in
	return &cp
}
