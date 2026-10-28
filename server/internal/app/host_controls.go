package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"librehold/internal/game"
)

// The host's controls. Each is logged like an admin change, so a replay of the log gives the same game.

const (
	actorHost        = "host"
	abandonAfter     = 2 * time.Hour
	playerSweepEvery = 10 * time.Second
)

type kickEntry struct {
	PlayerID game.PlayerID `json:"playerId"`
}

type joinRuleEntry struct {
	RegisteredOnly bool `json:"registeredOnly"`
}

// hostRun runs fn on the account's own player world at world time now, after the events due by then.
func (s *Service) hostRun(ctx context.Context, acct *Account, worldID int64, fn func(w WorldTx, wall int64) error) error {
	if _, _, err := s.ProcessDue(ctx, worldID, 0); err != nil {
		return err
	}
	err := s.worlds.Run(ctx, worldID, func(w WorldTx) error {
		row := w.Row()
		if row.Host == nil || row.Host.HostID != acct.ID {
			return ErrForbidden
		}
		wall := s.clock.NowMs()
		now := WorldNow(row, wall)
		w.SetNow(now)
		if _, err := processDue(w, now, 0); err != nil {
			return err
		}
		w.SetNow(now)
		return fn(w, wall)
	})
	if err == nil {
		s.wakeWorker(worldID)
	}
	return err
}

// running refuses a world that has not started or has ended.
func running(row *WorldRow) error {
	switch {
	case row.State.EndedAt != nil:
		return ErrWorldEnded
	case row.Host.StartedAtMs == nil:
		return &ErrValidation{Field: "world", Code: "notStarted"}
	}
	return nil
}

// HostStart runs the clock from world time 0; the waiting span counts as paused.
func (s *Service) HostStart(ctx context.Context, acct *Account, worldID int64) error {
	return s.hostRun(ctx, acct, worldID, func(w WorldTx, wall int64) error {
		row := w.Row()
		switch {
		case row.State.EndedAt != nil:
			return ErrWorldEnded
		case row.Host.StartedAtMs != nil:
			return &ErrValidation{Field: "world", Code: "alreadyStarted"}
		case activeHumans(w) == 0:
			return &ErrValidation{Field: "world", Code: "noPlayers"}
		}
		row.PausedTotalMs += wall - *row.PausedAtMs
		row.PausedAtMs = nil
		row.Host.StartedAtMs = &wall
		w.LogAction(actorHost, false, "start", resumeEntry{PausedTotalMs: row.PausedTotalMs})
		return nil
	})
}

func (s *Service) HostPause(ctx context.Context, acct *Account, worldID int64) error {
	return s.hostRun(ctx, acct, worldID, func(w WorldTx, wall int64) error {
		row := w.Row()
		if err := running(row); err != nil {
			return err
		}
		switch {
		case row.PausedAtMs != nil:
			return &ErrValidation{Field: "world", Code: "alreadyPaused"}
		case row.Host.HostPausedMs >= hostPauseTotalMs:
			return &ErrValidation{Field: "world", Code: "pauseLimit"}
		}
		row.PausedAtMs = &wall
		w.LogAction(actorHost, false, "pause", struct{}{})
		return nil
	})
}

func (s *Service) HostResume(ctx context.Context, acct *Account, worldID int64) error {
	return s.hostRun(ctx, acct, worldID, func(w WorldTx, wall int64) error {
		row := w.Row()
		if err := running(row); err != nil {
			return err
		}
		if row.PausedAtMs == nil {
			return &ErrValidation{Field: "world", Code: "notPaused"}
		}
		closeHostPause(w, wall, actorHost)
		return nil
	})
}

// closeHostPause ends the host's pause, counting at most its ten minutes and what is left of the thirty: a pause
// closed late resumes where it should have, and world time catches up.
func closeHostPause(w WorldTx, wall int64, actor string) {
	row := w.Row()
	span := max(0, min(wall-*row.PausedAtMs, hostPauseMaxMs, hostPauseTotalMs-row.Host.HostPausedMs))
	resumedAt := *row.PausedAtMs + span
	row.PausedTotalMs += span
	row.Host.HostPausedMs += span
	row.PausedAtMs = nil
	logResume(w, resumedAt, actor)
	w.SetNow(WorldNow(row, wall))
}

