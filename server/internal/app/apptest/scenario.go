package apptest

import (
	"encoding/json"
	"sort"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

// Snapshot is the stored state a scenario compares across adapters, as JSON per entity kind.
type Snapshot map[string]json.RawMessage

type playerWithAccount struct {
	*game.Player
	AccountID int64 `json:"accountId"`
}

func snapshot(t *testing.T, s app.WorldStore, id int64) Snapshot {
	t.Helper()
	out := Snapshot{}
	read(t, s, id, func(w app.WorldTx) {
		var players []playerWithAccount
		reports := map[game.PlayerID][]*game.Report{}
		for _, p := range w.Players() {
			players = append(players, playerWithAccount{p, p.AccountID})
			if p.IsHuman {
				reports[p.ID] = w.Reports(p.ID, 1<<62, 0)
			}
		}
		size := w.Config().MapSize
		var villages []*game.Village
		commands := map[string]*game.Command{}
		for _, ref := range w.VillagesIn(0, 0, size, size) {
			villages = append(villages, w.Village(ref.ID))
			for _, c := range append(w.CommandsHome(ref.ID), w.CommandsTo(ref.ID)...) {
				commands[c.ID] = c
			}
		}
		ids := make([]string, 0, len(commands))
		for cid := range commands {
			ids = append(ids, cid)
		}
		sort.Strings(ids)
		var cmds []*game.Command
		for _, cid := range ids {
			cmds = append(cmds, commands[cid])
		}
		for k, v := range map[string]any{"players": players, "villages": villages, "commands": cmds, "reports": reports} {
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			out[k] = b
		}
	})
	return out
}

// order runs fn as a player's order at world time at, the way the app does after catching up.
func order(t *testing.T, s app.WorldStore, id int64, at int64, me game.PlayerID, fn func(w app.WorldTx)) {
	t.Helper()
	run(t, s, id, func(w app.WorldTx) error {
		w.SetNow(at)
		fn(w)
		game.SettleQuests(w, me)
		return nil
	})
}

func nearestBotVillage(w app.WorldTx, from *game.Village) *game.Village {
	var best *game.Village
	bestD := 0.0
	for _, p := range w.Players() {
		if p.IsHuman {
			continue
		}
		for _, vid := range p.VillageIDs {
			v := w.Village(vid)
			if d := game.Distance(from.X, from.Y, v.X, v.Y); best == nil || d < bestD {
				best, bestD = v, d
			}
		}
	}
	return best
}

// DomainScenario plays three days of a small world through the store: two humans join, build, recruit, attack
// a bot and support each other, while the bots play. It returns the stored state at the end, plus under
// "day1/" the state right after the troops left. Order refusals are part of the scenario: both adapters must
// refuse the same ones.
func DomainScenario(t *testing.T, s app.WorldStore) Snapshot {
	t.Helper()
	row := NewWorldRow(3)
	id, err := s.Create(t.Context(), row, func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	const minute, hour, day = int64(60_000), int64(3_600_000), int64(86_400_000)
	var v1, v2 string
	order(t, s, id, 0, "h1", func(w app.WorldTx) {
		a, err1 := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		b, err2 := game.JoinHuman(w, "h2", "Bartek", 2, game.QuadrantRandom)
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		v1, v2 = a.ID, b.ID
	})
	now := Advance(t, s, id, 0, 10*minute, 50)
	order(t, s, id, now, "h1", func(w app.WorldTx) {
		_, _ = game.EnqueueBuild(w, "h1", v1, game.BMain)
		_, _ = game.EnqueueBuild(w, "h1", v1, game.BWood)
	})
	order(t, s, id, now, "h2", func(w app.WorldTx) { _, _ = game.EnqueueBuild(w, "h2", v2, game.BClay) })
	now = Advance(t, s, id, now, 6*hour, 50)
	order(t, s, id, now, "h1", func(w app.WorldTx) {
		_, _ = game.EnqueueBuild(w, "h1", v1, game.BBarracks)
		_, _ = game.EnqueueRecruit(w, "h1", v1, game.USpear, 3)
	})
	now = Advance(t, s, id, now, day, 50)
	order(t, s, id, now, "h1", func(w app.WorldTx) {
		w.Player("h1").ProtectedUntil = 0
		w.Player("h2").ProtectedUntil = 0
		home := w.Village(v1)
		if bot := nearestBotVillage(w, home); bot != nil {
			w.Player(game.PlayerID(bot.OwnerID)).ProtectedUntil = 0
			_, _ = game.SendCommand(w, "h1", v1, bot.X, bot.Y, someUnits(home), "attack", game.SendOpts{})
		}
		friend := w.Village(v2)
		_, _ = game.SendCommand(w, "h1", v1, friend.X, friend.Y, someUnits(w.Village(v1)), "support", game.SendOpts{})
	})
	mid := snapshot(t, s, id)
	now = Advance(t, s, id, now, 2*day, 50)
	order(t, s, id, now, "h2", func(w app.WorldTx) { _, _ = game.EnqueueRecruit(w, "h2", v2, game.USpear, 2) })
	Advance(t, s, id, now, 3*day, 50)
	out := snapshot(t, s, id)
	for k, v := range mid {
		out["day1/"+k] = v
	}
	return out
}
