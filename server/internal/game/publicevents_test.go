package game_test

import (
	"math"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// attackAndLand sends units from v at (x, y) and plays to the arrival.
func attackAndLand(t *testing.T, w *gametest.MemWorld, v *game.Village, x, y int, u game.UnitCounts) *game.Command {
	t.Helper()
	for id, n := range u {
		v.Units[id] += n
	}
	c, err := game.SendCommand(w, game.PlayerID(v.OwnerID), v.ID, x, y, u, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHumanRaidOnBarbariansWritesABattle(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	target := ensureNearestBarbarian(w, v)
	defenders := game.DefenderUnitsOf(target).Total()
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 100})
	events := w.PublicEvents()
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	e := events[0]
	if e.Kind != "battle" || e.Attacker != w.Player("h1").Name || e.Defender != "" || e.AttackerBot || e.DefenderBot ||
		e.Troops != 100+defenders || e.X != target.X || e.Y != target.Y {
		t.Fatalf("event %+v, defenders %d", e, defenders)
	}
}

func TestConquestWritesOneConquestAndNoBattle(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	target := ensureNearestBarbarian(w, v)
	disarm(w, target.ID)
	target.Loyalty = 1
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 50, game.UNoble: 1})
	if target.OwnerID != "h1" {
		t.Fatalf("the village was not conquered: owner %q", target.OwnerID)
	}
	events := w.PublicEvents()
	if len(events) != 1 || events[0].Kind != "conquest" || events[0].Attacker != w.Player("h1").Name || events[0].Defender != "" {
		t.Fatalf("events %+v", events)
	}
}

func TestBotOnlyFightsWriteNothing(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	bot := gametest.MakeBotVillage(w, v)
	gametest.NewBotPlayer(w, "bot_2")
	other := gametest.MakeBotVillage(w, bot)
	game.TransferVillage(w, other, "bot_2", "")
	barbarian := ensureNearestBarbarian(w, bot)
	attackAndLand(t, w, bot, barbarian.X, barbarian.Y, game.UnitCounts{game.UAxe: 100})
	attackAndLand(t, w, bot, other.X, other.Y, game.UnitCounts{game.UAxe: 100})
	if events := w.PublicEvents(); len(events) != 0 {
		t.Fatalf("events %+v", events)
	}
}

func TestBotAttackOnHumanIsPublic(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	w.Player("h1").ProtectedUntil = 0
	bot := gametest.MakeBotVillage(w, v)
	attackAndLand(t, w, bot, v.X, v.Y, game.UnitCounts{game.UAxe: 100})
	events := w.PublicEvents()
	if len(events) != 1 || !events[0].AttackerBot || events[0].Attacker != w.Player("bot_test").Name ||
		events[0].Defender != w.Player("h1").Name || events[0].DefenderBot {
		t.Fatalf("events %+v", events)
	}
}

func TestScoutingWritesNothing(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	if err := gametest.ScoutOnce(w, v, ensureNearestBarbarian(w, v)); err != nil {
		t.Fatal(err)
	}
	if events := w.PublicEvents(); len(events) != 0 {
		t.Fatalf("events %+v", events)
	}
}

func TestSmallBattlesAreWrittenToo(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	target := ensureNearestBarbarian(w, v)
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.USpear: 5})
	if events := w.PublicEvents(); len(events) != 1 || events[0].Troops < 5 {
		t.Fatalf("events %+v", events)
	}
}

func TestFieldBattleWritesABattleAtTheMeetingPoint(t *testing.T) {
	f := newFieldWorld(t)
	f.a.Units[game.UAxe] = 10
	c := meet(t, f, game.UnitCounts{game.UAxe: 10}, false)
	events := f.w.PublicEvents()
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	e := events[0]
	if e.Kind != "battle" || e.Attacker != f.w.Player("h1").Name || e.Defender != f.w.Player("h2").Name || e.Troops != 110 ||
		e.X != int(math.Round(c.Point.X)) || e.Y != int(math.Round(c.Point.Y)) {
		t.Fatalf("event %+v at %+v", e, *c.Point)
	}
}

func TestFieldScoutAndMissWriteNothing(t *testing.T) {
	f := newFieldWorld(t)
	meet(t, f, game.UnitCounts{game.UScout: 3}, true)
	if events := f.w.PublicEvents(); len(events) != 0 {
		t.Fatalf("scouting the army: events %+v", events)
	}

	f = newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	f.attack.ArrivesAt = c.ArrivesAt - 1
	c.Intercept.Leg.ArrivesAt = f.attack.ArrivesAt
	f.w.Schedule("cmd:"+f.attack.ID, f.attack.ArrivesAt)
	if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.w.PublicEvents() {
		if e.Attacker == f.w.Player("h1").Name {
			t.Fatalf("a missed interception is public: %+v", e)
		}
	}
}

func TestOtherWorldKindsWriteNothing(t *testing.T) {
	cfg := gametest.TestConfig()
	cfg.Kind = game.WorldKind("player")
	w, v := gametest.NewWorldWithHuman(7, cfg)
	v.Units = game.Units{}
	target := ensureNearestBarbarian(w, v)
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 100})
	if events := w.PublicEvents(); len(events) != 0 {
		t.Fatalf("events %+v", events)
	}
}

func TestNoEventWithoutAnAttacker(t *testing.T) {
	w, v := gametest.NewTestWorld(7)
	home := gametest.AddStartVillage(w, "h1", v.X+2, v.Y)
	target := ensureNearestBarbarian(w, home)
	home.Units[game.UAxe] = 100
	c, err := game.SendCommand(w, "h1", home.ID, target.X, target.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	game.TransferVillage(w, home, "", "")
	if err := gametest.Advance(w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	if events := w.PublicEvents(); len(events) != 0 {
		t.Fatalf("an attack from an abandoned village: events %+v", events)
	}

	w.Player("h1").Status = game.StatusQuit
	attackAndLand(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 100})
	if events := w.PublicEvents(); len(events) != 1 || events[0].Attacker != w.Player("h1").Name {
		t.Fatalf("a player who quit: events %+v", events)
	}
}