// HostEnd ends the game now (the top player by points wins); a world still waiting is deleted instead.
func (s *Service) HostEnd(ctx context.Context, acct *Account, worldID int64) error {
	waiting := false
	err := s.hostRun(ctx, acct, worldID, func(w WorldTx, wall int64) error {
		row := w.Row()
		switch {
		case row.State.EndedAt != nil:
			return ErrWorldEnded
		case row.Host.StartedAtMs == nil:
			waiting = true
			return nil
		}
		if row.PausedAtMs != nil {
			closeHostPause(w, wall, actorHost)
			if _, err := processDue(w, w.Now(), 0); err != nil {
				return err
			}
			w.SetNow(WorldNow(row, wall))
		}
		w.LogAction(actorHost, false, "end", struct{}{})
		game.EndByHost(w)
		return nil
	})
	if err != nil || !waiting {
		return err
	}
	return s.deleteHosted(ctx, worldID)
}

// deleteHosted removes a player world that never started, keeping its log.
func (s *Service) deleteHosted(ctx context.Context, worldID int64) error {
	if err := s.PackActionLog(ctx, worldID); err != nil {
		return err
	}
	if err := s.worlds.Delete(ctx, worldID); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// HostKick sends a player away for good: their villages turn barbarian and the account cannot join again.
func (s *Service) HostKick(ctx context.Context, acct *Account, worldID int64, playerID game.PlayerID) error {
	return s.hostRun(ctx, acct, worldID, func(w WorldTx, _ int64) error {
		row := w.Row()
		if row.State.EndedAt != nil {
			return ErrWorldEnded
		}
		p := w.Player(playerID)
		if p == nil || !p.IsHuman || p.Status != game.StatusActive {
			return &ErrValidation{Field: "playerId", Code: "notAPlayer"}
		}
		if p.AccountID != 0 {
			row.Host.Banned = append(row.Host.Banned, p.AccountID)
		}
		w.LogAction(actorHost, false, "kick", kickEntry{PlayerID: playerID})
		game.KickPlayer(w, playerID)
		p.AccountID = 0
		return nil
	})
}

// ReplaceInvite gives the world a new invite code; the old link stops working, players inside stay.
func (s *Service) ReplaceInvite(ctx context.Context, acct *Account, worldID int64) (string, error) {
	code := newCode()
	err := s.hostRun(ctx, acct, worldID, func(w WorldTx, _ int64) error {
		if w.Row().State.EndedAt != nil {
			return ErrWorldEnded
		}
		w.Row().Host.InviteCode = code
		w.LogAction(actorHost, false, "invite", struct{}{})
		return nil
	})
	if err != nil {
		return "", err
	}
	return code, nil
}

func (s *Service) SetRegisteredOnly(ctx context.Context, acct *Account, worldID int64, on bool) error {
	return s.hostRun(ctx, acct, worldID, func(w WorldTx, _ int64) error {
		w.Row().Host.RegisteredOnly = on
		w.LogAction(actorHost, false, "joinRule", joinRuleEntry{RegisteredOnly: on})
		return nil
	})
}

// HostCard is the host's view of their world after a control; nil once a waiting world was deleted.
func (s *Service) HostCard(ctx context.Context, acct *Account, worldID int64) (*HostedCard, error) {
	row, err := s.worlds.Get(ctx, worldID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Host == nil || row.Host.HostID != acct.ID {
		return nil, ErrForbidden
	}
	return s.hostedCard(ctx, worldID)
}

// SweepPlayerWorlds ends host pauses at their cap and deletes worlds left waiting for two hours.
func (s *Service) SweepPlayerWorlds(ctx context.Context) error {
	worlds, err := s.worlds.List(ctx)
	if err != nil {
		return err
	}
	wall := s.clock.NowMs()
	for _, w := range worlds {
		h := w.Host
		if h == nil || w.State.EndedAt != nil {
			continue
		}
		switch {
		case h.StartedAtMs == nil && wall-w.StartedAtMs >= abandonAfter.Milliseconds():
			if err := s.deleteHosted(ctx, w.ID); err != nil {
				return err
			}
		case h.StartedAtMs != nil && w.PausedAtMs != nil && wall >= hostPauseEndsAt(&w.WorldRow):
			err := s.worlds.Run(ctx, w.ID, func(tx WorldTx) error {
				if tx.Row().PausedAtMs != nil {
					closeHostPause(tx, wall, actorWorld)
				}
				return nil
			})
			if err != nil {
				return err
			}
			s.wakeWorker(w.ID)
		}
	}
	return nil
}

// RunPlayerWorlds sweeps the player worlds every Config.PlayerSweepEvery until ctx ends.
func (s *Service) RunPlayerWorlds(ctx context.Context) error {
	t := time.NewTicker(s.cfg.PlayerSweepEvery)
	defer t.Stop()
	for {
		if err := s.SweepPlayerWorlds(ctx); err != nil && ctx.Err() == nil {
			slog.Error("player worlds: sweep", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}
