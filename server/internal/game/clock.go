package game

import (
	"math"
	"time"
)

// The local clock. World time runs 1:1 with the wall clock, so world instant t is local wall
// time ClockOrigin()+t (ms since the epoch with the zone offset already applied).

// WorldTimezone is the one server-wide zone of every world's local time (bot routines, rankings, rumours).
const WorldTimezone = "Europe/Warsaw"

func LocalDayIndex(w World, t int64) int64 {
	return floorDiv(w.ClockOrigin()+t, MsPerDay)
}

func LocalMinuteOfDay(w World, t int64) int {
	ms := (w.ClockOrigin() + t) % MsPerDay
	if ms < 0 {
		ms += MsPerDay
	}
	return int(ms / MsPerMinute)
}

func IsWeekend(w World, t int64) bool { return IsWeekendDay(LocalDayIndex(w, t)) }

// IsWeekendDay: Saturday or Sunday of a local day index.
func IsWeekendDay(day int64) bool {
	dow := time.UnixMilli(day * MsPerDay).UTC().Weekday()
	return dow == time.Saturday || dow == time.Sunday
}

// WorldTimeOf is the world instant of minute `min` of local day `day` (min may exceed 1440).
func WorldTimeOf(w World, day int64, min float64) int64 {
	return day*MsPerDay + int64(math.Round(min*float64(MsPerMinute))) - w.ClockOrigin()
}
