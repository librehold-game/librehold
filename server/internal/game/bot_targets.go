package game

import (
	"math"
	"sort"
)

// Target scoring for bot attacks and raids.

const campaignRadius = 25

// KnownDefence is the defence of what the bot saw: the sum of unit defence, strengthened by the wall.
func KnownDefence(k *KnownVillage) (float64, bool) {
	if k.Units == nil {
		return 0, false
	}
	d := 0.0
	for _, u := range UnitIDs {
		d += UnitDefs[u].Defense * float64(k.Units[u])
	}
	wall := 0
	if k.Wall != nil {
		wall = *k.Wall
	}
	return d * (1 + 0.04*float64(wall)), true
}

// OwnOffence is the attack strength at home in the bot's strongest village.
func OwnOffence(w World, botID PlayerID) float64 {
	best := 0.0
	for _, v := range PlayerVillages(w, botID) {
		a := 0.0
		for _, u := range UnitIDs {
			a += float64(v.Units[u]) * UnitDefs[u].Attack
		}
		best = math.Max(best, a)
	}
	return best
}

func nearestOwn(w World, botID PlayerID, x, y int) float64 {
	d := math.Inf(1)
	for _, v := range PlayerVillages(w, botID) {
		d = math.Min(d, Distance(v.X, v.Y, x, y))
	}
	return d
}

func sortedKnown(b *BotBrain) []*KnownVillage {
	ids := make([]string, 0, len(b.Memory.Known))
	for id := range b.Memory.Known {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*KnownVillage, len(ids))
	for i, id := range ids {
		out[i] = b.Memory.Known[id]
	}
	return out
}

// FrontlineDistance is the distance from a village to the nearest known village of another player.
func FrontlineDistance(w World, botID PlayerID, v *Village) float64 {
	d := math.Inf(1)
	b := w.Brain(botID)
	if b == nil {
		return d
	}
	for _, k := range sortedKnown(b) {
		if k.OwnerID == "" || k.OwnerID == Ref(botID) {
			continue
		}
		d = math.Min(d, Distance(v.X, v.Y, k.X, k.Y))
	}
	return d
}

func flatFor(w World, owner PlayerRef, hours int) bool {
	if owner == "" {
		return false
	}
	p := w.Player(PlayerID(owner))
	if p == nil || len(p.History.Hourly) < hours {
		return false
	}
	tail := p.History.Hourly[len(p.History.Hourly)-hours:]
	for _, x := range tail {
		if x != tail[0] {
			return false
		}
	}
	return true
}

// ScoreTarget: points^0.8 / (1 + distance) × weakness × inactivity × human weight. Zero when the target may not
// be taken (world setting, protection) or its known defence exceeds 80 % of the bot's offence.
func ScoreTarget(w World, botID PlayerID, k *KnownVillage) float64 {
	if !MayTarget(w, botID, k.OwnerID) {
		return 0
	}
	if k.OwnerID != "" && IsProtected(w, PlayerID(k.OwnerID)) {
		return 0
	}
	off := OwnOffence(w, botID)
	def, knownDef := KnownDefence(k)
	if knownDef && def > 0.8*off {
		return 0
	}
	weakness := 0.5
	if knownDef {
		weakness = math.Max(0, 1-def/math.Max(1, off))
	}
	inactivity := 1.0
	if flatFor(w, k.OwnerID, 48) {
		inactivity = 1.5
	}
	payback := 1.0
	if k.OwnerID != "" && PlayerID(k.OwnerID) == HardFarmer(w, botID) {
		payback = 3
	}
	// With nobles at home the bot looks for villages it can take (a last village may be protected).
	takeable := 1.0
	if v := w.Village(k.ID); v != nil && CanBeConquered(w, v) {
		for _, o := range PlayerVillages(w, botID) {
			if o.Units[UNoble] > 0 {
				takeable = 3
				break
			}
		}
	}
	return (math.Pow(math.Max(1, float64(k.Points)), 0.8) / (1 + nearestOwn(w, botID, k.X, k.Y))) *
		weakness * inactivity * HumanTargetWeight(w, k.OwnerID) * payback * takeable
}

type OfflineWindow struct{ StartMin, EndMin int }

