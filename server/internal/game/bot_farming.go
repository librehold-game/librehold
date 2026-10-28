package game

import (
	"math"
	"sort"
)

// Bot farming: raids on barbarian and inactive villages.

var raidSpy func(botID PlayerID, c *Command, night bool, nextSessionAt int64)

// SpyRaids observes every bot raid (night = sent before leaving the game). Tests only; returns a restore func.
func SpyRaids(fn func(botID PlayerID, c *Command, night bool, nextSessionAt int64)) (restore func()) {
	prev := raidSpy
	raidSpy = fn
	return func() { raidSpy = prev }
}

func farmRadius(skill float64) int {
	return int(math.Round(BotConfig.FarmRadius.Weak + (BotConfig.FarmRadius.Strong-BotConfig.FarmRadius.Weak)*skill))
}

func isAvailable(w World, k *KnownVillage) bool {
	if k.LastAttackAt != nil && w.Now()-*k.LastAttackAt < BotDur(w, int64(BotConfig.FarmMinIntervalMin*float64(MsPerMinute))) {
		return false
	}
	if k.DeadSince != nil && w.Now()-*k.DeadSince < BotDur(w, int64(BotConfig.DeadFarmDays*float64(MsPerDay))) {
		return false
	}
	return true
}

// FarmCandidates are barbarians from the map, abandoned players, and active players half as strong if they may
// be attacked. radius 0 = the bot's usual farm radius.
func FarmCandidates(w World, botID PlayerID, from *Village, radius int) []*KnownVillage {
	brain := w.Brain(botID)
	if brain == nil {
		return nil
	}
	r := radius
	if r == 0 {
		r = farmRadius(brain.Skill)
	}
	stored := map[string]VillageRef{}
	for _, ref := range w.VillagesIn(from.X-r, from.Y-r, from.X+r, from.Y+r) {
		stored[ref.ID] = ref
	}
	var out []*KnownVillage
	for _, t := range GeneratedVillagesNear(w.Seed(), w.Config(), from.X, from.Y, r) {
		s, isStored := stored[t.ID]
		if isStored && s.OwnerID != "" {
			continue
		}
		k := brain.Memory.Known[t.ID]
		if k == nil {
			points := t.Points
			if isStored {
				points = s.Points
			}
			k = &KnownVillage{ID: t.ID, X: t.X, Y: t.Y, Points: points, Hauls: []float64{}}
		}
		if isAvailable(w, k) {
			out = append(out, k)
		}
	}
	own := float64(PlayerPoints(w, botID))
	for _, k := range sortedKnown(brain) {
		if k.OwnerID == "" || k.OwnerID == Ref(botID) {
			continue
		}
		owner := w.Player(PlayerID(k.OwnerID))
		// Abandoned players are farms; active ones only when much weaker and when allowed (world setting).
		inactive := owner != nil && owner.Status == StatusQuit
		if !inactive && (float64(k.Points) >= 0.5*own || !MayTarget(w, botID, k.OwnerID)) {
			continue
		}
		if Distance(from.X, from.Y, k.X, k.Y) > float64(r) || !isAvailable(w, k) {
			continue
		}
		out = append(out, k)
	}
	return out
}

// winsSafely: the group wins at the worst luck (−25 %) against what the bot knows of the target, without losses.
func winsSafely(from *Village, k *KnownVillage, units UnitCounts) bool {
	var def UnitCounts
	if k.Units != nil {
		def = *k.Units
	}
	wall := 0
	if k.Wall != nil {
		wall = *k.Wall
	}
	var b Buildings
	b[BWall] = wall
	r := ResolveBattle(BattleInput{
		Attacker: BattleSide{Units: units, Research: UnitCounts(from.Research)},
		Defender: BattleDefender{BattleSide: BattleSide{Units: def}, Wall: wall, Buildings: b},
		Luck:     -CombatBalance.LuckRange, Morale: CombatBalance.MoraleVsBarbarian, Rng: func() float64 { return 0.5 },
		LossMode: "normal",
	})
	return r.Outcome == "attackerWon" && r.AttackerSurvivors.Total() > 0 && r.AttackerLosses.Total() == 0
}

