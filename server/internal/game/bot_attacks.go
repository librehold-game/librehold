package game

import (
	"math"
	"sort"
)

// Bot attacks: choosing a target, sizing the army, timing the send.

func init() { launchConquest = LaunchConquest }

type Purpose string

func rngFor(w World, botID PlayerID, what, id string) func() float64 {
	return Mulberry32(HiddenSeed(w, what, botID, id, w.Now()))
}

func villagesByDistance(w World, botID PlayerID, x, y int) []*Village {
	vs := PlayerVillages(w, botID)
	sort.SliceStable(vs, func(i, j int) bool { return Distance(vs[i].X, vs[i].Y, x, y) < Distance(vs[j].X, vs[j].Y, x, y) })
	return vs
}

// noblesReady counts the bot's nobles at home in all villages (a train gathers from several).
func noblesReady(w World, botID PlayerID) int {
	n := 0
	for _, v := range PlayerVillages(w, botID) {
		n += v.Units[UNoble]
	}
	return n
}

// ChoosePurpose: conquest, demolition, farming the player or harassment — drawn with style weights, not always
// the best.
func ChoosePurpose(w World, botID PlayerID, k *KnownVillage) Purpose {
	b := brainOf(w, botID)
	v := w.Village(k.ID)
	def, knownDef := KnownDefence(k)
	clearable := knownDef && def < 0.8*OwnOffence(w, botID)
	payback := 1.0
	if k.OwnerID != "" && PlayerID(k.OwnerID) == HardFarmer(w, botID) {
		payback = 3
	}
	harass := 1.0
	if b.Style == StyleAggressor {
		harass = 2
	}
	options := []Option[Purpose]{{"harass", harass * payback}}
	// A player is taken only with a full train; a ready train on a clear road is the best option.
	if noblesReady(w, botID) >= NoblesNeededFor(w.Config(), true, ConquestBalance.MaxLoyalty) && clearable && v != nil && CanBeConquered(w, v) {
		score := 3.0
		if b.Style == StyleAggressor {
			score = 4
		}
		options = append(options, Option[Purpose]{"conquer", score})
	}
	cats := 0
	for _, x := range PlayerVillages(w, botID) {
		cats = max(cats, x.Units[UCatapult])
	}
	if cats >= 50 && (k.Buildings[BFarm] >= 10 || k.Buildings[BSmithy] >= 10) {
		options = append(options, Option[Purpose]{"demolish", 1})
	}
	if k.OwnerID != "" && float64(PlayerPoints(w, PlayerID(k.OwnerID))) < 0.5*float64(PlayerPoints(w, botID)) {
		score := 1.0
		if b.Style == StyleFarmer {
			score = 2
		}
		options = append(options, Option[Purpose]{"farmPlayer", score * payback})
	}
	return WeightedPick(rngFor(w, botID, "purpose", k.ID), options, b.Skill)
}

// nightLanding is the first instant ≥ earliest in the target's local night (00:00–06:00) or offline window.
func nightLanding(w World, owner PlayerRef, earliest float64, rng func() float64) float64 {
	start, end := 0, 6*60
	if owner != "" {
		if win, ok := LikelyOfflineWindow(w, PlayerID(owner)); ok {
			start, end = win.StartMin, win.EndMin
		}
	}
	l := ((end - start + 1440) % 1440)
	if l == 0 {
		l = 1440
	}
	minNow := LocalMinuteOfDay(w, int64(earliest))
	offset := (((start - minNow) % 1440) + 1440) % 1440
	return earliest + (float64(offset)+rng()*float64(l))*float64(MsPerMinute)
}

// pastTheBell: in a short world, a time after the game's end, when nothing a bot plans matters any more.
func pastTheBell(w World, at int64) bool {
	cfg := w.Config()
	return IsShort(cfg) && cfg.DurationMin > 0 && at > int64(cfg.DurationMin)*MsPerMinute
}

func botSchedule(w World, botID PlayerID, in ScheduleSendInput) {
	if pastTheBell(w, in.At) {
		return
	}
	actSoft(w, botID, "schedule-send", in, func() error { _, err := ScheduleSend(w, botID, in); return err })
}

