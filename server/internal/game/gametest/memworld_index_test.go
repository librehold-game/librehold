package gametest_test

import (
	"slices"
	"sort"
	"strconv"
	"testing"

	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

// VillagesIn answers from a grid index; it must give what a scan of every village gives, owners' changes included.
func TestVillagesInMatchesAFullScan(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount = 30
	w := gametest.NewMemWorld(3, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	for i, v := range w.Villages() {
		if i%3 == 0 {
			game.EnsureVillage(w, v.X+1, v.Y)
		}
	}
	rng := game.Mulberry32(9)
	for range 300 {
		x0, y0 := int(rng()*float64(cfg.MapSize)), int(rng()*float64(cfg.MapSize))
		x1, y1 := x0+int(rng()*40), y0+int(rng()*40)
		var want []game.VillageRef
		for _, v := range w.Villages() {
			if v.X >= x0 && v.X <= x1 && v.Y >= y0 && v.Y <= y1 {
				want = append(want, game.VillageRef{ID: v.ID, X: v.X, Y: v.Y, Name: v.Name, OwnerID: v.OwnerID, Points: v.Points})
			}
		}
		sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
		if got := w.VillagesIn(x0, y0, x1, y1); !slices.Equal(got, want) {
			t.Fatalf("box %d,%d–%d,%d: got %d villages, want %d", x0, y0, x1, y1, len(got), len(want))
		}
	}
}

// CommandsTo answers from indexes; it must give what a scan of every command gives, turned-back commands included.
func TestCommandsToMatchesAFullScan(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 30, 0
	w := gametest.NewMemWorld(4, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	check := func(when string) {
		t.Helper()
		all := w.Commands()
		for _, v := range w.Villages() {
			var want []*game.Command
			for _, c := range all {
				if c.TargetID == v.ID {
					want = append(want, c)
				}
			}
			if got := w.CommandsTo(v.ID); !slices.Equal(got, want) {
				t.Fatalf("%s, %s: got %d commands, want %d", when, v.ID, len(got), len(want))
			}
		}
	}
	for day := int64(1); day <= 3; day++ {
		if err := gametest.Advance(w, day*game.MsPerDay); err != nil {
			t.Fatal(err)
		}
		check("day " + strconv.FormatInt(day, 10))
	}
	for _, c := range w.Commands() {
		owner := game.PlayerID(w.Village(c.OriginID).OwnerID)
		if c.Type == "attack" && game.CanCancelCommand(w, c) {
			if err := game.CancelCommand(w, owner, c.ID); err != nil {
				t.Fatal(err)
			}
			check("after turning back " + c.ID)
			return
		}
	}
	t.Fatal("setup: no attack to turn back")
}

// A clone is a unit of work the memory store may throw away: what it adds or moves must stay out of the original.
func TestCloneKeepsItsIndexesToItself(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.BotCount, cfg.BeginnerProtectionDays = 30, 0
	w := gametest.NewMemWorld(4, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		t.Fatal(err)
	}
	if err := gametest.Advance(w, game.MsPerDay); err != nil {
		t.Fatal(err)
	}
	commands := len(w.Commands())
	c := w.Clone()
	added := 0
	for _, v := range c.Villages() {
		if nv := game.EnsureVillage(c, v.X+1, v.Y); nv != nil && w.Village(nv.ID) == nil {
			added++
		}
	}
	if err := gametest.Advance(c, 2*game.MsPerDay); err != nil {
		t.Fatal(err)
	}
	if added == 0 {
		t.Fatal("setup: the clone added no village")
	}
	if got := len(w.VillagesIn(0, 0, cfg.MapSize, cfg.MapSize)); got != len(w.Villages()) {
		t.Fatalf("original lists %d villages, has %d", got, len(w.Villages()))
	}
	for _, v := range w.Villages() {
		for _, cmd := range w.CommandsTo(v.ID) {
			if w.Command(cmd.ID) != cmd {
				t.Fatalf("original's CommandsTo(%s) gives %s from the clone", v.ID, cmd.ID)
			}
		}
	}
	if len(w.Commands()) != commands {
		t.Fatalf("original has %d commands, had %d", len(w.Commands()), commands)
	}
}
