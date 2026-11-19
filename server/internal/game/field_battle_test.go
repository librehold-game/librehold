package game_test

import (
	"encoding/json"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

func fieldReports(w *gametest.MemWorld, p game.PlayerID) []*game.Report {
	var out []*game.Report
	for _, r := range w.ReportsOf(p) {
		if r.Type == "field" || r.Type == "fieldScout" {
			out = append(out, r)
		}
	}
	return out
}

// returnsFromPoint are the commands going home to villageID that started in the field.
func returnsFromPoint(w *gametest.MemWorld, villageID string) []*game.Command {
	var out []*game.Command
	for _, c := range w.Commands() {
		if c.Type == "return" && c.OriginID == "" && c.Point != nil && c.TargetID == villageID {
			out = append(out, c)
		}
	}
	return out
}

func meet(t *testing.T, f *fieldWorld, u game.UnitCounts, scout bool) *game.Command {
	t.Helper()
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, u, scout)
	if err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFieldBattleTargetSurvivesHit(t *testing.T) {
	f := newFieldWorld(t)
	f.a.Units[game.UAxe] = 10
	c := meet(t, f, game.UnitCounts{game.UAxe: 10}, false)
	if f.w.Command(c.ID) != nil {
		t.Fatal("the intercept command is gone after the battle")
	}
	target := f.w.Command(f.attack.ID)
	if target == nil || target.Units[game.UAxe] >= 100 || target.Units[game.UAxe] == 0 {
		t.Fatalf("target after a lost interception = %+v", target)
	}
	if !target.FieldHit || !f.w.IsScheduled("cmd:"+target.ID) || len(target.Threats) != 0 {
		t.Fatalf("a hit army marches on: fieldHit=%v scheduled=%v threats=%v", target.FieldHit, f.w.IsScheduled("cmd:"+target.ID), target.Threats)
	}
	if len(returnsFromPoint(f.w, f.a.ID)) != 0 {
		t.Fatal("a beaten interceptor has nobody left to return")
	}
	for _, p := range []game.PlayerID{"h1", "h2"} {
		rs := fieldReports(f.w, p)
		if len(rs) != 1 || rs[0].Type != "field" || rs[0].Field == nil || rs[0].Field.TargetCommandID != f.attack.ID || rs[0].Field.TargetType != "attack" {
			t.Fatalf("%s field reports = %+v", p, rs)
		}
	}
}

func TestFieldBattleLeavesAutoFarmRaidReportsUnmarked(t *testing.T) {
	f := newFieldWorld(t)
	f.w.Command(f.attack.ID).AutoFarm = true
	f.a.Units[game.UAxe] = 10
	meet(t, f, game.UnitCounts{game.UAxe: 10}, false)
	for _, p := range []game.PlayerID{"h1", "h2"} {
		if rs := fieldReports(f.w, p); len(rs) != 1 || rs[0].AutoFarm {
			t.Fatalf("%s field reports = %+v", p, rs)
		}
	}
	if err := gametest.Advance(f.w, f.attack.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	arrival := func(p game.PlayerID) *game.Report {
		for _, r := range f.w.ReportsOf(p) {
			if r.Type == "attack" {
				return r
			}
		}
		t.Fatalf("no attack report for %s", p)
		return nil
	}
	if !arrival("h2").AutoFarm || arrival("h1").AutoFarm {
		t.Fatal("only the sender's arrival report is marked")
	}
}

func TestFieldBattleDestroysTarget(t *testing.T) {
	f := newFieldWorld(t)
	c := meet(t, f, game.UnitCounts{game.ULightCav: 50}, false)
	if f.w.Command(f.attack.ID) != nil || f.w.IsScheduled("cmd:"+f.attack.ID) {
		t.Fatal("a destroyed army is gone and never arrives")
	}
	back := returnsFromPoint(f.w, f.a.ID)
	if len(back) != 1 || *back[0].Point != *c.Point || back[0].Units[game.ULightCav] == 0 {
		t.Fatalf("survivors' return = %+v", back)
	}
	if err := gametest.Advance(f.w, f.attack.ArrivesAt+game.MsPerHour); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.w.ReportsOf("h1") {
		if r.Type == "attack" {
			t.Fatal("the village was attacked by an army destroyed on the road")
		}
	}
	if f.a.Units[game.ULightCav] != back[0].Units[game.ULightCav] {
		t.Fatalf("light cavalry home = %d, want the survivors %d", f.a.Units[game.ULightCav], back[0].Units[game.ULightCav])
	}
}

func TestFieldScoutRevealsUnits(t *testing.T) {
	f := newFieldWorld(t)
	meet(t, f, game.UnitCounts{game.UScout: 3}, true)
	rs := fieldReports(f.w, "h1")
	if len(rs) != 1 || rs[0].Type != "fieldScout" || rs[0].Scout == nil || rs[0].Scout.Units[game.UAxe] != 100 {
		t.Fatalf("scouting report = %+v", rs)
	}
	if f.w.Command(f.attack.ID).Units[game.UAxe] != 100 || f.w.Command(f.attack.ID).FieldHit {
		t.Fatal("scouting an army does not hurt it")
	}
	if len(returnsFromPoint(f.w, f.a.ID)) != 1 {
		t.Fatal("surviving scouts go home")
	}
}

func TestFieldScoutFailsWhenOutnumbered(t *testing.T) {
	w, a := gametest.NewTestWorld(31)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewHumanPlayer(w, "h2")
	b := gametest.AddStartVillage(w, "h2", a.X+8, a.Y)
	b.Units[game.UAxe], b.Units[game.UScout], a.Units[game.UScout] = 100, 50, 3
	attack, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100, game.UScout: 50}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	f := &fieldWorld{w: w, a: a, b: b, attack: attack}
	meet(t, f, game.UnitCounts{game.UScout: 3}, true)
	rs := fieldReports(w, "h1")
	if len(rs) != 1 || rs[0].Scout != nil || rs[0].Outcome != "defenderWon" {
		t.Fatalf("failed scouting report = %+v", rs[0])
	}
	if len(returnsFromPoint(w, a.ID)) != 0 {
		t.Fatal("every scout died")
	}
}

func TestFieldLootSplit(t *testing.T) {
	keep, take := game.SplitLoot(game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}, 1500, 600)
	if keep.Wood+keep.Clay+keep.Iron > 1500 || keep.Wood < 499 {
		t.Fatalf("keeps %+v of 3000 with room for 1500", keep)
	}
	if got := take.Wood + take.Clay + take.Iron; got > 600 || got < 599 {
		t.Fatalf("takes %+v, want 600 (its capacity)", take)
	}
	if keep, take := game.SplitLoot(game.Resources{Wood: 300}, 1000, 1000); keep.Wood != 300 || take.Wood != 0 {
		t.Fatalf("an army that can carry it all keeps it all: keep %+v take %+v", keep, take)
	}
}

