//go:build e2e

package e2e

import (
	"fmt"
	"math"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

// segmentDistance is how close the straight route a→b comes to p, and how far along the route (0 to 1) that is.
func segmentDistance(ax, ay, bx, by, px, py float64) (float64, float64) {
	dx, dy := bx-ax, by-ay
	s := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	s = math.Max(0, math.Min(1, s))
	return math.Hypot(ax+s*dx-px, ay+s*dy-py), s
}

// fieldScene picks, among the guests, an attacker, a bystander and a barbarian village whose route from the
// attacker passes within sight of the bystander's village, far enough out to take a while; ok is false when the
// guests did not spawn in such a line.
func fieldScene(wid int64, guests []*client) (attacker, bystander *client, from, by *game.Village, target app.MapTile, ok bool) {
	var tiles []app.MapTile
	guests[0].ok("GET", wpath(wid, "/map?x0=0&y0=0&x1=39&y1=39"), nil, &tiles)
	homes := make([]*game.Village, len(guests))
	for i, g := range guests {
		homes[i] = g.home(wid)
	}
	r := game.InstantWorldConfig.SightRadius - 1
	for i := range guests {
		for j := range guests {
			if i == j {
				continue
			}
			a, c := homes[i], homes[j]
			for _, tile := range tiles {
				if tile.OwnerID != "" || math.Hypot(float64(tile.X-a.X), float64(tile.Y-a.Y)) < 12 {
					continue
				}
				// Met in the first half of the march, so the hit army is still on the road to be turned back.
				d, along := segmentDistance(float64(a.X), float64(a.Y), float64(tile.X), float64(tile.Y), float64(c.X), float64(c.Y))
				if d <= r && along <= 0.5 && math.Hypot(float64(c.X-a.X), float64(c.Y-a.Y)) > r+1 {
					return guests[i], guests[j], a, c, tile, true
				}
			}
		}
	}
	return
}

func TestFieldBattleBetweenGuests(t *testing.T) {
	const scale = 10
	var (
		wid         int64
		a, c        *client
		home, watch *game.Village
		target      app.MapTile
		found       bool
	)
	// The scene hangs on where the guests spawn: five guests are in a line on 19 maps in 20, and a fresh world
	// covers the rest.
	for range 3 {
		s := startServerAt(t, newDB(t), time.Now().UnixMilli(), scale)
		guests := []*client{s.guest(t), s.guest(t), s.guest(t), s.guest(t), s.guest(t)}
		wid, _ = guests[0].play(t)
		for _, g := range guests[1:] {
			if w, _ := g.play(t); w != wid {
				t.Fatal("the guests play different worlds")
			}
		}
		if a, c, home, watch, target, found = fieldScene(wid, guests); found {
			break
		}
	}
	if !found {
		t.Fatal("no attacker, bystander and barbarian in a line on three maps")
	}
	// Beginner protection (ten world minutes) keeps everyone out of field battles at first.
	time.Sleep(time.Duration(10*60*1000/scale+2000) * time.Millisecond)

	// Strong enough against cavalry (about 1500) to outlive the bystander's horsemen (650) and be turned back.
	a.ok("POST", wpath(wid, "/orders/send"), map[string]any{"originId": home.ID, "x": target.X, "y": target.Y,
		"type": "attack", "units": game.UnitCounts{game.UAxe: 100, game.ULightCav: 25}}, nil)
	var attack *app.CommandView
	for _, cmd := range a.state(wid).Commands {
		if cmd.Type == "attack" && cmd.TargetID == target.ID {
			cmdCopy := cmd
			attack = &cmdCopy
		}
	}
	if attack == nil {
		t.Fatal("the attack is not in the attacker's state")
	}

	// The bystander sees it pass, without its troops, and sends a few horsemen to meet it.
	deadline := time.Now().Add(time.Duration(attack.ArrivesAt-attack.DepartedAt) * time.Millisecond / scale)
	sent := false
	for !sent && time.Now().Before(deadline) {
		for _, cmd := range c.state(wid).Commands {
			if cmd.ID != attack.ID {
				continue
			}
			if cmd.Units != nil {
				t.Fatalf("the bystander sees the passing army's troops: %+v", cmd.Units)
			}
			rep := c.do("POST", wpath(wid, "/orders/intercept"), map[string]any{"originId": watch.ID, "commandId": attack.ID,
				"units": game.UnitCounts{game.ULightCav: 5}})
			sent = rep.status == 200
			if !sent {
				t.Logf("intercept refused: %s", rep.body)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !sent {
		t.Fatal("the bystander never got to meet the passing army")
	}

	hasField := func(g *client) bool {
		var reports []game.Report
		g.ok("GET", wpath(wid, "/reports?limit=100"), nil, &reports)
		for _, r := range reports {
			if r.Type == "field" && r.Field != nil && r.Field.TargetCommandID == attack.ID {
				return true
			}
		}
		return false
	}
	for start := time.Now(); !hasField(a); time.Sleep(50 * time.Millisecond) {
		if time.Since(start) > 30*time.Second {
			t.Fatal("no field battle report for the attacker")
		}
	}
	// The attack was hit on the road: its owner turns it back, outside the usual cancel window if need be.
	rep := a.do("POST", wpath(wid, "/orders/cancel-command"), map[string]string{"commandId": attack.ID})
	if rep.status != 200 {
		t.Fatalf("turning back after the field battle: %d %s", rep.status, rep.body)
	}
	if !hasField(c) {
		t.Fatal("the bystander has no field battle report")
	}
	for _, cmd := range a.state(wid).Commands {
		if cmd.ID == attack.ID && cmd.Type != "return" {
			t.Fatalf("the army did not turn back: %+v", cmd.Command)
		}
	}
	for _, cmd := range c.state(wid).Commands {
		if cmd.ID == attack.ID && cmd.Units != nil {
			t.Fatal(fmt.Sprintf("the bystander sees the army's troops after the battle: %+v", cmd.Units))
		}
	}
}
