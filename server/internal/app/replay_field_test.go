package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
)

func TestReplayWithFieldBattles(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a, b, c := f.guest(t), f.guest(t), f.guest(t)
	id := f.play(t, a).WorldID
	for _, g := range []*app.Account{b, c} {
		if f.playIn(t, g, id).WorldID != id {
			t.Fatal("the guests play different worlds")
		}
	}
	order := func(acct *app.Account, name, body string) error {
		_, err := f.svc.Order(ctx, acct, id, name, json.RawMessage(body))
		return err
	}
	// Past beginner protection, a attacks b.
	f.clock.Add(11 * time.Minute.Milliseconds())
	homeA, _ := homeOf(t, f, id, a)
	homeB, _ := homeOf(t, f, id, b)
	homeC, _ := homeOf(t, f, id, c)
	if err := order(a, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"axe":60},"type":"attack"}`, homeA.ID, homeB.X, homeB.Y)); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.State(ctx, b, id)
	if err != nil {
		t.Fatal(err)
	}
	var attackID string
	for _, cmd := range st.Commands {
		if cmd.Type == "attack" && cmd.TargetID == homeB.ID {
			attackID = cmd.ID
		}
	}
	// b sends a handful of cavalry to meet it, c tries too (seen only if the army passes near c).
	if err := order(b, "intercept", fmt.Sprintf(`{"originId":%q,"commandId":%q,"units":{"lightCav":5}}`, homeB.ID, attackID)); err != nil {
		t.Fatal(err)
	}
	for range 30 {
		f.clock.Add(time.Second.Milliseconds())
		_ = order(c, "intercept", fmt.Sprintf(`{"originId":%q,"commandId":%q,"units":{"scout":1},"scout":true}`, homeC.ID, attackID))
		_ = order(a, "cancel-command", fmt.Sprintf(`{"commandId":%q}`, attackID))
	}
	// The replay runs to the last logged action, so the script ends on one.
	f.clock.Add(10 * time.Minute.Milliseconds())
	if err := order(a, "mark-all-reports-read", `{}`); err != nil {
		t.Fatal(err)
	}

	row, _ := f.worlds.Get(ctx, id)
	log := actionLog(t, f, id)
	if len(entriesOf(log, "intercept")) == 0 {
		t.Fatal("the scripted game sent no interception")
	}
	want := worldDump(t, f.worlds, id)
	if !strings.Contains(want, `"type": "field"`) {
		t.Fatal("the scripted game had no field battle")
	}
	replayWorlds := memory.NewWorlds()
	svc := app.NewService(memory.NewAccounts(), replayWorlds, memory.NewCounters(&fakeClock{}), &fakeClock{}, plainHasher{}, security.NewTokens(), app.Config{})
	rid, err := app.Replay(ctx, svc, log, row.Seed, row.Secret)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []*app.Account{a, b, c} {
		want = strings.ReplaceAll(want, `"`+g.Username+`"`, `"`+g.PlayerKey+`"`)
	}
	got := worldDump(t, replayWorlds, rid)
	if got != want {
		gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("replay differs at line %d:\n got %s\nwant %s", i, gl[i], wl[i])
			}
		}
		t.Fatalf("replay differs in length: %d vs %d lines", len(gl), len(wl))
	}
}
