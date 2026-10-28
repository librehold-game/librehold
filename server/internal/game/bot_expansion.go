package game

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Bot expansion: nobles, coins and the villages bots set out to conquer.

// academyPath: the academy's requirements, then the academy up to level 3 (3 nobles at once).
var academyPath = []BuildingRequirement{{BMain, 20}, {BSmithy, 20}, {BMarket, 10}, {BAcademy, 1}, {BAcademy, 2}, {BAcademy, 3}}

// launchConquest sends a hetman train at a known village (set by the bot attacks code).
var launchConquest func(w World, botID PlayerID, k *KnownVillage)

func unitsAttack(u UnitCounts) float64 {
	a := 0.0
	for _, id := range UnitIDs {
		a += float64(u[id]) * UnitDefs[id].Attack
	}
	return a
}

// readyForNobles: a strong bot starts on nobles when the village's army is ≥ 50 % of its role target; a weak one
// at random, some day.
func readyForNobles(w World, botID PlayerID, v *Village) bool {
	brain := brainOf(w, botID)
	if brain.Skill < 0.7 {
		rng := Mulberry32(HiddenSeed(w, "expandDay", botID, LocalDayIndex(w, w.Now())))
		return rng() < BotConfig.WeakExpandDailyChance
	}
	var have, target UnitCounts
	for _, u := range UnitIDs {
		have[u] = UnitCountIncludingQueued(v, u)
		target[u] = TroopTargets[ChooseRole(w, botID, v)][u]
	}
	return unitsAttack(have) >= 0.5*unitsAttack(target)
}

// academySteps are the path steps that can be built now (the storage first when it cannot hold the cost). A
// blocked step does not hold the next ones back.
func academySteps(v *Village) []BuildingID {
	levels := EffectiveBuildingLevels(v)
	var out []BuildingID
	for _, s := range academyPath {
		b := s.Building
		if levels[b] >= s.Level || levels[b] >= BuildingDefs[b].MaxLevel || !MeetsRequirements(levels, b) {
			continue
		}
		cost := BuildCost(b, levels[b]+1)
		capacity := VillageStorageCapacity(v)
		step := b
		if (cost.Wood > capacity || cost.Clay > capacity || cost.Iron > capacity) && levels[BStorage] < BuildingDefs[BStorage].MaxLevel {
			step = BStorage
		}
		if !slices.Contains(out, step) {
			out = append(out, step)
		}
	}
	return out
}

// nobleReach: how far nobles may go so waves beat loyalty regeneration — a wave of n nobles drops 27.5·n on
// average, and loyalty comes back while they return and march again (faster on faster worlds).
func nobleReach(w World, nobles int) float64 {
	cfg := w.Config()
	regenPerField := (ConquestBalance.LoyaltyPerHour * cfg.WorldSpeed * 2 * UnitDefs[UNoble].Speed) / (60 * cfg.UnitSpeed)
	return math.Min(12, (0.7*float64(nobles)*ConquestBalance.NobleDropAvg)/regenPerField)
}

// unowned: no playing owner — barbarian, or grey (a player who left).
func unowned(w World, v *Village) bool {
	if v.OwnerID == "" {
		return true
	}
	p := w.Player(PlayerID(v.OwnerID))
	return p != nil && p.Status == StatusQuit
}

// nobleClaimer says whether another player's hetman is already on its way to (x, y), by command or by a send
// planned. The planned sends are read once, on first use.
func nobleClaimer(w World, botID PlayerID) func(x, y int) bool {
	var planned map[[2]int]bool
	return func(x, y int) bool {
		for _, c := range w.CommandsTo(fmt.Sprintf("v:%d:%d", x, y)) {
			if c.Units[UNoble] > 0 {
				if o := w.Village(c.OriginID); o != nil && o.OwnerID != Ref(botID) {
					return true
				}
			}
		}
		if planned == nil {
			planned = map[[2]int]bool{}
			for _, p := range w.Players() {
				if p.ID == botID {
					continue
				}
				for _, s := range w.ScheduledSendsOf(p.ID) {
					if s.Units[UNoble] > 0 {
						planned[[2]int{s.X, s.Y}] = true
					}
				}
			}
		}
		return planned[[2]int{x, y}]
	}
}

