package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

func commandFields(t *testing.T, st *app.PlayerState, id string) map[string]any {
	t.Helper()
	for _, c := range st.Commands {
		if c.ID == id {
			raw, _ := json.Marshal(c)
			var out map[string]any
			_ = json.Unmarshal(raw, &out)
			return out
		}
	}
	return nil
}

func TestOnlyTheSenderSeesTheFarmMark(t *testing.T) {
	d := newDuel(t)
	var raid *game.Command
	err := d.svc.InWorld(context.Background(), ala, d.world, func(w app.WorldTx, me game.PlayerID) error {
		to := w.Village(d.v2)
		var err error
		raid, err = game.SendCommand(w, me, d.v1, to.X, to.Y, game.UnitCounts{game.USpear: 5}, "attack", game.SendOpts{Farm: true})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if mine := commandFields(t, d.state(t, ala), raid.ID); mine == nil || mine["farm"] != true {
		t.Fatalf("the sender sees %v", mine)
	}
	if theirs := commandFields(t, d.state(t, bartek), raid.ID); theirs == nil || theirs["farm"] != nil {
		t.Fatalf("the defender sees %v", theirs)
	}
}

func TestHideFarmOnMapOrder(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	st, err := d.svc.Order(ctx, ala, d.world, "hide-farm-on-map", json.RawMessage(`{"hide":true}`))
	if err != nil || !st.Human.FarmAssistant.HideOnMap {
		t.Fatalf("err %v state %+v", err, st)
	}
	if again := d.state(t, ala); !again.Human.FarmAssistant.HideOnMap {
		t.Fatal("the setting was not kept")
	}
	if theirs := d.state(t, bartek); theirs.Human.FarmAssistant.HideOnMap {
		t.Fatal("the setting leaked to another player")
	}
}