// LaunchHarass: 4 fakes (1 ram, else 1 axe) from different villages landing within a second; every fourth at the
// target's night.
func LaunchHarass(w World, botID PlayerID, k *KnownVillage) {
	rng := rngFor(w, botID, "harass", k.ID)
	sources := villagesByDistance(w, botID, k.X, k.Y)
	if len(sources) == 0 {
		return
	}
	type pick struct {
		v     *Village
		units UnitCounts
	}
	var picks []pick
	for i := range 4 {
		v := sources[i%len(sources)]
		rams := 0
		for _, p := range picks {
			if p.v == v && p.units[URam] > 0 {
				rams++
			}
		}
		units := UnitCounts{UAxe: 1}
		if v.Units[URam] > rams {
			units = UnitCounts{URam: 1}
		}
		picks = append(picks, pick{v, units})
	}
	latest := int64(0)
	for _, p := range picks {
		latest = max(latest, TravelTimeForUnits(w, p.v, k.X, k.Y, p.units))
	}
	T := float64(w.Now()+latest+BotDur(w, 5*MsPerMinute)) + float64(BotDur(w, int64(rng()*3*float64(MsPerHour))))
	landed := false
	if k.OwnerID != "" {
		if at, ok := LandAfterDodge(w, botID, PlayerID(k.OwnerID), w.Now()+latest+BotDur(w, 5*MsPerMinute), rng); ok {
			T, landed = float64(at), true
		}
	}
	if !landed && rng() < 0.25 && !IsShort(w.Config()) {
		T = nightLanding(w, k.OwnerID, T, rng)
	}
	at := int64(math.Round(math.Min(T, float64(w.Now()+latest+BotDur(w, MsPerDay)))))
	for i, p := range picks {
		botSchedule(w, botID, ScheduleSendInput{OriginID: p.v.ID, X: k.X, Y: k.Y, Units: p.units, Type: "attack",
			Mode: "arriveAt", At: at + int64(i*1000/4)})
	}
}

// LaunchFarmPlayer: an ordinary raid sized by simulation (repeated in later sessions while the hauls are good).
func LaunchFarmPlayer(w World, botID PlayerID, k *KnownVillage) {
	b := brainOf(w, botID)
	for _, v := range villagesByDistance(w, botID, k.X, k.Y) {
		units, ok := SizeRaid(w, v, k, b.Skill)
		if !ok {
			continue
		}
		botSchedule(w, botID, ScheduleSendInput{OriginID: v.ID, X: k.X, Y: k.Y, Units: units, Type: "attack",
			Mode: "sendAt", At: w.Now() + SendJitter(w, botID, k.ID)})
		return
	}
}

// CatapultsFor is the catapults to bring a building to zero: the table's sum over each level.
func CatapultsFor(level int) int {
	n := 0
	for l := 1; l <= level; l++ {
		n += CatapultsPerLevel[min(l, len(CatapultsPerLevel)-1)]
	}
	return n
}

// LaunchDemolish: catapults by the building's level, escorted by axes.
func LaunchDemolish(w World, botID PlayerID, k *KnownVillage, building BuildingID) {
	level, ok := k.Buildings[building]
	if !ok {
		level = 10
	}
	need := CatapultsFor(level)
	var from *Village
	for _, v := range villagesByDistance(w, botID, k.X, k.Y) {
		if v.Units[UCatapult] >= min(need, 50) && v.Units[UAxe] >= 100 {
			from = v
			break
		}
	}
	if from == nil {
		return
	}
	cats := min(from.Units[UCatapult], need)
	units := UnitCounts{UCatapult: cats, UAxe: min(from.Units[UAxe], max(100, cats*20))}
	b := building
	botSchedule(w, botID, ScheduleSendInput{OriginID: from.ID, X: k.X, Y: k.Y, Units: units, Type: "attack",
		CatapultTarget: &b, Mode: "sendAt", At: w.Now() + SendJitter(w, botID, k.ID)})
}

// decoys: 2–5 fakes on the owner's other villages landing ±30 min from the main hit (strong bots split defence).
func decoys(w World, botID PlayerID, owner PlayerID, exceptID string, T int64, rng func() float64) {
	var others []*Village
	for _, v := range PlayerVillages(w, owner) {
		if v.ID != exceptID {
			others = append(others, v)
		}
	}
	n := min(len(others), 2+int(math.Floor(rng()*4)))
	sources := PlayerVillages(w, botID)
	for i := 0; i < n && len(sources) > 0; i++ {
		t := others[i]
		v := sources[i%len(sources)]
		if v.Units[UAxe] < 1 {
			continue
		}
		at := int64(math.Round(float64(T) + (rng()*2-1)*30*float64(MsPerMinute)))
		botSchedule(w, botID, ScheduleSendInput{OriginID: v.ID, X: t.X, Y: t.Y, Units: UnitCounts{UAxe: 1}, Type: "attack", Mode: "arriveAt", At: at})
	}
}

