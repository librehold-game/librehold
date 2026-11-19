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
	party := game.ScoutPartyFor(kept, &game.KnownVillage{})
	if party < game.ScoutParty {
		t.Fatalf("off keeps %d scouts but sends only %d, less than a full party of %d", kept, party, game.ScoutParty)
	}
	if kept-party < game.ScoutParty/2 {
		t.Fatalf("off keeps %d scouts and sends %d, leaving too few for another look", kept, party)
	}
}

func TestScoutPartyEscalatesAfterAFailedLook(t *testing.T) {
	// A party and a guard are not constants: the bot tries, and a party that came home changes what it sends next. If
	// the two were fixed numbers, "who can see whom" would be settled by the playbook instead of by play.
	lost, won := ptr(true), ptr(false)
	first := game.ScoutPartyFor(5000, &game.KnownVillage{ScoutLoss: lost, LastScoutParty: game.ScoutParty})
	second := game.ScoutPartyFor(5000, &game.KnownVillage{ScoutLoss: lost, LastScoutParty: first})
	if second <= first {
		t.Fatalf("a caught party of %d was followed by %d, which is not an escalation", first, second)
	}
	// Once it gets through, the size holds rather than climbing for ever.
	held := game.ScoutPartyFor(5000, &game.KnownVillage{ScoutLoss: won, LastScoutParty: first})
	if held != first {
		t.Fatalf("a party that got through changed from %d to %d", first, held)
	}
	// And it never asks for more than the village has.
	if got := game.ScoutPartyFor(650, &game.KnownVillage{ScoutLoss: lost, LastScoutParty: game.ScoutParty}); got != 650 {
		t.Fatalf("sent %d of 650", got)
	}
}

func TestCaughtScoutsTeachNoGarrison(t *testing.T) {
	// A caught party must not tell the bot what beat it — that is the fog of war. All it learns is that its scouts did
	// not come home, which is enough to try again bigger.
	w, home, den := scoutBotWorld(t, 95, 0)
	target := w.Village(den.ID)
	target.Units[game.USpear] = 5000
	game.ObserveBattle(w, "bot_0", target, game.BattleSeen{Won: ptr(false), ScoutParty: game.ScoutParty})

	k := w.Brain("bot_0").Memory.Known[den.ID]
	if k == nil {
		t.Fatal("the attempt should be remembered")
	}
	if k.Units != nil && k.Units.Total() > 0 {
		t.Fatalf("a caught party reported the garrison: %+v", k.Units)
	}
	if k.ScoutLoss == nil || !*k.ScoutLoss || k.LastScoutParty != game.ScoutParty {
		t.Fatalf("the failed attempt was not recorded: loss=%v party=%d", k.ScoutLoss, k.LastScoutParty)
	}
	_ = home
}

func TestScoutsThatGotThroughDoRecordTheGarrison(t *testing.T) {
	w, _, den := scoutBotWorld(t, 96, 0)
	target := w.Village(den.ID)
	target.Units[game.USpear] = 40
	units := game.UnitCounts{game.USpear: 40}
	game.ObserveBattle(w, "bot_0", target, game.BattleSeen{Units: &units, Won: ptr(true), ScoutParty: game.ScoutParty})

	k := w.Brain("bot_0").Memory.Known[den.ID]
	if k == nil || k.Units == nil || k.Units.Total() == 0 {
		t.Fatalf("a party that got through must record the garrison: %+v", k)
	}
	if k.ScoutLoss == nil || *k.ScoutLoss {
		t.Fatalf("a party that got through is not a loss: %v", k.ScoutLoss)
	}
}

