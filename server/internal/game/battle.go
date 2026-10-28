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

// humanOwner returns the owner as a human id, or "" for bots and barbarians.
func humanOwner(w World, v *Village) PlayerID {
	if v == nil || v.OwnerID == "" || w.Human(PlayerID(v.OwnerID)) == nil {
		return ""
	}
	return PlayerID(v.OwnerID)
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

func sendReports(w World, to []PlayerID, r Report) {
	for _, id := range to {
		copy := r
		w.AddReport(id, &copy)
	}
}

// ResolveAttackArrival: the full battle, reports, loot, return, loyalty and conquest.
func ResolveAttackArrival(w World, c *Command, target *Village) error {
	origin := w.Village(c.OriginID)
	// The attack reached a village that became the attacker's own meanwhile (e.g. after the first noble of a
	// train): the troops stay there as support instead of fighting their own defenders.
	if origin != nil && origin.OwnerID != "" && origin.OwnerID == target.OwnerID {
		slot := target.SupportFrom[origin.ID]
		for _, u := range UnitIDs {
			slot[u] += c.Units[u]
		}
		target.SupportFrom[origin.ID] = slot
		return nil
	}
	Touch(w, target)

	rng := Mulberry32(HiddenSeed(w, c.ID))
	luck := -CombatBalance.LuckRange + rng()*2*CombatBalance.LuckRange
	defenderUnits := DefenderUnitsOf(target)
	shares := DefenderShares(w, target)
	loyaltyBefore := target.Loyalty
	var attackerResearch UnitCounts
	if origin != nil {
		attackerResearch = UnitCounts(origin.Research)
	}
	result := ResolveBattle(BattleInput{
		Attacker: BattleSide{Units: c.Units, Research: attackerResearch},
		Defender: BattleDefender{
			BattleSide: BattleSide{Units: defenderUnits, Research: UnitCounts(target.Research)},
			Wall:       target.Buildings[BWall], Buildings: target.Buildings, Resources: target.Resources,
			HideLevel: target.Buildings[BHide],
		},
		Luck: luck, Morale: CombatBalance.MoraleVsBarbarian, CatapultTarget: c.CatapultTarget, Rng: rng,
		LossMode: w.Config().BattleLosses,
	})

	attackerLost, defenderLost := result.AttackerLosses.Total(), result.DefenderLosses.Total()
	var attackerOwner PlayerRef
	if origin != nil {
		attackerOwner = origin.OwnerID
	}
	targetOwner := target.OwnerID
	A, D := humanOwner(w, origin), humanOwner(w, target)
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
		if result.AttackerSurvivors.Total() > 0 && origin != nil {
			CreateReturn(w, target.ID, c.OriginID, result.AttackerSurvivors, nil)
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
				in.FarmTargetFlags = &FarmTargetFlags{HasWall: target.Buildings[BWall] > 0, ScoutedAt: ptrTo(w.Now())}
			}
			in.Intel = vi
			w.PutIntel(A, target.ID, in)
		}
		r := Report{
			Time: w.Now(), Type: "scout", OriginID: c.OriginID, TargetID: target.ID,
			Origin: ReportVillageOf(w, c.OriginID), Target: ReportVillageOf(w, target.ID),
			Attacker: ReportSide{Units: c.Units, Losses: result.AttackerLosses},
			Morale:   result.Morale, WallBefore: target.Buildings[BWall], WallAfter: target.Buildings[BWall],
			Outcome: result.Outcome,
		}
		if scoutOK {
			r.Defender.Units = defenderUnits
			r.Scout = &ScoutIntel{
				Resources: Resources{math.Floor(target.Resources.Wood), math.Floor(target.Resources.Clay), math.Floor(target.Resources.Iron)},
				Buildings: target.Buildings, Units: defenderUnits, Loyalty: target.Loyalty,
			}
		}
		sendReports(w, recipients, r)
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
		if !conquered {
			for _, r := range ResourceIDs {
				target.Resources.Set(r, math.Max(0, target.Resources.Get(r)-haul.Get(r)))
			}
			if A != "" {
				s := &w.Human(A).Stats
				s.ResourcesLooted.Wood += haul.Wood
				s.ResourcesLooted.Clay += haul.Clay
				s.ResourcesLooted.Iron += haul.Iron
				day := w.Now() / MsPerDay
				if s.LootedToday.Day != day {
					s.LootedToday = DayTotal{Day: day}
				}
				s.LootedToday.Total += SumResources(haul)
			}
		}
	}

	looted := 0.0
	if result.Outcome == "attackerWon" && !conquered {
		looted = SumResources(haul)
	}
	RecordBattle(w, BattleRecord{AttackerID: attackerOwner, DefenderID: targetOwner, Shares: shares,
		AttackerLost: result.AttackerLosses, DefenderLost: result.DefenderLosses, Loot: looted})
	countBattle(w, c.Units.Total()+defenderUnits.Total(), "")
	kind := "battle"
	if conquered {
		kind = "conquest"
	}
	recordPublicEvent(w, kind, attackerOwner, targetOwner, c.Units.Total()+defenderUnits.Total(), target.X, target.Y)
	battle := TimelineEntry{Kind: "battle", X: float64(target.X), Y: float64(target.Y), Attacker: attackerOwner, Defender: targetOwner,
		Troops: c.Units.Total() + defenderUnits.Total()}
	if origin != nil {
		battle.FromX, battle.FromY = origin.X, origin.Y
	}
	recordTimeline(w, battle)
	if botAttacker != "" {
		u := defenderUnits
		won := result.Outcome == "attackerWon"
		seen := BattleSeen{Units: &u, Wall: ptrTo(result.WallAfter), Losses: attackerLost > 0, AttackerLost: &attackerLost, Won: &won}
		if won && !conquered && result.HaulCapacity > 0 {
			seen.HaulRatio = ptrTo(SumResources(haul) / result.HaulCapacity)
		}
		ObserveBattle(w, botAttacker, target, seen)
	}

	if A != "" {
		in := intelCopy(w.Intel(A, target.ID))
		h := haul
		in.Intel = &VillageIntel{At: w.Now(), Units: defenderUnits, Wall: result.WallAfter, Loyalty: loyaltyAfter,
			LastHaul: &h, LastHaulCapacity: ptrTo(result.HaulCapacity), LastLosses: ptrTo(attackerLost), LastOutcome: result.Outcome}
		var scoutedAt *int64
		if in.FarmTargetFlags != nil {
			scoutedAt = in.FarmTargetFlags.ScoutedAt
		}
		in.FarmTargetFlags = &FarmTargetFlags{HasWall: result.WallAfter > 0, ScoutedAt: scoutedAt}
		if result.Outcome == "attackerWon" && !conquered {
			in.LastFarmedAt = ptrTo(w.Now())
		}
		w.PutIntel(A, target.ID, in)
	}
	if len(recipients) > 0 {
		r := Report{
			Time: w.Now(), Type: "attack", OriginID: c.OriginID, TargetID: target.ID,
			Origin: ReportVillageOf(w, c.OriginID), Target: ReportVillageOf(w, target.ID),
			Attacker: ReportSide{Units: c.Units, Losses: result.AttackerLosses},
			Defender: ReportSide{Units: defenderUnits, Losses: result.DefenderLosses},
			Luck:     result.Luck, Morale: result.Morale, WallBefore: result.WallBefore, WallAfter: result.WallAfter,
			BuildingDamage: result.BuildingDamage, Conquered: conquered, Outcome: result.Outcome,
		}
		if conquered {
			r.Type = "conquest"
		}
		if result.Outcome == "attackerWon" && !conquered {
			h := haul
			r.Haul, r.HaulCapacity = &h, ptrTo(result.HaulCapacity)
		}
		if c.Units[UNoble] > 0 {
			r.LoyaltyBefore, r.LoyaltyAfter = ptrTo(loyaltyBefore), ptrTo(loyaltyAfter)
		}
		sendReports(w, recipients, r)
	}

	if D != "" && D != A && result.Outcome == "defenderWon" {
		w.Human(D).Stats.AttacksRepelled++
	}
	if conquered && origin != nil {
		TransferVillage(w, target, origin.OwnerID, origin.ID)
	}
	if survivors.Total() > 0 && origin != nil {
		var h *Resources
		if SumResources(haul) > 0 {
			h = &haul
		}
		CreateReturn(w, target.ID, c.OriginID, survivors, h)
	}
	return nil
}

func intelCopy(in *Intel) *Intel {
	if in == nil {
		return &Intel{}
	}
	cp := *in
	return &cp
}
