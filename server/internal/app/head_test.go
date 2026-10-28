package app_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/app/apptest"
	"librehold/internal/game"
)

func text(s string) app.HeadMessage { return app.HeadMessage{Text: s} }

func plural(forms ...string) app.HeadMessage {
	names := []string{"one", "other"}
	if len(forms) == 4 {
		names = []string{"one", "few", "many", "other"}
	}
	m := app.HeadMessage{Plural: map[string]string{}}
	for i, f := range forms {
		m.Plural[names[i]] = f
	}
	return m
}

var testHeadText = app.HeadText{
	"en": {
		"world.gameName":                text("Game #{n}"),
		"head.formerPlayer":             text("a former player"),
		"head.count.players":            plural("# player", "# players"),
		"head.count.bots":               plural("# bot", "# bots"),
		"head.count.conquests":          plural("# village conquered", "# villages conquered"),
		"head.count.games":              plural("# game", "# games"),
		"head.count.wins":               plural("# win", "# wins"),
		"head.game.ended.title":         text("{name} — won by {winner} · {players}, {conquests}"),
		"head.game.ended.titleBot":      text("{name} — won by {winner} (bot) · {players}, {conquests}"),
		"head.game.ended.description":   text("A finished game."),
		"head.game.running.title":       text("{name} — live now · {players}, {bots}"),
		"head.game.running.description": text("A live game."),
		"head.player.title":             text("{name} — {games}, {wins}, {conquests}"),
		"head.player.description":       text("{name} in LibreHold."),
		"head.world.title":              text("{name} — day {day} · {players} and {bots}"),
		"head.world.description":        text("A long world."),
		"head.worlds.title":             text("Worlds — LibreHold"),
		"head.worlds.description":       text("Every world."),
		"head.join.title":               text("{name} — join the game"),
		"head.join.description":         text("{host} invites you to a game."),
		"head.onlyEnglish":              text("English only"),
	},
	"pl": {
		"world.gameName":        text("Gra #{n}"),
		"head.count.players":    plural("# gracz", "# gracze", "# graczy", "# gracza"),
		"head.count.conquests":  plural("# przejęta wioska", "# przejęte wioski", "# przejętych wiosek", "# przejętej wioski"),
		"head.game.ended.title": text("{name} — wygrywa {winner} · {players}, {conquests}"),
	},
}

func newHeadFixture(t *testing.T) *instantFixture {
	t.Helper()
	return newInstantFixture(t, app.Config{HeadText: testHeadText})
}

func saveGame(t *testing.T, f *instantFixture, g app.ArchivedGame) {
	t.Helper()
	if err := f.worlds.SaveArchive(context.Background(), g); err != nil {
		t.Fatal(err)
	}
}

func head(t *testing.T, f *instantFixture, route, locale string, params map[string]string) (app.PageHead, bool) {
	t.Helper()
	h, ok, err := f.svc.Head(context.Background(), route, locale, params)
	if err != nil {
		t.Fatal(err)
	}
	return h, ok
}

func TestPluralCategoryPolish(t *testing.T) {
	cases := []struct {
		locale string
		n      int
		want   string
	}{{"pl", 1, "one"}, {"pl", 2, "few"}, {"pl", 5, "many"}, {"pl", 12, "many"}, {"pl", 22, "few"}, {"en", 1, "one"}, {"en", 2, "other"}}
	for _, c := range cases {
		if got := app.PluralCategory(c.locale, c.n); got != c.want {
			t.Errorf("%s %d: %s, want %s", c.locale, c.n, got, c.want)
		}
	}
}

func TestHeadTextCountPolish(t *testing.T) {
	for n, want := range map[int]string{1: "1 gracz", 2: "2 gracze", 5: "5 graczy", 12: "12 graczy", 22: "22 gracze"} {
		if got := testHeadText.Count("pl", "head.count.players", n); got != want {
			t.Errorf("%d: %q, want %q", n, got, want)
		}
	}
}

