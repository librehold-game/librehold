package game_test

import (
	"errors"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// lostWhileAway: h1 holds home and a second village x, h2 joins with h2v, neither protected. h1 sends u from x at
// the village aim picks (aim may prepare x first), then h2 takes x.
func lostWhileAway(t *testing.T, w *gametest.MemWorld, home *game.Village, u game.UnitCounts, aim func(x, h2v *game.Village) *game.Village) (*game.Village, *game.Village, *game.Command) {
	t.Helper()
	x := ensureNearestBarbarian(w, home)
	game.TransferVillage(w, x, "h1", home.ID)
	h2v, err := game.JoinHuman(w, "h2", "Bartek", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	w.Player("h1").ProtectedUntil, w.Player("h2").ProtectedUntil = 0, 0
	x.Units = game.Units(u)
	to := aim(x, h2v)
	c := mustSend(t, w, x, to.X, to.Y, u, "attack")
	game.TransferVillage(w, x, "h2", h2v.ID)
	return x, h2v, c
}

func TestLooseAttackLandsForItsSender(t *testing.T) {
	w, home := gametest.NewTestWorld(51)
	var target *game.Village
	var research game.Units
	haul, capacity, farmedAt := game.Resources{Wood: 7}, 800.0, int64(-5)
	x, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 200}, func(x, _ *game.Village) *game.Village {
		x.Research[game.UAxe] = 1
		research = x.Research
		target = ensureNearestBarbarian(w, x)
		disarm(w, target.ID)
		target.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
		w.PutIntel("h1", target.ID, &game.Intel{Intel: &game.VillageIntel{LastHaul: &haul, LastHaulCapacity: &capacity}, LastFarmedAt: &farmedAt})
		return target
	})
	if c.OwnerID != "h1" || c.Research == nil || *c.Research != research || x.Research != (game.Units{}) {
		t.Fatalf("cut loose: owner %q research %v, village research %v", c.OwnerID, c.Research, x.Research)
	}
	if slices.Contains(w.CommandsHome(x.ID), c) || !slices.Equal(game.LooseArmies(w, "h1"), []*game.Command{c}) {
		t.Fatalf("filed under the village %v, under the owner %v", w.CommandsHome(x.ID), game.LooseArmies(w, "h1"))
	}
	if away := game.UnitsAway(w, x.ID); away != (game.Units{}) {
		t.Fatalf("the new owner counts the army as away: %v", away)
	}
	advance(t, w, c.ArrivesAt)
	r := lastReport(w, "h1")
	if r == nil || r.Type != "attack" || r.Outcome != "attackerWon" || r.Origin.OwnerID != "h1" || r.Origin.OwnerName != w.Player("h1").Name {
		t.Fatalf("report %+v", r)
	}
	if !r.NoHome || r.Haul != nil || r.HaulCapacity != nil {
		t.Fatalf("report haul %v capacity %v noHome %v", r.Haul, r.HaulCapacity, r.NoHome)
	}
	if target.Resources.Wood < 1000 || target.Resources.Clay < 1000 || target.Resources.Iron < 1000 {
		t.Fatalf("the target was looted: %+v", target.Resources)
	}
	if s := w.Human("h1").Stats; s.ResourcesLooted != (game.Resources{}) || s.BattlesWon != 1 {
		t.Fatalf("looted %+v won %d", s.ResourcesLooted, s.BattlesWon)
	}
	if len(w.Commands()) != 0 {
		t.Fatalf("survivors went somewhere: %+v", w.Commands())
	}
	in := w.Intel("h1", target.ID)
	if in.Intel == nil || in.Intel.At != w.Now() || in.Intel.LastHaul == nil || *in.Intel.LastHaul != haul ||
		in.Intel.LastHaulCapacity == nil || *in.Intel.LastHaulCapacity != capacity || in.LastFarmedAt == nil || *in.LastFarmedAt != farmedAt {
		t.Fatalf("intel %+v %+v", in, in.Intel)
	}
}

