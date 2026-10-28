package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

// orderBodies is one plausible body per order, on ala's own village where one is needed.
func orderBodies(d *duel) map[string]string {
	v, x2, y2 := d.v1, 0, 0
	_ = d.worlds.Read(context.Background(), d.world, func(w app.WorldTx) error {
		t := w.Village(d.v2)
		x2, y2 = t.X, t.Y
		return nil
	})
	return map[string]string{
		"build":                 fmt.Sprintf(`{"villageId":%q,"building":"wood"}`, v),
		"demolish":              fmt.Sprintf(`{"villageId":%q,"building":"hide"}`, v),
		"cancel-build":          fmt.Sprintf(`{"villageId":%q,"jobId":"nie_ma"}`, v),
		"recruit":               fmt.Sprintf(`{"villageId":%q,"unit":"spear","count":1}`, v),
		"cancel-recruit":        fmt.Sprintf(`{"villageId":%q,"jobId":"nie_ma"}`, v),
		"research":              fmt.Sprintf(`{"villageId":%q,"unit":"axe"}`, v),
		"cancel-research":       fmt.Sprintf(`{"villageId":%q,"jobId":"nie_ma"}`, v),
		"send":                  fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"spear":1},"type":"attack"}`, v, x2, y2),
		"cancel-command":        `{"commandId":"nie_ma"}`,
		"withdraw-support":      fmt.Sprintf(`{"hostId":%q,"homeId":%q}`, d.v2, v),
		"schedule-send":         fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"spear":1},"type":"attack","mode":"sendAt","at":3600000}`, v, x2, y2),
		"cancel-scheduled":      `{"id":"nie_ma"}`,
		"dodge":                 `{"commandId":"nie_ma"}`,
		"intercept":             fmt.Sprintf(`{"originId":%q,"commandId":"nie_ma","units":{"spear":1}}`, v),
		"snipe":                 fmt.Sprintf(`{"commandIds":["nie_ma"],"fromVillageId":%q,"units":{"spear":1}}`, v),
		"mint":                  fmt.Sprintf(`{"villageId":%q}`, v),
		"trade":                 fmt.Sprintf(`{"originId":%q,"targetId":%q,"amount":{"wood":100,"clay":0,"iron":0}}`, v, d.v2),
		"accept-trader":         fmt.Sprintf(`{"villageId":%q}`, v),
		"set-build-plan":        fmt.Sprintf(`{"villageId":%q,"steps":["wood","clay"]}`, v),
		"clear-build-plan":      fmt.Sprintf(`{"villageId":%q}`, v),
		"apply-build-template":  fmt.Sprintf(`{"villageId":%q,"templateId":"nie_ma"}`, v),
		"save-build-template":   `{"id":"","name":"Mine","steps":["wood"]}`,
		"delete-build-template": `{"id":"nie_ma"}`,
		"set-farm-template":     `{"key":"a","units":{"spear":2}}`,
		"send-farm-template":    fmt.Sprintf(`{"originId":%q,"key":"a","x":%d,"y":%d}`, v, x2, y2),
		"send-template-c":       fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d}`, v, x2, y2),
		"set-farm-filters":      `{"radius":12}`,
		"set-auto-farm":         fmt.Sprintf(`{"enabled":false,"villageIds":[%q]}`, v),
		"save-group":            fmt.Sprintf(`{"id":"","name":"Front","villageIds":[%q]}`, v),
		"delete-group":          `{"id":"nie_ma"}`,
		"mass-build":            fmt.Sprintf(`{"villageIds":[%q],"building":"clay"}`, v),
		"mass-recruit":          fmt.Sprintf(`{"villageIds":[%q],"unit":"spear","upTo":3}`, v),
		"rename-village":        fmt.Sprintf(`{"villageId":%q,"name":"Café Hill"}`, v),
		"mark-reports-read":     `{"ids":["nie_ma"],"read":true}`,
		"mark-all-reports-read": `{}`,
		"delete-report":         `{"id":"nie_ma"}`,
		"delete-read-reports":   ``,
		"claim-quest":           fmt.Sprintf(`{"questId":"nie_ma","villageId":%q}`, v),
	}
}

func isDomainError(err error) bool {
	var be *game.BlockedError
	var re *game.RuleError
	return errors.As(err, &be) || errors.As(err, &re)
}

func TestEveryOrderNameWired(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	bodies := orderBodies(d)
	names := app.OrderNames()
	var want []string
	for n := range bodies {
		want = append(want, n)
	}
	slices.Sort(want)
	if !slices.Equal(names, want) {
		t.Fatalf("order names %v\nwant %v", names, want)
	}
	for _, name := range names {
		st, err := d.svc.Order(ctx, ala, d.world, name, json.RawMessage(bodies[name]))
		if err != nil && !isDomainError(err) {
			t.Errorf("%s: %v", name, err)
		}
		if err == nil && (st == nil || st.Me != "h1") {
			t.Errorf("%s: no fresh state", name)
		}
		_, err = d.svc.Order(ctx, ala, d.world, name, json.RawMessage(`[1]`))
		var ve *app.ErrValidation
		if !errors.As(err, &ve) {
			t.Errorf("%s with a bad body: %v", name, err)
		}
	}
	_, err := d.svc.Order(ctx, ala, d.world, "launch-nukes", nil)
	wantValidation(t, err, "order", "unknown")
}

func TestOrdersRefuseNegativeAmounts(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	for name, body := range map[string]string{
		"send":    fmt.Sprintf(`{"originId":%q,"x":1,"y":1,"units":{"spear":-5},"type":"attack"}`, d.v1),
		"recruit": fmt.Sprintf(`{"villageId":%q,"unit":"spear","count":-1}`, d.v1),
		"trade":   fmt.Sprintf(`{"originId":%q,"targetId":%q,"amount":{"wood":-100,"clay":0,"iron":0}}`, d.v1, d.v2),
	} {
		_, err := d.svc.Order(ctx, ala, d.world, name, json.RawMessage(body))
		var ve *app.ErrValidation
		if !errors.As(err, &ve) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestOrderOnForeignVillageRejected(t *testing.T) {
	d := newDuel(t)
	ctx := context.Background()
	support := d.send(t, ala, d.v1, d.v2, "support", game.UnitCounts{game.USpear: 3})
	d.clock.Set(accountsStart + support.ArrivesAt + 1)
	carol := &app.Account{ID: 3, Username: "karol"}
	if _, err := d.svc.JoinWorld(ctx, carol, d.world, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	var x1, y1 int
	_ = d.worlds.Read(ctx, d.world, func(w app.WorldTx) error { v := w.Village(d.v1); x1, y1 = v.X, v.Y; return nil })
	for _, tc := range []struct {
		who  *app.Account
		name string
		body string
	}{
		{bartek, "build", fmt.Sprintf(`{"villageId":%q,"building":"wood"}`, d.v1)},
		{bartek, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"spear":1},"type":"attack"}`, d.v1, x1+1, y1)},
		{carol, "withdraw-support", fmt.Sprintf(`{"hostId":%q,"homeId":%q}`, d.v2, d.v1)},
		{bartek, "mint", fmt.Sprintf(`{"villageId":%q}`, d.v1)},
	} {
		_, err := d.svc.Order(ctx, tc.who, d.world, tc.name, json.RawMessage(tc.body))
		var be *game.BlockedError
		if !errors.As(err, &be) {
			t.Errorf("%s by %s: %v", tc.name, tc.who.Username, err)
		}
	}
	var v game.Village
	_ = d.worlds.Read(ctx, d.world, func(w app.WorldTx) error { v = *w.Village(d.v2); return nil })
	if v.SupportFrom[d.v1][game.USpear] != 3 {
		t.Fatal("a stranger moved the support")
	}
}
