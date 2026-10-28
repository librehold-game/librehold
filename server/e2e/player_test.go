//go:build e2e

package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

const guessed = "guessedguessedguesse"

func TestHostedGameEndToEnd(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1)
	host := s.register(t, "gamehost")
	card := host.host(t, app.HostForm{Name: "Friday e2e", DurationMin: 30, Seats: 10, Bots: ptr(5), SightRadius: ptr(3),
		DominationPct: ptr(0)})
	wid, invite := card.WorldID, card.InviteCode

	var guests []*client
	for i := range 10 {
		g := s.guest(t)
		start := time.Now()
		res, r := g.joinInvite(t, invite)
		if r.status != 200 || res.WorldID != wid {
			t.Fatalf("guest %d: %d %s", i, r.status, r.body)
		}
		st := g.state(wid)
		if took := time.Since(start); took > 2*time.Second || st.Hosted == nil || !st.Hosted.Waiting {
			t.Fatalf("guest %d joined in %v, hosted %+v", i, took, st.Hosted)
		}
		guests = append(guests, g)
	}
	if _, r := s.guest(t).joinInvite(t, invite); r.status != 400 || r.code() != "worldFull" {
		t.Fatalf("the eleventh: %d %s", r.status, r.body)
	}
	stranger := s.guest(t)
	for _, r := range []reply{stranger.do("GET", "/api/v1/public/invites/"+guessed, nil),
		stranger.do("POST", "/api/v1/invites/"+guessed+"/join", map[string]string{}),
		stranger.do("GET", "/api/v1/public/games/"+guessed, nil)} {
		if r.status != 404 {
			t.Fatalf("a guessed code: %d %s", r.status, r.body)
		}
	}
	if r := host.do("GET", wpath(wid, "/state"), nil); r.status != 403 {
		t.Fatalf("the host's state: %d %s", r.status, r.body)
	}
	for _, name := range app.OrderNames() {
		body := map[string]string{}
		if name == "send" || name == "schedule-send" {
			body["type"] = "attack"
		}
		if r := host.order(wid, name, body); r.status != 403 {
			t.Fatalf("the host's %s: %d %s", name, r.status, r.body)
		}
	}

	if r := host.hostAction(wid, "start", map[string]string{}); r.status != 200 {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	attacker := guests[0]
	home := attacker.home(wid)
	barb := nearest(attacker.tiles(wid, home.X, home.Y, 10), home, func(t app.MapTile) bool { return t.OwnerID == "" })
	sent := attacker.mustOrder(wid, "send", map[string]any{"originId": home.ID, "x": barb.X, "y": barb.Y,
		"units": map[string]int{"axe": 10}, "type": "attack"})
	if len(sent.Commands) == 0 {
		t.Fatal("the attack did not leave")
	}
	var watch app.SpectatorState
	host.ok("GET", wpath(wid, "/watch"), nil, &watch)
	owners := map[game.PlayerRef]bool{}
	for _, v := range watch.Villages {
		owners[v.OwnerID] = true
	}
	for id := range watch.Players {
		if !owners[game.Ref(id)] {
			t.Errorf("player %s has no village on the god view", id)
		}
	}
	if len(watch.Players) != 15 || !slices.ContainsFunc(watch.Commands, func(c app.CommandView) bool {
		return c.OriginID == home.ID && c.Units != nil && c.Units[game.UAxe] == 10
	}) {
		t.Fatalf("god view: %d players, commands %+v", len(watch.Players), watch.Commands)
	}

	kicked := guests[3]
	var kickedID game.PlayerID
	for id, p := range watch.Players {
		if p.Name == kicked.name {
			kickedID = id
		}
	}
	if r := host.hostAction(wid, "kick", map[string]any{"playerId": kickedID}); r.status != 200 {
		t.Fatalf("kick: %d %s", r.status, r.body)
	}
	if r := kicked.do("GET", wpath(wid, "/state"), nil); r.status != 403 {
		t.Fatalf("the kicked guest's state: %d %s", r.status, r.body)
	}
	if _, r := kicked.joinInvite(t, invite); r.status != 400 || r.code() != "banned" {
		t.Fatalf("the kicked guest rejoins: %d %s", r.status, r.body)
	}

	ended := host.hostAction(wid, "end", map[string]string{})
	var final app.HostedCard
	if ended.status != 200 || json.Unmarshal(ended.body, &final) != nil || final.Status != "ended" || final.ResultsCode == "" {
		t.Fatalf("end: %d %s", ended.status, ended.body)
	}
	code := final.ResultsCode
	var results struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	stranger.ok("GET", "/api/v1/public/games/"+code, nil, &results)
	if results.Kind != "player" || results.Name != "Friday e2e" {
		t.Fatalf("results %+v", results)
	}
	tl := s.fetch(t, "GET", "/api/v1/public/games/"+code+"/timeline", "", "Accept-Encoding", "gzip")
	if tl.status != 200 || tl.header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("timeline: %d %v", tl.status, tl.header)
	}
	zr, err := gzip.NewReader(strings.NewReader(tl.body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(zr)
	if !bytes.Contains(raw, []byte(`"v":1`)) {
		t.Fatalf("timeline body %.200s", raw)
	}
	if r := stranger.do("GET", fmt.Sprintf("/api/v1/public/games/%d", wid), nil); r.status != 404 {
		t.Fatalf("the game by its id: %d %s", r.status, r.body)
	}
	public := []string{s.fetch(t, "GET", "/sitemap.xml", "").body}
	for _, path := range []string{"/api/v1/public/worlds", "/api/v1/public/live", "/api/v1/public/stats"} {
		public = append(public, string(stranger.do("GET", path, nil).body))
	}
	for i, body := range public {
		for _, secret := range []string{"Friday e2e", invite, code} {
			if strings.Contains(body, secret) {
				t.Errorf("public answer %d holds %q", i, secret)
			}
		}
	}
}

// quiet sends a request and keeps only its status: the load test's clients would otherwise hold every body.
func (c *client) quiet(method, path string, body any) int {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.s.url+path, rd)
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.s.url)
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return res.StatusCode
}