func TestLooseBotRaidLeavesNoHaulInMemory(t *testing.T) {
	w, bv := oneBotWorld(t, 7, 0.9)
	for _, kind := range []string{"session", "think", "sessionEnd", "phone"} {
		w.Unschedule(kind + ":bot_0")
	}
	home := game.PlayerVillages(w, "h1")[0]
	y := ensureNearestBarbarian(w, bv)
	game.TransferVillage(w, y, "bot_0", bv.ID)
	y.Units = game.Units{game.UAxe: 200}
	target := ensureNearestBarbarian(w, y)
	disarm(w, target.ID)
	target.Resources = game.Resources{Wood: 1000, Clay: 1000, Iron: 1000}
	c, err := game.SendCommand(w, "bot_0", y.ID, target.X, target.Y, game.UnitCounts{game.UAxe: 200}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	game.TransferVillage(w, y, "h1", home.ID)
	advance(t, w, c.ArrivesAt)
	k := w.Brain("bot_0").Memory.Known[target.ID]
	if k == nil || k.Units == nil || len(k.Hauls) != 0 {
		t.Fatalf("the bot remembers %+v", k)
	}
}

func TestLooseAttackFightsTheNewOwner(t *testing.T) {
	w, home := gametest.NewTestWorld(52)
	x, h2v, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 300}, func(_, h2v *game.Village) *game.Village {
		h2v.Units = game.Units{game.USpear: 5}
		return h2v
	})
	c.AutoFarm = true
	advance(t, w, c.ArrivesAt)
	if _, ok := h2v.SupportFrom[x.ID]; ok || h2v.Units[game.USpear] != 0 {
		t.Fatalf("support %v spears %d", h2v.SupportFrom, h2v.Units[game.USpear])
	}
	mine, theirs := lastReport(w, "h1"), lastReport(w, "h2")
	if mine == nil || theirs == nil || mine.Origin.OwnerID != "h1" || theirs.Origin.OwnerID != "h1" {
		t.Fatalf("reports %+v %+v", mine, theirs)
	}
	if !mine.NoHome || !theirs.NoHome || !mine.AutoFarm || theirs.AutoFarm {
		t.Fatalf("noHome %v/%v autoFarm %v/%v", mine.NoHome, theirs.NoHome, mine.AutoFarm, theirs.AutoFarm)
	}
}

func TestLooseAttackOnTheSendersVillageIsLost(t *testing.T) {
	w, home := gametest.NewTestWorld(53)
	var target *game.Village
	_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 50}, func(x, _ *game.Village) *game.Village {
		target = ensureNearestBarbarian(w, x)
		return target
	})
	game.TransferVillage(w, target, "h1", home.ID)
	units := target.Units
	if !slices.Equal(game.LooseArmies(w, "h1"), []*game.Command{c}) {
		t.Fatal("setup: the army was not cut loose")
	}
	advance(t, w, c.ArrivesAt)
	if len(target.SupportFrom) != 0 || target.Units != units || len(w.ReportsOf("h1")) != 0 || len(w.Commands()) != 0 {
		t.Fatalf("support %v units %v reports %d commands %d", target.SupportFrom, target.Units, len(w.ReportsOf("h1")), len(w.Commands()))
	}
}

func TestLooseNoblesConquerForTheSender(t *testing.T) {
	w, v, target := setupConquest(t, 54)
	v.Units[game.UNoble] = 3
	h2v, err := game.JoinHuman(w, "h2", "Bartek", 2, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	first := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 2}, "attack")
	advance(t, w, w.Now()+game.MsPerMinute)
	second := mustSend(t, w, v, target.X, target.Y, game.UnitCounts{game.UAxe: 500, game.UNoble: 1}, "attack")
	game.TransferVillage(w, v, "h2", h2v.ID)
	advance(t, w, first.ArrivesAt)
	if target.OwnerID != "h1" {
		t.Fatalf("owner %q", target.OwnerID)
	}
	r := lastReport(w, "h1")
	if r == nil || r.Type != "conquest" || r.Origin.OwnerID != "h1" {
		t.Fatalf("report %+v", r)
	}
	if all := w.ConquestsSince(0); len(all) == 0 || all[len(all)-1].ToID != "h1" || all[len(all)-1].VillageID != target.ID {
		t.Fatalf("conquests %+v", all)
	}
	if slices.ContainsFunc(w.Commands(), func(c *game.Command) bool { return c.Type == "return" }) {
		t.Fatal("the nobles' escort went home")
	}
	reports := len(w.ReportsOf("h1"))
	advance(t, w, second.ArrivesAt)
	if len(target.SupportFrom) != 0 || len(w.ReportsOf("h1")) != reports || len(w.Commands()) != 0 {
		t.Fatalf("the second attack: support %v reports %d commands %d", target.SupportFrom, len(w.ReportsOf("h1")), len(w.Commands()))
	}
}

