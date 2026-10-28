package game_test

import (
	"math"
	"testing"

	"librehold/internal/game"
)

const spearMin, lightCavMin = 18.0, 10.0

func TestMeetingHeadOn(t *testing.T) {
	from, to, home := game.FieldPoint{X: 10}, game.FieldPoint{}, game.FieldPoint{}
	arrives := int64(10 * spearMin * float64(game.MsPerMinute))
	at, p, ok := game.MeetingTime(from, to, 0, arrives, home, lightCavMin, 1, 0)
	if !ok {
		t.Fatal("head-on armies should meet")
	}
	want := 10 / (1 + lightCavMin/spearMin)
	if math.Abs(p.X-want) > 0.01 || p.Y != 0 {
		t.Fatalf("meet at %+v, want x≈%.3f", p, want)
	}
	if d := at - game.TravelTimeMs(p.X, lightCavMin, 1); d < -1 || d > 1 {
		t.Fatalf("meeting time %d is not the travel time to the point (%d)", at, game.TravelTimeMs(p.X, lightCavMin, 1))
	}
}

func TestMeetingChaseImpossible(t *testing.T) {
	// The target walks away from home at spear speed; rams are slower still.
	from, to, home := game.FieldPoint{X: 2}, game.FieldPoint{X: 40}, game.FieldPoint{}
	arrives := int64(38 * spearMin * float64(game.MsPerMinute))
	if _, _, ok := game.MeetingTime(from, to, 0, arrives, home, 30, 1, 0); ok {
		t.Fatal("a slower chaser never catches up")
	}
}

func TestMeetingTooLate(t *testing.T) {
	from, to, home := game.FieldPoint{X: 0}, game.FieldPoint{X: 10}, game.FieldPoint{X: 9, Y: 20}
	arrives := int64(10 * spearMin * float64(game.MsPerMinute))
	now := arrives - game.MsPerMinute
	if _, _, ok := game.MeetingTime(from, to, 0, arrives, home, lightCavMin, 1, now); ok {
		t.Fatal("a target a minute from home cannot be reached from 20 tiles away")
	}
}

func TestMeetingLateTargetStillReachable(t *testing.T) {
	from, to, home := game.FieldPoint{X: 0}, game.FieldPoint{X: 10}, game.FieldPoint{X: 10, Y: 1}
	arrives := int64(10 * spearMin * float64(game.MsPerMinute))
	at, _, ok := game.MeetingTime(from, to, 0, arrives, home, lightCavMin, 1, arrives/2)
	if !ok || at >= arrives || at < arrives/2 {
		t.Fatalf("meet at %d ok=%v, want within (%d, %d)", at, ok, arrives/2, arrives)
	}
}

func TestPositionOfReturnFromPoint(t *testing.T) {
	c := &game.Command{Type: "return", TargetID: game.VillageIDFor(10, 0), Point: &game.FieldPoint{X: 4, Y: 0}, DepartedAt: 1000, ArrivesAt: 7000}
	p, ok := game.PositionAt(c, 4000)
	if !ok || math.Abs(p.X-7) > 1e-9 || p.Y != 0 {
		t.Fatalf("halfway from (4,0) to (10,0) = %+v ok=%v, want (7,0)", p, ok)
	}
	if p, _ := game.PositionAt(c, 0); p.X != 4 {
		t.Fatalf("before departure the army stands at its start, got %+v", p)
	}
	if p, _ := game.PositionAt(c, 9000); p.X != 10 {
		t.Fatalf("after arrival the army stands at its target, got %+v", p)
	}
}

func TestLegEndsOfIntercept(t *testing.T) {
	c := &game.Command{Type: "intercept", OriginID: game.VillageIDFor(3, 4), Point: &game.FieldPoint{X: 5.5, Y: 6}}
	from, to, ok := game.LegEnds(c)
	if !ok || from != (game.FieldPoint{X: 3, Y: 4}) || to != (game.FieldPoint{X: 5.5, Y: 6}) {
		t.Fatalf("intercept leg = %+v → %+v ok=%v", from, to, ok)
	}
	if _, _, ok := game.LegEnds(&game.Command{OriginID: "nowhere", TargetID: game.VillageIDFor(1, 1)}); ok {
		t.Fatal("an unparseable end has no leg")
	}
}
