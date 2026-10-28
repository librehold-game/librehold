package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func TestInstantPlayerPlaysTheHour(t *testing.T) {
	run, err := gametest.RunInstantPlayer(gametest.InstantPlayerOptions{Seed: 1, IdleHumans: 2})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("villages %d rank %d topBot %d ownedChanges %d firstConquest %dms botSendsAfterEnd %d",
		run.Villages, run.Rank, run.TopBotVillages, run.OwnedChanges, run.FirstConquestMs, run.BotSendsAfterEnd)
	if run.FirstConquestMs <= 0 || run.FirstConquestMs > 5*game.MsPerMinute {
		t.Fatalf("first conquest at %d ms", run.FirstConquestMs)
	}
	if run.Villages < 2 {
		t.Fatalf("villages %d", run.Villages)
	}
	if want := int64(game.InstantWorldConfig.DurationMin) * game.MsPerMinute; run.World.Now() != want {
		t.Fatalf("world stopped at %d, want %d", run.World.Now(), want)
	}
	if run.Rank < 1 {
		t.Fatalf("rank %d", run.Rank)
	}
}