func TestLooseArmyNeverTurnsBack(t *testing.T) {
	w, home := gametest.NewTestWorld(55)
	x, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 50}, func(x, _ *game.Village) *game.Village {
		return ensureNearestBarbarian(w, x)
	})
	wantRule(t, game.CancelCommand(w, "h1", c.ID), "cannotCancel")
	var be *game.BlockedError
	if err := game.CancelCommand(w, "h2", c.ID); !errors.As(err, &be) || !be.HasKind("notOwner") {
		t.Fatalf("the new owner recalls: %v", err)
	}
	c.FieldHit = true
	if game.CanCancelCommand(w, c) {
		t.Fatal("a loose army hit in the field may turn back")
	}
	w.Schedule("schedCancel:"+c.ID, w.Now()+1)
	advance(t, w, w.Now()+1)
	if c.Type != "attack" || !w.IsScheduled("cmd:"+c.ID) {
		t.Fatalf("a planned recall turned it back: %+v", c)
	}
	game.TransferVillage(w, x, "h1", home.ID)
	if c.OwnerID != "h1" || slices.Contains(w.CommandsHome(x.ID), c) {
		t.Fatalf("retaking the village took the army back: owner %q", c.OwnerID)
	}
	wantRule(t, game.CancelCommand(w, "h1", c.ID), "cannotCancel")
}

