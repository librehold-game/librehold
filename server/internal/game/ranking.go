package game

// Rankings.

const (
	hourlyKeep = 168
	dailyKeep  = 120
)

func init() {
	registerHandler("stats", onRankingEvent)
	worldStartHooks = append(worldStartHooks, func(w World) {
		SnapshotRanking(w)
		ScheduleRanking(w)
	})
	humanJoinHooks = append(humanJoinHooks, func(w World, id PlayerID) {
		w.Player(id).Points = PlayerPoints(w, id)
	})
}

// ScheduleRanking schedules the next refresh on the full hour of the local clock (09:00, 10:00…), or on
// the full minute in a short world, which would otherwise see one refresh in its whole game.
func ScheduleRanking(w World) {
	every := int64(MsPerHour)
	if IsShort(w.Config()) {
		every = MsPerMinute
	}
	off := ((every-w.ClockOrigin()%every)%every + every) % every
	w.Schedule("stats", NextGridTime(w.Now(), off, every))
}

// SnapshotRanking sets public points from the start, so the ranking is not all zeros until the first full hour.
func SnapshotRanking(w World) {
	points := w.PointsByOwner()
	for _, p := range w.Players() {
		p.Points = points[p.ID]
	}
}

// onRankingEvent recounts each player's points. On the full local hour they also go to the hourly
// history, and at local midnight to the daily one. Rankings and bots read these.
func onRankingEvent(w World, _ string) error {
	minute := LocalMinuteOfDay(w, w.Now())
	fullHour := minute%60 == 0
	midnight := minute == 0
	points := w.PointsByOwner()
	for _, p := range w.Players() {
		p.Points = points[p.ID]
		if !fullHour {
			continue
		}
		p.History.Hourly = appendKeep(p.History.Hourly, p.Points, hourlyKeep)
		if midnight {
			p.History.Daily = appendKeep(p.History.Daily, p.Points, dailyKeep)
		}
	}
	ScheduleRanking(w)
	return nil
}

func appendKeep(s []int, v, keep int) []int {
	s = append(s, v)
	if len(s) > keep {
		s = append([]int{}, s[len(s)-keep:]...)
	}
	return s
}
