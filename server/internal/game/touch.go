package game

// Touch settles a village to the world's now before its resources, units or loyalty are used. The server
// calls it on every village an order or handler reads or changes, rather than settling every village on every event.
func Touch(w World, v *Village) {
	now := w.Now()
	Accrue(v, now, w.Config())
	SettleRecruitQueues(v, now)
	SettleLoyalty(v, now, w.Config().WorldSpeed)
}