func TestFieldBattleTakesLootFromReturningArmy(t *testing.T) {
	f := newFieldWorld(t)
	// h2's army comes home with loot past... h1's village: met by hand, as sight comes later.
	haul := game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	ret := &game.Command{ID: f.w.NewID("c"), Type: "return", OriginID: game.VillageIDFor(f.a.X-8, f.a.Y), TargetID: f.b.ID,
		Units: game.UnitCounts{game.UAxe: 10}, Haul: &haul, DepartedAt: f.w.Now(), ArrivesAt: f.w.Now() + 16*18*game.MsPerMinute}
	game.AddCommand(f.w, ret)
	in := &game.Command{ID: f.w.NewID("c"), Type: "intercept", OriginID: f.a.ID, Units: game.UnitCounts{game.ULightCav: 50},
		Point: &game.FieldPoint{X: float64(f.a.X), Y: float64(f.a.Y)}, DepartedAt: f.w.Now(), ArrivesAt: f.w.Now() + 8*18*game.MsPerMinute,
		Intercept: &game.InterceptOrder{CommandID: ret.ID, Leg: game.Leg{OriginID: ret.OriginID, TargetID: ret.TargetID, DepartedAt: ret.DepartedAt, ArrivesAt: ret.ArrivesAt}}}
	f.a.Units[game.ULightCav] -= 50
	f.w.PutCommand(in)
	f.w.Schedule("field:"+in.ID, in.ArrivesAt)
	if err := gametest.Advance(f.w, in.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	back := returnsFromPoint(f.w, f.a.ID)
	if len(back) != 1 || back[0].Haul == nil {
		t.Fatalf("the winner carries loot home: %+v", back)
	}
	capacity := float64(back[0].Units[game.ULightCav]) * game.UnitDefs[game.ULightCav].Haul
	if got := back[0].Haul.Wood + back[0].Haul.Clay + back[0].Haul.Iron; got < 2999 && got < capacity-1 {
		t.Fatalf("captured %v, capacity %v", got, capacity)
	}
}

func TestInterceptMissAfterArrival(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Rounding put the target home a millisecond before the meeting.
	f.attack.ArrivesAt = c.ArrivesAt - 1
	c.Intercept.Leg.ArrivesAt = f.attack.ArrivesAt
	f.w.Schedule("cmd:"+f.attack.ID, f.attack.ArrivesAt)
	if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	rs := fieldReports(f.w, "h1")
	if len(rs) != 1 || !rs[0].Field.Missed {
		t.Fatalf("a late interceptor finds nothing: %+v", rs)
	}
	if back := returnsFromPoint(f.w, f.a.ID); len(back) != 1 || back[0].Units[game.ULightCav] != 20 {
		t.Fatalf("it marches home whole: %+v", back)
	}
}

func TestInterceptMissWhenTargetChanged(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.CancelCommand(f.w, "h2", f.attack.ID); err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	if rs := fieldReports(f.w, "h1"); len(rs) != 1 || !rs[0].Field.Missed {
		t.Fatalf("an army that turned back is missed: %+v", rs)
	}
	if len(fieldReports(f.w, "h2")) != 0 {
		t.Fatal("the missed army's owner hears nothing")
	}
}

func TestInterceptMissWhenRouteChangedOnTheRoad(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	// Still marching at the meeting time, but no longer on the route the interceptor was sent at.
	f.attack.Type, f.attack.OriginID, f.attack.TargetID = "return", f.attack.TargetID, f.attack.OriginID
	f.attack.ArrivesAt += game.MsPerHour
	f.w.Schedule("cmd:"+f.attack.ID, f.attack.ArrivesAt)
	if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	if rs := fieldReports(f.w, "h1"); len(rs) != 1 || !rs[0].Field.Missed {
		t.Fatalf("a changed route is missed: %+v", rs)
	}
	if f.w.Command(f.attack.ID).Units[game.UAxe] != 100 {
		t.Fatal("the missed army is untouched")
	}
}

func TestFieldReturnHomeLost(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 50}, false)
	if err != nil {
		t.Fatal(err)
	}
	gametest.NewHumanPlayer(f.w, "h3")
	game.SetOwner(f.w, f.a, "h3")
	if err := gametest.Advance(f.w, c.ArrivesAt+game.MsPerHour*3); err != nil {
		t.Fatal(err)
	}
	if f.a.Units[game.ULightCav] == 0 {
		t.Fatal("survivors come back to their village like any return, whoever holds it now")
	}
}

