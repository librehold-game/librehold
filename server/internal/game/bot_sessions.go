package game

import (
	"math"
	"sort"
)

// Bot sessions: when a bot is online, and what it does in one sitting.

func init() {
	registerHandler("session", onSessionEvent)
	registerHandler("think", onThinkEvent)
	registerHandler("sessionEnd", onSessionEndEvent)
	registerHandler("phone", onPhoneEvent)
	scheduleFirstSession = ScheduleNextSession
	phoneCheck = MaybePhoneCheck
}

type Session struct{ StartMin, EndMin float64 }

// SessionsForDay are one day's sessions (local minutes; the end may pass midnight): short daytime check-ins and
// the evening. Every day a little different — ±30 min shifts, a skipped check-in (10 %), a free evening (7 %), a
// late night (10 %), a day off (3 %). All between waking and sleep. rushRng (world start, BotConfig.StartRush)
// adds check-ins every hour or two all day, from a separate stream.
func SessionsForDay(b *BotBrain, weekend bool, rng func() float64, rushRng func() float64) []Session {
	r := b.Routine
	shift := 0.0
	if weekend {
		shift = r.WeekendWakeShiftMin
	}
	wake, sleep := r.WakeMin+shift, r.SleepMin
	clampTo := func(s Session) (Session, bool) {
		start := math.Max(wake, math.Round(s.StartMin))
		end := math.Min(sleep, math.Round(s.EndMin))
		return Session{start, end}, end-start >= 2
	}
	jitter := func() float64 { return uniform(rng, -30, 30) }
	skip := 0.1 + BotConfig.WeakSkipCheckIn*(1-b.Skill)
	var out []Session
	if rushRng != nil {
		rush := BotConfig.StartRush
		for t := wake + uniform(rushRng, rush.GapMin[0], rush.GapMin[1]); t < sleep; t += uniform(rushRng, rush.GapMin[0], rush.GapMin[1]) {
			l := uniform(rushRng, rush.LenMin[0], rush.LenMin[1])
			if rushRng() < 0.1+rush.WeakSkip*(1-b.Skill) {
				continue
			}
			if s, ok := clampTo(Session{t, t + l}); ok {
				out = append(out, s)
			}
		}
	}
	if rng() < 0.03 {
		c := r.CheckIns[0]
		if s, ok := clampTo(Session{c.AtMin + shift + jitter(), c.AtMin + shift + 3}); ok {
			return []Session{s}
		}
		return nil
	}
	for _, c := range r.CheckIns {
		if rng() < skip {
			continue
		}
		start := c.AtMin + shift + jitter()
		if s, ok := clampTo(Session{start, start + c.LenMin}); ok {
			out = append(out, s)
		}
	}
	if rng() >= 0.07 {
		late := 0.0
		if rng() < 0.1 {
			late = uniform(rng, 60, 150)
		}
		start := r.EveningStartMin + jitter() + late
		if s, ok := clampTo(Session{start, start + r.EveningLenMin}); ok {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartMin < out[j].StartMin })
	var merged []Session
	for _, s := range out {
		if n := len(merged); n > 0 && s.StartMin <= merged[n-1].EndMin {
			merged[n-1].EndMin = math.Max(merged[n-1].EndMin, s.EndMin)
		} else {
			merged = append(merged, s)
		}
	}
	return merged
}

// DaySessions are a bot's sessions on local day `day` — from one stream (seed, 'day', id, day).
func DaySessions(w World, id PlayerID, day int64) []Session {
	p, b := w.Player(id), w.Brain(id)
	if p == nil || b == nil {
		return nil
	}
	var rushRng func() float64
	if WorldTimeOf(w, day, 0) < p.JoinAt+int64(BotConfig.StartRush.Days*float64(MsPerDay)) {
		rushRng = Mulberry32(HiddenSeed(w, "rush", id, day))
	}
	return SessionsForDay(b, IsWeekendDay(day), Mulberry32(HiddenSeed(w, "day", id, day)), rushRng)
}

