package game

// A bot's brain: its skill, style, routine and memory, drawn from the world seed.

var thinkSpy func(id PlayerID, defenceOnly bool)

// SpyThinks observes every bot think (tests only); returns a restore func.
func SpyThinks(fn func(id PlayerID, defenceOnly bool)) (restore func()) {
	prev := thinkSpy
	thinkSpy = fn
	return func() { thinkSpy = prev }
}

var offlineSpy func(id PlayerID, nextSessionAt int64)

// SpyOffline observes a bot's villages right after it prepared to leave the game (tests only).
func SpyOffline(fn func(id PlayerID, nextSessionAt int64)) (restore func()) {
	prev := offlineSpy
	offlineSpy = fn
	return func() { offlineSpy = prev }
}

// Think is one bot think: defence always, then (unless defenceOnly) map, farming, expansion, campaign and
// each village's economy. A weak player sometimes does nothing at all.
func Think(w World, id PlayerID, defenceOnly bool) {
	if thinkSpy != nil {
		thinkSpy(id, defenceOnly)
	}
	TouchVillages(w, id)
	PlanDefence(w, id)
	PlanFieldBattles(w, id)
	if defenceOnly {
		return
	}
	DiscoverIfStale(w, id)
	if IdlesNow(w, id) {
		return
	}
	PlanExpansion(w, id)
	PlanFarming(w, id)
	PlanCampaign(w, id)
	for _, v := range PlayerVillages(w, id) {
		PlanBuilding(w, id, v)
		PlanRecruitment(w, id, v)
		PlanResearch(w, id, v)
	}
}

// PrepareForOffline queues the villages for the break and sends the night raids.
func PrepareForOffline(w World, id PlayerID) {
	TouchVillages(w, id)
	next, ok := NextSessionStart(w, id, w.Now())
	if !ok {
		next = w.Now()
	}
	for _, v := range PlayerVillages(w, id) {
		QueueForOffline(w, id, v, next)
	}
	PlanNightRuns(w, id, next)
	if offlineSpy != nil {
		offlineSpy(id, next)
	}
}
