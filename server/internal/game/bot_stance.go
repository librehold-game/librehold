package game

// The bots' stance towards humans: grudges and the aggressor reputation are kept per human player.

func grudgeActive(w World, botID, against PlayerID) bool {
	b := w.Brain(botID)
	if b == nil {
		return false
	}
	g := b.Grudges[against]
	return g != nil && w.Now()-g.LastAt < BotDur(w, int64(BotConfig.GrudgeFadeDays*float64(MsPerDay)))
}

// MayTarget says whether a bot may pick a village of this owner. Barbarians and other bots: always (protection
// is CheckSend's). Humans: by the world's BotStance.
func MayTarget(w World, botID PlayerID, owner PlayerRef) bool {
	if owner == "" || !IsHumanID(PlayerID(owner)) {
		return true
	}
	human := PlayerID(owner)
	switch w.Config().BotStance {
	case StancePeacefulRetaliate:
		return grudgeActive(w, botID, human)
	case StancePeacefulReputation:
		h := w.Human(human)
		return h != nil && h.AggressorUntil > w.Now()
	}
	return true
}

// HumanTargetWeight multiplies the score of human targets: 5 when all bots are against humans.
func HumanTargetWeight(w World, owner PlayerRef) float64 {
	if w.Config().BotStance == StanceAllAgainstHumans && owner != "" && IsHumanID(PlayerID(owner)) {
		return 5
	}
	return 1
}

// OnHumanAggression: a human attacked or scouted an active bot — that bot's grudge against this human, and this
// human's aggressor reputation, for BotConfig.GrudgeFadeDays.
func OnHumanAggression(w World, botID, humanID PlayerID) {
	p, b := w.Player(botID), w.Brain(botID)
	if p == nil || b == nil || p.Status != StatusActive {
		return
	}
	prev := 0.0
	if g := b.Grudges[humanID]; g != nil {
		prev = g.Level
	}
	b.Grudges[humanID] = &Grudge{LastAt: w.Now(), Level: prev + 1}
	if h := w.Human(humanID); h != nil {
		h.AggressorUntil = max(h.AggressorUntil, w.Now()+BotDur(w, int64(BotConfig.GrudgeFadeDays*float64(MsPerDay))))
	}
}