// nextSession is the nearest session starting no earlier than `from`. It looks from yesterday (a session past
// midnight) up to two weeks ahead — two empty days in a row happen, and a bot without a next session would
// never come back.
func nextSession(w World, id PlayerID, from int64) (start, end int64, ok bool) {
	today := LocalDayIndex(w, from)
	for d := today - 1; d <= today+14 && (d <= today+2 || !ok); d++ {
		for _, s := range DaySessions(w, id, d) {
			st := WorldTimeOf(w, d, s.StartMin)
			if st < from {
				continue
			}
			if !ok || st < start {
				start, end, ok = st, WorldTimeOf(w, d, s.EndMin), true
			}
		}
	}
	return start, end, ok
}

// NextSessionStart is the start of the nearest session from `from`, or false if there is none for two weeks.
func NextSessionStart(w World, id PlayerID, from int64) (int64, bool) {
	start, _, ok := nextSession(w, id, from)
	return start, ok
}

func ScheduleNextSession(w World, id PlayerID) {
	p := w.Player(id)
	if p == nil || w.Brain(id) == nil || p.Status != StatusActive {
		return
	}
	if w.Config().BotThinkEverySec > 0 {
		startInstantBot(w, id)
		return
	}
	if start, _, ok := nextSession(w, id, w.Now()); ok {
		w.Schedule("session:"+id, start)
	}
}

var sessionSpy func(id PlayerID, at int64)

// SpySessions observes every session start (tests only); returns a restore func.
func SpySessions(fn func(id PlayerID, at int64)) (restore func()) {
	prev := sessionSpy
	sessionSpy = fn
	return func() { sessionSpy = prev }
}

func onSessionEvent(w World, id PlayerID) error {
	p, b := w.Player(id), w.Brain(id)
	if p == nil || b == nil || p.Status != StatusActive {
		return nil
	}
	start, end, ok := nextSession(w, id, w.Now())
	if !ok || start != w.Now() {
		ScheduleNextSession(w, id)
		return nil
	}
	if sessionSpy != nil {
		sessionSpy(id, w.Now())
	}
	UpdateMood(w, id)
	UpdateOverwhelm(w, id)
	if p.Status != StatusActive {
		return nil
	}
	b.Online = true
	b.SessionEndsAt = end
	w.Unschedule("phone:" + id)
	w.Schedule("think:"+id, w.Now())
	w.Schedule("sessionEnd:"+id, end)
	return nil
}

func onThinkEvent(w World, id PlayerID) error {
	b := w.Brain(id)
	if b == nil || !b.Online {
		return nil
	}
	if w.Config().BotThinkEverySec > 0 {
		UpdateMood(w, id)
		Think(w, id, false)
		w.Schedule("think:"+id, w.Now()+instantThinkGap(w, id))
		return nil
	}
	Think(w, id, false)
	rng := Mulberry32(HiddenSeed(w, "think", id, w.Now()))
	next := w.Now() + int64(math.Round(BotConfig.ThinkEveryMin*float64(MsPerMinute)*uniform(rng, 0.6, 1.4)))
	if next < b.SessionEndsAt {
		w.Schedule("think:"+id, next)
	}
	return nil
}

func onSessionEndEvent(w World, id PlayerID) error {
	b := w.Brain(id)
	if b == nil || !b.Online {
		return nil
	}
	b.Online = false
	w.Unschedule("think:" + id)
	PrepareForOffline(w, id)
	ScheduleNextSession(w, id)
	return nil
}

func onPhoneEvent(w World, id PlayerID) error {
	p, b := w.Player(id), w.Brain(id)
	if p == nil || b == nil || b.Online || p.Status != StatusActive {
		return nil
	}
	Think(w, id, true)
	return nil
}

// MaybePhoneCheck: an offline bot under attack glances at its phone with probability 0.15 + 0.5 × caution, at a
// random moment before the hit (at the latest 5 min before). The earliest planned glance is kept.
func MaybePhoneCheck(w World, owner PlayerID, commandID string, arrivesAt int64) {
	p, b := w.Player(owner), w.Brain(owner)
	if p == nil || b == nil || b.Online || p.Status != StatusActive {
		return
	}
	rng := Mulberry32(HiddenSeed(w, "phone", commandID))
	if rng() >= 0.15+0.5*b.Caution {
		return
	}
	latest := arrivesAt - 5*MsPerMinute
	if latest <= w.Now() {
		return
	}
	t := int64(math.Round(float64(w.Now()) + rng()*float64(latest-w.Now())))
	key := "phone:" + owner
	if at, ok := w.ScheduledAt(key); ok && at <= t {
		return
	}
	w.Schedule(key, t)
}
