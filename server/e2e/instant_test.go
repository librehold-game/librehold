//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/adapters/postgres/pgtest"
	"librehold/internal/app"
	"librehold/internal/game"
)

func dbPool(t *testing.T, db string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), pgtest.URL(db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestColdGuestPlaysWithinSeconds(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1)
	start := time.Now()
	g := s.guest(t)
	wid, wait := g.play(t)
	if wid == 0 {
		t.Fatalf("no seat for the first guest, wait %d ms", wait)
	}
	st := g.state(wid)
	took := time.Since(start)
	t.Logf("guest → play → state: %v", took)
	if st.Instant == nil || len(st.Villages) != 1 {
		t.Fatalf("state %+v", st.Instant)
	}
	home := g.home(wid)
	if home.Units[game.UNoble] != 1 || home.Buildings[game.BMain] != game.InstantStartVillage.Buildings[game.BMain] {
		t.Fatalf("the preset village: %+v", home)
	}
	if took > 2*time.Second {
		t.Fatalf("a cold guest waited %v", took)
	}
}

func TestGuestClaimsMidGame(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1)
	g := s.guest(t)
	wid, _ := g.play(t)
	home := g.home(wid)
	g.claim(t, "Cat")
	var me struct {
		Username string `json:"username"`
		IsGuest  bool   `json:"isGuest"`
	}
	g.ok("GET", "/api/v1/me", nil, &me)
	if me.Username != "Cat" || me.IsGuest {
		t.Fatalf("me after claim %+v", me)
	}
	st := g.state(wid)
	if g.home(wid).ID != home.ID || st.Player.Name != "Cat" {
		t.Fatalf("after claim: village %s (was %s), name %q", g.home(wid).ID, home.ID, st.Player.Name)
	}
	again := s.login(t, "Cat", password)
	if again.home(wid).ID != home.ID {
		t.Fatal("the claimed account does not log in to its game")
	}
}

func TestInstantWorldEndsArchivesAndPurges(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 120, "LH_INSTANT_KEEP_MIN=1", "LH_INSTANT_SWEEP_SEC=1")
	g := s.guest(t)
	wid, _ := g.play(t)
	path := fmt.Sprintf("/api/v1/public/games/%d", wid)
	var archived app.ArchivedGame
	eventually(t, 3*time.Minute, "the game's results", func() bool {
		r := g.do("GET", path, nil)
		return r.status == 200 && json.Unmarshal(r.body, &archived) == nil
	})
	if archived.Summary.Humans != 1 || archived.Summary.Bots == 0 || len(archived.Rows) != archived.Summary.Humans+archived.Summary.Bots {
		t.Fatalf("results %+v", archived)
	}
	pool := dbPool(t, s.db)
	eventually(t, 3*time.Minute, "the purge", func() bool {
		return count(t, pool, `select count(*) from worlds where id = $1`, wid) == 0
	})
	if n := count(t, pool, `select count(*) from instant_leaderboard where world_id = $1`, wid); n != len(archived.Rows) {
		t.Fatalf("%d leaderboard rows after the purge", n)
	}
	if r := g.do("GET", path, nil); r.status != 200 {
		t.Fatalf("results after the purge: %d %s", r.status, r.body)
	}
	if n := count(t, pool, `select count(*) from action_log where world_id = $1`, wid); n != 0 {
		t.Fatalf("%d log rows left after packing", n)
	}
	var rows, size int
	if err := pool.QueryRow(context.Background(), `select rows, length(data) from action_log_packed where world_id = $1`, wid).
		Scan(&rows, &size); err != nil {
		t.Fatal(err)
	}
	t.Logf("packed action log of an hour-long instant world (1 guest, %d bots): %d entries, %d bytes", archived.Summary.Bots, rows, size)
	if r := g.do("GET", path+"/timeline", nil); r.status != 200 {
		t.Fatalf("timeline after the purge: %d %s", r.status, r.body)
	}
	var entries, packed int
	if err := pool.QueryRow(context.Background(), `select length(data), entries from timeline_packed where world_id = $1`, wid).
		Scan(&packed, &entries); err != nil {
		t.Fatal(err)
	}
	t.Logf("packed timeline of an hour-long instant world (1 guest, %d bots): %d entries, %d bytes", archived.Summary.Bots, entries, packed)
	var stats app.PublicStats
	g.ok("GET", "/api/v1/public/stats", nil, &stats)
	if stats.AllTime.Games != 1 || stats.AllTime.Players != 1 || stats.Visible {
		t.Fatalf("stats %+v", stats)
	}
}

// orderP95 sends n mixed orders from each player at once and returns the 95th percentile of their latency.
func orderP95(t *testing.T, wid int64, players []*client, n int) time.Duration {
	t.Helper()
	homes := map[*client]*game.Village{}
	barbs := map[*client]*app.MapTile{}
	for _, c := range players {
		v := c.home(wid)
		homes[c] = v
		barbs[c] = nearest(c.tiles(wid, v.X, v.Y, 20), v, func(t app.MapTile) bool { return t.OwnerID == "" })
	}
	orders := []func(c *client, i int) (string, any){
		func(c *client, _ int) (string, any) {
			return "build", map[string]string{"villageId": homes[c].ID, "building": "wood"}
		},
		func(c *client, _ int) (string, any) {
			return "recruit", map[string]any{"villageId": homes[c].ID, "unit": "spear", "count": 1}
		},
		func(c *client, i int) (string, any) {
			return "rename-village", map[string]string{"villageId": homes[c].ID, "name": fmt.Sprintf("Café Hill %d", i)}
		},
		func(c *client, _ int) (string, any) {
			b := barbs[c]
			return "send", map[string]any{"originId": homes[c].ID, "x": b.X, "y": b.Y, "units": map[string]int{"scout": 1}, "type": "attack"}
		},
	}
	var mu sync.Mutex
	var took []time.Duration
	var wg sync.WaitGroup
	for _, c := range players {
		wg.Go(func() {
			for i := range n {
				name, body := orders[i%len(orders)](c, i)
				start := time.Now()
				r := c.order(wid, name, body)
				d := time.Since(start)
				if r.status != 200 && r.status != 409 {
					t.Errorf("%s: %d %s", name, r.status, r.body)
					return
				}
				mu.Lock()
				took = append(took, d)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return percentile(took, 0.95)
}

func TestInstantLoadDoesNotSlowLongWorld(t *testing.T) {
	s := startServerAt(t, newDB(t), time.Now().UnixMilli(), 1)
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

	worlds := map[int64]int{}
	for len(worlds) < 4 || worlds[maxKey(worlds)] < game.InstantWorldConfig.HumanSeats {
		w, wait := s.guest(t).play(t)
		if w == 0 {
			t.Fatalf("no seat with %d instant worlds (%v), wait %d ms", len(worlds), worlds, wait)
		}
		worlds[w]++
	}
	if _, wait := s.guest(t).play(t); wait == 0 {
		t.Fatal("a fifth instant world opened past the cap")
	}
	time.Sleep(5 * time.Second)
	loaded := orderP95(t, wid, players, 150)
	t.Logf("long-world order p95: alone %v, beside %d full instant worlds %v", alone, len(worlds), loaded)
	if loaded > alone*3/2+5*time.Millisecond || loaded > 200*time.Millisecond {
		t.Fatalf("p95 went from %v to %v", alone, loaded)
	}
}

func maxKey(m map[int64]int) int64 {
	var k int64
	for id := range m {
		k = max(k, id)
	}
	return k
}