func finishedGame(id int64, winner string, bot bool, humans int) app.ArchivedGame {
	return app.ArchivedGame{WorldID: id, Name: "Game #41", GameNumber: 41, StartedAtMs: 0, EndedAtMs: 3_600_000,
		WinnerName: winner, WinnerIsBot: bot, Summary: game.ArchiveSummary{EndReason: "time", Humans: humans, Bots: 4,
			ConquestsHumans: 20, ConquestsBots: 3},
		Rows: []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: winner, IsBot: bot, Points: 10, Villages: 1}}}
}

func TestHeadForFinishedGame(t *testing.T) {
	f := newHeadFixture(t)
	saveGame(t, f, finishedGame(500, "kind-otter", false, 18))
	saveGame(t, f, finishedGame(501, "kind-otter", false, 1))
	h, ok := head(t, f, "game", "en", map[string]string{"id": "500"})
	if !ok || h.Title != "Game #41 — won by kind-otter · 18 players, 23 villages conquered" || h.Description != "A finished game." ||
		h.NoIndex || h.Image != "" {
		t.Fatalf("en head %+v %v", h, ok)
	}
	if h, _ := head(t, f, "game", "pl", map[string]string{"id": "500"}); !strings.HasPrefix(h.Title, "Gra #41 — ") {
		t.Fatalf("pl title %q", h.Title)
	}
	if h, _ := head(t, f, "game", "en", map[string]string{"id": "501"}); !strings.Contains(h.Title, "· 1 player,") {
		t.Fatalf("one player %q", h.Title)
	}
}

func TestHeadNamesABotWinner(t *testing.T) {
	f := newHeadFixture(t)
	saveGame(t, f, finishedGame(500, "Borko", true, 3))
	if h, _ := head(t, f, "game", "en", map[string]string{"id": "500"}); !strings.Contains(h.Title, "won by Borko (bot)") {
		t.Fatalf("title %q", h.Title)
	}
	g := finishedGame(502, "", false, 2)
	saveGame(t, f, g)
	if h, _ := head(t, f, "game", "en", map[string]string{"id": "502"}); !strings.Contains(h.Title, "won by a former player") {
		t.Fatalf("former player %q", h.Title)
	}
}