func TestScoutPartyNeverDribbles(t *testing.T) {
	// A handful at a time is a handful lost at a time.
	if got := game.ScoutPartyFor(50, &game.KnownVillage{}); got != 50 {
		t.Fatalf("a village with 50 scouts sends all 50, got %d", got)
	}
	if got := game.ScoutPartyFor(5000, &game.KnownVillage{}); got != game.ScoutParty {
		t.Fatalf("an unknown target opens with the baseline party of %d, got %d", game.ScoutParty, got)
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

func TestStaleScoutUsesTheVillageItChecked(t *testing.T) {
	// The party has to come from the village whose scouts were counted, and the road has to be marked, or the next
	// session in the same hour sends another party at the same target from somewhere else.
	w, near := gametest.NewTestWorld(77)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewBotPlayer(w, "bot_0")
	w.PutBrain("bot_0", &game.BotBrain{Skill: 0.8, Memory: game.BotMemory{Known: map[string]*game.KnownVillage{}}})
	// The village with the scouts is the far one, so "the nearest village that has any" is a different village.
	far := gametest.AddStartVillage(w, "bot_0", near.X-9, near.Y)
	far.Units[game.UScout] = 5000
	nearby := gametest.AddStartVillage(w, "bot_0", near.X+3, near.Y)
	nearby.Units[game.UScout] = 10
	den := gametest.AddStartVillage(w, "h1", near.X+5, near.Y)
	den.Units[game.USpear] = 50

	// A garrison the bot no longer trusts: seen, but long enough ago to be stale.
	stale := w.Now() - 3*int64(game.BotConfig.IntelStaleHours*float64(game.MsPerHour))
	k := &game.KnownVillage{ID: den.ID, X: den.X, Y: den.Y, OwnerID: "h1", Points: den.Points,
		SeenAt: w.Now(), UnitsSeenAt: &stale, Units: &game.UnitCounts{game.USpear: 10}, Wall: ptr(0), Hauls: []float64{}}
	w.Brain("bot_0").Memory.Known[den.ID] = k

	game.PlanFarming(w, "bot_0")
	var scouting []*game.ScheduledSend
	for _, s := range w.ScheduledSendsOf("bot_0") {
		if s.Units[game.UScout] > 0 {
			scouting = append(scouting, s)
		}
	}
	if len(scouting) == 0 {
		t.Fatal("a stale target was not scouted at all")
	}
	for _, s := range scouting {
		if s.OriginID != far.ID {
			t.Fatalf("the scouting party came from %s, but %s is the village whose scouts were counted",
				w.Village(s.OriginID).Name, far.Name)
		}
	}
}

func TestASecondSessionDoesNotScoutTheSameTargetTwice(t *testing.T) {
	// The party is on the road, so the garrison is being counted. The next session in the same hour must not spend a
	// second party on the same question.
	w, near := gametest.NewTestWorld(78)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewBotPlayer(w, "bot_0")
	w.PutBrain("bot_0", &game.BotBrain{Skill: 0.8, Memory: game.BotMemory{Known: map[string]*game.KnownVillage{}}})
	home := gametest.AddStartVillage(w, "bot_0", near.X+3, near.Y)
	home.Units[game.UScout] = 5000
	den := gametest.AddStartVillage(w, "h1", near.X+5, near.Y)
	den.Units[game.USpear] = 50
	stale := w.Now() - 3*int64(game.BotConfig.IntelStaleHours*float64(game.MsPerHour))
	w.Brain("bot_0").Memory.Known[den.ID] = &game.KnownVillage{ID: den.ID, X: den.X, Y: den.Y, OwnerID: "h1",
		Points: den.Points, SeenAt: w.Now(), UnitsSeenAt: &stale, Units: &game.UnitCounts{game.USpear: 10},
		Wall: ptr(0), Hauls: []float64{}}

	game.PlanFarming(w, "bot_0")
	first := len(w.ScheduledSendsOf("bot_0"))
	if first == 0 {
		t.Fatal("nothing was scouted")
	}
	game.PlanFarming(w, "bot_0")
	if got := len(w.ScheduledSendsOf("bot_0")); got != first {
		t.Fatalf("the second session sent %d more parties at a target already being scouted", got-first)
	}
}

func TestHarassmentIsNotGatedOnScoutParties(t *testing.T) {
	// A probe is a fake: one ram, else one axe. Its job is to spoil a defender's day and make him keep troops at home,
	// not to count a garrison — and under the fog of war a probe that dies reports nothing at all, so a bigger one
	// would buy the bot nothing. Gating harassment on four full scouting parties (3,600 scouts) took it away from
	// almost every bot, which is the one thing the probes were good at.
	w, a, target := warWorld(t, 0.95, "canBeLost")
	v := game.PlayerVillages(w, a)[0]
	// No scouts at all, and no train: harassment is the only thing this bot could still do to the player.
	v.Units[game.UScout] = 0
	v.Units[game.UAxe] = 40
	v.Units[game.URam] = 0
	v.Units[game.UNoble] = 0
	v.Units[game.UCatapult] = 0
	k := knownOf(w, a, target)
	// A defence it has counted and cannot clear, so a train is out of the question on the numbers alone.
	def := game.UnitCounts{game.USpear: 100000}
	k.Units = &def
	// The bot is the weaker player, so "farm the player" is not on the table either: harass is what is left.
	target.Points = 500000
	if got := game.ChoosePurpose(w, a, k); got != "harass" {
		t.Fatalf("a bot with 40 axes and no scouts drew %q instead of harassing", got)
	}
}
