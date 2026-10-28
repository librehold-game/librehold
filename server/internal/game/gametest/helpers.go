package gametest

import (
	"fmt"

	"librehold/internal/game"
)

// TestConfig is the test world's config: speed 1, no bots.
func TestConfig() game.WorldConfig {
	cfg := game.DefaultWorldConfig
	cfg.WorldSpeed = 1
	cfg.BotCount = 0
	return cfg
}

// NewHumanPlayer registers a human player with its HumanState, without a village.
func NewHumanPlayer(w *MemWorld, id game.PlayerID) *game.Player {
	p := &game.Player{
		ID: id, Name: "Player " + id, IsHuman: true, Color: "#c8a54b", VillageIDs: []string{},
		BuildQueueSlots: 2, Status: game.StatusActive, History: game.PointsHistory{Hourly: []int{}, Daily: []int{}},
	}
	w.PutPlayer(p)
	w.PutHuman(id, game.NewHumanState())
	return p
}

// NewBotPlayer registers a bot player without a brain or village.
func NewBotPlayer(w game.World, id game.PlayerID) *game.Player {
	p := &game.Player{
		ID: id, Name: "Bot " + id, Color: "#5a7fbf", VillageIDs: []string{},
		BuildQueueSlots: 2, Status: game.StatusActive, History: game.PointsHistory{Hourly: []int{}, Daily: []int{}},
	}
	w.PutPlayer(p)
	return p
}

// AddStartVillage gives a player a start village at (x, y): StartVillage buildings and resources, no units.
func AddStartVillage(w game.World, owner game.PlayerID, x, y int) *game.Village {
	v := game.CreateVillage(x, y, "", "", w.Now())
	for b, lvl := range game.StartVillage.Buildings {
		v.Buildings[b] = lvl
	}
	v.Resources = game.StartVillage.Resources
	game.RecomputePoints(v)
	w.PutVillage(v)
	game.SetOwner(w, v, game.Ref(owner))
	return v
}

// NewWorldWithHuman creates a world (game.NewWorld) and joins human "h1"; it panics on failure.
func NewWorldWithHuman(seed uint32, cfg game.WorldConfig) (*MemWorld, *game.Village) {
	w := NewMemWorld(seed, cfg, 0)
	if err := game.NewWorld(w); err != nil {
		panic(err)
	}
	v, err := game.JoinHuman(w, "h1", "Player", 1, game.QuadrantRandom)
	if err != nil {
		panic(err)
	}
	return w, v
}

// NewTestWorld is a world with speed 1, no bots, and h1's start village without its starting units.
func NewTestWorld(seed uint32) (*MemWorld, *game.Village) {
	w, v := NewWorldWithHuman(seed, TestConfig())
	v.Units = game.Units{}
	return w, v
}

// NewDefaultWorld is a world with default settings (100 bots) and h1 with the starting guard.
func NewDefaultWorld(seed uint32) (*MemWorld, *game.Village) {
	return NewWorldWithHuman(seed, game.DefaultWorldConfig)
}

// NearestBarbarianXY is the closest barbarian tile in rings around v, unmaterialised.
func NearestBarbarianXY(w *MemWorld, v *game.Village) (int, int) {
	size := w.Config().MapSize
	for r := 1; r < size; r++ {
		bestX, bestY, bestD, found := 0, 0, 0.0, false
		for x := v.X - r; x <= v.X+r; x++ {
			for y := v.Y - r; y <= v.Y+r; y++ {
				if max(abs(x-v.X), abs(y-v.Y)) != r {
					continue
				}
				t := game.TileInfo(w, x, y)
				if t.Kind != "village" || t.OwnerID != "" {
					continue
				}
				if d := game.Distance(v.X, v.Y, x, y); !found || d < bestD {
					bestX, bestY, bestD, found = x, y, d, true
				}
			}
		}
		if found {
			return bestX, bestY
		}
	}
	panic("no barbarian village on the map")
}

// MakeBotVillage is a village of bot "bot_test" (registered on first use) on the nearest free
// field around near, with StartVillage buildings and no units.
func MakeBotVillage(w game.World, near *game.Village) *game.Village {
	if w.Player("bot_test") == nil {
		NewBotPlayer(w, "bot_test")
	}
	for r := 1; r < 20; r++ {
		for dx := -r; dx <= r; dx++ {
			for dy := -r; dy <= r; dy++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				if game.TileInfo(w, near.X+dx, near.Y+dy).Kind == "empty" {
					return AddStartVillage(w, "bot_test", near.X+dx, near.Y+dy)
				}
			}
		}
	}
	panic("no free tile for a bot village")
}

// NearestBotVillage is the bot village closest to v.
func NearestBotVillage(w *MemWorld, v *game.Village) *game.Village {
	var best *game.Village
	bestD := 0.0
	for _, o := range w.Villages() {
		if o.OwnerID == "" || game.IsHumanID(game.PlayerID(o.OwnerID)) {
			continue
		}
		if d := game.Distance(v.X, v.Y, o.X, o.Y); best == nil || d < bestD {
			best, bestD = o, d
		}
	}
	if best == nil {
		panic("no bot village")
	}
	return best
}

// ScoutOnce sends one scout from v (added to its units) at target and plays to its arrival.
func ScoutOnce(w *MemWorld, v, target *game.Village) error {
	v.Units[game.UScout]++
	c, err := game.SendCommand(w, game.PlayerID(v.OwnerID), v.ID, target.X, target.Y, game.UnitCounts{game.UScout: 1}, "attack", game.SendOpts{})
	if err != nil {
		return err
	}
	return Advance(w, c.ArrivesAt)
}

// SendManyRaids sends n attacks on barbarians within 15 fields, topping up v's units as needed.
func SendManyRaids(w *MemWorld, v *game.Village, n int, units game.UnitCounts) ([]*game.Command, error) {
	var targets [][2]int
	for x := v.X - 15; x <= v.X+15; x++ {
		for y := v.Y - 15; y <= v.Y+15; y++ {
			if (x != v.X || y != v.Y) && game.Distance(v.X, v.Y, x, y) <= 15 {
				if t := game.TileInfo(w, x, y); t.Kind == "village" && t.OwnerID == "" {
					targets = append(targets, [2]int{x, y})
				}
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets within 15 tiles")
	}
	var out []*game.Command
	for i := range n {
		for _, u := range game.UnitIDs {
			if v.Units[u] < units[u] {
				v.Units[u] = units[u]
			}
		}
		t := targets[i%len(targets)]
		c, err := game.SendCommand(w, game.PlayerID(v.OwnerID), v.ID, t[0], t[1], units, "attack", game.SendOpts{})
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