// LaunchConquest: a clearing nuke at T, up to 4 nobles (from several villages) right behind it (strong: every
// 100 ms, weak: randomly up to 5 s), the first escorted by 300 axes, the others by 100. A strong bot aims at the
// owner's offline window when it knows it, and scatters decoys.
func LaunchConquest(w World, botID PlayerID, k *KnownVillage) {
	b := brainOf(w, botID)
	for _, s := range w.ScheduledSendsOf(botID) {
		if s.Units[UNoble] > 0 {
			return
		}
	}
	rng := rngFor(w, botID, "conquest", k.ID)
	strong := b.Skill >= 0.7
	var sources []*Village
	for _, v := range villagesByDistance(w, botID, k.X, k.Y) {
		if v.Units[UNoble] > 0 && Distance(v.X, v.Y, k.X, k.Y) <= ConquestBalance.NobleMaxDistance {
			sources = append(sources, v)
		}
	}
	var nobleFrom []*Village
	for _, v := range sources {
		for i := 0; i < v.Units[UNoble] && len(nobleFrom) < 4; i++ {
			nobleFrom = append(nobleFrom, v)
		}
	}
	if len(nobleFrom) == 0 {
		return
	}
	candidates := append(append([]*Village{}, sources...), villagesByDistance(w, botID, k.X, k.Y)...)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Units[UAxe] > candidates[j].Units[UAxe] })
	nukeVillage := candidates[0]
	escorts := make([]int, len(nobleFrom))
	for i := range escorts {
		escorts[i] = 100
	}
	escorts[0] = 300
	reserved := func(v *Village) int {
		n := 0
		for i, x := range nobleFrom {
			if x == v {
				n += escorts[i]
			}
		}
		return min(n, v.Units[UAxe]/2)
	}
	var nuke UnitCounts
	if left := max(0, nukeVillage.Units[UAxe]-reserved(nukeVillage)); left > 0 {
		nuke[UAxe] = left
	}
	nuke[ULightCav] = nukeVillage.Units[ULightCav]
	nuke[URam] = nukeVillage.Units[URam]
	slowest := TravelTimeForUnits(w, nukeVillage, k.X, k.Y, nuke)
	for _, v := range nobleFrom {
		slowest = max(slowest, TravelTimeForUnits(w, v, k.X, k.Y, UnitCounts{UNoble: 1}))
	}
	T := float64(w.Now() + slowest + BotDur(w, 10*MsPerMinute))
	if strong && k.OwnerID != "" {
		if at, ok := LandAfterDodge(w, botID, PlayerID(k.OwnerID), int64(T), rng); ok {
			T = float64(at)
		} else if _, ok := LikelyOfflineWindow(w, PlayerID(k.OwnerID)); ok && !IsShort(w.Config()) {
			T = nightLanding(w, k.OwnerID, T, rng)
		}
	}
	at := int64(math.Round(math.Min(T, float64(w.Now()+slowest+BotDur(w, MsPerDay)))))
	if nuke.Total() > 0 {
		botSchedule(w, botID, ScheduleSendInput{OriginID: nukeVillage.ID, X: k.X, Y: k.Y, Units: nuke, Type: "attack", Mode: "arriveAt", At: at})
	}
	offset := int64(0)
	used := map[*Village]int{}
	for i, v := range nobleFrom {
		if strong {
			offset += 100
		} else {
			offset += 100 + int64(math.Floor(rng()*1100))
		}
		units := UnitCounts{UNoble: 1}
		spare := v.Units[UAxe] - used[v]
		if v == nukeVillage {
			spare -= nuke[UAxe]
		}
		if escort := min(escorts[i], max(0, spare)); escort > 0 {
			units[UAxe] = escort
			used[v] += escort
		}
		botSchedule(w, botID, ScheduleSendInput{OriginID: v.ID, X: k.X, Y: k.Y, Units: units, Type: "attack", Mode: "arriveAt", At: at + offset})
	}
	if strong && k.OwnerID != "" {
		decoys(w, botID, PlayerID(k.OwnerID), k.ID, at, rng)
	}
}

// LaunchOperation runs an operation on the campaign target, with a purpose fitted to the situation.
func LaunchOperation(w World, botID PlayerID, k *KnownVillage) Purpose {
	purpose := ChoosePurpose(w, botID, k)
	switch purpose {
	case "conquer":
		LaunchConquest(w, botID, k)
	case "demolish":
		b := BSmithy
		if k.Buildings[BFarm] >= k.Buildings[BSmithy] {
			b = BFarm
		}
		LaunchDemolish(w, botID, k, b)
	case "farmPlayer":
		LaunchFarmPlayer(w, botID, k)
	default:
		LaunchHarass(w, botID, k)
	}
	return purpose
}
