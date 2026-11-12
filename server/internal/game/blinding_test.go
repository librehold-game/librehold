package game

import "testing"

// The fog-of-war rule, tested on its own so the awkward combinations do not depend on finding a battle that
// happens to produce them.

func reportWith(attacker, defender ReportSide) Report {
	return Report{Type: "attack", Origin: ReportVillage{OwnerID: "h1"}, Target: ReportVillage{OwnerID: "h2"},
		Attacker: attacker, Defender: defender}
}

func TestBlindedForKeepsTheReadersOwnSide(t *testing.T) {
	// Both armies gone. Each copy is blinded from scratch, so a reader always keeps his own side whatever the other
	// one did.
	gone := ReportSide{Units: UnitCounts{UAxe: 5}, Losses: UnitCounts{UAxe: 5}}
	r := reportWith(gone, gone)

	asAttacker := blindedFor(r, "h1", nil)
	if asAttacker.Attacker.Units.Total() == 0 {
		t.Fatal("the attacker lost sight of his own troops")
	}
	if asAttacker.Defender.Units.Total() != 0 || !asAttacker.DefenderHidden {
		t.Fatalf("the wiped-out attacker still sees the enemy: %+v", asAttacker.Defender)
	}

	// The village owner stands behind his wall, so the wall exception leaves him the whole report.
	asOwner := blindedFor(r, "h2", map[PlayerID]bool{"h2": true})
	if asOwner.Defender.Units.Total() == 0 || asOwner.Attacker.Units.Total() == 0 {
		t.Fatalf("the owner of the village read an incomplete report: %+v", asOwner)
	}
}

func TestBlindedForSupporter(t *testing.T) {
	// A supporter's troops died on somebody else's wall: he keeps his own side's numbers and loses the attacker's.
	r := reportWith(ReportSide{Units: UnitCounts{UAxe: 100}}, ReportSide{Units: UnitCounts{USpear: 10}, Losses: UnitCounts{USpear: 10}})
	defenders := map[PlayerID]bool{"h2": true, "h3": true}

	supporter := blindedFor(r, "h3", defenders)
	if supporter.Attacker.Units.Total() != 0 || !supporter.AttackerHidden {
		t.Fatalf("the supporter learned the attacker's army: %+v", supporter.Attacker)
	}
	if supporter.Defender.Units.Total() == 0 {
		t.Fatal("the supporter lost sight of his own side")
	}
	// The wall is the owner's alone: the same battle read by the owner is complete.
	owner := blindedFor(r, "h2", defenders)
	if owner.AttackerHidden || owner.DefenderHidden {
		t.Fatalf("the village owner was blinded: %+v", owner)
	}
}

func TestBlindedForClearsTheWire(t *testing.T) {
	// The hidden side must leave the stored report, not merely be flagged: otherwise it is still readable from the
	// player state.
	gone := ReportSide{Units: UnitCounts{ULightCav: 3000}, Losses: UnitCounts{ULightCav: 3000}}
	r := reportWith(gone, ReportSide{Units: UnitCounts{UAxe: 1}, Losses: UnitCounts{UAxe: 1}})
	asAttacker := blindedFor(r, "h1", nil)
	if asAttacker.Defender.Units.Total() != 0 || asAttacker.Defender.Losses.Total() != 0 {
		t.Fatalf("the hidden side is still on the wire: %+v", asAttacker.Defender)
	}
}

func TestBlindedForLeavesOtherReportsAlone(t *testing.T) {
	gone := ReportSide{Units: UnitCounts{UAxe: 1}, Losses: UnitCounts{UAxe: 1}}
	for _, typ := range []string{"support", "scout", "market"} {
		r := reportWith(gone, gone)
		r.Type = typ
		if out := blindedFor(r, "h1", nil); out.AttackerHidden || out.DefenderHidden {
			t.Fatalf("%s must not be blinded", typ)
		}
	}
}

func TestWipedOut(t *testing.T) {
	if wipedOut(ReportSide{Units: UnitCounts{}, Losses: UnitCounts{}}) {
		t.Fatal("an empty side (e.g. a missed interception) is not a wiped-out side")
	}
	if wipedOut(ReportSide{Units: UnitCounts{UAxe: 3}, Losses: UnitCounts{UAxe: 2}}) {
		t.Fatal("a side with survivors is not wiped out")
	}
	if !wipedOut(ReportSide{Units: UnitCounts{UAxe: 3}, Losses: UnitCounts{UAxe: 3}}) {
		t.Fatal("a side that lost every man is wiped out")
	}
}
