package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// TestInstantPacing holds the instant world to the owner's picture of the hour: the active player ends with three to
// five villages and in the top two, villages with an owner change hands in every game, and no bot runs away with the
// map.
func TestInstantPacing(t *testing.T) {
	if testing.Short() {
		t.Skip("plays five instant hours")
	}
	for seed := uint32(1); seed <= 5; seed++ {
		run, err := gametest.RunInstantPlayer(gametest.InstantPlayerOptions{Seed: seed, IdleHumans: 2})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: villages %d rank %d topBot %d ownedChanges %d firstConquest %.0fs botSendsAfterEnd %d",
			seed, run.Villages, run.Rank, run.TopBotVillages, run.OwnedChanges, float64(run.FirstConquestMs)/1000, run.BotSendsAfterEnd)
		if run.Villages < 3 || run.Villages > 5 {
			t.Errorf("seed %d: the player holds %d villages", seed, run.Villages)
		}
		if run.Rank > 2 {
			t.Errorf("seed %d: the player ranks %d", seed, run.Rank)
		}
		if run.OwnedChanges == 0 {
			t.Errorf("seed %d: no village with an owner changed hands", seed)
		}
		if run.TopBotVillages > 6 {
			t.Errorf("seed %d: a bot holds %d villages", seed, run.TopBotVillages)
		}
		if run.BotSendsAfterEnd != 0 {
			t.Errorf("seed %d: %d bot sends after the bell", seed, run.BotSendsAfterEnd)
		}
		if run.FirstConquestMs <= 0 || run.FirstConquestMs > 5*game.MsPerMinute {
			t.Errorf("seed %d: first conquest at %d ms", seed, run.FirstConquestMs)
		}
	}
}