// SizeRaid is the smallest group that wins without losses: light cavalry (spears and swords before a stable).
// Unknown wall → a 4-LC scouting group (+ scout); wall 1–2 without defenders → 2 rams per level + 20 axes
// (strong bots only); otherwise none.
func SizeRaid(w World, from *Village, k *KnownVillage, skill float64) (UnitCounts, bool) {
	home := from.Units
	cavalry := home[ULightCav] > 0
	if k.Wall == nil {
		if cavalry && home[ULightCav] >= 4 {
			if home[UScout] > 0 {
				return UnitCounts{ULightCav: 4, UScout: 1}, true
			}
			return UnitCounts{ULightCav: 4}, true
		}
		if home[USpear] >= 15 {
			return UnitCounts{USpear: 15}, true
		}
		return UnitCounts{}, false
	}
	if *k.Wall > 0 {
		noDefenders := k.Units == nil || k.Units.Total() == 0
		if *k.Wall <= 2 && noDefenders && skill >= 0.7 && home[URam] >= 2**k.Wall && home[UAxe] >= 20 {
			return UnitCounts{URam: 2 * *k.Wall, UAxe: 20}, true
		}
		return UnitCounts{}, false
	}
	maxN := home[USpear] + home[USword]
	if cavalry {
		maxN = home[ULightCav]
	}
	if maxN == 0 {
		return UnitCounts{}, false
	}
	group := func(n int) UnitCounts {
		if cavalry {
			return UnitCounts{ULightCav: n}
		}
		spear := min(home[USpear], n)
		if spear < n {
			return UnitCounts{USpear: spear, USword: min(home[USword], n-spear)}
		}
		return UnitCounts{USpear: spear}
	}
	minimum := 10
	if cavalry {
		minimum = min(maxN, cavalryForLoot(w, k))
	}
	if maxN < minimum || !winsSafely(from, k, group(maxN)) {
		return UnitCounts{}, false
	}
	lo, hi := minimum, maxN
	if winsSafely(from, k, group(lo)) {
		return group(lo), true
	}
	for hi-lo > 1 {
		mid := (lo + hi) >> 1
		if winsSafely(from, k, group(mid)) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return group(hi), true
}

func expectedHaulRatio(k *KnownVillage) float64 {
	if len(k.Hauls) == 0 {
		return 1
	}
	s := 0.0
	for _, h := range k.Hauls {
		s += h
	}
	return s / float64(len(k.Hauls))
}

// cavalryForLoot: light cavalry for the expected loot — the farm rests BotConfig.FarmRefillHours, and full past
// hauls make the group bigger.
func cavalryForLoot(w World, k *KnownVillage) int {
	restedH := BotConfig.FarmRefillHours
	if k.LastAttackAt != nil {
		restedH = float64(w.Now()-*k.LastAttackAt) / float64(MsPerHour)
	}
	loot := BotConfig.FarmLoot * math.Min(1, restedH/BotConfig.FarmRefillHours) * (0.5 + expectedHaulRatio(k))
	return max(4, int(math.Ceil(loot/UnitDefs[ULightCav].Haul)))
}

func sendRaid(w World, botID PlayerID, from *Village, k *KnownVillage, units UnitCounts, night bool, nextSessionAt int64) bool {
	if len(CheckSend(w, botID, from.ID, k.X, k.Y, units, "attack").Blockers) > 0 {
		return false
	}
	var c *Command
	if !act(w, botID, "send", sendArgs(from.ID, k.X, k.Y, units, "attack"), func() error {
		var err error
		c, err = SendCommand(w, botID, from.ID, k.X, k.Y, units, "attack", SendOpts{})
		return err
	}) {
		return false
	}
	brain := brainOf(w, botID)
	entry := brain.Memory.Known[k.ID]
	if entry == nil {
		cp := *k
		cp.Hauls = append([]float64{}, k.Hauls...)
		entry = &cp
		brain.Memory.Known[k.ID] = entry
	}
	entry.LastAttackAt = ptrTo(w.Now())
	if raidSpy != nil {
		raidSpy(botID, c, night, nextSessionAt)
	}
	return true
}

// PlanFarming, in a session: from each village, groups sized by simulation at targets by expected haul ÷
// distance, until the troops run out.
func PlanFarming(w World, botID PlayerID) {
	brain := w.Brain(botID)
	if brain == nil {
		return
	}
	for _, v := range PlayerVillages(w, botID) {
		type scored struct {
			k     *KnownVillage
			score float64
		}
		var targets []scored
		for _, k := range FarmCandidates(w, botID, v, 0) {
			targets = append(targets, scored{k, expectedHaulRatio(k) / math.Max(1, Distance(v.X, v.Y, k.X, k.Y))})
		}
		sort.SliceStable(targets, func(i, j int) bool { return targets[i].score > targets[j].score })
		misses, sent := 0, 0
		for _, t := range targets {
			if UnitCounts(v.Units).Total() == 0 || misses > 20 || sent >= BotConfig.MaxRaidsPerThink {
				break
			}
			units, ok := SizeRaid(w, v, t.k, brain.Skill)
			if !ok {
				misses++
				continue
			}
			if sendRaid(w, botID, v, t.k, units, false, 0) {
				sent++
			} else {
				misses++
			}
		}
	}
}

const nightReach = 60

// PlanNightRuns, before sleep: the remaining light cavalry on raids that come back ±60 min from the next session
// (a strong bot picks the distance, a weak one sends anywhere in its usual range). At night it reaches further.
func PlanNightRuns(w World, botID PlayerID, nextSessionAt int64) {
	brain := w.Brain(botID)
	if brain == nil {
		return
	}
	rng := Mulberry32(HiddenSeed(w, "night", botID, w.Now()))
	strong := brain.Skill >= 0.7 || rng() < brain.Skill
	for _, v := range PlayerVillages(w, botID) {
		if v.Units[ULightCav] < 4 {
			continue
		}
		reach := 0
		if strong {
			reach = nightReach
		}
		var pool []*KnownVillage
		for _, k := range FarmCandidates(w, botID, v, reach) {
			if (k.Wall == nil || *k.Wall == 0) && (k.Units == nil || k.Units.Total() == 0) {
				pool = append(pool, k)
			}
		}
		if strong {
			kept := pool[:0]
			for _, k := range pool {
				back := 2 * TravelTimeForUnits(w, v, k.X, k.Y, UnitCounts{ULightCav: 1})
				if abs64(w.Now()+back-nextSessionAt) <= 60*MsPerMinute {
					kept = append(kept, k)
				}
			}
			pool = kept
		} else {
			for i := len(pool) - 1; i > 0; i-- {
				j := int(math.Floor(rng() * float64(i+1)))
				pool[i], pool[j] = pool[j], pool[i]
			}
		}
		sent := 0
		for _, k := range pool {
			if v.Units[ULightCav] < 4 || sent >= BotConfig.MaxNightRuns {
				break
			}
			// A night is many hours of rest for the farm — a group as for a full farm.
			n := min(v.Units[ULightCav], max(4, int(math.Ceil(BotConfig.FarmLoot*(0.5+expectedHaulRatio(k))/UnitDefs[ULightCav].Haul))))
			if sendRaid(w, botID, v, k, UnitCounts{ULightCav: n}, true, nextSessionAt) {
				sent++
			}
		}
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
