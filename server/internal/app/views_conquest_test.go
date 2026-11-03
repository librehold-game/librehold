package app_test

import (
	"context"
	"slices"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

func TestLooseArmyShowsAsTheSendersOnly(t *testing.T) {
	d := newDuel(t)
	attack := d.send(t, ala, d.v1, d.v2, "attack", game.UnitCounts{game.USpear: 5})
	err := d.worlds.Run(context.Background(), d.world, func(w app.WorldTx) error {
		game.TransferVillage(w, w.Village(d.v1), "h2", d.v2)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mine, theirs := d.state(t, ala), d.state(t, bartek)
	own := commandJSON(t, mine, attack.ID)
	if own == nil || own["ownerId"] != "h1" || own["cutLoose"] != true || own["units"] == nil || own["research"] != nil {
		t.Fatalf("the sender sees %v", own)
	}
	foreign := commandJSON(t, theirs, attack.ID)
	keys := []string{}
	for k := range foreign {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"arrivesAt", "cutLoose", "departedAt", "id", "originId", "targetId", "type"}; !slices.Equal(keys, want) {
		t.Fatalf("the defender sees %v, want only %v", keys, want)
	}
	for who, st := range map[string]*app.PlayerState{"ala": mine, "bartek": theirs} {
		n := 0
		for _, c := range st.Commands {
			if c.ID == attack.ID {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%s lists the army %d times", who, n)
		}
	}
}
