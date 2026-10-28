package game

import (
	"math"
	"sort"
	"strconv"
)

// Bots adapting to the players they meet: habits, grudges and the timing of their replies.

const habitKeep = 20

// Strong bots draw conclusions from observations, weak ones rarely; each new observation is a new chance.
var habitLearns = StrongWeak{Strong: 0.9, Weak: 0.1}

type HabitObservation struct {
	DodgeReturnAt *int64
	FarmedAt      *int64
}

func habitsOf(b *BotBrain, owner PlayerID) *Habits {
	h := b.Habits[owner]
	if h == nil {
		h = &Habits{DodgeReturnMins: []float64{}, FarmIntervalsMin: []float64{}}
		b.Habits[owner] = h
	}
	return h
}

func keepLast(s []float64, n int) []float64 {
	if len(s) > n {
		return append([]float64{}, s[len(s)-n:]...)
	}
	return s
}

// RecordHabit records an opponent's habit: the local minute when their village turned out empty despite a
// garrison seen before (a dodge), or the gap since their previous attack on the bot (farming rhythm).
func RecordHabit(w World, botID, owner PlayerID, o HabitObservation) {
	b := w.Brain(botID)
	if b == nil {
		return
	}
	h := habitsOf(b, owner)
	if o.DodgeReturnAt != nil {
		h.DodgeReturnMins = keepLast(append(h.DodgeReturnMins, float64(LocalMinuteOfDay(w, *o.DodgeReturnAt))), habitKeep)
	}
	if o.FarmedAt != nil {
		if h.LastHitAt != nil {
			h.FarmIntervalsMin = keepLast(append(h.FarmIntervalsMin, float64(*o.FarmedAt-*h.LastHitAt)/float64(MsPerMinute)), habitKeep)
		}
		h.LastHitAt = ptrTo(*o.FarmedAt)
	}
}

func learns(w World, botID PlayerID, what string) bool {
	return Follows(Mulberry32(HiddenSeed(w, "learns", botID, what)), w.Brain(botID).Skill, habitLearns)
}

// HardFarmer is a player farming the bot more often than hourly (median of ≥ 6 attacks): the bot answers with
// wall, hiding place and revenge. Owners are checked in id order.
func HardFarmer(w World, botID PlayerID) PlayerID {
	b := w.Brain(botID)
	if b == nil {
		return ""
	}
	owners := make([]PlayerID, 0, len(b.Habits))
	for id := range b.Habits {
		owners = append(owners, id)
	}
	sort.Strings(owners)
	for _, owner := range owners {
		h := b.Habits[owner]
		last := int64(0)
		if h.LastHitAt != nil {
			last = *h.LastHitAt
		}
		if len(h.FarmIntervalsMin) < 6 || w.Now()-last > MsPerDay {
			continue
		}
		sorted := append([]float64{}, h.FarmIntervalsMin...)
		sort.Float64s(sorted)
		if sorted[len(sorted)/2] < 60 && learns(w, botID, "farm:"+owner+":"+strconv.Itoa(len(h.FarmIntervalsMin))) {
			return owner
		}
	}
	return ""
}

// DodgeMinute is an opponent's steady dodge-return minute (≥ 3 observations, deviation < 30 min), averaged on
// the circle of the day.
func DodgeMinute(w World, botID, owner PlayerID) (int, bool) {
	b := w.Brain(botID)
	if b == nil || b.Habits[owner] == nil || len(b.Habits[owner].DodgeReturnMins) < 3 {
		return 0, false
	}
	mins := b.Habits[owner].DodgeReturnMins
	ref := mins[0]
	unwrapped := make([]float64, len(mins))
	mean := 0.0
	for i, m := range mins {
		switch {
		case m-ref > 720:
			m -= 1440
		case ref-m > 720:
			m += 1440
		}
		unwrapped[i] = m
		mean += m
	}
	mean /= float64(len(mins))
	variance := 0.0
	for _, m := range unwrapped {
		variance += (m - mean) * (m - mean)
	}
	sd := math.Sqrt(variance / float64(len(mins)))
	if sd >= 30 || !learns(w, botID, "dodge:"+owner+":"+strconv.Itoa(len(mins))) {
		return 0, false
	}
	return ((int(math.Round(mean)) % 1440) + 1440) % 1440, true
}

// LandAfterDodge is the first instant ≥ earliest, 2–10 min after the minute the opponent usually comes back.
func LandAfterDodge(w World, botID, owner PlayerID, earliest int64, rng func() float64) (int64, bool) {
	m, ok := DodgeMinute(w, botID, owner)
	if !ok {
		return 0, false
	}
	want := float64(m) + 2 + rng()*8
	now := float64(LocalMinuteOfDay(w, earliest))
	offset := math.Mod(math.Mod(want-now, 1440)+1440, 1440)
	return int64(math.Round(float64(earliest) + offset*float64(MsPerMinute))), true
}

// SendJitter is a random 5–90 min send delay, so the send hour does not give a bot away.
func SendJitter(w World, botID PlayerID, salt string) int64 {
	rng := Mulberry32(HiddenSeed(w, "jitter", botID, salt, w.Now()))
	return BotDur(w, int64(math.Round((5+rng()*85)*float64(MsPerMinute))))
}
