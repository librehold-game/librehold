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

// watchedGame is a started player world of hoster's with two guests and three bots, a guest's attack on a bot and
// a bot's attack on another bot on the road.
type watchedGame struct {
	f           *instantFixture
	id          int64
	a, b        *app.Account
	guestAttack string
	botAttack   string
}

func newWatchedGame(t *testing.T) *watchedGame {
	t.Helper()
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	card := f.host(t, hoster, app.HostForm{Name: "Watched", Bots: ptr(3), ProtectionMin: ptr(0)})
	g := &watchedGame{f: f, id: card.WorldID, a: f.guest(t), b: f.guest(t)}
	for _, acct := range []*app.Account{g.a, g.b} {
		if _, err := f.svc.JoinByInvite(ctx, acct, card.InviteCode); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.HostStart(ctx, hoster, g.id); err != nil {
		t.Fatal(err)
	}
	home, _ := homeOf(t, f, g.id, g.a)
	var bots []*game.Player
	err := f.worlds.Read(ctx, g.id, func(w app.WorldTx) error {
		for _, p := range w.Players() {
			if !p.IsHuman {
				bots = append(bots, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var target *game.Village
	_ = f.worlds.Read(ctx, g.id, func(w app.WorldTx) error { target = w.Village(bots[2].VillageIDs[0]); return nil })
	if _, err := f.svc.Order(ctx, g.a, g.id, "send", json.RawMessage(fmt.Sprintf(
		`{"originId":%q,"x":%d,"y":%d,"units":{"axe":10},"type":"attack"}`, home.ID, target.X, target.Y))); err != nil {
		t.Fatalf("send %v", err)
	}
	err = f.worlds.Run(ctx, g.id, func(w app.WorldTx) error {
		w.SetNow(app.WorldNow(w.Row(), f.clock.NowMs()))
		for _, c := range w.MovingCommands(w.Now()) {
			g.guestAttack = c.ID
		}
		from, to := w.Village(bots[0].VillageIDs[0]), w.Village(bots[1].VillageIDs[0])
		from.Units[game.UAxe] += 10
		c, err := game.SendCommand(w, bots[0].ID, from.ID, to.X, to.Y, game.UnitCounts{game.UAxe: 10}, "attack", game.SendOpts{})
		if err != nil {
			return err
		}
		g.botAttack = c.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestWatchShowsEveryVillageAndArmy(t *testing.T) {
	g := newWatchedGame(t)
	ctx := context.Background()
	st, err := g.f.svc.Watch(ctx, hoster, g.id)
	if err != nil {
		t.Fatal(err)
	}
	if st.WorldID != g.id || st.WorldName != "Watched" || st.Hosted == nil || st.Hosted.Status != "running" || st.Instant == nil || st.Paused {
		t.Fatalf("state %+v", st)
	}
	byID := map[string]app.CommandView{}
	for _, c := range st.Commands {
		byID[c.ID] = c
	}
	for _, id := range []string{g.guestAttack, g.botAttack} {
		if c, ok := byID[id]; !ok || c.Units == nil || c.Units.Total() != 10 {
			t.Fatalf("army %s: %+v (all %v)", id, c, st.Commands)
		}
	}
	want := 0
	err = g.f.worlds.Read(ctx, g.id, func(w app.WorldTx) error {
		size := w.Config().MapSize
		stored := map[[2]int]bool{}
		for _, ref := range w.VillagesIn(0, 0, size-1, size-1) {
			stored[[2]int{ref.X, ref.Y}] = true
			want++
		}
		for x := range size {
			for y := range size {
				if !stored[[2]int{x, y}] && game.TileHasVillage(w.Seed(), w.Config(), x, y) {
					want++
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Villages) != want {
		t.Fatalf("%d villages, want %d", len(st.Villages), want)
	}
	bots := 0
	for _, r := range st.Ranking {
		if r.IsBot {
			bots++
		}
	}
	if bots != 3 || len(st.Ranking) != 5 || len(st.Players) != 5 {
		t.Fatalf("ranking %+v", st.Ranking)
	}
	if !slices.IsSortedFunc(st.Ranking, func(x, y app.PlayerRow) int { return y.Points - x.Points }) {
		t.Fatalf("ranking not by points: %+v", st.Ranking)
	}
}

func TestWatchOnlyForHost(t *testing.T) {
	g := newWatchedGame(t)
	ctx := context.Background()
	if ok, err := g.f.svc.IsHost(ctx, hoster, g.id); err != nil || !ok {
		t.Fatalf("host %v %v", ok, err)
	}
	instant := g.f.play(t, g.f.guest(t)).WorldID
	for _, c := range []struct {
		acct *app.Account
		id   int64
	}{{g.a, g.id}, {ola, g.id}, {hoster, instant}} {
		if ok, _ := g.f.svc.IsHost(ctx, c.acct, c.id); ok {
			t.Fatalf("%s hosts %d", c.acct.Username, c.id)
		}
		if _, err := g.f.svc.Watch(ctx, c.acct, c.id); !errors.Is(err, app.ErrForbidden) {
			t.Fatalf("%s watches %d: %v", c.acct.Username, c.id, err)
		}
		if _, err := g.f.svc.WatchVillage(ctx, c.acct, c.id, "v:1:1"); !errors.Is(err, app.ErrForbidden) {
			t.Fatalf("%s watches a village of %d: %v", c.acct.Username, c.id, err)
		}
	}
}

func TestWatchVillageDetailsReadOnly(t *testing.T) {
	g := newWatchedGame(t)
	ctx := context.Background()
	home, _ := homeOf(t, g.f, g.id, g.a)
	logBefore := len(actionLog(t, g.f, g.id))
	stored := func() string {
		var out []byte
		_ = g.f.worlds.Read(ctx, g.id, func(w app.WorldTx) error {
			out, _ = json.Marshal(w.Village(home.ID))
			return nil
		})
		return string(out)
	}
	before := stored()
	v, err := g.f.svc.WatchVillage(ctx, hoster, g.id, home.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != home.ID || v.OwnerName != g.a.Username || v.OwnerIsBot || v.Buildings != home.Buildings || v.UnitsHome[game.UAxe] != home.Units[game.UAxe] {
		t.Fatalf("village %+v, home %+v", v, home)
	}
	row := g.f.row(t, g.id)
	gx, gy, found := 0, 0, false
	err = g.f.worlds.Read(ctx, g.id, func(w app.WorldTx) error {
		size := w.Config().MapSize
		for x := range size {
			for y := range size {
				if !found && game.TileHasVillage(w.Seed(), w.Config(), x, y) && len(w.VillagesIn(x, y, x, y)) == 0 {
					gx, gy, found = x, y, true
				}
			}
		}
		return nil
	})
	if err != nil || !found {
		t.Fatalf("no generated village: %v", err)
	}
	tp := game.BarbarianTemplate(row.Seed, gx, gy)
	barb, err := g.f.svc.WatchVillage(ctx, hoster, g.id, game.VillageIDFor(gx, gy))
	if err != nil || barb.OwnerID != "" || barb.Buildings != tp.Buildings || barb.Points != tp.Points || barb.X != gx || barb.Y != gy {
		t.Fatalf("generated barbarian %+v %v, template %+v", barb, err, tp)
	}
	for _, id := range []string{"v:0:0x", "nonsense", game.VillageIDFor(9999, 1)} {
		if _, err := g.f.svc.WatchVillage(ctx, hoster, g.id, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("village %q: %v", id, err)
		}
	}
	if after := stored(); after != before {
		t.Fatalf("watching wrote the village:\n%s\n%s", before, after)
	}
	if n := len(actionLog(t, g.f, g.id)); n != logBefore {
		t.Fatalf("watching logged %d entries", n-logBefore)
	}
}

func TestSpectatorOrdersRefused(t *testing.T) {
	g := newWatchedGame(t)
	ctx := context.Background()
	logBefore := len(actionLog(t, g.f, g.id))
	for _, name := range app.OrderNames() {
		body := `{}`
		if name == "send" || name == "schedule-send" {
			body = `{"type":"attack"}`
		}
		if _, err := g.f.svc.Order(ctx, hoster, g.id, name, json.RawMessage(body)); !errors.Is(err, app.ErrNotMember) {
			t.Fatalf("order %s by the host: %v", name, err)
		}
	}
	if _, err := g.f.svc.State(ctx, hoster, g.id); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("state for the host: %v", err)
	}
	if n := len(actionLog(t, g.f, g.id)); n != logBefore {
		t.Fatalf("refused orders logged %d entries", n-logBefore)
	}
}