func TestTwoInterceptsSameMoment(t *testing.T) {
	f := newFieldWorld(t)
	f.a.Units[game.UAxe] = 20
	first, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.UAxe: 10}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.UAxe: 10}, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ArrivesAt != second.ArrivesAt {
		t.Fatalf("both meet at %d and %d", first.ArrivesAt, second.ArrivesAt)
	}
	if err := gametest.Advance(f.w, first.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	rs := fieldReports(f.w, "h1")
	if len(rs) != 2 {
		t.Fatalf("two field reports, got %d", len(rs))
	}
	afterFirst := rs[0].Defender.Units[game.UAxe] - rs[0].Defender.Losses[game.UAxe]
	if rs[1].Defender.Units[game.UAxe] != afterFirst {
		t.Fatalf("the second meets what the first left: %d, want %d", rs[1].Defender.Units[game.UAxe], afterFirst)
	}
}

func TestTurnBackAfterFieldHit(t *testing.T) {
	f := newFieldWorld(t)
	f.w.SetNow(f.w.Now() + 20*game.MsPerMinute)
	if err := game.CancelCommand(f.w, "h2", f.attack.ID); err == nil {
		t.Fatal("outside the cancel window an unhurt attack cannot turn back")
	}
	f.attack.FieldHit = true
	if err := game.CancelCommand(f.w, "h2", f.attack.ID); err != nil {
		t.Fatalf("an army hit in the field turns back at any time: %v", err)
	}
	if f.attack.Type != "return" {
		t.Fatalf("turned back: type %s", f.attack.Type)
	}
	if err := game.CancelCommand(f.w, "h2", f.attack.ID); err == nil {
		t.Fatal("a return still cannot be cancelled")
	}
}