func TestLooseArmiesOfALeaverVanish(t *testing.T) {
	barbarian := func(w *gametest.MemWorld) func(x, _ *game.Village) *game.Village {
		return func(x, _ *game.Village) *game.Village { return ensureNearestBarbarian(w, x) }
	}
	t.Run("a kicked player's", func(t *testing.T) {
		w, home := gametest.NewTestWorld(56)
		_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 50}, barbarian(w))
		game.KickPlayer(w, "h1")
		if len(game.LooseArmies(w, "h1")) != 0 || w.Command(c.ID) != nil || w.IsScheduled("cmd:"+c.ID) {
			t.Fatal("the army marches on")
		}
	})
	t.Run("a player who left, once their villages are abandoned", func(t *testing.T) {
		w, home := gametest.NewTestWorld(57)
		_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 50}, barbarian(w))
		game.QuitPlayer(w, "h1")
		at, ok := w.ScheduledAt("abandon:h1")
		if !ok {
			t.Fatal("setup: no abandon")
		}
		c.ArrivesAt = at + game.MsPerHour
		w.Schedule("cmd:"+c.ID, c.ArrivesAt)
		advance(t, w, at)
		if w.Command(c.ID) != nil || w.IsScheduled("cmd:"+c.ID) {
			t.Fatal("the army marches on")
		}
	})
	t.Run("an eliminated bot's", func(t *testing.T) {
		w, home := gametest.NewTestWorld(58)
		bv := gametest.MakeBotVillage(w, home)
		bv.Units[game.UAxe] = 50
		bx, by := gametest.NearestBarbarianXY(w, home)
		if _, err := game.SendCommand(w, "bot_test", bv.ID, bx, by, game.UnitCounts{game.UAxe: 50}, "attack", game.SendOpts{}); err != nil {
			t.Fatal(err)
		}
		game.TransferVillage(w, bv, "h1", home.ID)
		if w.Player("bot_test").Status != game.StatusEliminated || len(w.Commands()) != 0 {
			t.Fatalf("status %s commands %d", w.Player("bot_test").Status, len(w.Commands()))
		}
	})
	t.Run("a bot's, once its last village is taken", func(t *testing.T) {
		w, home := gametest.NewTestWorld(59)
		bv, other := gametest.MakeBotVillage(w, home), gametest.MakeBotVillage(w, home)
		bv.Units[game.UAxe] = 50
		bx, by := gametest.NearestBarbarianXY(w, home)
		c, err := game.SendCommand(w, "bot_test", bv.ID, bx, by, game.UnitCounts{game.UAxe: 50}, "attack", game.SendOpts{})
		if err != nil {
			t.Fatal(err)
		}
		game.TransferVillage(w, bv, "h1", home.ID)
		if !slices.Equal(game.LooseArmies(w, "bot_test"), []*game.Command{c}) {
			t.Fatal("the bot still plays, and its army marches on")
		}
		game.TransferVillage(w, other, "h1", home.ID)
		if len(game.LooseArmies(w, "bot_test")) != 0 || w.Command(c.ID) != nil {
			t.Fatal("an eliminated bot's army marches on")
		}
	})
	t.Run("not a human's who starts again", func(t *testing.T) {
		f := newFieldWorld(t)
		game.TransferVillage(f.w, f.b, "h1", f.a.ID)
		if p := f.w.Player("h2"); p.Status != game.StatusActive || len(p.VillageIDs) != 1 || p.VillageIDs[0] == f.b.ID {
			t.Fatalf("h2 did not start again: %+v", p)
		}
		advance(t, f.w, f.attack.ArrivesAt)
		r := lastReport(f.w, "h2")
		if r == nil || r.OriginID != f.b.ID || r.Origin.OwnerID != "h2" || r.TargetID != f.a.ID {
			t.Fatalf("report %+v", r)
		}
	})
}

// interceptLoadedReturn: h1 meets h2's return carrying loot past a, by hand as sight comes later.
func interceptLoadedReturn(f *fieldWorld) *game.Command {
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
	return in
}

func TestLooseInterceptorTakesNothing(t *testing.T) {
	f := newFieldWorld(t)
	in := interceptLoadedReturn(f)
	game.TransferVillage(f.w, f.a, "h2", f.b.ID)
	if in.OwnerID != "h1" || in.Research == nil {
		t.Fatalf("intercept owner %q research %v", in.OwnerID, in.Research)
	}
	if err := gametest.Advance(f.w, in.ArrivesAt); err != nil {
		t.Fatal(err)
	}
	if back := returnsFromPoint(f.w, f.a.ID); len(back) != 0 {
		t.Fatalf("the interceptor went home: %+v", back)
	}
	mine, theirs := fieldReports(f.w, "h1"), fieldReports(f.w, "h2")
	if len(mine) != 1 || mine[0].Origin.OwnerID != "h1" || !mine[0].NoHome || mine[0].Haul != nil || mine[0].Outcome != "attackerWon" {
		t.Fatalf("h1's field reports %+v", mine)
	}
	if len(theirs) != 1 || !theirs[0].NoHome {
		t.Fatalf("h2's field reports %+v", theirs)
	}

	t.Run("one that misses marches nowhere", func(t *testing.T) {
		f := newFieldWorld(t)
		c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
		if err != nil {
			t.Fatal(err)
		}
		game.TransferVillage(f.w, f.a, "h2", f.b.ID)
		f.w.DeleteCommand(f.attack.ID)
		if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
			t.Fatal(err)
		}
		if rs := fieldReports(f.w, "h1"); len(rs) != 1 || !rs[0].Field.Missed || rs[0].Origin.OwnerID != "h1" {
			t.Fatalf("h1's field reports %+v", rs)
		}
		if back := returnsFromPoint(f.w, f.a.ID); len(back) != 0 {
			t.Fatalf("the interceptor went home: %+v", back)
		}
	})
}

