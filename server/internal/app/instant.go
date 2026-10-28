package app

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"librehold/internal/game"
)

// PlayResult is where Play puts a player: a world to open, or how long to wait for a seat (exactly one is set).
type PlayResult struct {
	WorldID int64 `json:"worldId,omitempty"`
	WaitMs  int64 `json:"waitMs,omitempty"`
}

const (
	instantMaxWorlds  = 4
	instantKeep       = 24 * time.Hour
	instantSweepEvery = time.Minute
	minWaitMs         = 1000
)

// instantInfo is an instant world's row read against the wall clock.
type instantInfo struct {
	WorldSummary
	ready    bool // created in advance, paused until opened
	running  bool // opened and not yet over
	endsAtMs int64
}

// shortWorlds are the instant and the player worlds, which end, are archived and are purged alike.
func (s *Service) shortWorlds(ctx context.Context) ([]instantInfo, error) {
	list, err := s.worlds.List(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock.NowMs()
	var out []instantInfo
	for _, w := range list {
		if game.IsShort(&w.Config) {
			out = append(out, instantInfoOf(w, now))
		}
	}
	return out, nil
}

func (s *Service) instantWorlds(ctx context.Context) ([]instantInfo, error) {
	short, err := s.shortWorlds(ctx)
	out := short[:0]
	for _, w := range short {
		if game.IsInstant(&w.Config) {
			out = append(out, w)
		}
	}
	return out, err
}

func instantInfoOf(w WorldSummary, now int64) instantInfo {
	info := instantInfo{WorldSummary: w}
	info.endsAtMs = w.StartedAtMs + w.PausedTotalMs + int64(w.Config.DurationMin)*game.MsPerMinute
	switch {
	case w.State.EndedAt != nil:
	case w.PausedAtMs != nil:
		info.ready = w.Humans == 0
	default:
		info.running = now < info.endsAtMs
	}
	return info
}

// takesPlayers: a running instant world still inside its join window with a free seat.
func (s *Service) takesPlayers(w instantInfo) bool {
	return w.running && WorldNow(&w.WorldRow, s.clock.NowMs()) < int64(w.Config.JoinWindowMin)*game.MsPerMinute &&
		w.Humans < w.Config.HumanSeats
}

// Play puts the account into an instant game: the one it already plays, else the open one, else a newly opened
// one while fewer than Config.InstantMaxWorlds run; with all of them full, it says how long until one ends.
func (s *Service) Play(ctx context.Context, acct *Account) (PlayResult, error) {
	s.instantMu.Lock()
	defer s.instantMu.Unlock()
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return PlayResult{}, err
	}
	members, err := s.worlds.MembershipsOf(ctx, acct.ID)
	if err != nil {
		return PlayResult{}, err
	}
	for _, m := range members {
		for _, w := range worlds {
			if w.ID == m.WorldID && w.running {
				return PlayResult{WorldID: w.ID}, nil
			}
		}
	}
	for _, w := range worlds {
		if !s.takesPlayers(w) {
			continue
		}
		// The join window may close, or the world end or pause, while its due events run, and a player who left this
		// one too often may not come back; the player then gets the next game instead.
		res, err := s.joinInstant(ctx, acct, w.ID)
		var ve *ErrValidation
		closed := errors.As(err, &ve) && (ve.Code == "worldClosed" || ve.Code == "rejoinLimit")
		if !closed && !errors.Is(err, ErrWorldEnded) && !errors.Is(err, ErrWorldPaused) {
			return res, err
		}
	}
	running, wait := 0, int64(0)
	var ready *instantInfo
	for i, w := range worlds {
		if w.running {
			running++
			if left := w.endsAtMs - s.clock.NowMs(); wait == 0 || left < wait {
				wait = left
			}
		}
		if w.ready && ready == nil {
			ready = &worlds[i]
		}
	}
	if running >= s.cfg.InstantMaxWorlds {
		return PlayResult{WaitMs: max(minWaitMs, wait)}, nil
	}
	id := int64(0)
	if ready != nil {
		id = ready.ID
	} else if id, err = s.createInstant(ctx, false); err != nil {
		return PlayResult{}, err
	}
	if err := s.resumeInstant(ctx, id); err != nil {
		return PlayResult{}, err
	}
	res, err := s.joinInstant(ctx, acct, id)
	if err != nil {
		return PlayResult{}, err
	}
	if _, err := s.createInstant(ctx, true); err != nil {
		slog.Error("instant: ready world", "err", err)
	}
	return res, nil
}