func TestHeadForRunningGame(t *testing.T) {
	f := newHeadFixture(t)
	id := f.play(t, f.guest(t)).WorldID
	row, _ := f.worlds.Get(context.Background(), id)
	live, err := f.svc.PublicGame(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	h, ok := head(t, f, "game", "en", map[string]string{"id": itoa(id)})
	want := "Game #" + itoa(int64(row.GameNumber)) + " — live now · 1 player, " + itoa(int64(live.Summary.Bots)) + " bots"
	if !ok || h.Title != want || h.Description != "A live game." {
		t.Fatalf("running head %+v, want %q", h, want)
	}
}

func TestHeadMissingIsNotFound(t *testing.T) {
	f := newHeadFixture(t)
	for _, c := range []struct {
		route  string
		params map[string]string
	}{{"game", map[string]string{"id": "999"}}, {"game", map[string]string{"id": "abc"}}, {"game", map[string]string{"id": "-3"}},
		{"player", map[string]string{"name": "nobody"}}, {"world", map[string]string{"id": "x"}}, {"lobby", nil}} {
		if h, ok := head(t, f, c.route, "en", c.params); ok {
			t.Errorf("%s %v found: %+v", c.route, c.params, h)
		}
	}
}

func TestHeadWorldIsOnlyForLongWorlds(t *testing.T) {
	ctx := context.Background()
	f := newHeadFixture(t)
	running := f.play(t, f.guest(t)).WorldID
	ended, _ := endedGame(t, f)
	for _, id := range []int64{running, ended} {
		if h, ok := head(t, f, "world", "en", map[string]string{"id": itoa(id)}); ok {
			t.Fatalf("instant world %d as a world page: %+v", id, h)
		}
	}
	long, err := f.worlds.Create(ctx, apptest.NewWorldRow(2), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	h, ok := head(t, f, "world", "en", map[string]string{"id": itoa(long)})
	if !ok || !strings.Contains(h.Title, " — day 1 · 0 players and 2 bots") {
		t.Fatalf("long world %+v %v", h, ok)
	}
	if h, ok := head(t, f, "worlds", "en", nil); !ok || h.Title != "Worlds — LibreHold" {
		t.Fatalf("worlds %+v %v", h, ok)
	}
}

func TestHeadGuestProfileIsNoIndex(t *testing.T) {
	f := newHeadFixture(t)
	g := f.guest(t)
	h, ok := head(t, f, "player", "en", map[string]string{"name": g.Username})
	if !ok || !h.NoIndex || !strings.HasPrefix(h.Title, g.Username+" — 0 games, 0 wins, 0 villages conquered") {
		t.Fatalf("guest head %+v %v", h, ok)
	}
}

func TestHeadPlayerIgnoresCase(t *testing.T) {
	ctx := context.Background()
	f := newHeadFixture(t)
	if _, _, err := f.svc.Register(ctx, app.RegisterInput{Username: "ala", Password: "long-password",
		PasswordConfirm: "long-password", Client: "k"}); err != nil {
		t.Fatal(err)
	}
	h, ok := head(t, f, "player", "en", map[string]string{"name": "ALA"})
	if !ok || h.NoIndex || !strings.HasPrefix(h.Title, "ala —") {
		t.Fatalf("head %+v %v", h, ok)
	}
}

func TestHeadFallsBackToEnglish(t *testing.T) {
	if got := testHeadText.Format("pl", "head.onlyEnglish", nil); got != "English only" {
		t.Fatalf("fallback %q", got)
	}
	if got := testHeadText.Format("pl", "head.missing", nil); got != "head.missing" {
		t.Fatalf("missing %q", got)
	}
	if got := testHeadText.Format("en", "head.player.title", map[string]string{"name": "Ala"}); got != "Ala — {games}, {wins}, {conquests}" {
		t.Fatalf("unknown placeholders %q", got)
	}
}

func TestSitemapEntries(t *testing.T) {
	ctx := context.Background()
	f := newHeadFixture(t)
	_, acct, err := f.svc.Register(ctx, app.RegisterInput{Username: "ala", Password: "long-password",
		PasswordConfirm: "long-password", Client: "k"})
	if err != nil {
		t.Fatal(err)
	}
	guest := f.guest(t)
	g := finishedGame(500, "ala", false, 2)
	g.EndedAtMs = 7_200_000
	g.Rows = []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "ala", AccountID: acct.ID}, {Rank: 2, PlayerID: "h2", Name: guest.Username,
		AccountID: guest.ID}}
	saveGame(t, f, g)
	running := f.play(t, f.guest(t)).WorldID
	long, err := f.worlds.Create(ctx, apptest.NewWorldRow(1), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	entries, err := f.svc.SitemapEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]app.SitemapEntry{}
	for _, e := range entries {
		key := e.Route
		for _, v := range e.Params {
			key += ":" + v
		}
		got[key] = e
	}
	if _, ok := got["world:"+itoa(long)]; !ok {
		t.Errorf("long world missing: %+v", entries)
	}
	if _, ok := got["world:"+itoa(running)]; ok {
		t.Errorf("instant world listed as a world: %+v", entries)
	}
	if e, ok := got["game:500"]; !ok || !e.LastMod.Equal(time.UnixMilli(7_200_000)) {
		t.Errorf("game: %+v", e)
	}
	if e, ok := got["player:ala"]; !ok || !e.LastMod.Equal(time.UnixMilli(7_200_000)) {
		t.Errorf("player: %+v", e)
	}
	if _, ok := got["player:"+guest.Username]; ok {
		t.Errorf("guest listed: %+v", entries)
	}
	if len(entries) != 3 {
		t.Errorf("entries %+v", entries)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
