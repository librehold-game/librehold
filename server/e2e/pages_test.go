//go:build e2e

package e2e

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

var titleTag = regexp.MustCompile(`<title>([^<]+)</title>`)

func TestPublicSitePages(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), timeScale, "LH_TRUSTED_PROXIES=127.0.0.1/32")

	// A guest raids the nearest barbarian village; the battle shows on the live strip.
	g := s.guest(t)
	wid, _ := g.play(t)
	st := g.state(wid)
	home := g.home(wid)
	// A bot may take the barbarian between the map read and the order, so a blocked order picks again.
	eventually(t, 10*time.Second, "a raid on a barbarian village", func() bool {
		target := nearest(g.tiles(wid, home.X, home.Y, 8), home, func(t app.MapTile) bool { return t.OwnerID == "" })
		if target == nil {
			t.Fatal("no barbarian village near the guest")
		}
		r := g.order(wid, "send", map[string]any{"originId": home.ID, "x": target.X, "y": target.Y, "type": "attack",
			"units": game.UnitCounts{game.UAxe: 100}})
		if r.status != 200 && r.code() != "blocked" {
			t.Fatalf("send: %d %s", r.status, r.body)
		}
		return r.status == 200
	})
	var live app.LiveStrip
	eventually(t, 20*time.Second, "the raid on the live strip", func() bool {
		g.ok("GET", "/api/v1/public/live", nil, &live)
		return len(live.Events) > 0
	})
	e := live.Events[0]
	if live.Games != 1 || live.Humans != 1 || live.Bots != game.InstantWorldConfig.BotCount || e.Attacker != g.name || !e.Instant ||
		e.GameNumber != st.GameNumber || e.GameNumber == 0 {
		t.Fatalf("live %+v, first event %+v (guest %s, game number %d)", live, e, g.name, st.GameNumber)
	}

	var worlds app.PublicWorldList
	g.ok("GET", "/api/v1/public/worlds", nil, &worlds)
	if len(worlds.Running) != 1 || worlds.Running[0].ID != wid {
		t.Fatalf("running %+v", worlds.Running)
	}
	long := s.admin(t).world(func(cfg *game.WorldConfig) { cfg.BotCount = 5 })
	var m app.PublicMap
	g.ok("GET", fmt.Sprintf("/api/v1/public/worlds/%d/villages", long), nil, &m)
	if m.Size != game.DefaultWorldConfig.MapSize || len(m.Villages) == 0 {
		t.Fatalf("map of the long world: size %d, %d villages", m.Size, len(m.Villages))
	}
	if r := g.do("GET", fmt.Sprintf("/api/v1/public/worlds/%d", wid), nil); r.status != 200 {
		t.Fatalf("a running instant game as a public world: %d", r.status)
	}
	if r := g.do("GET", fmt.Sprintf("/api/v1/public/worlds/%d/ranking", wid), nil); r.status != 200 {
		t.Fatalf("its ranking: %d", r.status)
	}

	// Five reports an hour from one internet address; the server's own network is not limited per address.
	visitor := &client{t: t, s: s, forwarded: "203.0.113.7"}
	for i := range 6 {
		r := visitor.do("POST", "/api/v1/reports", map[string]string{"kind": "bug", "text": "The map stays black after login."})
		want := 201
		if i == 5 {
			want = 429
		}
		if r.status != want || (i == 5 && r.code() != "tooManyReports") {
			t.Fatalf("report %d: %d %s", i+1, r.status, r.body)
		}
	}

	// Every page of the site, in both languages, with its own title and canonical address.
	longID := strconv.FormatInt(long, 10)
	pages := []string{
		"/how-to-play", "/how-to-play/first-hour", "/units", "/units/spearman", "/buildings", "/buildings/town-hall",
		"/faq", "/changelog", "/terms", "/worlds", "/worlds/" + longID, "/report",
		"/pl/jak-grac", "/pl/jak-grac/pierwsza-godzina", "/pl/jednostki", "/pl/jednostki/pikinier", "/pl/budynki",
		"/pl/budynki/ratusz", "/pl/faq", "/pl/zmiany", "/pl/regulamin", "/pl/swiaty", "/pl/swiaty/" + longID, "/pl/zglos",
	}
	titles := map[string]string{}
	for _, path := range pages {
		p := s.fetch(t, "GET", path, "")
		m := titleTag.FindStringSubmatch(p.body)
		if p.status != 200 || m == nil || !strings.Contains(p.body, `<link rel="canonical" href="`+s.url+path+`">`) {
			t.Fatalf("%s: %d, title %v, canonical missing: %v", path, p.status, m, !strings.Contains(p.body, `rel="canonical"`))
		}
		if other, dup := titles[m[1]]; dup {
			t.Errorf("%s and %s share the title %q", path, other, m[1])
		}
		titles[m[1]] = path
		form := strings.HasSuffix(path, "/report") || strings.HasSuffix(path, "/zglos")
		if noindex := strings.Contains(p.body, `content="noindex`); noindex != form {
			t.Errorf("%s: noindex %v", path, noindex)
		}
	}
	for _, path := range []string{"/units/nope", "/pl/jak-grac/nic", "/worlds/999999"} {
		if p := s.fetch(t, "GET", path, ""); p.status != 404 {
			t.Errorf("%s: %d, want 404", path, p.status)
		}
	}

	if p := s.fetch(t, "GET", "/worlds/"+strconv.FormatInt(wid, 10), ""); p.status != 200 || !strings.Contains(p.body, `content="noindex`) {
		t.Errorf("a running instant game's world page: %d, noindex %v", p.status, strings.Contains(p.body, `content="noindex`))
	}

	var sm struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(s.fetch(t, "GET", "/sitemap.xml", "").body), &sm); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, u := range sm.URLs {
		listed[strings.TrimPrefix(u.Loc, s.url)] = true
	}
	for _, path := range []string{"/how-to-play/first-hour", "/pl/jednostki/pikinier", "/buildings/town-hall", "/faq", "/changelog",
		"/terms", "/worlds/" + longID} {
		if !listed[path] {
			t.Errorf("the sitemap misses %s", path)
		}
	}
	if listed["/report"] || listed["/pl/zglos"] {
		t.Error("the sitemap lists the report form")
	}
}
