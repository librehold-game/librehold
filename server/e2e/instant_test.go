//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestGuestJoinsRunningGameLate(t *testing.T) {
	s := startServer(t, newDB(t), time.Now().UnixMilli())
	first := s.guest(t)
	wid, _ := first.play(t)
	// Past the old ten-minute join window, with most of the hour left: the server runs a hundred times faster here, and
	// its state calls get slower as the bots play, so waiting longer risks the game ending first.
	eventually(t, time.Minute, "11 game minutes", func() bool {
		return first.state(wid).Now >= 11*game.MsPerMinute
	})
	late := s.guest(t)
	var list app.InstantList
	late.ok("GET", "/api/v1/instant/games", nil, &list)
	if len(list.Games) != 1 || list.Games[0].ID != wid || list.Games[0].Humans != 1 || list.Games[0].Seats != game.InstantWorldConfig.HumanSeats {
		t.Fatalf("list %+v", list)
	}
	var res app.PlayResult
	late.ok("POST", "/api/v1/instant/play", map[string]int64{"worldId": wid}, &res)
	if res.WorldID != wid {
		t.Fatalf("play %+v", res)
	}
	if st := late.state(wid); st.Instant == nil || len(st.Villages) != 1 {
		t.Fatalf("late joiner's state %+v", st.Instant)
	}
}