func (s *Service) joinInstant(ctx context.Context, acct *Account, id int64) (PlayResult, error) {
	if _, err := s.JoinWorld(ctx, acct, id, game.QuadrantRandom); err != nil {
		return PlayResult{}, err
	}
	return PlayResult{WorldID: id}, nil
}

// createInstant creates an instant world from the preset; a paused one waits as the next ready world.
func (s *Service) createInstant(ctx context.Context, paused bool) (int64, error) {
	n, err := s.worlds.NextGameNumber(ctx)
	if err != nil {
		return 0, err
	}
	now := s.clock.NowMs()
	row := WorldRow{Name: "Game #" + strconv.FormatInt(n, 10), GameNumber: int(n), Status: "open", Seed: newSeed(), Secret: newSecret(),
		Config: game.InstantWorldConfig, StartedAtMs: now}
	if paused {
		row.PausedAtMs = &now
	}
	id, err := s.worlds.Create(ctx, row, newWorld)
	if err != nil {
		return 0, err
	}
	s.notifyWorldCreated(id)
	return id, nil
}

func (s *Service) resumeInstant(ctx context.Context, id int64) error {
	err := s.worlds.Run(ctx, id, func(w WorldTx) error {
		row := w.Row()
		if row.PausedAtMs != nil {
			row.PausedTotalMs += s.clock.NowMs() - *row.PausedAtMs
			row.PausedAtMs = nil
			logResume(w, s.clock.NowMs(), actorWorld)
		}
		return nil
	})
	if err == nil {
		s.wakeWorker(id)
	}
	return err
}

// EnsureReadyInstant keeps one paused instant world waiting, so Play never waits for map generation.
func (s *Service) EnsureReadyInstant(ctx context.Context) error {
	s.instantMu.Lock()
	defer s.instantMu.Unlock()
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return err
	}
	for _, w := range worlds {
		if w.ready {
			return nil
		}
	}
	_, err = s.createInstant(ctx, true)
	return err
}

// PurgeEndedInstant deletes short worlds whose results have been readable for Config.InstantKeep.
func (s *Service) PurgeEndedInstant(ctx context.Context) (int, error) {
	worlds, err := s.shortWorlds(ctx)
	if err != nil {
		return 0, err
	}
	now, n := s.clock.NowMs(), 0
	for _, w := range worlds {
		if w.State.EndedAt == nil {
			continue
		}
		endedWall := w.StartedAtMs + w.PausedTotalMs + *w.State.EndedAt
		if now-endedWall < s.cfg.InstantKeep.Milliseconds() {
			continue
		}
		if err := s.archiveWorld(ctx, w.ID); err != nil {
			return n, err
		}
		if err := s.PackActionLog(ctx, w.ID); err != nil {
			return n, err
		}
		if err := s.worlds.Delete(ctx, w.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return n, err
		}
		n++
	}
	return n, nil
}

// RunInstant keeps the instant worlds going until ctx ends: a ready world, the purge of old results, guests and
// closed problem reports.
func (s *Service) RunInstant(ctx context.Context) error {
	sweep := func() {
		if err := s.EnsureReadyInstant(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: ready world", "err", err)
		}
		if err := s.ArchiveEnded(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: archive", "err", err)
		}
		if _, err := s.PurgeEndedInstant(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: purge", "err", err)
		}
		if _, err := s.PurgeGuests(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: guests", "err", err)
		}
		if _, err := s.PurgeClosedReports(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: problem reports", "err", err)
		}
	}
	sweep()
	t := time.NewTicker(s.cfg.InstantSweepEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			sweep()
		}
	}
}
