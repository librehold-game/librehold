package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"librehold/internal/game"
)

var ErrNoCreateEntry = errors.New("replay: the log does not start with the world's create entry")

// Replay rebuilds a world in svc's store from its action log, seed and secret, and returns the new world's id. The
// create entry gives the starting settings and start time; humans' joins, leaves and orders, admin changes and a
// host's controls are applied again at their world times, each after the events due by then. Bots are recomputed
// from the seed and secret, so their entries are skipped. It works only on the rules version the log was recorded with.
func Replay(ctx context.Context, svc *Service, log []game.LoggedAction, seed uint32, secret []byte) (int64, error) {
	if len(log) == 0 || log[0].Kind != "create" {
		return 0, ErrNoCreateEntry
	}
	var c createEntry
	if err := json.Unmarshal(log[0].Payload, &c); err != nil {
		return 0, err
	}
	row := WorldRow{Name: "Replay", Status: "open", Seed: seed, Secret: secret, Config: c.Config, StartedAtMs: c.StartedAtMs}
	id, err := svc.worlds.Create(ctx, row, func(w WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		return 0, err
	}
	players := map[string]game.PlayerID{}
	for _, e := range log[1:] {
		if e.IsBot {
			continue
		}
		err := svc.worlds.Run(ctx, id, func(w WorldTx) error {
			w.SetNow(e.At)
			if _, err := processDue(w, e.At, 0); err != nil {
				return err
			}
			if game.Ended(w) {
				return nil
			}
			w.SetNow(e.At)
			return replayEntry(w, e, players)
		})
		if err != nil {
			return id, fmt.Errorf("replay: entry %d (%s at %d): %w", e.Seq, e.Kind, e.At, err)
		}
	}
	return id, nil
}

func replayEntry(w WorldTx, e game.LoggedAction, players map[string]game.PlayerID) error {
	switch e.Kind {
	case "snapshot", "observe", "pause", "invite", "joinRule":
		return nil
	case "join":
		var j joinEntry
		if err := json.Unmarshal(e.Payload, &j); err != nil {
			return err
		}
		players[e.Actor] = j.PlayerID
		return joinAs(w, j.PlayerID, e.Actor, 0, e.Actor, j.Guest, j.Quadrant)
	case "claim":
		if p := w.Player(players[e.Actor]); p != nil {
			p.IsGuest = false
		}
		return nil
	case "leave":
		leaveAs(w, players[e.Actor])
		return nil
	case "resume", "start":
		var r resumeEntry
		if err := json.Unmarshal(e.Payload, &r); err != nil {
			return err
		}
		w.Row().PausedTotalMs = r.PausedTotalMs
		return nil
	case "abandon":
		game.EndAbandoned(w)
		return nil
	case "end":
		game.EndByHost(w)
		return nil
	case "kick":
		var k kickEntry
		if err := json.Unmarshal(e.Payload, &k); err != nil {
			return err
		}
		game.KickPlayer(w, k.PlayerID)
		if p := w.Player(k.PlayerID); p != nil {
			p.AccountID = 0
		}
		return nil
	case "settings":
		var patch WorldSettingsPatch
		if err := json.Unmarshal(e.Payload, &patch); err != nil {
			return err
		}
		return applySettings(w, e.At, patch)
	}
	spec, ok := orders[e.Kind]
	if !ok {
		return fmt.Errorf("unknown entry kind %q", e.Kind)
	}
	run, err := spec(e.Payload)
	if err != nil {
		return err
	}
	me, ok := players[e.Actor]
	if !ok {
		return fmt.Errorf("order by %q, who never joined", e.Actor)
	}
	if err := run(w, me); err != nil {
		return err
	}
	game.SettleQuests(w, me)
	return nil
}
