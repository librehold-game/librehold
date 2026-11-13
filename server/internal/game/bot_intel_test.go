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
	// A scouting mission gets through only when it beats ScoutDefenseFactor per defending scout, and a caught mission
	// reports nothing. The party therefore has to clear a village that does not make scouts its business.
	kept := float64(game.TroopTargets[game.RoleMixed][game.UScout])
	need := kept * game.CombatBalance.ScoutDefenseFactor
	if float64(game.ScoutParty) <= need {
		t.Fatalf("a party of %d scouts cannot see a village keeping %.0f (needs more than %.0f)",
			game.ScoutParty, kept, need)
	}
}

func TestMilitaryVillagesKeepTwoScoutParties(t *testing.T) {
	// Arriving at a defended village is normal and often fatal, so an offence village must be able to look twice: it
	// keeps a full party and has another one behind it.
	kept := game.TroopTargets[game.RoleOff][game.UScout]
	party := game.ScoutPartyFor(kept)
	if party < game.ScoutParty {
		t.Fatalf("off keeps %d scouts but sends only %d, less than a full party of %d", kept, party, game.ScoutParty)
	}
	if kept-party < game.ScoutParty/2 {
		t.Fatalf("off keeps %d scouts and sends %d, leaving too few for another look", kept, party)
	}
}

func TestScoutPartyScalesWithTheVillage(t *testing.T) {
	// A small village can only send what it has; a big one commits a full party and keeps a reserve.
	if got := game.ScoutPartyFor(50); got != 50 {
		t.Fatalf("a village with 50 scouts sends all 50, got %d", got)
	}
	if got := game.ScoutPartyFor(4 * game.ScoutParty); got >= 4*game.ScoutParty {
		t.Fatalf("a village with %d scouts sent everything (%d)", 4*game.ScoutParty, got)
	}
	if got := game.ScoutPartyFor(4 * game.ScoutParty); got < game.ScoutParty {
		t.Fatalf("a village with %d scouts sent only %d, less than a full party", 4*game.ScoutParty, got)
	}
}

func TestScoutsDoNotCrowdOutTheArmy(t *testing.T) {
	// A military role's troop plan is a cap, and the plan fills its farm: what it spends on scouts is army the village
	// never gets. A scout defends as little as it attacks, so a scouting fleet in a defence village is the worst case.
	// (A mixed village's plan is deliberately small and it over-recruits past it, so the share there says little.)
	for _, role := range []game.VillageRole{game.RoleOff, game.RoleDef} {
		plan := 0
		for u, n := range game.TroopTargets[role] {
			plan += n * game.UnitDefs[u].Pop
		}
		scoutPop := game.TroopTargets[role][game.UScout] * game.UnitDefs[game.UScout].Pop
		if share := float64(scoutPop) / float64(plan); share > game.ScoutShare(role) {
			t.Fatalf("%s spends %.0f%% of its troop plan on scouts, over its %.0f%% allowance",
				role, 100*share, 100*game.ScoutShare(role))
		}
	}
}

func TestMilitaryVillagesCanStillSeeAHomeGuardVillage(t *testing.T) {
	// A defence village holds a home guard rather than a fleet, so an offence village's full party must be able to look
	// into it: reconnaissance has to work in the direction a bot actually needs it.
	guard := float64(game.TroopTargets[game.RoleDef][game.UScout])
	if float64(game.ScoutParty) <= guard*game.CombatBalance.ScoutDefenseFactor {
		t.Fatalf("a party of %d scouts cannot see a defence village keeping %.0f", game.ScoutParty, guard)
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
