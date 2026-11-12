package game_test

import (
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// Recon is what a bot sends to learn a garrison. It has to be a real scouting mission, and it has to be big enough to
// beat the scouts a village keeps at home: a scouting mission that is caught brings nothing back at all.

// scoutBotWorld is a world with bot "bot_0" holding one village that has scouts, and a human village next to it to
// scout. A scouting send needs a real village at the target, since there is nothing to look into otherwise.
func scoutBotWorld(t *testing.T, seed uint32, scouts int) (*gametest.MemWorld, *game.Village, *game.Village) {
	t.Helper()
	w, near := gametest.NewTestWorld(seed)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewBotPlayer(w, "bot_0")
	w.PutBrain("bot_0", &game.BotBrain{Skill: 0.8, Memory: game.BotMemory{Known: map[string]*game.KnownVillage{}}})
	home := gametest.AddStartVillage(w, "bot_0", near.X+3, near.Y)
	home.Units[game.UScout] = scouts
	den := gametest.AddStartVillage(w, "h1", near.X+5, near.Y)
	return w, home, den
}

func TestScoutPartyIsBigEnoughToSeeAVillage(t *testing.T) {
	// A scouting mission only gets through when it beats ScoutDefenseFactor per defending scout. The party has to clear
	// a village keeping the deciding share of its playbook's scouts, or the bot sends scouts to be caught — and a
	// caught mission reports nothing.
	kept := float64(game.TroopTargets[game.RoleMixed][game.UScout])
	need := kept * game.CombatBalance.ScoutDefenseFactor
	if float64(game.ScoutParty) < need {
		t.Fatalf("a party of %d scouts cannot be expected to see a village keeping %.0f (needs %.0f)",
			game.ScoutParty, kept, need)
	}
}

func TestCampaignScoutSendsTheWholeParty(t *testing.T) {
	w, home, den := scoutBotWorld(t, 91, game.ScoutParty+50)

	if !game.CampaignScout(w, "bot_0", den.X, den.Y) {
		t.Fatal("the bot refused to scout")
	}
	sends := w.ScheduledSendsOf("bot_0")
	if len(sends) != 1 {
		t.Fatalf("one scouting send, got %d", len(sends))
	}
	if got := sends[0].Units[game.UScout]; got != game.ScoutParty {
		t.Fatalf("scouted with %d scouts from %s, want the whole party of %d",
			got, home.ID, game.ScoutParty)
	}
	// Nothing but scouts: anything else would be an attack, and an attack only teaches what it survives.
	if sends[0].Units.Total() != sends[0].Units[game.UScout] {
		t.Fatalf("a scouting mission carries scouts only: %+v", sends[0].Units)
	}
}

func TestCampaignScoutSendsWhatItHas(t *testing.T) {
	w, _, den := scoutBotWorld(t, 92, 7)

	if !game.CampaignScout(w, "bot_0", den.X, den.Y) {
		t.Fatal("a bot with a few scouts should still use them")
	}
	if sends := w.ScheduledSendsOf("bot_0"); len(sends) != 1 || sends[0].Units[game.UScout] != 7 {
		t.Fatalf("sends %+v", sends)
	}
}

func TestIntelStale(t *testing.T) {
	w, _, _ := scoutBotWorld(t, 94, 0)
	now := w.Now()

	if !game.IntelStale(w, &game.KnownVillage{}) {
		t.Fatal("a village never scouted is stale")
	}
	fresh := &game.KnownVillage{Units: &game.UnitCounts{game.USpear: 10}, UnitsSeenAt: ptr(now)}
	if game.IntelStale(w, fresh) {
		t.Fatal("a garrison seen now is fresh")
	}
	// Well past the setting, not exactly on it: the boundary itself is not what this test is about.
	old := &game.KnownVillage{Units: &game.UnitCounts{game.USpear: 10},
		UnitsSeenAt: ptr(now - int64(game.BotConfig.IntelStaleHours)*game.MsPerHour - game.MsPerHour)}
	if !game.IntelStale(w, old) {
		t.Fatal("a garrison seen well over a day ago is stale")
	}
}

func TestObserveBattleStampsWhenTheGarrisonWasSeen(t *testing.T) {
	// SeenAt moves whenever the map is rediscovered, so it cannot answer "how old is this garrison?".
	w, _, den := scoutBotWorld(t, 93, 0)
	target := w.Village(den.ID)
	units := game.UnitCounts{game.USpear: 40}
	game.ObserveBattle(w, "bot_0", target, game.BattleSeen{Units: &units})

	k := w.Brain("bot_0").Memory.Known[den.ID]
	if k == nil || k.UnitsSeenAt == nil {
		t.Fatal("a garrison the bot saw must carry the time it saw it")
	}
	if *k.UnitsSeenAt != w.Now() {
		t.Fatalf("stamped %d, want now %d", *k.UnitsSeenAt, w.Now())
	}
	if game.IntelStale(w, k) {
		t.Fatal("a garrison just seen is not stale")
	}
}
