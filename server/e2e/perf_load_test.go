//go:build e2e && perf

package e2e

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

func TestPerfInstantLoadDoesNotSlowLongWorld(t *testing.T) {
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
	for range 4 {
		w, wait := s.guest(t).play(t)
		if w == 0 {
			t.Fatalf("no seat with %d instant worlds (%v), wait %d ms", len(worlds), worlds, wait)
		}
		worlds[w]++
	}
	for w := range worlds {
		for worlds[w] < game.InstantWorldConfig.HumanSeats {
			s.guest(t).playIn(t, w)
			worlds[w]++
		}
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

// TestPerfPlayerWorldsLoadDoesNotSlowLongWorld: a long world's order latency beside four full instant worlds and K busy
// player worlds. LH_LOAD_SEARCH=1 doubles K from 2 to 64 while it holds, to measure the safety cap.
func TestPerfPlayerWorldsLoadDoesNotSlowLongWorld(t *testing.T) {
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
	for range 4 {
		w, wait := s.guest(t).play(t)
		if w == 0 {
			t.Fatalf("no seat with %d instant worlds, wait %d ms", len(instant), wait)
		}
		instant[w]++
	}
	for w := range instant {
		for instant[w] < game.InstantWorldConfig.HumanSeats {
			s.guest(t).playIn(t, w)
			instant[w]++
		}
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
