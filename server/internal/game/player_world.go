package game

import "math"

// Player worlds: a short game a player hosts for friends. They play by the instant rules (IsShort); what is their
// own is here: the map sized to the company, the host ending the game, and the host sending a player away.

const (
	playerMapMin = 20
	playerMapMax = 40
)

// PlayerMapSize is the side of a player world's map: room for every player at the spawn spacing, with a margin.
func PlayerMapSize(players int) int {
	n := 2*int(math.Ceil(MapConfig.SpawnSpacing*0.56*math.Sqrt(float64(players)))) + 12
	return min(playerMapMax, max(playerMapMin, n))
}

// EndByHost ends the game now, as the time limit would: the top player by points wins.
func EndByHost(w World) {
	endWorld(w, "host", topPlayer(w))
}

// KickPlayer sends an active human away: they quit, and their villages turn barbarian at once.
func KickPlayer(w World, id PlayerID) {
	p := w.Player(id)
	if p == nil || !p.IsHuman || p.Status != StatusActive {
		return
	}
	QuitPlayer(w, id)
	for _, v := range PlayerVillages(w, id) {
		TransferVillage(w, v, "", "")
	}
	w.Unschedule("abandon:" + id)
}
