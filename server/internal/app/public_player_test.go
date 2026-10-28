package app_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

// playedGames is an instant game and a player world, both ended and archived, with Kasia in both and Tomek only in
// the player world.
type playedGames struct {
	f               *instantFixture
	kasia, tomek    *app.Account
	instant, hosted int64
	code            string
}

func (f *instantFixture) registered(t *testing.T, name string) *app.Account {
	t.Helper()
	f.n++
	_, a, err := f.svc.Register(context.Background(), app.RegisterInput{Username: name, Password: "long-password",
		PasswordConfirm: "long-password", Client: "10.1.0." + strconv.Itoa(f.n), Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func playGames(t *testing.T, f *instantFixture) *playedGames {
	t.Helper()
	ctx := context.Background()
	p := &playedGames{f: f, kasia: f.registered(t, "Kasia"), tomek: f.registered(t, "Tomek")}
	p.instant = f.play(t, p.kasia).WorldID
	card := f.host(t, hoster, hostForm("Our Friday"))
	p.hosted, p.code = card.WorldID, f.row(t, card.WorldID).Host.ResultsCode
	for _, a := range []*app.Account{p.kasia, p.tomek} {
		if _, err := f.svc.JoinByInvite(ctx, a, card.InviteCode); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.HostStart(ctx, hoster, p.hosted); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ResolveGameRef(ctx, p.code); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("a running player world by its code: %v", err)
	}
	f.clock.Add(2 * game.MsPerMinute)
	if err := f.svc.HostEnd(ctx, hoster, p.hosted); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(time.Hour.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, p.instant, 0); err != nil {
		t.Fatal(err)
	}
	if f.row(t, p.instant).State.EndedAt == nil {
		t.Fatal("the instant game did not end")
	}
	return p
}

func TestResultsReachableOnlyByCode(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	p := playGames(t, f)
	check := func(stage string) {
		t.Helper()
		if id, err := f.svc.ResolveGameRef(ctx, p.code); err != nil || id != p.hosted {
			t.Fatalf("%s: by code %d %v", stage, id, err)
		}
		if id, err := f.svc.ResolveGameRef(ctx, strconv.FormatInt(p.instant, 10)); err != nil || id != p.instant {
			t.Fatalf("%s: the instant game by id %d %v", stage, id, err)
		}
		for _, ref := range []string{strconv.FormatInt(p.hosted, 10), "guessedguessedguesse", "", "-1"} {
			if _, err := f.svc.ResolveGameRef(ctx, ref); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("%s: ref %q: %v", stage, ref, err)
			}
		}
		if g, err := f.svc.PublicGame(ctx, p.hosted); err != nil || g.Name != "Our Friday" || g.HostName != "gamehost" {
			t.Fatalf("%s: game %+v %v", stage, g, err)
		}
	}
	check("ended")
	st, err := f.svc.State(ctx, p.kasia, p.hosted)
	if err != nil || st.Instant.Ended == nil || st.Instant.Ended.ResultsRef != p.code {
		t.Fatalf("player world ended view %+v %v", st.Instant, err)
	}
	if st, err := f.svc.State(ctx, p.kasia, p.instant); err != nil || st.Instant.Ended.ResultsRef != strconv.FormatInt(p.instant, 10) {
		t.Fatalf("instant ended view %+v %v", st.Instant, err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	check("archived")
	f.clock.Add(25 * time.Hour.Milliseconds())
	if _, err := f.svc.PurgeEndedInstant(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.worlds.Get(ctx, p.hosted); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("not purged: %v", err)
	}
	check("purged")
}

func TestProfileListsPlayerGamesOnlyToOwner(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	p := playGames(t, f)
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []*app.Account{nil, p.tomek} {
		prof, err := f.svc.PublicProfile(ctx, "Kasia", viewer)
		if err != nil || len(prof.Games) != 1 || prof.Games[0].WorldID != p.instant || prof.Tally.Games != 1 || prof.Games[0].Code != "" {
			t.Fatalf("seen by %v: %+v %v", viewer, prof, err)
		}
	}
	own, err := f.svc.PublicProfile(ctx, "Kasia", p.kasia)
	if err != nil || len(own.Games) != 2 || own.Tally.Games != 2 {
		t.Fatalf("own %+v %v", own, err)
	}
	i := slices.IndexFunc(own.Games, func(g app.GameRef) bool { return g.WorldID == p.hosted })
	if i < 0 || own.Games[i].Code != p.code || own.Games[i].Kind != game.KindPlayer {
		t.Fatalf("own player game %+v", own.Games)
	}
}

func TestPlayerWorldsInNoPublicList(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	p := playGames(t, f)
	running := f.host(t, ola, hostForm("Still going"))
	if _, err := f.svc.JoinByInvite(ctx, member(7), running.InviteCode); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HostStart(ctx, ola, running.WorldID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	entries, err := f.svc.SitemapEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var kasia bool
	for _, e := range entries {
		id := e.Params["id"]
		if id == strconv.FormatInt(p.hosted, 10) || id == strconv.FormatInt(running.WorldID, 10) || e.Params["name"] == "Tomek" {
			t.Fatalf("sitemap lists %+v", e)
		}
		kasia = kasia || e.Params["name"] == "Kasia"
	}
	if !kasia {
		t.Fatalf("sitemap misses Kasia: %+v", entries)
	}
	if stats, err := f.svc.PublicStats(ctx); err != nil || stats.AllTime.Games != 1 {
		t.Fatalf("stats %+v %v", stats, err)
	}
	list, err := f.svc.PublicWorlds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range append(list.Running, list.Long...) {
		if c.ID == p.hosted || c.ID == running.WorldID {
			t.Fatalf("public worlds list %+v", c)
		}
	}
	for _, g := range list.Finished {
		if g.WorldID == p.hosted {
			t.Fatalf("finished list %+v", g)
		}
	}
	live, err := f.svc.Live(ctx)
	if err != nil || live.Games != 0 || live.Humans != 0 {
		t.Fatalf("live %+v %v", live, err)
	}
	for _, id := range []int64{p.hosted, running.WorldID} {
		if _, err := f.svc.PublicWorld(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("public world %d: %v", id, err)
		}
	}
}

func TestHeadForCodesIsNoIndex(t *testing.T) {
	f := newHeadFixture(t)
	p := playGames(t, f)
	h, ok := head(t, f, "game", "en", map[string]string{"id": p.code})
	if !ok || !h.NoIndex || !strings.HasPrefix(h.Title, "Our Friday — won by") {
		t.Fatalf("game by code %+v %v", h, ok)
	}
	if _, ok := head(t, f, "game", "en", map[string]string{"id": strconv.FormatInt(p.hosted, 10)}); ok {
		t.Fatal("a player game by its id has a head")
	}
	if h, ok := head(t, f, "game", "en", map[string]string{"id": strconv.FormatInt(p.instant, 10)}); !ok || h.NoIndex {
		t.Fatalf("instant game %+v %v", h, ok)
	}
	card := f.host(t, ola, hostForm("Come along"))
	h, ok = head(t, f, "join", "en", map[string]string{"code": card.InviteCode})
	if !ok || !h.NoIndex || h.Title != "Come along — join the game" || h.Description == "" {
		t.Fatalf("join %+v %v", h, ok)
	}
	if _, ok := head(t, f, "join", "en", map[string]string{"code": "nosuchcodenosuchcode"}); ok {
		t.Fatal("an unknown invite has a head")
	}
	if _, ok := head(t, f, "world", "en", map[string]string{"id": strconv.FormatInt(card.WorldID, 10)}); ok {
		t.Fatal("a player world has a long world's head")
	}
}
