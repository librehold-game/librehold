package game

// Overwhelmed bots give up. QuitPlayer serves any player ("leave world").

func init() { registerHandler("abandon", onAbandonEvent) }

// UpdateOverwhelm, at session start: overwhelm decays (×0.85) and grows with lost villages and attacks. Attacks weigh
// less the better the player ("strong players rarely leave"). Falling behind is no reason to leave, and weak bots
// never leave: they keep playing however hard they are hit.
func UpdateOverwhelm(w World, botID PlayerID) {
	p, b := w.Player(botID), w.Brain(botID)
	if p == nil || b == nil || p.Status != StatusActive {
		return
	}
	st := b.LastSessionStats
	b.Overwhelm = 0.85*b.Overwhelm + 1.0*float64(st.VillagesLost) + 0.1*(1-0.9*b.Skill)*float64(st.IncomingAttacks)
	b.LastSessionStats = SessionStats{}
	if b.Overwhelm > BotConfig.QuitScale*b.Resilience && TierOf(b.Skill) != TierWeak {
		QuitPlayer(w, botID)
	}
}

// QuitPlayer: the player leaves — no more sessions or thinking; the villages stand until abandoned.
func QuitPlayer(w World, id PlayerID) {
	p := w.Player(id)
	if p == nil || p.Status != StatusActive {
		return
	}
	p.Status = StatusQuit
	p.QuitAt = ptrTo(w.Now())
	if b := w.Brain(id); b != nil {
		b.Online = false
	}
	for _, kind := range []string{"session", "think", "sessionEnd", "phone"} {
		w.Unschedule(kind + ":" + id)
	}
	w.Schedule("abandon:"+id, w.Now()+BotDur(w, int64(BotConfig.AbandonAfterDays*float64(MsPerDay))))
}

// onAbandonEvent: the villages of a player who left turn barbarian with the buildings they had.
func onAbandonEvent(w World, id PlayerID) error {
	p := w.Player(id)
	if p == nil || p.Status != StatusQuit {
		return nil
	}
	for _, v := range PlayerVillages(w, id) {
		TransferVillage(w, v, "", "")
	}
	return nil
}
