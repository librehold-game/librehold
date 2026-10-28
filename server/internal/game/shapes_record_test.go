package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// TestShapesGolden records testdata/golden/shapes.json, the entity samples TestEntityJSONRoundTrips reads: a
// default world after two hours, with a bot village next to the human that the human scouted and raids in flight.
func TestShapesGolden(t *testing.T) {
	w, v := gametest.NewDefaultWorld(7)
	if err := gametest.Advance(w, 2*game.MsPerHour); err != nil {
		t.Fatal(err)
	}
	bv := gametest.MakeBotVillage(w, v)
	w.Player("h1").ProtectedUntil, w.Player("bot_test").ProtectedUntil = 0, 0
	if err := gametest.ScoutOnce(w, v, bv); err != nil {
		t.Fatal(err)
	}
	if _, err := gametest.SendManyRaids(w, v, 2, game.UnitCounts{game.UAxe: 10}); err != nil {
		t.Fatal(err)
	}
	var bots []game.PlayerID
	for _, p := range w.Players() {
		if !p.IsHuman && p.ID != "bot_test" {
			bots = append(bots, p.ID)
		}
	}
	slices.Sort(bots)
	got := map[string]any{
		"villages": []*game.Village{v, bv},
		"players":  []*game.Player{w.Player("h1"), w.Player("bot_test")},
		"brains":   map[game.PlayerID]*game.BotBrain{bots[0]: w.Brain(bots[0])},
		"commands": w.CommandsHome(v.ID),
		"reports":  w.ReportsOf("h1"),
		"human":    w.Human("h1"),
		"news":     w.News(3),
		"intel":    w.Intel("h1", bv.ID),
	}
	game.Golden(t, "shapes", got)
}
