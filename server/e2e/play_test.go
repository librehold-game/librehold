//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

const worldMinute = int64(60_000)

func nearest(tiles []app.MapTile, from *game.Village, keep func(t app.MapTile) bool) *app.MapTile {
	var best *app.MapTile
	bestD := 0.0
	for i, t := range tiles {
		if t.ID == from.ID || !keep(t) {
			continue
		}
		if d := game.Distance(from.X, from.Y, t.X, t.Y); best == nil || d < bestD {
			best, bestD = &tiles[i], d
		}
	}
	return best
}

func countReports(rs []game.Report, keep func(r game.Report) bool) int {
	n := 0
	for _, r := range rs {
		if keep(r) {
			n++
		}
	}
	return n
}

func units(m map[string]int) map[string]int { return m }

// DoD 3 and 7: two humans play every kind of order against barbarians, a bot and each other.
func TestPlayLoop(t *testing.T) {
	s := newServer(t)
	a := s.admin(t)
	wid := a.world(fast(10))
	ala, bob := s.register(t, "ala"), s.register(t, "bob")
	// Opposite quadrants keep the homes far enough apart for Ala to dodge Bob's rams (a dodge needs two cancel
	// windows of march).
	ala.joinIn(wid, "nw")
	bob.joinIn(wid, "se")
	av, bv := ala.home(wid), bob.home(wid)
	for _, v := range []*game.Village{av, bv} {
		boost(t, s.db, wid, v.ID, func(v *game.Village) {
			army(v)
			v.Research[game.URam] = 0
		})
	}
	homes := map[*client]*game.Village{ala: av, bob: bv}

	for _, c := range []*client{ala, bob} {
		v := homes[c]
		c.mustOrder(wid, "build", map[string]string{"villageId": v.ID, "building": "main"})
		c.mustOrder(wid, "recruit", map[string]any{"villageId": v.ID, "unit": "spear", "count": 10})
		c.mustOrder(wid, "research", map[string]string{"villageId": v.ID, "unit": "ram"})
		st := c.mustOrder(wid, "mint", map[string]string{"villageId": v.ID})
		if st.Player.Coins != 1 {
			t.Fatalf("%s: coins %d after minting", c.name, st.Player.Coins)
		}
	}

	// Scout a barbarian village and attack a bot, each human.
	barbs := map[*client]*app.MapTile{}
	for _, c := range []*client{ala, bob} {
		v := homes[c]
		tiles := c.tiles(wid, v.X, v.Y, 50)
		barb := nearest(tiles, v, func(t app.MapTile) bool { return t.OwnerID == "" })
		bot := nearest(tiles, v, func(t app.MapTile) bool { return strings.HasPrefix(string(t.OwnerID), "bot_") })
		if barb == nil || bot == nil {
			t.Fatalf("%s: no barbarian or bot near %d|%d", c.name, v.X, v.Y)
		}
		barbs[c] = barb
		c.mustOrder(wid, "send", map[string]any{"originId": v.ID, "x": barb.X, "y": barb.Y, "units": units(map[string]int{"scout": 5}), "type": "attack"})
		c.mustOrder(wid, "send", map[string]any{"originId": v.ID, "x": bot.X, "y": bot.Y, "units": units(map[string]int{"axe": 200, "lightCav": 50}), "type": "attack"})
	}
	for _, c := range []*client{ala, bob} {
		barb := barbs[c]
		eventually(t, 3*time.Minute, c.name+"'s scout and bot reports", func() bool {
			rs := c.reports(wid)
			return countReports(rs, func(r game.Report) bool { return r.Type == "scout" && r.TargetID == barb.ID }) == 1 &&
				countReports(rs, func(r game.Report) bool { return r.Type == "attack" }) >= 1
		})
	}

	// Support each other; each host sends the support back.
	for _, pair := range [][2]*client{{ala, bob}, {bob, ala}} {
		from, host := pair[0], pair[1]
		fv, hv := homes[from], homes[host]
		from.mustOrder(wid, "send", map[string]any{"originId": fv.ID, "x": hv.X, "y": hv.Y, "units": units(map[string]int{"spear": 50}), "type": "support"})
		eventually(t, 3*time.Minute, from.name+"'s support at "+host.name, func() bool {
			_, ok := host.state(wid).Villages[hv.ID].SupportFrom[fv.ID]
			return ok
		})
		host.mustOrder(wid, "withdraw-support", map[string]string{"hostId": hv.ID, "homeId": fv.ID})
		eventually(t, 3*time.Minute, host.name+" sends the support back", func() bool {
			st := from.state(wid)
			_, still := host.state(wid).Villages[hv.ID].SupportFrom[fv.ID]
			return !still && st.Villages[fv.ID].Units[game.USpear] >= 1000-50+10-50
		})
	}

	// A scheduled send leaves at its time.
	now := ala.state(wid).Now
	ala.mustOrder(wid, "schedule-send", map[string]any{"originId": av.ID, "x": barbs[ala].X, "y": barbs[ala].Y,
		"units": units(map[string]int{"axe": 20}), "type": "attack", "mode": "sendAt", "at": now + 20*worldMinute})
	if st := ala.state(wid); len(st.Scheduled) != 1 {
		t.Fatalf("scheduled %v", st.Scheduled)
	}
	eventually(t, 2*time.Minute, "the scheduled send to leave", func() bool {
		st := ala.state(wid)
		return len(st.Scheduled) == 0 && slices.ContainsFunc(st.Commands, func(c app.CommandView) bool {
			return c.Type == "attack" && c.TargetID == barbs[ala].ID
		})
	})

	// Bob attacks Ala with rams (slow, so there is time to see it coming); Ala dodges it.
	attack := bob.mustOrder(wid, "send", map[string]any{"originId": bv.ID, "x": av.X, "y": av.Y, "units": units(map[string]int{"ram": 10, "axe": 100}), "type": "attack"})
	var incoming string
	for _, c := range attack.Commands {
		if c.Type == "attack" && c.TargetID == av.ID {
			incoming = c.ID
		}
	}
	eventually(t, 10*time.Second, "the attack to show at Ala's", func() bool {
		return slices.ContainsFunc(ala.state(wid).Commands, func(c app.CommandView) bool { return c.ID == incoming && c.Units == nil })
	})
	if st := ala.mustOrder(wid, "dodge", map[string]string{"commandId": incoming}); len(st.Scheduled) == 0 {
		t.Fatal("the dodge planned nothing")
	}

	// Ala takes the scouted barbarian village with a noble, then snipes Bob's hetman train from it.
	barb := barbs[ala]
	// boost holds the world's lock, so the API (which takes it too) must not be called inside it.
	lowAt := ala.state(wid).Now
	boost(t, s.db, wid, barb.ID, func(v *game.Village) { v.Loyalty = 5; v.LoyaltyUpdatedAt = lowAt })
	ala.mustOrder(wid, "send", map[string]any{"originId": av.ID, "x": barb.X, "y": barb.Y, "units": units(map[string]int{"noble": 1, "axe": 200}), "type": "attack"})
	eventually(t, 3*time.Minute, "Ala's conquest", func() bool { return slices.Contains(ala.state(wid).Player.VillageIDs, barb.ID) })
	boost(t, s.db, wid, barb.ID, func(v *game.Village) { v.Units[game.USpear] = 300 })
	var train []string
	for range 2 {
		st := bob.mustOrder(wid, "send", map[string]any{"originId": bv.ID, "x": av.X, "y": av.Y, "units": units(map[string]int{"noble": 1, "axe": 30}), "type": "attack"})
		for _, c := range st.Commands {
			if c.Type == "attack" && c.TargetID == av.ID && (*c.Units)[game.UNoble] == 1 && !slices.Contains(train, c.ID) {
				train = append(train, c.ID)
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	if len(train) != 2 {
		t.Fatalf("hetman train %v", train)
	}
	ala.mustOrder(wid, "snipe", map[string]any{"commandIds": train, "fromVillageId": barb.ID, "units": units(map[string]int{"spear": 100})})

	// Ala attacks Bob back; both get the reports of both attacks.
	ala.mustOrder(wid, "send", map[string]any{"originId": av.ID, "x": bv.X, "y": bv.Y, "units": units(map[string]int{"axe": 100}), "type": "attack"})
	for _, c := range []*client{ala, bob} {
		eventually(t, 5*time.Minute, c.name+"'s reports of the duel", func() bool {
			rs := c.reports(wid)
			return countReports(rs, func(r game.Report) bool { return r.Type == "attack" && r.OriginID == bv.ID && r.TargetID == av.ID }) >= 1 &&
				countReports(rs, func(r game.Report) bool { return r.Type == "attack" && r.OriginID == av.ID && r.TargetID == bv.ID }) >= 1
		})
	}
	var ranking []app.PlayerRow
	ala.ok("GET", wpath(wid, "/ranking?tab=points"), nil, &ranking)
	names := []string{}
	for _, r := range ranking {
		names = append(names, r.Name)
	}
	if !slices.Contains(names, "ala") || !slices.Contains(names, "bob") {
		t.Fatalf("ranking %v", names)
	}

	t.Run("TestNoHiddenInfoInAPI", func(t *testing.T) {
		alaVillages := map[string]bool{}
		for _, id := range ala.state(wid).Player.VillageIDs {
			alaVillages[id] = true
		}
		bob.mu.Lock()
		bodies := slices.Clone(bob.bodies)
		bob.mu.Unlock()
		if len(bodies) < 20 {
			t.Fatalf("only %d bodies recorded", len(bodies))
		}
		for _, b := range bodies {
			var v any
			if json.Unmarshal([]byte(b), &v) != nil {
				continue
			}
			if problem := hidden(v, alaVillages); problem != "" {
				t.Fatalf("bob received %s in %.300s", problem, b)
			}
		}
	})
}

// hidden names what a body shows that bob must not see: Ala's village economy, the troops of Ala's commands
// (her own, or hers returning home), the world's seed or a bot's brain.
func hidden(v any, alaVillages map[string]bool) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range []string{"worldSeed", "seed", "brain"} {
			if _, ok := x[k]; ok {
				return "the key " + k
			}
		}
		id, _ := x["id"].(string)
		if _, ok := x["resources"]; ok && alaVillages[id] {
			return "the resources of " + id
		}
		// A command belongs to its home village: the origin, or the target of troops returning home.
		origin, _ := x["originId"].(string)
		home := origin
		if typ, _ := x["type"].(string); typ == "return" || (typ == "trade" && x["returning"] == true) {
			home, _ = x["targetId"].(string)
		}
		if _, ok := x["units"]; ok && origin != "" && alaVillages[home] {
			obj, _ := json.Marshal(x)
			return fmt.Sprintf("the troops of a command from %s: %s", origin, obj)
		}
		for _, sub := range x {
			if p := hidden(sub, alaVillages); p != "" {
				return p
			}
		}
	case []any:
		for _, sub := range x {
			if p := hidden(sub, alaVillages); p != "" {
				return p
			}
		}
	}
	return ""
}

// DoD 5: a kill -9 with attacks in flight and orders on the way loses nothing acknowledged, resolves every
// attack once, and ends exactly like a server that never stopped.
func TestKillNineRestart(t *testing.T) {
	epoch := time.Now().UnixMilli()
	db1 := newDB(t)
	s1 := startServer(t, db1, epoch)
	a := s1.admin(t)
	wid := a.world(fast(0))
	setup := s1.register(t, "ala")
	setup.join(wid)
	v := setup.home(wid)
	boost(t, db1, wid, v.ID, army)
	var targets []app.MapTile
	for _, tile := range setup.tiles(wid, v.X, v.Y, 25) {
		if d := game.Distance(v.X, v.Y, tile.X, tile.Y); tile.OwnerID == "" && d >= 8 && d <= 25 {
			targets = append(targets, tile)
		}
	}
	if len(targets) < 20 {
		t.Fatalf("only %d barbarian villages 8–25 fields away", len(targets))
	}
	targets = targets[:20]

	// The control server starts from a copy of the same database and gets every acknowledged order too.
	s1.stop()
	clone := clonedDB(t, db1)
	s1.start()
	s2 := startServer(t, clone, epoch)
	c1, c2 := s1.login(t, "ala", password), s2.login(t, "ala", password)

	now := c1.state(wid).Now
	for i, tile := range targets {
		body := map[string]any{"originId": v.ID, "x": tile.X, "y": tile.Y, "units": units(map[string]int{"lightCav": 10}),
			"type": "attack", "mode": "sendAt", "at": now + 15*worldMinute + int64(i)*1000}
		c1.mustOrder(wid, "schedule-send", body)
		c2.mustOrder(wid, "schedule-send", body)
	}
	attacksOut := func(c *client) int {
		n := 0
		for _, cmd := range c.state(wid).Commands {
			if cmd.Type == "attack" {
				n++
			}
		}
		return n
	}
	eventually(t, time.Minute, "20 attacks in flight", func() bool { return attacksOut(c1) == 20 })

	// Orders keep coming while the server dies; renames take no ids, so the control can follow exactly.
	var acked int
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 1; ; i++ {
			body := map[string]string{"villageId": v.ID, "name": fmt.Sprintf("Café Hill %03d", i)}
			if r := c1.order(wid, "rename-village", body); r.status != 200 {
				return
			}
			acked = i
			c2.mustOrder(wid, "rename-village", body)
		}
	})
	time.Sleep(300 * time.Millisecond)
	if attacksOut(c2) != 20 {
		t.Fatal("the control has a different number of attacks out")
	}
	s1.kill9()
	wg.Wait()
	if acked == 0 {
		t.Fatal("no order was acknowledged before the kill")
	}
	time.Sleep(30 * time.Second)
	s1.start()

	name := c1.home(wid).Name
	if name != fmt.Sprintf("Café Hill %03d", acked) && name != fmt.Sprintf("Café Hill %03d", acked+1) {
		t.Fatalf("after the restart the village is %q; the last acknowledged rename was %d", name, acked)
	}
	settled := func(c *client) bool {
		return countReports(c.reports(wid), func(r game.Report) bool { return r.Type == "attack" }) == 20 && len(c.state(wid).Commands) == 0
	}
	eventually(t, 3*time.Minute, "every attack and return to settle on both servers", func() bool { return settled(c1) && settled(c2) })

	// The rename cut off by the kill may have committed, so the attacker's name in a label may be one ahead of
	// the control's; the name itself was checked above.
	r1, r2 := c1.reports(wid), c2.reports(wid)
	for i := range r1 {
		r1[i].Origin.Name = ""
	}
	for i := range r2 {
		r2[i].Origin.Name = ""
	}
	seen := map[string]bool{}
	for _, r := range r1 {
		if seen[r.TargetID] {
			t.Fatalf("two reports of the attack on %s", r.TargetID)
		}
		seen[r.TargetID] = true
	}
	j1, _ := json.Marshal(r1)
	j2, _ := json.Marshal(r2)
	if string(j1) != string(j2) {
		t.Fatalf("the restarted server's reports differ from the control's:\n%s\n%s", j1, j2)
	}
	if u1, u2 := c1.home(wid).Units, c2.home(wid).Units; u1 != u2 {
		t.Fatalf("troops at home differ: %v vs %v", u1, u2)
	}
}