func TestInterceptCannotBeCancelled(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := game.CancelCommand(f.w, "h1", c.ID); err == nil {
		t.Fatal("an interception marches to its meeting point; it cannot be called back")
	}
}

func TestFieldBattleDeterministic(t *testing.T) {
	run := func() string {
		f := newFieldWorld(t)
		f.a.Units[game.UAxe] = 10
		meet(t, f, game.UnitCounts{game.UAxe: 10}, false)
		b, _ := json.Marshal(fieldReports(f.w, "h1"))
		return string(b)
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("same world, same orders, different battles:\n%s\n%s", a, b)
	}
}

// A wiped-out side has nobody left to count the enemy: it must not be told what killed it, wherever the fighting
// happened. The survivor reports the battle in full.
func TestFieldBattleFogOfWar(t *testing.T) {
	t.Run("an interceptor wiped out does not learn the marching army", func(t *testing.T) {
		f := newFieldWorld(t)
		// A handful of spearmen on the road meet 100 axes. In a field report the interceptor is the attacker, and his
		// report is the one that must go blind.
		f.a.Units[game.USpear] = 10
		meet(t, f, game.UnitCounts{game.USpear: 10}, false)
		mine, theirs := fieldReports(f.w, "h1"), fieldReports(f.w, "h2")
		if len(mine) != 1 || len(theirs) != 1 {
			t.Fatalf("a field battle must reach both players: h1 %d, h2 %d", len(mine), len(theirs))
		}
		if mine[0].Attacker.Losses.Total() < mine[0].Attacker.Units.Total() {
			t.Fatalf("the interceptor was supposed to be wiped out: %d of %d",
				mine[0].Attacker.Losses.Total(), mine[0].Attacker.Units.Total())
		}
		if !mine[0].DefenderHidden {
			t.Fatal("the wiped-out interceptor still sees the marching army")
		}
		if mine[0].AttackerHidden {
			t.Fatal("the interceptor must still see his own troops")
		}
		if theirs[0].AttackerHidden || theirs[0].DefenderHidden {
			t.Fatal("the surviving army's report must stay complete")
		}
	})

	t.Run("a marching army destroyed does not learn the interceptor", func(t *testing.T) {
		f := newFieldWorld(t)
		// 50 light cavalry wipe the marching axes out entirely.
		meet(t, f, game.UnitCounts{game.ULightCav: 50}, false)
		mine, theirs := fieldReports(f.w, "h1"), fieldReports(f.w, "h2")
		if len(mine) != 1 || len(theirs) != 1 {
			t.Fatalf("a field battle must reach both players: h1 %d, h2 %d", len(mine), len(theirs))
		}
		if theirs[0].Defender.Losses.Total() < theirs[0].Defender.Units.Total() {
			t.Fatalf("the marching army was supposed to be destroyed: %d of %d",
				theirs[0].Defender.Losses.Total(), theirs[0].Defender.Units.Total())
		}
		if !theirs[0].AttackerHidden {
			t.Fatal("the destroyed army's owner still sees what killed it")
		}
		if mine[0].AttackerHidden || mine[0].DefenderHidden {
			t.Fatal("the surviving interceptor's report must stay complete")
		}
	})
}