func hasTopic(m map[string]any, topic string) bool {
	ts, _ := m["topics"].([]any)
	return slices.ContainsFunc(ts, func(t any) bool { return t == topic })
}

// busyPlayerWorld hosts a started player world of 5 bots and 10 guests; every guest keeps its live socket,
// refetches its state on each state update and sends an order every 20 s until ctx ends.
func busyPlayerWorld(t *testing.T, s *server, n int, ctx context.Context, wg *sync.WaitGroup) int64 {
	t.Helper()
	host := s.register(t, fmt.Sprintf("gamehost%d", n))
	card := host.host(t, app.HostForm{Name: fmt.Sprintf("Load %d", n), Seats: 10, Bots: ptr(5)})
	wid := card.WorldID
	type member struct {
		c    *client
		home string
	}
	var members []member
	for range 10 {
		g := s.guest(t)
		if _, r := g.joinInvite(t, card.InviteCode); r.status != 200 {
			t.Fatalf("join: %d %s", r.status, r.body)
		}
		members = append(members, member{g, g.home(wid).ID})
	}
	if r := host.hostAction(wid, "start", map[string]string{}); r.status != 200 {
		t.Fatalf("start: %d %s", r.status, r.body)
	}
	for i, m := range members {
		sock := m.c.live(wid)
		wg.Go(func() {
			tick := time.NewTicker(20 * time.Second)
			defer tick.Stop()
			for k := i; ; k++ {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-sock.msgs:
					if !ok {
						return
					}
					if msg["type"] == "update" && hasTopic(msg, app.TopicState) {
						m.c.quiet("GET", wpath(wid, "/state"), nil)
					}
				case <-tick.C:
					if k%2 == 0 {
						m.c.quiet("POST", wpath(wid, "/orders/build"), map[string]string{"villageId": m.home, "building": "wood"})
					} else {
						m.c.quiet("POST", wpath(wid, "/orders/recruit"), map[string]any{"villageId": m.home, "unit": "spear", "count": 1})
					}
				}
			}
		})
	}
	return wid
}

// TestPlayerWorldsLoadDoesNotSlowLongWorld: a long world's order latency beside four full instant worlds and K busy
// player worlds. LH_LOAD_SEARCH=1 doubles K from 2 to 64 while it holds, to measure the safety cap.
func TestPlayerWorldsLoadDoesNotSlowLongWorld(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1,
		"LH_REGISTRATIONS_PER_HOUR=100000", "LH_GUESTS_PER_HOUR=100000", "LH_PLAYER_MAX_WORLDS=1000")
	a := s.admin(t)
	wid := a.world(func(cfg *game.WorldConfig) { cfg.BotCount, cfg.BeginnerProtectionDays = 100, 0 })
	var players []*client
	for _, name := range []string{"ala", "bob"} {
		c := s.register(t, name)
		c.join(wid)
		boost(t, s.db, wid, c.home(wid).ID, army)
		players = append(players, c)
	}
	alone := orderP95(t, wid, players, 150)

	instant := map[int64]int{}
	for len(instant) < 4 || instant[maxKey(instant)] < game.InstantWorldConfig.HumanSeats {
		w, wait := s.guest(t).play(t)
		if w == 0 {
			t.Fatalf("no seat with %d instant worlds, wait %d ms", len(instant), wait)
		}
		instant[w]++
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(wg.Wait)
	t.Cleanup(cancel)
	hosted := 0
	measure := func(k int) bool {
		for ; hosted < k; hosted++ {
			busyPlayerWorld(t, s, hosted, ctx, &wg)
		}
		time.Sleep(30 * time.Second)
		loaded := orderP95(t, wid, players, 150)
		pass := loaded <= alone*3/2+5*time.Millisecond && loaded <= 200*time.Millisecond
		t.Logf("K=%d p95 alone=%v loaded=%v pass=%v", k, alone, loaded, pass)
		return pass
	}
	if os.Getenv("LH_LOAD_SEARCH") != "1" {
		if !measure(4) {
			t.Fatal("four player worlds slowed the long world")
		}
		return
	}
	for k := 2; k <= 64; k *= 2 {
		if !measure(k) {
			break
		}
	}
}
