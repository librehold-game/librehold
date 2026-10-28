package game_test

import (
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// fieldWorld: humans h1 (village a) and h2 (village b, 8 tiles east), both unprotected, b attacking a with
// 100 axes; a holds 50 light cavalry and 10 scouts.
type fieldWorld struct {
	w      *gametest.MemWorld
	a, b   *game.Village
	attack *game.Command
}

func newFieldWorld(t *testing.T) *fieldWorld {
	t.Helper()
	w, a := gametest.NewTestWorld(31)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewHumanPlayer(w, "h2")
	b := gametest.AddStartVillage(w, "h2", a.X+8, a.Y)
	a.Units[game.ULightCav], a.Units[game.UScout] = 50, 10
	b.Units[game.UAxe] = 100
	attack, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return &fieldWorld{w: w, a: a, b: b, attack: attack}
}

func TestInterceptIncomingAttackSchedulesFieldEvent(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 20}, false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Type != "intercept" || c.Intercept == nil || c.Intercept.CommandID != f.attack.ID || c.Point == nil {
		t.Fatalf("intercept command = %+v", c)
	}
	if want := (game.Leg{OriginID: f.b.ID, TargetID: f.a.ID, DepartedAt: f.attack.DepartedAt, ArrivesAt: f.attack.ArrivesAt}); c.Intercept.Leg != want {
		t.Fatalf("leg = %+v, want %+v", c.Intercept.Leg, want)
	}
	if !f.w.IsScheduled("field:"+c.ID) || f.w.IsScheduled("cmd:"+c.ID) {
		t.Fatal("an intercept resolves at its field event, not as a command arrival")
	}
	if at, _ := f.w.ScheduledAt("field:" + c.ID); at != c.ArrivesAt || at >= f.attack.ArrivesAt {
		t.Fatalf("field event at %d, arrives %d, target arrives %d", at, c.ArrivesAt, f.attack.ArrivesAt)
	}
	if !slices.Contains(f.attack.Threats, c.ID) {
		t.Fatalf("target threats = %v", f.attack.Threats)
	}
	if f.a.Units[game.ULightCav] != 30 {
		t.Fatalf("light cavalry left home = %d, want 30", f.a.Units[game.ULightCav])
	}
	if f.w.Human("h1").Stats.AttacksSent != 1 {
		t.Fatal("an interception counts as an attack sent")
	}
}

func TestInterceptBlockers(t *testing.T) {
	lc := game.UnitCounts{game.ULightCav: 5}
	cases := []struct {
		name  string
		setup func(f *fieldWorld) (origin, command string, u game.UnitCounts, scout bool)
		want  string
	}{
		{"fieldBattlesOff", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			f.w.Config().NoFieldBattles = true
			return f.a.ID, f.attack.ID, lc, false
		}, "fieldBattlesOff"},
		{"notOwner", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			return f.b.ID, f.attack.ID, lc, false
		}, "notOwner"},
		{"noUnits", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			return f.a.ID, f.attack.ID, game.UnitCounts{}, false
		}, "noUnits"},
		{"notEnough", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			return f.a.ID, f.attack.ID, game.UnitCounts{game.ULightCav: 500}, false
		}, "notEnough"},
		{"missing command", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			return f.a.ID, "nope", lc, false
		}, "notInterceptable"},
		{"own command", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			bx, by := gametest.NearestBarbarianXY(f.w, f.a)
			own, err := game.SendCommand(f.w, "h1", f.a.ID, bx, by, game.UnitCounts{game.ULightCav: 1}, "attack", game.SendOpts{})
			if err != nil {
				panic(err)
			}
			return f.a.ID, own.ID, lc, false
		}, "notInterceptable"},
		{"trade", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			tr := &game.Command{ID: f.w.NewID("c"), Type: "trade", OriginID: f.b.ID, TargetID: f.a.ID, Merchants: 1,
				DepartedAt: f.w.Now(), ArrivesAt: f.w.Now() + game.MsPerHour}
			game.AddCommand(f.w, tr)
			return f.a.ID, tr.ID, lc, false
		}, "notInterceptable"},
		{"an intercept", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			f.b.Units[game.ULightCav] = 10
			// h1 attacks h2, h2 sends cavalry to meet it, then h1 tries to meet that interceptor.
			own, err := game.SendCommand(f.w, "h1", f.a.ID, f.b.X, f.b.Y, game.UnitCounts{game.ULightCav: 1}, "attack", game.SendOpts{})
			if err != nil {
				panic(err)
			}
			in, err := game.SendIntercept(f.w, "h2", f.b.ID, own.ID, game.UnitCounts{game.ULightCav: 5}, false)
			if err != nil {
				panic(err)
			}
			return f.a.ID, in.ID, lc, false
		}, "notInterceptable"},
		{"not visible", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			gametest.NewHumanPlayer(f.w, "h3")
			far := gametest.AddStartVillage(f.w, "h3", f.b.X+20, f.b.Y+20)
			f.b.Units[game.UAxe] = 10
			c, err := game.SendCommand(f.w, "h2", f.b.ID, far.X, far.Y, game.UnitCounts{game.UAxe: 10}, "attack", game.SendOpts{})
			if err != nil {
				panic(err)
			}
			return f.a.ID, c.ID, lc, false
		}, "notVisible"},
		{"self protected", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			f.w.Player("h1").ProtectedUntil = f.w.Now() + game.MsPerHour
			return f.a.ID, f.attack.ID, lc, false
		}, "selfProtected"},
		{"target protected", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			f.w.Player("h2").ProtectedUntil = f.w.Now() + game.MsPerHour
			return f.a.ID, f.attack.ID, lc, false
		}, "targetProtected"},
		{"scouts only", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			return f.a.ID, f.attack.ID, game.UnitCounts{game.UScout: 2, game.ULightCav: 1}, true
		}, "scoutsOnly"},
		{"too late", func(f *fieldWorld) (string, string, game.UnitCounts, bool) {
			far := gametest.AddStartVillage(f.w, "h1", f.a.X, f.a.Y+15)
			far.Units[game.ULightCav] = 5
			f.w.SetNow(f.attack.ArrivesAt - 60_000)
			return far.ID, f.attack.ID, lc, false
		}, "tooLate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFieldWorld(t)
			origin, cmd, u, scout := tc.setup(f)
			got := blockerKinds(game.CheckIntercept(f.w, "h1", origin, cmd, u, scout).Blockers)
			if !slices.Contains(got, tc.want) {
				t.Fatalf("blockers %v, want %s", got, tc.want)
			}
			if _, err := game.SendIntercept(f.w, "h1", origin, cmd, u, scout); err == nil {
				t.Fatal("a blocked interception must not be sent")
			}
		})
	}
}

func TestInterceptScoutsOnlySendsScouts(t *testing.T) {
	f := newFieldWorld(t)
	c, err := game.SendIntercept(f.w, "h1", f.a.ID, f.attack.ID, game.UnitCounts{game.UScout: 3}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Intercept.Scout || f.w.Human("h1").Stats.ScoutsSent != 1 {
		t.Fatalf("scouting interception = %+v, scouts sent %d", c.Intercept, f.w.Human("h1").Stats.ScoutsSent)
	}
}