func TestLooseArmyMetOnTheRoad(t *testing.T) {
	f := newFieldWorld(t)
	game.TransferVillage(f.w, f.b, "h1", f.a.ID)
	h2 := f.w.Player("h2")
	f.a.Units[game.UAxe] = 10
	if kinds := blockerKinds(game.CheckIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.UAxe: 10}, false).Blockers); slices.Contains(kinds, "notInterceptable") ||
		slices.Contains(kinds, "notVisible") || slices.Contains(kinds, "targetProtected") {
		t.Fatalf("h1 cannot meet the army: %v", kinds)
	}
	if kinds := blockerKinds(game.CheckIntercept(f.w, "h2", h2.VillageIDs[0], f.attack.ID, game.UnitCounts{game.UAxe: 10}, false).Blockers); !slices.Contains(kinds, "notInterceptable") {
		t.Fatalf("h2 meets its own army: %v", kinds)
	}
	meet(t, f, game.UnitCounts{game.UAxe: 10}, false)
	for _, p := range []game.PlayerID{"h1", "h2"} {
		rs := fieldReports(f.w, p)
		if len(rs) != 1 || rs[0].Target.OwnerID != "h2" || rs[0].TargetID != f.b.ID || rs[0].Origin.OwnerID != "h1" {
			t.Fatalf("%s field reports %+v", p, rs)
		}
	}
	target := f.w.Command(f.attack.ID)
	if target == nil || !target.FieldHit || game.CanCancelCommand(f.w, target) {
		t.Fatalf("the hit army %+v", target)
	}
}

func TestBotSeesALooseAttackFromItsOwnVillage(t *testing.T) {
	w, bv, c := botUnderAttack(t, 10, 0.9)
	home := game.PlayerVillages(w, "h1")[0]
	game.TransferVillage(w, home, "bot_0", bv.ID)
	game.PlanFieldBattles(w, "bot_0")
	var aimed []*game.Command
	for _, in := range w.Commands() {
		if in.Type == "intercept" && in.Intercept.CommandID == c.ID {
			aimed = append(aimed, in)
		}
	}
	if c.OwnerID != "h1" || len(aimed) != 1 {
		t.Fatalf("owner %q intercepts %+v", c.OwnerID, aimed)
	}
}

func TestRespawnProtectionLeavesLooseArmiesOpen(t *testing.T) {
	f := newFieldWorld(t)
	game.TransferVillage(f.w, f.b, "h1", f.a.ID)
	if !game.IsProtected(f.w, "h2") {
		t.Fatal("setup: h2 starts again unprotected")
	}
	if kinds := blockerKinds(game.CheckIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false).Blockers); len(kinds) != 0 {
		t.Fatalf("an army sent before the fresh start is protected: %v", kinds)
	}
}

func TestLooseHetmansCountTowardTheCoinPrice(t *testing.T) {
	w, home := gametest.NewTestWorld(61)
	_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 50, game.UNoble: 1}, func(x, _ *game.Village) *game.Village {
		return ensureNearestBarbarian(w, x)
	})
	if c.OwnerID != "h1" || game.TotalNoblesOwned(w, "h1") != 1 || game.TotalNoblesOwned(w, "h2") != 0 {
		t.Fatalf("nobles owned: h1 %d, h2 %d", game.TotalNoblesOwned(w, "h1"), game.TotalNoblesOwned(w, "h2"))
	}
}