// The same rule holds against a village wall: an attack that never came home is not a report, so the attacker must
// not read the defender's army off it. The defender, who held the walls, sees everything.
func TestVillageBattleFogOfWar(t *testing.T) {
	newVillageWorld := func(attackerAxe, defenderSpear int) (*gametest.MemWorld, *game.Command) {
		t.Helper()
		w, home := gametest.NewTestWorld(77)
		w.Player("h1").ProtectedUntil = 0
		gametest.NewHumanPlayer(w, "h2")
		den := gametest.AddStartVillage(w, "h2", home.X+2, home.Y)
		den.Units = game.Units{}
		den.Buildings[game.BWall] = 5
		den.Units[game.USpear] = defenderSpear
		home.Units[game.UAxe] = attackerAxe
		c, err := game.SendCommand(w, "h1", home.ID, den.X, den.Y, game.UnitCounts{game.UAxe: attackerAxe}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if err := gametest.Advance(w, c.ArrivesAt); err != nil {
			t.Fatal(err)
		}
		return w, c
	}
	attackReport := func(w *gametest.MemWorld, id game.PlayerID) *game.Report {
		for _, r := range w.ReportsOf(id) {
			if r.Type == "attack" {
				return r
			}
		}
		return nil
	}

	t.Run("an attacker wiped out does not learn the defender's army", func(t *testing.T) {
		w, _ := newVillageWorld(5, 300)
		mine, theirs := attackReport(w, "h1"), attackReport(w, "h2")
		if mine == nil || theirs == nil {
			t.Fatal("both sides must get the attack report")
		}
		if mine.Attacker.Losses.Total() < mine.Attacker.Units.Total() {
			t.Fatalf("the attack was supposed to be wiped out: %d of %d",
				mine.Attacker.Losses.Total(), mine.Attacker.Units.Total())
		}
		if !mine.DefenderHidden {
			t.Fatal("the wiped-out attacker still sees the defender's army")
		}
	})

	t.Run("a defender wiped out still sees the attack that broke on his wall", func(t *testing.T) {
		// The defender lost his garrison, but the wall was his and he stood behind it: he watched the attack break,
		// so he reads the whole battle even with nobody left.
		w, _ := newVillageWorld(3000, 1)
		mine, theirs := attackReport(w, "h1"), attackReport(w, "h2")
		if mine == nil || theirs == nil {
			t.Fatal("both sides must get the attack report")
		}
		if theirs.Defender.Losses.Total() < theirs.Defender.Units.Total() {
			t.Fatalf("the garrison was supposed to be wiped out: %d of %d",
				theirs.Defender.Losses.Total(), theirs.Defender.Units.Total())
		}
		if theirs.AttackerHidden || theirs.DefenderHidden {
			t.Fatal("the defender stood behind the wall and must read the whole attack")
		}
		if theirs.Attacker.Units.Total() == 0 {
			t.Fatal("the defender must see what was thrown at him")
		}
	})

	t.Run("a successful attack sees the defender's army", func(t *testing.T) {
		w, _ := newVillageWorld(2000, 1)
		mine := attackReport(w, "h1")
		if mine == nil {
			t.Fatal("no attack report")
		}
		if mine.AttackerHidden || mine.DefenderHidden {
			t.Fatal("an attacker who survived must read the whole battle")
		}
		if mine.Defender.Units.Total() == 0 {
			t.Fatal("an attacker who survived must see what he killed")
		}
	})

	t.Run("the hidden side leaves the report, not only the view", func(t *testing.T) {
		// The counts must not be readable from the player state either.
		w, _ := newVillageWorld(5, 300)
		mine := attackReport(w, "h1")
		if mine == nil || !mine.DefenderHidden {
			t.Fatal("the attack was supposed to be blinded")
		}
		if mine.Defender.Units.Total() != 0 || mine.Defender.Losses.Total() != 0 {
			t.Fatalf("the hidden side is still on the wire: %+v", mine.Defender)
		}
		if mine.Attacker.Units.Total() == 0 {
			t.Fatal("the wiped-out attacker must still see his own troops")
		}
	})

	t.Run("a wiped-out probe leaves no garrison in the attacker's intel", func(t *testing.T) {
		// Blinding the report is pointless if the map's intel still lists the garrison.
		w, c := newVillageWorld(5, 300)
		if in := w.Intel("h1", c.TargetID); in != nil && in.Intel != nil && in.Intel.Units.Total() > 0 {
			t.Fatalf("the wiped-out attack still learned the garrison: %+v", in.Intel.Units)
		}
	})

	// A defender cannot look at the attacker, so the report is the only place he can learn whether his garrison was
	// counted. It is a fact about the other side's knowledge, told only to the defending side.
	seenByAttacker := func(r *game.Report) bool {
		t.Helper()
		if r == nil || r.DefenderTroopsSeen == nil {
			t.Fatalf("the defender was not told whether his troops were seen: %+v", r)
		}
		return *r.DefenderTroopsSeen
	}

	t.Run("the defender is told his troops were seen when the attack came home", func(t *testing.T) {
		w, _ := newVillageWorld(2000, 1)
		if !seenByAttacker(attackReport(w, "h2")) {
			t.Fatal("an attack that came home must be reported as having seen the garrison")
		}
	})

	t.Run("the defender is told his troops are unknown when the attack was wiped out", func(t *testing.T) {
		w, _ := newVillageWorld(5, 300)
		if seenByAttacker(attackReport(w, "h2")) {
			t.Fatal("an attack with no survivors cannot have described the garrison")
		}
	})

	t.Run("a defender whose garrison died still learns whether it was seen", func(t *testing.T) {
		// The wall holder reads the whole attack, so his own side is no measure of what the attacker learned.
		w, _ := newVillageWorld(3000, 1)
		if !seenByAttacker(attackReport(w, "h2")) {
			t.Fatal("the attacker survived and counted the garrison")
		}
	})

	t.Run("the attacker is not told what the defender knows", func(t *testing.T) {
		// It is the defending side's fact: the attacker sees the garrison anyway, so there is nothing to tell him.
		w, _ := newVillageWorld(2000, 1)
		if mine := attackReport(w, "h1"); mine != nil && mine.DefenderTroopsSeen != nil {
			t.Fatalf("the attacker was told the defender's fact: %+v", *mine.DefenderTroopsSeen)
		}
	})

	t.Run("a supporter is told too, because his troops stood there", func(t *testing.T) {
		// A supporter's spears are part of the garrison the attacker counted, so the same answer is his business.
		sw, home := gametest.NewTestWorld(82)
		sw.Player("h1").ProtectedUntil = 0
		gametest.NewHumanPlayer(sw, "h2")
		gametest.NewHumanPlayer(sw, "h3")
		den := gametest.AddStartVillage(sw, "h2", home.X+2, home.Y)
		sup := gametest.AddStartVillage(sw, "h3", home.X+4, home.Y)
		den.Units = game.Units{}
		den.Buildings[game.BWall] = 5
		den.SupportFrom[sup.ID] = game.UnitCounts{game.USpear: 10}
		home.Units[game.UAxe] = 3000
		c, err := game.SendCommand(sw, "h1", home.ID, den.X, den.Y, game.UnitCounts{game.UAxe: 3000}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if err := gametest.Advance(sw, c.ArrivesAt); err != nil {
			t.Fatal(err)
		}
		var supported *game.Report
		for _, r := range sw.ReportsOf("h3") {
			if r.Type == "attack" {
				supported = r
			}
		}
		if supported == nil {
			t.Fatal("a supporter gets the report of the battle his troops died in")
		}
		if supported.DefenderTroopsSeen == nil || !*supported.DefenderTroopsSeen {
			t.Fatalf("the attacker walked the field and counted the support: %+v", supported.DefenderTroopsSeen)
		}
	})

	t.Run("a surviving attack does record the garrison it saw", func(t *testing.T) {
		w, c := newVillageWorld(2000, 1)
		if in := w.Intel("h1", c.TargetID); in == nil || in.Intel == nil || in.Intel.Units.Total() == 0 {
			t.Fatalf("an attack that came home must report what it met: %+v", in)
		}
	})

	t.Run("scouts marching with a surviving heavy attack do not matter either way", func(t *testing.T) {
		// Scouts are only a scouting mission when the army is nothing but scouts. Mixed into a real attack they are
		// ordinary light troops, and the garrison is revealed by the attack surviving at all — not by the scouts
		// living or dying. Two cases, so nobody "improves" the rule into keying off scouts that came home.
		for _, name := range []string{"scouts live", "scouts die"} {
			t.Run(name, func(t *testing.T) {
				w, home := gametest.NewTestWorld(81)
				w.Player("h1").ProtectedUntil = 0
				gametest.NewHumanPlayer(w, "h2")
				den := gametest.AddStartVillage(w, "h2", home.X+2, home.Y)
				den.Units = game.Units{}
				den.Units[game.USpear] = 5
				den.Units[game.UScout] = 400 // enough defending scouts to eat any scouts sent along
				home.Units[game.UAxe] = 3000
				home.Units[game.UScout] = 1
				units := game.UnitCounts{game.UAxe: 3000, game.UScout: 1}
				c, err := game.SendCommand(w, "h1", home.ID, den.X, den.Y, units, "attack", game.SendOpts{})
				if err != nil {
					t.Fatal(err)
				}
				if err := gametest.Advance(w, c.ArrivesAt); err != nil {
					t.Fatal(err)
				}
				mine := attackReport(w, "h1")
				if mine == nil {
					t.Fatal("the attacker gets a report")
				}
				if mine.Type != "attack" {
					t.Fatalf("scouts in a real army do not make a scouting mission: %q", mine.Type)
				}
				if mine.DefenderHidden || mine.AttackerHidden {
					t.Fatalf("%s: the attack survived and must read the garrison: %+v", name, mine)
				}
				if mine.Defender.Units.Total() == 0 {
					t.Fatalf("%s: a surviving attack must see what it met", name)
				}
				// The intel has to agree with the report: the map is not allowed to know more.
				if in := w.Intel("h1", c.TargetID); in == nil || in.Intel == nil || in.Intel.Units.Total() == 0 {
					t.Fatalf("%s: a surviving attack records the garrison it saw: %+v", name, in)
				}
			})
		}
	})

	t.Run("a supporter wiped out on somebody else's wall does not learn the attacker", func(t *testing.T) {
		// h3's spears stand in h2's village and die there. He is not the owner, so there is no wall of his to watch
		// from: he learns nothing of the army that killed his troops, but still reads his own losses.
		w, home := gametest.NewTestWorld(78)
		w.Player("h1").ProtectedUntil = 0
		gametest.NewHumanPlayer(w, "h2")
		gametest.NewHumanPlayer(w, "h3")
		den := gametest.AddStartVillage(w, "h2", home.X+2, home.Y)
		sup := gametest.AddStartVillage(w, "h3", home.X+4, home.Y)
		den.Units = game.Units{}
		den.Buildings[game.BWall] = 5
		den.SupportFrom[sup.ID] = game.UnitCounts{game.USpear: 10}
		home.Units[game.UAxe] = 3000
		c, err := game.SendCommand(w, "h1", home.ID, den.X, den.Y, game.UnitCounts{game.UAxe: 3000}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if err := gametest.Advance(w, c.ArrivesAt); err != nil {
			t.Fatal(err)
		}
		var supported *game.Report
		for _, r := range w.ReportsOf("h3") {
			if r.Type == "attack" {
				supported = r
			}
		}
		if supported == nil {
			t.Fatal("a supporter gets the report of the battle his troops died in")
		}
		if !supported.AttackerHidden {
			t.Fatal("the wiped-out supporter still sees what destroyed his troops")
		}
		if supported.Defender.Units.Total() == 0 {
			t.Fatal("the supporter must still see his own side")
		}
	})
}

// A scouting mission is a battle report too, so it follows the same rule, with its own way of telling whether it
// brought anything home: what matters is whether the scouts got through, not how many of them died.
func TestVillageScoutFogOfWar(t *testing.T) {
	newScoutWorld := func(scouts, defenderScouts int) (*gametest.MemWorld, *game.Command) {
		t.Helper()
		w, home := gametest.NewTestWorld(79)
		w.Player("h1").ProtectedUntil = 0
		gametest.NewHumanPlayer(w, "h2")
		den := gametest.AddStartVillage(w, "h2", home.X+2, home.Y)
		den.Units = game.Units{}
		// Each defending scout counts double, so pair up the numbers to make the outcome clear either way.
		den.Units[game.UScout] = defenderScouts
		den.Units[game.USpear] = 120
		home.Units[game.UScout] = scouts
		c, err := game.SendCommand(w, "h1", home.ID, den.X, den.Y, game.UnitCounts{game.UScout: scouts}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if err := gametest.Advance(w, c.ArrivesAt); err != nil {
			t.Fatal(err)
		}
		return w, c
	}
	scoutReport := func(w *gametest.MemWorld, id game.PlayerID) *game.Report {
		for _, r := range w.ReportsOf(id) {
			if r.Type == "scout" {
				return r
			}
		}
		return nil
	}

	t.Run("caught scouts report no garrison to their sender", func(t *testing.T) {
		// Three scouts against fifty defending scouts: nowhere near enough, so every one of them dies unlooked.
		w, _ := newScoutWorld(3, 50)
		mine := scoutReport(w, "h1")
		if mine == nil {
			t.Fatal("a scouting mission reports to its sender")
		}
		if mine.Outcome != "defenderWon" {
			t.Fatalf("the scouts were supposed to be caught: %q", mine.Outcome)
		}
		if !mine.DefenderHidden {
			t.Fatal("the sender of caught scouts was told the garrison")
		}
		if mine.Defender.Units.Total() != 0 || mine.Defender.Losses.Total() != 0 {
			t.Fatalf("the garrison is still on the wire: %+v", mine.Defender)
		}
		if mine.Scout != nil {
			t.Fatal("a caught scouting mission must not carry scout intel")
		}
		if mine.Attacker.Units.Total() != 3 || mine.Attacker.Losses.Total() != 3 {
			t.Fatalf("the sender must still see the scouts he lost: %+v", mine.Attacker)
		}
	})

	t.Run("a scouting mission that got through reports the garrison in full", func(t *testing.T) {
		w, _ := newScoutWorld(400, 5)
		mine := scoutReport(w, "h1")
		if mine == nil || mine.Outcome != "attackerWon" {
			t.Fatalf("the scouts were supposed to get through: %+v", mine)
		}
		if mine.DefenderHidden || mine.AttackerHidden {
			t.Fatal("a scout that got through must report what it saw")
		}
		if mine.Defender.Units.Total() == 0 {
			t.Fatal("the whole point of a scout is the garrison")
		}
		if mine.Scout == nil || mine.Scout.Units.Total() == 0 {
			t.Fatalf("the scout intel is missing: %+v", mine.Scout)
		}
	})

	t.Run("the village owner always knows his own garrison", func(t *testing.T) {
		// Being scouted tells the owner nothing he did not already know, and a report that hid his own troops would
		// be nonsense.
		w, _ := newScoutWorld(3, 50)
		if theirs := scoutReport(w, "h2"); theirs != nil && theirs.DefenderHidden {
			t.Fatalf("the owner was blinded to his own village: %+v", theirs)
		}
	})

	t.Run("a caught scout leaves no garrison in the attacker's intel", func(t *testing.T) {
		w, c := newScoutWorld(3, 50)
		if in := w.Intel("h1", c.TargetID); in != nil && in.Intel != nil && in.Intel.Units.Total() > 0 {
			t.Fatalf("caught scouts still filled the intel: %+v", in.Intel.Units)
		}
	})
}
