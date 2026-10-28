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

// ScheduleRanking schedules the next refresh on the full hour of the local clock (09:00, 10:00…).
func ScheduleRanking(w World) {
	off := ((MsPerHour-w.ClockOrigin()%MsPerHour)%MsPerHour + MsPerHour) % MsPerHour
	w.Schedule("stats", NextGridTime(w.Now(), off, MsPerHour))
}

// SnapshotRanking sets public points from the start, so the ranking is not all zeros until the first full hour.
func SnapshotRanking(w World) {
	points := w.PointsByOwner()
	for _, p := range w.Players() {
		p.Points = points[p.ID]
	}
}

// onRankingEvent (every full local hour): each player's points go to the hourly history, and at local
// midnight also to the daily one. Rankings and bots read these.
func onRankingEvent(w World, _ string) error {
	midnight := LocalMinuteOfDay(w, w.Now()) < 60
	points := w.PointsByOwner()
	for _, p := range w.Players() {
		p.Points = points[p.ID]
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
