package gametest

import "librehold/internal/game"

// InstantSimResult sums up one simulated instant world.
type InstantSimResult struct {
	FirstConquestMs         int64 // world time of the first human conquest; -1 if none
	BotConquests            int
	BotAttacksOnHumansEarly int // bot attacks launched at humans in the first 10 minutes
	BotActions              int // bot commands seen plus bots whose points grew
	HumanVillages           int
}

const simStep = 10_000

// RunInstantSim plays an hour of an instant world with `humans` scripted guests who do what the tutorial says:
// at 30 s each sends its hetman with 50 axemen at the nearest barbarian village.
func RunInstantSim(seed uint32, humans int) (InstantSimResult, error) {
	cfg := game.InstantWorldConfig
	w := NewMemWorld(seed, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		return InstantSimResult{}, err
	}
	var ids []game.PlayerID
	for i := range humans {
		id := game.PlayerID("h" + string(rune('1'+i)))
		if _, err := game.JoinHuman(w, id, "Guest"+string(rune('1'+i)), int64(i+1), game.QuadrantRandom); err != nil {
			return InstantSimResult{}, err
		}
		ids = append(ids, id)
	}
	startPoints := map[game.PlayerID]int{}
	for _, p := range w.Players() {
		if !p.IsHuman {
			startPoints[p.ID] = game.PlayerPoints(w, p.ID)
		}
	}
	r := InstantSimResult{FirstConquestMs: -1}
	sent := false
	seen := map[string]bool{}
	end := int64(cfg.DurationMin) * game.MsPerMinute
	for now := int64(simStep); now <= end; now += simStep {
		if err := Advance(w, now); err != nil {
			return r, err
		}
		if !sent && now >= 30_000 {
			sent = true
			for _, id := range ids {
				home := w.Village(w.Player(id).VillageIDs[0])
				x, y := NearestBarbarianXY(w, home)
				units := game.UnitCounts{game.UNoble: 1, game.UAxe: 50}
				if _, err := game.SendCommand(w, id, home.ID, x, y, units, "attack", game.SendOpts{}); err != nil {
					return r, err
				}
			}
		}
		for _, c := range w.Commands() {
			if seen[c.ID] {
				continue
			}
			origin := w.Village(c.OriginID)
			if origin == nil || origin.OwnerID == "" || game.IsHumanID(game.PlayerID(origin.OwnerID)) {
				continue
			}
			seen[c.ID] = true
			r.BotActions++
			if c.Type != "attack" || now > 10*game.MsPerMinute {
				continue
			}
			if t := w.Village(c.TargetID); t != nil && t.OwnerID != "" && game.IsHumanID(game.PlayerID(t.OwnerID)) {
				r.BotAttacksOnHumansEarly++
			}
		}
		if r.FirstConquestMs < 0 {
			for _, c := range w.ConquestsSince(0) {
				if game.IsHumanID(c.ToID) {
					r.FirstConquestMs = c.T
					break
				}
			}
		}
	}
	for _, c := range w.ConquestsSince(0) {
		if !game.IsHumanID(c.ToID) {
			r.BotConquests++
		}
	}
	for _, p := range w.Players() {
		if p.IsHuman {
			r.HumanVillages += len(p.VillageIDs)
		} else if game.PlayerPoints(w, p.ID) > startPoints[p.ID] {
			r.BotActions++
		}
	}
	return r, nil
}