// pickConquestTarget: the village without a playing owner with the most points in noble reach (≥ 300 preferred).
func pickConquestTarget(w World, botID PlayerID, v *Village, nobles int) string {
	reach := nobleReach(w, nobles)
	if reach < 1 {
		return ""
	}
	type option struct {
		id     string
		points int
		d      float64
	}
	var options []option
	for _, t := range GeneratedVillagesNear(w.Seed(), w.Config(), v.X, v.Y, int(math.Ceil(reach))) {
		if t.D > reach {
			continue
		}
		points := t.Points
		if stored := w.Village(t.ID); stored != nil {
			if !unowned(w, stored) {
				continue
			}
			points = stored.Points
		}
		options = append(options, option{t.ID, points, t.D})
	}
	for _, p := range w.Players() {
		if p.Status != StatusQuit {
			continue
		}
		for _, id := range p.VillageIDs {
			q := w.Village(id)
			if q == nil {
				continue
			}
			d := Distance(v.X, v.Y, q.X, q.Y)
			if d <= reach && !slices.ContainsFunc(options, func(o option) bool { return o.id == id }) {
				options = append(options, option{id, q.Points, d})
			}
		}
	}
	var big []option
	for _, o := range options {
		if o.points >= 300 {
			big = append(big, o)
		}
	}
	pool := options
	if len(big) > 0 {
		pool = big
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].points != pool[j].points {
			return pool[i].points > pool[j].points
		}
		return pool[i].d < pool[j].d
	})
	claimed := nobleClaimer(w, botID)
	for _, o := range pool {
		if x, y, ok := parseVillageID(o.id); ok && !claimed(x, y) {
			return o.id
		}
	}
	return ""
}

// coinsAndNobles mints coins for the next nobles (up to 4 or the academy limit), then recruits a noble.
func coinsAndNobles(w World, botID PlayerID, v *Village) {
	p := w.Player(botID)
	room := NobleLimit(w.Config(), v) - NobleCountOf(w, v)
	if room <= 0 {
		return
	}
	want := CoinsNeededFor(w, botID, min(4, room))
	for guard := 0; !w.Config().NoCoins && p.Coins < want && guard < 4 && len(CheckMint(v).Blockers) == 0; guard++ {
		act(w, botID, "mint", villageArgs(v.ID), func() error { return MintCoin(w, botID, v.ID) })
	}
	if len(v.RecruitQueues.Academy) == 0 && len(CheckRecruit(w, v, UNoble, 1).Blockers) == 0 {
		act(w, botID, "recruit", villageArgs(v.ID, "unit", UNoble, "count", 1), func() error {
			_, err := EnqueueRecruit(w, botID, v.ID, UNoble, 1)
			return err
		})
	}
}

func knownDefenceOf(k *KnownVillage) (float64, bool) {
	if k == nil {
		return 0, false
	}
	return KnownDefence(k)
}

func parseVillageID(id string) (int, int, bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 || parts[0] != "v" {
		return 0, 0, false
	}
	x, ex := strconv.Atoi(parts[1])
	y, ey := strconv.Atoi(parts[2])
	return x, y, ex == nil && ey == nil
}

// sendNobles takes the chosen barbarian or grey village with a hetman train (as in a campaign on a player).
func sendNobles(w World, botID PlayerID, v *Village) {
	brain := brainOf(w, botID)
	if v.Units[UNoble] < 1 || launchConquest == nil {
		return
	}
	if brain.ConquestTarget != "" {
		if cur := w.Village(brain.ConquestTarget); cur != nil && !unowned(w, cur) {
			brain.ConquestTarget = ""
		}
	}
	if brain.ConquestTarget == "" {
		brain.ConquestTarget = pickConquestTarget(w, botID, v, v.Units[UNoble])
	}
	id := brain.ConquestTarget
	if id == "" {
		return
	}
	x, y, ok := parseVillageID(id)
	if !ok {
		brain.ConquestTarget = ""
		return
	}
	if Distance(v.X, v.Y, x, y) > nobleReach(w, v.Units[UNoble])+1 {
		brain.ConquestTarget = ""
		return
	}
	if nobleClaimer(w, botID)(x, y) {
		brain.ConquestTarget = ""
		return
	}
	k := brain.Memory.Known[id]
	stored := w.Village(id)
	if stored != nil && stored.OwnerID != "" {
		// A grey village still has its army: nobles go only where scouting says the road is clear.
		def, known := knownDefenceOf(k)
		if !known {
			campaignScout(w, botID, x, y)
			return
		}
		if def >= 0.8*OwnOffence(w, botID) {
			brain.ConquestTarget = ""
			return
		}
		if noblesReady(w, botID) < NoblesNeededFor(w.Config(), true, ConquestBalance.MaxLoyalty) {
			return
		}
	}
	if k == nil {
		points := 0
		if stored != nil {
			points = stored.Points
		}
		k = &KnownVillage{ID: id, X: x, Y: y, Points: points, SeenAt: w.Now(), Hauls: []float64{}, Units: &UnitCounts{}, Wall: ptrTo(0)}
	}
	launchConquest(w, botID, k)
}

