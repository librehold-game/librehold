package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
)

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	var ve *app.ErrValidation
	if !errors.As(err, &ve) || ve.Field != field {
		t.Fatalf("err %v, want a refused %s", err, field)
	}
}

func ids(n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%q", fmt.Sprintf("v:%d:0", i))
	}
	return "[" + strings.Join(out, ",") + "]"
}

func TestOrderRefusesAHugeRecruitCountAtOnce(t *testing.T) {
	f := newFixture(t)
	start := time.Now()
	_, err := f.svc.Order(context.Background(), f.acct, f.world, "recruit",
		json.RawMessage(fmt.Sprintf(`{"villageId":%q,"unit":"noble","count":9223372036854775807}`, f.home)))
	wantField(t, err, "count")
	_, err = f.svc.Order(context.Background(), f.acct, f.world, "mass-recruit",
		json.RawMessage(fmt.Sprintf(`{"villageIds":[%q],"unit":"noble","upTo":9223372036854775807}`, f.home)))
	wantField(t, err, "upTo")
	if d := time.Since(start); d > time.Second {
		t.Fatalf("refusing took %v", d)
	}
}

func TestOrderRefusesLongIDLists(t *testing.T) {
	f := newFixture(t)
	for name, body := range map[string]string{
		"mass-build":        `{"villageIds":` + ids(app.MaxOrderIDs+1) + `,"building":"wood"}`,
		"mass-recruit":      `{"villageIds":` + ids(app.MaxOrderIDs+1) + `,"unit":"spear","upTo":1}`,
		"set-auto-farm":     `{"enabled":true,"villageIds":` + ids(app.MaxOrderIDs+1) + `}`,
		"mark-reports-read": `{"ids":` + ids(app.MaxOrderIDs+1) + `,"read":true}`,
		"snipe":             `{"commandIds":` + ids(app.MaxOrderIDs+1) + `,"fromVillageId":"v:0:0","units":{}}`,
		"save-group":        `{"name":"g","villageIds":` + ids(app.MaxOrderIDs+1) + `}`,
	} {
		_, err := f.svc.Order(context.Background(), f.acct, f.world, name, json.RawMessage(body))
		var ve *app.ErrValidation
		if !errors.As(err, &ve) || ve.Code != "tooMany" {
			t.Errorf("%s with %d ids: %v", name, app.MaxOrderIDs+1, err)
		}
	}
}

func TestOrderSendsOnlyAttacksAndSupport(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"send", "schedule-send"} {
		body := fmt.Sprintf(`{"originId":%q,"x":1,"y":1,"units":{"spear":1},"type":"attac","mode":"sendAt","at":99999999999}`, f.home)
		if name == "send" {
			body = fmt.Sprintf(`{"originId":%q,"x":1,"y":1,"units":{"spear":1},"type":"attac"}`, f.home)
		}
		_, err := f.svc.Order(context.Background(), f.acct, f.world, name, json.RawMessage(body))
		wantField(t, err, "type")
	}
}

func TestSavedTemplatesAndGroupsTakeOnlyShortIDs(t *testing.T) {
	f := newFixture(t)
	for name, body := range map[string]string{
		"save-build-template": `{"id":"` + strings.Repeat("A", 1000) + `","name":"X","steps":["main"]}`,
		"save-group":          `{"id":"` + strings.Repeat("A", 1000) + `","name":"X","villageIds":[]}`,
	} {
		_, err := f.svc.Order(context.Background(), f.acct, f.world, name, json.RawMessage(body))
		wantField(t, err, "id")
	}
	if _, err := f.svc.Order(context.Background(), f.acct, f.world, "save-group",
		json.RawMessage(`{"id":"grp_mg1x2k","name":"Front","villageIds":[]}`)); err != nil {
		t.Fatalf("an id as the game client makes them: %v", err)
	}
}
