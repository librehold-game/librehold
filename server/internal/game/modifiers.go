package game

var neutralModifiers = SideModifiers{Resources: 1, Recruitment: 1, Building: 1}

// IsHumanID reports whether a player id is a human's: humans are `h<accountID>` (plus an optional rejoin
// suffix), bots `bot_<i>`. Pure formulas use it to pick a village's side without touching the world.
func IsHumanID(id PlayerID) bool {
	return len(id) >= 2 && id[0] == 'h' && id[1] >= '0' && id[1] <= '9'
}

func sideModifiers(cfg *WorldConfig, owner PlayerRef) SideModifiers {
	if owner == "" {
		return neutralModifiers
	}
	if IsHumanID(PlayerID(owner)) {
		return cfg.HumanModifiers
	}
	return cfg.BotModifiers
}

// ModifiersFor returns the multipliers of the side a village belongs to: humans, bots or barbarians (all 1).
func ModifiersFor(w World, v *Village) SideModifiers {
	return sideModifiers(w.Config(), v.OwnerID)
}
