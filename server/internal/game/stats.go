package game

// The human's statistics and points history.

var StatsBalance = struct {
	SampleIntervalMs int64 // minimum gap between point history samples
	MaxSamples       int   // above this the history is thinned (every other old sample)
}{MsPerHour, 1000}

// SamplePoints appends a points sample to a human's history when ≥ 1 h passed since the last one.
func SamplePoints(w World, humanID PlayerID) {
	h := w.Human(humanID)
	if h == nil {
		return
	}
	hist := h.Stats.PointsHistory
	if n := len(hist); n > 0 && w.Now()-hist[n-1][0] < StatsBalance.SampleIntervalMs {
		return
	}
	hist = append(hist, [2]int64{w.Now(), int64(PlayerPoints(w, humanID))})
	if len(hist) > StatsBalance.MaxSamples {
		keep := make([][2]int64, 0, len(hist))
		for i, s := range hist {
			if i%2 == 0 || i >= len(hist)-100 {
				keep = append(keep, s)
			}
		}
		hist = keep
	}
	h.Stats.PointsHistory = hist
}