// LikelyOfflineWindow is the longest daily run of local hours without points growth in the last 7 days of
// public history — when the owner most likely sleeps. Public data only (the hourly ranking).
func LikelyOfflineWindow(w World, owner PlayerID) (OfflineWindow, bool) {
	p := w.Player(owner)
	if p == nil || len(p.History.Hourly) < 48 {
		return OfflineWindow{}, false
	}
	h := p.History.Hourly
	last := w.Now() / MsPerHour * MsPerHour
	var growth, seen [24]int
	n := len(h)
	for i := 1; i < n; i++ {
		hour := LocalMinuteOfDay(w, last-int64(n-1-i)*MsPerHour-1) / 60
		growth[hour] += h[i] - h[i-1]
		seen[hour]++
	}
	var quiet [24]bool
	all, any := true, false
	for i := range quiet {
		quiet[i] = growth[i] <= 0 && seen[i] > 0
		all = all && quiet[i]
		any = any || quiet[i]
	}
	if all || !any {
		return OfflineWindow{}, false
	}
	bestStart, bestLen := -1, 0
	for s := range 24 {
		if !quiet[s] || quiet[(s+23)%24] {
			continue
		}
		l := 0
		for l < 24 && quiet[(s+l)%24] {
			l++
		}
		if l > bestLen {
			bestStart, bestLen = s, l
		}
	}
	if bestStart < 0 {
		return OfflineWindow{}, false
	}
	return OfflineWindow{bestStart * 60, ((bestStart + bestLen) % 24) * 60}, true
}

// rememberHumanVillages: with "all against humans" every human village is in every bot's memory (position and
// points are public on the map); defence is learnt only by scouting.
func rememberHumanVillages(w World, botID PlayerID) {
	known := brainOf(w, botID).Memory.Known
	for _, p := range w.Players() {
		if !p.IsHuman {
			continue
		}
		for _, id := range p.VillageIDs {
			if k := known[id]; k != nil {
				k.OwnerID = Ref(p.ID)
				continue
			}
			v := w.Village(id)
			known[id] = &KnownVillage{ID: id, X: v.X, Y: v.Y, OwnerID: Ref(p.ID), Points: v.Points, SeenAt: w.Now(), Hauls: []float64{}}
		}
	}
}

// PickCampaignTarget picks, in a campaign, a score-weighted target among known player villages within 25 fields;
// dropped after 3 failed operations or 2 days without progress. With "all against humans" humans are the main
// target: their villages at any distance and before all others, if any may and can be attacked.
func PickCampaignTarget(w World, botID PlayerID) {
	brain := w.Brain(botID)
	if brain == nil || brain.Mood.Phase != "campaign" {
		return
	}
	m := &brain.Mood
	if m.TargetVillage != "" {
		progress := w.Now()
		if m.ProgressAt != nil {
			progress = *m.ProgressAt
		}
		stale := m.Failures >= 3 || w.Now()-progress > BotDur(w, 2*MsPerDay)
		v := w.Village(m.TargetVillage)
		gone := v == nil || v.OwnerID != Ref(m.TargetPlayer)
		if stale || gone {
			m.TargetVillage, m.TargetPlayer, m.Failures = "", "", 0
		}
	}
	if m.TargetVillage != "" {
		return
	}
	allAgainst := w.Config().BotStance == StanceAllAgainstHumans
	if allAgainst {
		rememberHumanVillages(w, botID)
	}
	var options []Option[*KnownVillage]
	humanOption := false
	for _, k := range sortedKnown(brain) {
		if k.OwnerID == "" || k.OwnerID == Ref(botID) {
			continue
		}
		primary := allAgainst && IsHumanID(PlayerID(k.OwnerID))
		if !primary && nearestOwn(w, botID, k.X, k.Y) > campaignRadius {
			continue
		}
		if score := ScoreTarget(w, botID, k); score > 0 {
			options = append(options, Option[*KnownVillage]{k, score})
			humanOption = humanOption || primary
		}
	}
	if allAgainst && humanOption {
		kept := options[:0]
		for _, o := range options {
			if IsHumanID(PlayerID(o.Value.OwnerID)) {
				kept = append(kept, o)
			}
		}
		options = kept
	}
	if len(options) == 0 {
		return
	}
	pick := WeightedPick(Mulberry32(HiddenSeed(w, "campaign", botID, w.Now())), options, brain.Skill)
	m.TargetVillage = pick.ID
	m.TargetPlayer = PlayerID(pick.OwnerID)
	m.ProgressAt = ptrTo(w.Now())
}