// shortBarbarianVillages: how many villages a bot in a short world gathers from barbarians before it only takes
// villages with an owner.
const shortBarbarianVillages = 3

// takeOwned, in a short world with a full train at home: the nobles go at the best scouted village of an active
// player within reach whose defence the army can clear, or wait while the nearest unscouted (or stale) one is
// scouted. False when there is no such village, or no scouts: then the nobles go for barbarians.
func takeOwned(w World, botID PlayerID) bool {
	off := OwnOffence(w, botID)
	reach := min(ConquestBalance.NobleMaxDistance, campaignRadius)
	var best, unscouted *KnownVillage
	bestScore, unscoutedD := 0.0, math.Inf(1)
	for _, k := range sortedKnown(brainOf(w, botID)) {
		if k.OwnerID == "" || k.OwnerID == Ref(botID) || !MayTarget(w, botID, k.OwnerID) {
			continue
		}
		owner, v := w.Player(PlayerID(k.OwnerID)), w.Village(k.ID)
		if owner == nil || owner.Status != StatusActive || IsProtected(w, owner.ID) || v == nil || !CanBeConquered(w, v) {
			continue
		}
		d := nearestOwn(w, botID, k.X, k.Y)
		if d > reach {
			continue
		}
		def, known := KnownDefence(k)
		if !known || w.Now()-k.SeenAt > BotDur(w, MsPerDay) {
			if d < unscoutedD {
				unscouted, unscoutedD = k, d
			}
			continue
		}
		if def >= 0.8*off {
			continue
		}
		if score := ScoreTarget(w, botID, k); best == nil || score > bestScore {
			best, bestScore = k, score
		}
	}
	if best != nil {
		LaunchConquest(w, botID, best)
		return true
	}
	return unscouted != nil && campaignScout(w, botID, unscouted.X, unscouted.Y)
}

// PlanExpansion: the academy in the best village, coins, nobles, and taking barbarian and grey villages.
func PlanExpansion(w World, botID PlayerID) {
	villages := PlayerVillages(w, botID)
	if len(villages) == 0 {
		return
	}
	// The academy in the best village; from three villages also in the second — a full train is 4 nobles and
	// one academy holds at most 3.
	ranked := slices.Clone(villages)
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Points > ranked[j].Points })
	n := 1
	if len(villages) >= 3 {
		n = 2
	}
	for _, best := range ranked[:n] {
		steps := academySteps(best)
		if len(steps) == 0 || !(best.Buildings[BAcademy] > 0 || readyForNobles(w, botID, best)) {
			continue
		}
		for _, step := range steps {
			if len(best.BuildQueue) >= BuildQueueSlotsFor(w, OwnerOf(w, best)) {
				break
			}
			if len(CheckBuild(w, best, step).Blockers) == 0 {
				act(w, botID, "build", villageArgs(best.ID, "building", step), func() error {
					_, err := EnqueueBuild(w, botID, best.ID, step)
					return err
				})
			}
		}
	}
	// In a campaign on a village that can be taken, the nobles wait for it rather than going for barbarians.
	mood := brainOf(w, botID).Mood
	campaign := false
	if mood.Phase == "campaign" && mood.TargetVillage != "" {
		if t := w.Village(mood.TargetVillage); t != nil && CanBeConquered(w, t) {
			campaign = true
		}
	}
	if !campaign && IsShort(w.Config()) && noblesReady(w, botID) >= NoblesNeededFor(w.Config(), true, ConquestBalance.MaxLoyalty) {
		campaign = takeOwned(w, botID)
	}
	// In a short world a bot past shortBarbarianVillages villages leaves the barbarians to the others.
	campaign = campaign || IsShort(w.Config()) && len(villages) >= shortBarbarianVillages
	for _, v := range villages {
		if v.Buildings[BAcademy] < 1 {
			continue
		}
		coinsAndNobles(w, botID, v)
		if !campaign {
			sendNobles(w, botID, v)
		}
	}
}
