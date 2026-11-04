package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// lastVillageWorld builds a world with the given lastVillage mode and one bot owning a single village.
func lastVillageWorld(t *testing.T, mode string) (*gametest.MemWorld, *game.Player, *game.Village) {
	t.Helper()
	cfg := gametest.TestConfig()
	cfg.LastVillage = mode
	w, _ := gametest.NewWorldWithHuman(1234, cfg)
	bot := gametest.NewBotPlayer(w, "b1")
	v := gametest.AddStartVillage(w, "b1", 50, 50)
	return w, bot, v
}

func TestLastVillageModes(t *testing.T) {
	t.Run("protected (the old default): a one-village bot's last village is safe", func(t *testing.T) {
		w, bot, v := lastVillageWorld(t, "protected")
		if game.CanBeConquered(w, v) {
			t.Fatalf("bot %s: the last village should be protected for bots too", bot.ID)
		}
	})

	t.Run("playersOnly: a player can conquer the last village of a one-village bot", func(t *testing.T) {
		w, bot, v := lastVillageWorld(t, "playersOnly")
		if !game.CanBeConquered(w, v) {
			t.Fatalf("bot %s: its last village should be conquerable in playersOnly mode", bot.ID)
		}
	})

	t.Run("playersOnly: bots can conquer other bots' last villages too", func(t *testing.T) {
		w, _, _ := lastVillageWorld(t, "playersOnly")
		gametest.NewBotPlayer(w, "b2")
		v := gametest.AddStartVillage(w, "b2", 60, 60)
		if !game.CanBeConquered(w, v) {
			t.Fatal("a second bot's last village should be conquerable in playersOnly mode")
		}
	})

	t.Run("playersOnly: a human's last village stays protected", func(t *testing.T) {
		cfg := gametest.TestConfig()
		cfg.LastVillage = "playersOnly"
		w, home := gametest.NewWorldWithHuman(1234, cfg)
		if len(w.Player("h1").VillageIDs) != 1 || game.CanBeConquered(w, w.Village(home.ID)) {
			t.Fatal("a one-village human's last village is conquerable")
		}
	})

	t.Run("canBeLost: everything is conquerable", func(t *testing.T) {
		w, bot, v := lastVillageWorld(t, "canBeLost")
		if !game.CanBeConquered(w, v) {
			t.Fatalf("bot %s: a village should be conquerable in a canBeLost world", bot.ID)
		}
	})

	t.Run("an abandoned player's last village is conquerable in every protected mode", func(t *testing.T) {
		for _, mode := range []string{"protected", "playersOnly"} {
			cfg := gametest.TestConfig()
			cfg.LastVillage = mode
			w, home := gametest.NewWorldWithHuman(1234, cfg)
			w.Player("h1").Status = game.StatusQuit
			if !game.CanBeConquered(w, w.Village(home.ID)) {
				t.Fatalf("%s: an abandoned player's last village should be conquerable", mode)
			}
		}
	})
}
