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
