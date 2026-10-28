package game

import (
	"math"
	"sort"
)

// Bot names and colours.

// botStyleOrder is the order the style draw walks BotConfig.StyleWeights in (a map).
var botStyleOrder = [...]BotStyle{StyleFarmer, StyleTurtle, StyleAggressor, StylePointBuilder, StyleOpportunist}

func uniform(rng func() float64, a, b float64) float64 { return a + rng()*(b-a) }

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// DrawSkill: strong with the difficulty's floor probability, otherwise Beta(2, 3) — most players are average or weak.
func DrawSkill(rng func() float64, d Difficulty) float64 {
	if rng() < DifficultyFloor[d] {
		return uniform(rng, BotConfig.StrongSkill, 1)
	}
	u := []float64{rng(), rng(), rng(), rng()}
	sort.Float64s(u)
	return u[1]
}

// DrawRoutine draws a day in local minutes: waking, short daytime check-ins, the evening session, sleep.
func DrawRoutine(rng func() float64) Routine {
	wakeMin := math.Round(uniform(rng, 6*60, 8.5*60))
	sleepLen := math.Round(uniform(rng, 6*60, 9*60))
	n := 1 + int(math.Floor(rng()*3))
	checkIns := make([]CheckIn, n)
	for i := range checkIns {
		checkIns[i] = CheckIn{AtMin: math.Round(uniform(rng, 9*60, 17*60-10)), LenMin: math.Round(uniform(rng, 3, 10))}
	}
	sort.SliceStable(checkIns, func(i, j int) bool { return checkIns[i].AtMin < checkIns[j].AtMin })
	return Routine{
		WakeMin: wakeMin, SleepMin: wakeMin + 1440 - sleepLen, CheckIns: checkIns,
		EveningStartMin:     math.Round(uniform(rng, 18*60, 21*60)),
		EveningLenMin:       math.Round(uniform(rng, 60, 180)),
		WeekendWakeShiftMin: math.Round(uniform(rng, 60, 120)),
	}
}

func drawStyle(rng func() float64) BotStyle {
	total := 0.0
	for _, s := range botStyleOrder {
		total += BotConfig.StyleWeights[s]
	}
	r := rng() * total
	for _, s := range botStyleOrder {
		r -= BotConfig.StyleWeights[s]
		if r <= 0 {
			return s
		}
	}
	return botStyleOrder[len(botStyleOrder)-1]
}

// CreateBotBrain draws a bot's personality from one stream (seed, 'brain', id): it depends only on the world and id.
func CreateBotBrain(seed uint32, id PlayerID, d Difficulty) *BotBrain {
	return drawBotBrain(Mulberry32(HashSeed(seed, "brain", id)), d)
}

// worldBotBrain is CreateBotBrain on the world's hidden stream, so the map does not give away which bots are weak
// or when they sleep.
func worldBotBrain(w World, id PlayerID, d Difficulty) *BotBrain {
	return drawBotBrain(Mulberry32(HiddenSeed(w, "brain", id)), d)
}

func drawBotBrain(rng func() float64, d Difficulty) *BotBrain {
	skill := DrawSkill(rng, d)
	style := drawStyle(rng)
	resilience := clamp01(0.25 + 0.6*skill + uniform(rng, -0.1, 0.1))
	caution := rng()
	return &BotBrain{
		Skill: skill, Style: style, Resilience: resilience, Caution: caution, Routine: DrawRoutine(rng),
		ActionLog: []int64{}, Roles: map[string]VillageRole{},
		Memory:  BotMemory{Known: map[string]*KnownVillage{}, DiscoveredAt: -1},
		Grudges: map[PlayerID]*Grudge{}, Habits: map[PlayerID]*Habits{},
		Mood: Mood{Phase: "build"},
	}
}
