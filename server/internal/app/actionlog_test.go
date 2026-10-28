package app_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

func actionLog(t *testing.T, f *instantFixture, id int64) []game.LoggedAction {
	t.Helper()
	log, err := f.worlds.ActionLog(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func entriesOf(log []game.LoggedAction, kind string) []game.LoggedAction {
	var out []game.LoggedAction
	for _, e := range log {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// homeOf is the account's first village in the world, with the nearest barbarian village to it.
func homeOf(t *testing.T, f *instantFixture, id int64, a *app.Account) (home, barbarian *game.Village) {
	t.Helper()
	err := f.worlds.Read(context.Background(), id, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			if p.IsHuman && p.AccountID == a.ID && len(p.VillageIDs) > 0 {
				home = w.Village(p.VillageIDs[0])
			}
		}
		if home == nil {
			return fmt.Errorf("no village of %s", a.Username)
		}
		size := w.Config().MapSize
		for _, ref := range w.VillagesIn(0, 0, size-1, size-1) {
			v := w.Village(ref.ID)
			if v.OwnerID == "" && (barbarian == nil ||
				game.Distance(home.X, home.Y, v.X, v.Y) < game.Distance(home.X, home.Y, barbarian.X, barbarian.Y)) {
				barbarian = v
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return home, barbarian
}

func TestOrderAndLogShareTransaction(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	id := f.play(t, g).WorldID
	home, _ := homeOf(t, f, id, g)
	before := len(actionLog(t, f, id))
	for range 2 {
		if _, err := f.svc.Order(ctx, g, id, "recruit", json.RawMessage(fmt.Sprintf(`{"villageId":%q,"unit":"spear","count":100000}`, home.ID))); err == nil {
			t.Fatal("an order beyond the resources went through")
		}
	}
	if _, err := f.svc.Order(ctx, g, id, "rename-village", json.RawMessage(`{"villageId":"nie_ma","name":"X"}`)); err == nil {
		t.Fatal("renaming a missing village went through")
	}
	if log := actionLog(t, f, id); len(log) != before {
		t.Fatalf("refused orders logged: %+v", log[before:])
	}
	rejections, err := f.worlds.Rejections(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range rejections {
		counts[r.Kind+"/"+r.Reason] = r.Count
	}
	if len(counts) != 2 || counts["rename-village/notOwner"] == 0 && counts["rename-village/noVillage"] == 0 {
		t.Fatalf("rejections %+v", rejections)
	}
	for k, n := range counts {
		if strings.HasPrefix(k, "recruit/") && n != 2 {
			t.Fatalf("rejections %+v", rejections)
		}
	}
	body := fmt.Sprintf(`{"villageId":%q,"unit":"spear","count":1}`, home.ID)
	if _, err := f.svc.Order(ctx, g, id, "recruit", json.RawMessage(body)); err != nil {
		t.Fatal(err)
	}
	log := actionLog(t, f, id)
	if len(log) != before+1 {
		t.Fatalf("accepted order logged %d times", len(log)-before)
	}
	e := log[len(log)-1]
	if e.Kind != "recruit" || e.IsBot || string(e.Payload) != body || e.Seq != log[len(log)-2].Seq+1 {
		t.Fatalf("entry %+v %s", e, e.Payload)
	}
}

func TestLogUsesPlayerKeyNotUsername(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	id := f.play(t, g).WorldID
	home, _ := homeOf(t, f, id, g)
	if _, err := f.svc.Order(ctx, g, id, "recruit", json.RawMessage(fmt.Sprintf(`{"villageId":%q,"unit":"spear","count":1}`, home.ID))); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LeaveWorld(ctx, g, id); err != nil {
		t.Fatal(err)
	}
	log := actionLog(t, f, id)
	for _, kind := range []string{"join", "recruit", "leave", "snapshot"} {
		es := entriesOf(log, kind)
		if len(es) == 0 || es[0].Actor != g.PlayerKey {
			t.Fatalf("%s entries %+v (key %s)", kind, es, g.PlayerKey)
		}
	}
	raw, _ := json.Marshal(log)
	if g.PlayerKey == "" || bytes.Contains(raw, []byte(g.Username)) {
		t.Fatalf("the log names the player: %s", raw)
	}
}

func TestBotActionsLogged(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	id := f.play(t, g).WorldID
	f.clock.Add(10 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, e := range actionLog(t, f, id) {
		if e.IsBot {
			if !strings.HasPrefix(e.Actor, "bot_") || !json.Valid(e.Payload) {
				t.Fatalf("bot entry %+v", e)
			}
			kinds[e.Kind] = true
		}
	}
	if !kinds["build"] || !kinds["recruit"] {
		t.Fatalf("bot kinds %v", kinds)
	}
	for k := range kinds {
		if _, ok := map[string]bool{"build": true, "recruit": true, "research": true, "send": true, "schedule-send": true,
			"dodge": true, "snipe": true, "mint": true}[k]; !ok {
			t.Fatalf("bot kind %q is not an order name", k)
		}
	}
}

func TestSnapshotsEveryFiveMinutesInInstant(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	g := f.guest(t)
	id := f.play(t, g).WorldID
	row, _ := f.worlds.Get(ctx, id)
	joinedAt := app.WorldNow(row, f.clock.NowMs())
	f.clock.Add(16 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	snaps := entriesOf(actionLog(t, f, id), "snapshot")
	var at []int64
	for _, s := range snaps {
		at = append(at, s.At)
	}
	min := game.MsPerMinute
	want := []int64{joinedAt, 5 * min, 10 * min, 15 * min}
	if fmt.Sprint(at) != fmt.Sprint(want) {
		t.Fatalf("snapshots at %v, want %v", at, want)
	}
	var snap game.Snapshot
	if err := json.Unmarshal(snaps[1].Payload, &snap); err != nil {
		t.Fatal(err)
	}
	v := snap.Villages[0]
	if len(snap.Villages) != 1 || snap.Points == 0 || v.Buildings[game.BMain] != game.InstantStartVillage.Buildings[game.BMain] || v.UnitsHome[game.UNoble] != 1 ||
		v.Resources.Wood == 0 {
		t.Fatalf("snapshot %s", snaps[1].Payload)
	}
}

func TestPackThenDeleteRows(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	log := actionLog(t, f, id)
	row, _ := f.worlds.Get(ctx, id)
	if err := f.svc.PackActionLog(ctx, id); err != nil {
		t.Fatal(err)
	}
	if rest := actionLog(t, f, id); len(rest) != 0 {
		t.Fatalf("%d rows left", len(rest))
	}
	p, err := f.worlds.PackedLog(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Seed != row.Seed || p.Config.Kind != "instant" || p.Build == "" || p.Rows != len(log) {
		t.Fatalf("packed %+v", p)
	}
	zr, err := gzip.NewReader(bytes.NewReader(p.Data))
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(nil, 1<<24)
	n := 0
	for sc.Scan() {
		var e game.LoggedAction
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		if e.Seq != log[n].Seq || e.Kind != log[n].Kind {
			t.Fatalf("line %d: %+v, want %+v", n, e, log[n])
		}
		n++
	}
	if n != len(log) {
		t.Fatalf("%d lines, want %d", n, len(log))
	}
	f.clock.Add(25 * time.Hour.Milliseconds())
	if _, err := f.svc.PurgeEndedInstant(ctx); err != nil {
		t.Fatal(err)
	}
	if again, err := f.worlds.PackedLog(ctx, id); err != nil || again.Rows != len(log) {
		t.Fatalf("after purge %+v %v", again, err)
	}
}

func TestPurgePacksTheLog(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	f.clock.Add(25 * time.Hour.Milliseconds())
	if _, err := f.svc.PurgeEndedInstant(ctx); err != nil {
		t.Fatal(err)
	}
	if p, err := f.worlds.PackedLog(ctx, id); err != nil || p.Rows == 0 {
		t.Fatalf("packed %+v %v", p, err)
	}
}

// worldDump is what a replay must reproduce: villages, players and every player's reports.
func worldDump(t *testing.T, s *memory.Worlds, id int64) string {
	t.Helper()
	var out []byte
	err := s.Read(context.Background(), id, func(w app.WorldTx) error {
		size := w.Config().MapSize
		var villages []*game.Village
		for _, ref := range w.VillagesIn(0, 0, size-1, size-1) {
			villages = append(villages, w.Village(ref.ID))
		}
		reports := map[game.PlayerID][]*game.Report{}
		for _, p := range w.Players() {
			reports[p.ID] = w.Reports(p.ID, 1<<62, 0)
		}
		b, err := json.MarshalIndent(map[string]any{"villages": villages, "players": w.Players(), "reports": reports}, "", " ")
		out = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestReplayReproducesFinalState(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	a, b := f.guest(t), f.guest(t)
	id := f.play(t, a).WorldID
	if f.play(t, b).WorldID != id {
		t.Fatal("the two guests play different worlds")
	}
	order := func(acct *app.Account, name, body string) {
		_, _ = f.svc.Order(ctx, acct, id, name, json.RawMessage(body))
	}
	buildings := []string{"wood", "clay", "iron", "main", "barracks"}
	guestName := a.Username
	for minute := range 20 {
		f.clock.Add(time.Minute.Milliseconds())
		for i, acct := range []*app.Account{a, b} {
			home, barb := homeOf(t, f, id, acct)
			order(acct, "build", fmt.Sprintf(`{"villageId":%q,"building":%q}`, home.ID, buildings[(minute+i)%len(buildings)]))
			order(acct, "recruit", fmt.Sprintf(`{"villageId":%q,"unit":"axe","count":3}`, home.ID))
			switch {
			case barb == nil:
			case minute == 2:
				order(acct, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"axe":60,"lightCav":20},"type":"attack"}`,
					home.ID, barb.X, barb.Y))
			case minute == 3:
				order(acct, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"axe":20,"noble":1},"type":"attack"}`,
					home.ID, barb.X, barb.Y))
			case minute%4 == 1:
				order(acct, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"scout":1},"type":"attack"}`,
					home.ID, barb.X, barb.Y))
			}
		}
		if minute == 10 {
			order(b, "mark-all-reports-read", `{}`)
		}
		if minute == 12 {
			claimed, err := f.svc.ClaimGuest(ctx, a, app.RegisterInput{Username: "Winner", Password: "long-password",
				PasswordConfirm: "long-password", Client: "10.9.9.9"})
			if err != nil {
				t.Fatal(err)
			}
			a = claimed
		}
	}
	if err := f.svc.LeaveWorld(ctx, b, id); err != nil {
		t.Fatal(err)
	}
	row, _ := f.worlds.Get(ctx, id)
	log := actionLog(t, f, id)
	if len(entriesOf(log, "send")) < 4 {
		t.Fatalf("the scripted game sent too little: %d", len(entriesOf(log, "send")))
	}

	replayWorlds := memory.NewWorlds()
	clock := &fakeClock{}
	svc := app.NewService(memory.NewAccounts(), replayWorlds, memory.NewCounters(clock), clock, plainHasher{}, security.NewTokens(), app.Config{})
	rid, err := app.Replay(ctx, svc, log, row.Seed, row.Secret)
	if err != nil {
		t.Fatal(err)
	}
	want := worldDump(t, f.worlds, id)
	for name, key := range map[string]string{a.Username: a.PlayerKey, guestName: a.PlayerKey, b.Username: b.PlayerKey} {
		want = strings.ReplaceAll(want, `"`+name+`"`, `"`+key+`"`)
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
	if !strings.Contains(want, `"conqueredAt"`) {
		t.Log("note: the scripted game had no conquest")
	}
}
