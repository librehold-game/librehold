package gametest

// HomeBuckets is how many villages and players CommandsHome has commands filed under.
func (w *MemWorld) HomeBuckets() int { return len(w.byHome) }
