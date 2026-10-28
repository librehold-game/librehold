package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// Goal 2 of the instant-worlds design: a guest who follows the tutorial conquers a village within five minutes,
// bots play and leave newcomers alone for their first ten minutes.
func TestInstantGuestConquersWithinFiveMinutes(t *testing.T) {
	for seed := uint32(1); seed <= 5; seed++ {
		r, err := gametest.RunInstantSim(seed, 3)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		t.Logf("seed %d: %+v", seed, r)
		if r.FirstConquestMs < 0 || r.FirstConquestMs > 5*game.MsPerMinute {
			t.Errorf("seed %d: first conquest at %d ms", seed, r.FirstConquestMs)
		}
		if r.BotActions == 0 {
			t.Errorf("seed %d: the bots did nothing", seed)
		}
		if r.BotAttacksOnHumansEarly != 0 {
			t.Errorf("seed %d: %d bot attacks on newcomers in the first 10 minutes", seed, r.BotAttacksOnHumansEarly)
		}
		if r.BotConquests < 1 {
			t.Errorf("seed %d: bots conquered nothing in an hour", seed)
		}
	}
}
