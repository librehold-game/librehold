package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

// interceptDuel: ala attacks bartek with 5 spearmen, bartek sends 10 light cavalry to meet them.
func interceptDuel(t *testing.T) (*duel, *game.Command, *game.Command) {
	t.Helper()
	d := newDuel(t)
	err := d.svc.InWorld(context.Background(), ala, d.world, func(w app.WorldTx, me game.PlayerID) error {
		w.Village(d.v1).Units[game.USpear] = 5
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	attack := d.send(t, ala, d.v1, d.v2, "attack", game.UnitCounts{game.USpear: 5})
	var in *game.Command
	err = d.svc.InWorld(context.Background(), bartek, d.world, func(w app.WorldTx, me game.PlayerID) error {
		w.Village(d.v2).Units[game.ULightCav] = 10
		var err error
		in, err = game.SendIntercept(w, me, d.v2, attack.ID, game.UnitCounts{game.ULightCav: 10}, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, attack, in
}

func commandJSON(t *testing.T, st *app.PlayerState, id string) map[string]any {
	t.Helper()
	for _, c := range st.Commands {
		if c.ID == id {
			var out map[string]any
			raw, _ := json.Marshal(c)
			_ = json.Unmarshal(raw, &out)
			return out
		}
	}
	return nil
}

var cezary = &app.Account{ID: 3, Username: "cezary"}

// bystanderDuel: ala attacks bartek; cezary holds a village halfway along the route.
func bystanderDuel(t *testing.T) (*duel, *game.Command) {
	t.Helper()
	d := newDuel(t)
	ctx := context.Background()
	me, err := d.svc.JoinWorld(ctx, cezary, d.world, game.QuadrantRandom)
	if err != nil {
		t.Fatal(err)
	}
	// Start villages land anywhere; the route gets a fixed length: bartek takes a village 10 tiles from ala,
	// cezary one halfway there. The target must be out of sight of cezary's own start village, or the army would
	// stay visible all the way.
	var target string
	place := func(w app.WorldTx, x, y int, owner game.PlayerRef) *game.Village {
		v := game.CreateVillage(x, y, "", "", w.Now())
		w.PutVillage(v)
		game.SetOwner(w, v, owner)
		return v
	}
	err = d.worlds.Run(ctx, d.world, func(w app.WorldTx) error {
		a := w.Village(d.v1)
		size := w.Config().MapSize
		free := func(x, y int) bool { return x >= 0 && y >= 0 && x < size && y < size && !game.VillageExistsAt(w, x, y) }
		r := game.SightRadiusOf(w.Config())
		unseen := func(x, y int) bool {
			for _, v := range game.PlayerVillages(w, me) {
				if math.Hypot(float64(v.X-x), float64(v.Y-y)) <= r+2 {
					return false
				}
			}
			return true
		}
		for _, dx := range []int{10, -10, 12, -12, 14, -14, 16, -16} {
			for _, dy := range []int{0, 2, -2, 4, -4} {
				if free(a.X+dx, a.Y+dy) && free(a.X+dx/2, a.Y+dy/2) && unseen(a.X+dx, a.Y+dy) {
					target = place(w, a.X+dx, a.Y+dy, "h2").ID
					place(w, a.X+dx/2, a.Y+dy/2, game.Ref(me))
					a.Units[game.USpear] = 5
					return nil
				}
			}
		}
		return fmt.Errorf("no free line from %s", a.ID)
	})
	if err != nil {
		t.Fatal(err)
	}
	return d, d.send(t, ala, d.v1, target, "attack", game.UnitCounts{game.USpear: 5})
}

func TestBystanderSeesPassingArmyWithoutUnits(t *testing.T) {
	d, attack := bystanderDuel(t)
	if commandJSON(t, d.state(t, cezary), attack.ID) != nil {
		t.Fatal("at the start the army is far from the bystander")
	}
	step := (attack.ArrivesAt - attack.DepartedAt) / 40
	var seen []bool
	for i := 1; i < 40; i++ {
		d.clock.Add(step)
		passing := commandJSON(t, d.state(t, cezary), attack.ID)
		if passing != nil {
			if _, ok := passing["units"]; ok {
				t.Fatalf("a passing army shows no troops: %v", passing)
			}
		}
		seen = append(seen, passing != nil)
	}
	if !slices.Contains(seen, true) {
		t.Fatal("halfway the army passes the bystander's village")
	}
	if seen[len(seen)-1] {
		t.Fatal("near its target the army is out of the bystander's sight")
	}
}

func TestForeignCommandViewHasNoFieldHit(t *testing.T) {
	d := newDuel(t)
	err := d.svc.InWorld(context.Background(), ala, d.world, func(w app.WorldTx, me game.PlayerID) error {
		w.Village(d.v1).Units[game.USpear] = 5
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	attack := d.send(t, ala, d.v1, d.v2, "attack", game.UnitCounts{game.USpear: 5})
	err = d.worlds.Run(context.Background(), d.world, func(w app.WorldTx) error {
		w.Command(attack.ID).FieldHit = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if theirs := commandJSON(t, d.state(t, bartek), attack.ID); theirs == nil || theirs["fieldHit"] != nil {
		t.Fatalf("the defender learns the attack was hit on the road: %v", theirs)
	}
	if mine := commandJSON(t, d.state(t, ala), attack.ID); mine["fieldHit"] != true {
		t.Fatalf("the owner sees their army was hit: %v", mine)
	}
}

func TestViewShowsOwnInterceptWithUnits(t *testing.T) {
	d, _, in := interceptDuel(t)
	own := commandJSON(t, d.state(t, bartek), in.ID)
	if own == nil {
		t.Fatal("the interceptor's owner sees the intercept")
	}
	if units, ok := own["units"].(map[string]any); !ok || units["lightCav"] != float64(10) {
		t.Fatalf("own intercept %v", own)
	}
	if order, ok := own["intercept"].(map[string]any); !ok || order["leg"] == nil {
		t.Fatalf("own intercept shows what it is after: %v", own)
	}
}

func TestViewShowsInterceptAimedAtMyArmyWithoutUnits(t *testing.T) {
	d, attack, in := interceptDuel(t)
	st := d.state(t, ala)
	threat := commandJSON(t, st, in.ID)
	if threat == nil {
		t.Fatal("the threatened army's owner sees the intercept coming")
	}
	for _, banned := range []string{"units", "intercept", "haul"} {
		if _, ok := threat[banned]; ok {
			t.Fatalf("a foreign intercept leaks %q: %v", banned, threat)
		}
	}
	if threat["point"] == nil || threat["type"] != "intercept" {
		t.Fatalf("the threat shows where it is going: %v", threat)
	}
	mine := commandJSON(t, st, attack.ID)
	if threats, ok := mine["threats"].([]any); !ok || len(threats) != 1 || threats[0] != in.ID {
		t.Fatalf("own army lists its threats: %v", mine)
	}
	if theirs := commandJSON(t, d.state(t, bartek), attack.ID); theirs["threats"] != nil {
		t.Fatalf("someone else's army does not show its threats: %v", theirs)
	}
}
