package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"librehold/internal/game"
)

// WorldNow is the world time at a wall-clock instant: ms since the start, without paused spans; it stands
// still while the world is paused.
func WorldNow(row *WorldRow, wallMs int64) int64 {
	if row.PausedAtMs != nil {
		wallMs = *row.PausedAtMs
	}
	return wallMs - row.StartedAtMs - row.PausedTotalMs
}

// WallTime is the wall-clock instant of world time worldMs, paused spans so far included.
func WallTime(row *WorldRow, worldMs int64) time.Time {
	return time.UnixMilli(row.StartedAtMs + row.PausedTotalMs + worldMs).UTC()
}

// processDue dispatches the events due by until in (due, seq) order, each at its own time, settling quests
// of the humans it touched; at most max of them when max > 0.
func processDue(w WorldTx, until int64, max int) (int, error) {
	n := 0
	for max <= 0 || n < max {
		if game.Ended(w) {
			break
		}
		key, at, ok := w.PopDue(until)
		if !ok {
			break
		}
		w.SetNow(at)
		if err := game.Dispatch(w, key); err != nil {
			return n, fmt.Errorf("%s at %d: %w", key, at, err)
		}
		for _, h := range w.TouchedHumans() {
			game.SettleQuests(w, h)
		}
		n++
	}
	return n, nil
}

// ProcessDue runs, in one unit of work, the world's events that are due now; at most maxEvents when
// maxEvents > 0. more says whether due events remain.
func (s *Service) ProcessDue(ctx context.Context, worldID int64, maxEvents int) (processed int, more bool, err error) {
	var now int64
	err = s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		now = WorldNow(w.Row(), s.clock.NowMs())
		w.SetNow(now)
		processed, err = processDue(w, now, maxEvents)
		return err
	})
	if err != nil {
		return 0, false, err
	}
	if maxEvents > 0 && processed == maxEvents {
		at, ok, err := s.worlds.NextDue(ctx, worldID)
		if err != nil {
			return processed, false, err
		}
		more = ok && at <= now
	}
	return processed, more, nil
}

func (s *Service) memberID(ctx context.Context, acct *Account, worldID int64) (game.PlayerID, error) {
	ms, err := s.worlds.MembershipsOf(ctx, acct.ID)
	if err != nil {
		return "", err
	}
	for _, m := range ms {
		if m.WorldID == worldID {
			s.MarkPresent(ctx, worldID)
			return m.PlayerID, nil
		}
	}
	return "", ErrNotMember
}

// inWorld runs a player's order: first the due events in a unit of work of their own, then, in a second one,
// the events that became due meanwhile, fn at world time now and the player's quests. fn's error is returned
// unchanged and undoes the second unit of work.
// ErrTooFast refuses a state read or an order over Config.WorldRequestsPerMinute.
var ErrTooFast = errors.New("too many requests from this account")

const worldRequestsLimit = 300

// paced counts a state read or an order of the account against Config.WorldRequestsPerMinute.
func (s *Service) paced(ctx context.Context, acct *Account) error {
	switch ok, err := s.take(ctx, s.pace, strconv.FormatInt(acct.ID, 10)); {
	case err != nil:
		return err
	case !ok:
		return ErrTooFast
	}
	return nil
}

func (s *Service) inWorld(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx, me game.PlayerID) error) error {
	me, err := s.memberID(ctx, acct, worldID)
	if err != nil {
		return err
	}
	if _, _, err := s.ProcessDue(ctx, worldID, 0); err != nil {
		return err
	}
	return s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		if w.Row().PausedAtMs != nil {
			return ErrWorldPaused
		}
		now := WorldNow(w.Row(), s.clock.NowMs())
		w.SetNow(now)
		if _, err := processDue(w, now, 0); err != nil {
			return err
		}
		if game.Ended(w) {
			return ErrWorldEnded
		}
		w.SetNow(now)
		if err := fn(w, me); err != nil {
			return err
		}
		game.SettleQuests(w, me)
		return nil
	})
}