func TestLooseArmyFightsWithTheResearchItLeftWith(t *testing.T) {
	t.Run("an attack", func(t *testing.T) {
		lost := func(level int) int {
			w, home := gametest.NewTestWorld(62)
			_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UAxe: 100}, func(x, _ *game.Village) *game.Village {
				x.Research[game.UAxe] = level
				target := ensureNearestBarbarian(w, x)
				disarm(w, target.ID)
				target.Units = game.Units{game.USpear: 150}
				return target
			})
			advance(t, w, c.ArrivesAt)
			r := lastReport(w, "h1")
			if r == nil || r.Outcome != "attackerWon" {
				t.Fatalf("report %+v", r)
			}
			return r.Attacker.Losses[game.UAxe]
		}
		if with, without := lost(3), lost(0); with >= without {
			t.Fatalf("axes lost with research %d, without %d", with, without)
		}
	})
	t.Run("an interception", func(t *testing.T) {
		lost := func(level int) int {
			f := newFieldWorld(t)
			f.a.Research[game.ULightCav] = level
			c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 10}, false)
			if err != nil {
				t.Fatal(err)
			}
			game.TransferVillage(f.w, f.a, "h2", f.b.ID)
			if err := gametest.Advance(f.w, c.ArrivesAt); err != nil {
				t.Fatal(err)
			}
			rs := fieldReports(f.w, "h1")
			if len(rs) != 1 || rs[0].Outcome != "attackerWon" {
				t.Fatalf("field reports %+v", rs)
			}
			return rs[0].Attacker.Losses[game.ULightCav]
		}
		if with, without := lost(3), lost(0); with >= without {
			t.Fatalf("cavalry lost with research %d, without %d", with, without)
		}
	})
}

func TestBystanderSeesALooseArmyPass(t *testing.T) {
	s := newSightWorld(t, 5, 3)
	game.TransferVillage(s.w, s.b, "h1", s.a.ID)
	win := windowFor(game.SightWindows(s.w, s.attack), "h3")
	if win == nil {
		t.Fatal("no sight window for the bystander")
	}
	at := (win.From + win.To) / 2
	s.w.SetNow(at)
	if !slices.Contains(game.Sighted(s.w, "h3", at), s.attack) || !game.CanSeeCommand(s.w, "h3", s.attack) {
		t.Fatal("the bystander does not see the army pass")
	}
}

func TestLooseScoutsAreLost(t *testing.T) {
	w, home := gametest.NewTestWorld(63)
	_, _, c := lostWhileAway(t, w, home, game.UnitCounts{game.UScout: 10}, func(x, _ *game.Village) *game.Village {
		target := ensureNearestBarbarian(w, x)
		disarm(w, target.ID)
		return target
	})
	advance(t, w, c.ArrivesAt)
	if r := lastReport(w, "h1"); r == nil || r.Type != "scout" || r.Outcome != "attackerWon" || len(w.Commands()) != 0 {
		t.Fatalf("report %+v commands %d", r, len(w.Commands()))
	}
}

func TestBotDefendsAgainstALooseAttackFromItsOwnVillage(t *testing.T) {
	w, home, bv := defenceWorld(t)
	bv.Units[game.USpear] = 100
	def := addBotVillage(w, bv, 0, 2, game.Units{game.USpear: 3000})
	w.Brain("bot_0").Roles[def.ID] = game.RoleDef
	home.Units[game.UAxe] = 20
	c := mustSend(t, w, home, bv.X, bv.Y, game.UnitCounts{game.UAxe: 20}, "attack")
	game.TransferVillage(w, home, "bot_0", bv.ID)
	game.PlanDefence(w, "bot_0")
	if c.OwnerID != "h1" || !slices.ContainsFunc(w.Commands(), func(x *game.Command) bool {
		return x.Type == "support" && x.OriginID == def.ID && x.TargetID == bv.ID
	}) {
		t.Fatal("the bot does not stack against an attack from a village it took")
	}
}

func TestBotSeesALooseHetmanFromItsOwnVillage(t *testing.T) {
	w, bv := oneBotWorld(t, 7, 0.9)
	home := game.PlayerVillages(w, "h1")[0]
	home.Units[game.UAxe], home.Units[game.UNoble] = 100, 1
	target := ensureNearestBarbarian(w, home)
	mustSend(t, w, home, target.X, target.Y, game.UnitCounts{game.UAxe: 100, game.UNoble: 1}, "attack")
	game.TransferVillage(w, home, "bot_0", bv.ID)
	if !game.NobleClaimer(w, "bot_0")(target.X, target.Y) {
		t.Fatal("the bot takes another player's hetman from a village it took for its own")
	}
}
