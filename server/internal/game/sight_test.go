package game_test

import (
	"math"
	"slices"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// sightWorld: h2 at b attacks h1 at a, 10 tiles east and 5 south; h3 watches from c next to the route.
type sightWorld struct {
	w       *gametest.MemWorld
	a, b, c *game.Village
	attack  *game.Command
}

func newSightWorld(t *testing.T, cx, cy int) *sightWorld {
	t.Helper()
	w, a := gametest.NewTestWorld(41)
	w.Player("h1").ProtectedUntil = 0
	gametest.NewHumanPlayer(w, "h2")
	gametest.NewHumanPlayer(w, "h3")
	b := gametest.AddStartVillage(w, "h2", a.X+10, a.Y+5)
	c := gametest.AddStartVillage(w, "h3", a.X+cx, a.Y+cy)
	b.Units[game.UAxe] = 100
	c.Units[game.ULightCav] = 50
	attack, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return &sightWorld{w: w, a: a, b: b, c: c, attack: attack}
}

func distanceAt(c *game.Command, v *game.Village, t int64) float64 {
	p, _ := game.PositionAt(c, t)
	return math.Hypot(p.X-float64(v.X), p.Y-float64(v.Y))
}

func windowFor(ws []game.SightWindow, p game.PlayerID) *game.SightWindow {
	for i := range ws {
		if ws[i].Player == p {
			return &ws[i]
		}
	}
	return nil
}

func TestSightWindowOneTile(t *testing.T) {
	// (5,3) is under half a tile from the line (10,5)→(0,0).
	s := newSightWorld(t, 5, 3)
	ws := game.SightWindows(s.w, s.attack)
	win := windowFor(ws, "h3")
	if win == nil || win.From >= win.To {
		t.Fatalf("windows %+v", ws)
	}
	if d := distanceAt(s.attack, s.c, (win.From+win.To)/2); d > 1 {
		t.Fatalf("mid-window the army is %.2f tiles away", d)
	}
	if d := distanceAt(s.attack, s.c, win.From-game.MsPerMinute); d <= 1 {
		t.Fatalf("a minute before the window it is already %.2f tiles away", d)
	}
	if d := distanceAt(s.attack, s.c, win.To+game.MsPerMinute); d <= 1 {
		t.Fatalf("a minute after the window it is still %.2f tiles away", d)
	}
	if windowFor(ws, "h1") != nil || windowFor(ws, "h2") != nil {
		t.Fatal("the army's owner and its target's owner need no window")
	}
	if !s.w.IsScheduled("sight:"+s.attack.ID+":h3:in") || !s.w.IsScheduled("sight:"+s.attack.ID+":h3:out") {
		t.Fatal("entering and leaving sight are events")
	}

	far := newSightWorld(t, 5, 6)
	if ws := game.SightWindows(far.w, far.attack); windowFor(ws, "h3") != nil {
		t.Fatalf("three tiles from the route nobody sees it: %+v", ws)
	}

	wide := newSightWorld(t, 5, 3)
	wide.w.Config().SightRadius = 5
	ww := windowFor(game.SightWindows(wide.w, wide.attack), "h3")
	if ww == nil || ww.From >= win.From || ww.To <= win.To {
		t.Fatalf("a wider radius sees longer: %+v vs %+v", ww, win)
	}
}

func TestBystanderSightFollowsTheArmy(t *testing.T) {
	s := newSightWorld(t, 5, 3)
	win := windowFor(game.SightWindows(s.w, s.attack), "h3")
	seen := func(at int64) bool {
		s.w.SetNow(at)
		return slices.ContainsFunc(game.Sighted(s.w, "h3", at), func(c *game.Command) bool { return c.ID == s.attack.ID })
	}
	if seen(win.From-1000) || !seen((win.From+win.To)/2) || seen(win.To+1000) {
		t.Fatal("a passing army is seen only while it is near")
	}
	if len(game.Sighted(s.w, "h1", (win.From+win.To)/2)) != 0 {
		t.Fatal("the target's owner sees it as incoming, not as passing")
	}
}

func TestBystanderCanIntercept(t *testing.T) {
	s := newSightWorld(t, 5, 3)
	win := windowFor(game.SightWindows(s.w, s.attack), "h3")
	s.w.SetNow((win.From + win.To) / 2)
	if _, err := game.SendIntercept(s.w, "h3", s.c.ID, s.attack.ID, game.UnitCounts{game.ULightCav: 20}, false); err != nil {
		t.Fatalf("a bystander meets a passing army: %v", err)
	}
}

func TestBystanderCannotInterceptAfterWindow(t *testing.T) {
	s := newSightWorld(t, 5, 3)
	win := windowFor(game.SightWindows(s.w, s.attack), "h3")
	s.w.SetNow(win.To + game.MsPerMinute)
	got := blockerKinds(game.CheckIntercept(s.w, "h3", s.c.ID, s.attack.ID, game.UnitCounts{game.ULightCav: 20}, false).Blockers)
	if !slices.Contains(got, "notVisible") {
		t.Fatalf("blockers %v, want notVisible", got)
	}
}

func TestSightEventsNotifyBystander(t *testing.T) {
	s := newSightWorld(t, 5, 3)
	win := windowFor(game.SightWindows(s.w, s.attack), "h3")
	s.w.TakeNotified()
	if err := gametest.Advance(s.w, win.From); err != nil {
		t.Fatal(err)
	}
	if got := s.w.TakeNotified(); !slices.Contains(got, game.PlayerID("h3")) {
		t.Fatalf("notified %v, want h3", got)
	}
}

func TestNoSightWithoutFieldBattles(t *testing.T) {
	w, a := gametest.NewTestWorld(41)
	w.Config().NoFieldBattles = true
	gametest.NewHumanPlayer(w, "h2")
	gametest.NewHumanPlayer(w, "h3")
	b := gametest.AddStartVillage(w, "h2", a.X+10, a.Y+5)
	gametest.AddStartVillage(w, "h3", a.X+5, a.Y+3)
	b.Units[game.UAxe] = 100
	w.Player("h1").ProtectedUntil = 0
	c, err := game.SendCommand(w, "h2", b.ID, a.X, a.Y, game.UnitCounts{game.UAxe: 100}, "attack", game.SendOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if w.IsScheduled("sight:"+c.ID+":h3:in") || len(game.Sighted(w, "h3", c.ArrivesAt/2)) != 0 {
		t.Fatal("a world without field battles has no bystanders")
	}
}
