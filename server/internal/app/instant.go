package app

import (
	"context"
	"errors"
	"log/slog"
	"sort"
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
	instantMaxWorlds    = 4
	instantAbandonAfter = 5 * time.Minute
	presenceWriteEvery  = 30 * time.Second
	instantKeep         = 24 * time.Hour
	instantSweepEvery   = time.Minute
	minWaitMs           = 1000
)

// instantInfo is an instant world's row read against the wall clock.
type instantInfo struct {
	WorldSummary
	ready    bool // created in advance, paused until opened
	running  bool // opened and not yet over
	held     bool // running, or paused by the admin with players in it: the players' game all the same
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
		info.held = !info.ready && now < info.endsAtMs
	default:
		info.running = now < info.endsAtMs
		info.held = info.running
	}
	return info
}

// takesPlayers: a running instant world with a free seat (it accepts players until it ends).
func (s *Service) takesPlayers(w instantInfo) bool {
	return w.running && WorldNow(&w.WorldRow, s.clock.NowMs()) < int64(w.Config.DurationMin)*game.MsPerMinute &&
		w.Humans < w.Config.HumanSeats
}

// InstantGame is one row of the list a player picks a game from.
type InstantGame struct {
	ID           int64  `json:"id"`
	GameNumber   int    `json:"gameNumber"`
	EndsAt       int64  `json:"endsAt"` // wall ms
	Humans       int    `json:"humans"`
	Seats        int    `json:"seats"`
	Leader       string `json:"leader"` // "" while nobody has points
	LeaderPoints int    `json:"leaderPoints"`
	Mine         bool   `json:"mine"`
}

// InstantList is the running instant games, the account's own first, then the newest.
type InstantList struct {
	Games    []InstantGame `json:"games"`
	CanStart bool          `json:"canStart"`         // fewer games run than the cap and the account is in none
	WaitMs   int64         `json:"waitMs,omitempty"` // set when nothing has a free seat and no game can be started
}

// runningInstantOf is the running instant game the account plays in, or 0.
func (s *Service) runningInstantOf(ctx context.Context, accountID int64) (int64, error) {
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return 0, err
	}
	members, err := s.worlds.MembershipsOf(ctx, accountID)
	if err != nil {
		return 0, err
	}
	for _, m := range members {
		for _, w := range worlds {
			if w.ID == m.WorldID && w.held {
				return w.ID, nil
			}
		}
	}
	return 0, nil
}

// InstantGames lists the running instant games for the account to choose from.
func (s *Service) InstantGames(ctx context.Context, acct *Account) (InstantList, error) {
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return InstantList{}, err
	}
	mine, err := s.runningInstantOf(ctx, acct.ID)
	if err != nil {
		return InstantList{}, err
	}
	out := InstantList{Games: []InstantGame{}}
	running, free, wait := 0, false, int64(0)
	for _, w := range worlds {
		if !w.running {
			continue
		}
		running++
		if left := w.endsAtMs - s.clock.NowMs(); wait == 0 || left < wait {
			wait = left
		}
		if w.Humans < w.Config.HumanSeats {
			free = true
		}
		g := InstantGame{ID: w.ID, GameNumber: w.GameNumber, EndsAt: w.endsAtMs, Humans: w.Humans, Seats: w.Config.HumanSeats, Mine: w.ID == mine}
		err := s.worlds.Read(ctx, w.ID, func(tx WorldTx) error {
			if p, points := game.Leader(tx); p != nil && points > 0 {
				g.Leader, g.LeaderPoints = p.Name, points
			}
			return nil
		})
		if err != nil {
			return InstantList{}, err
		}
		out.Games = append(out.Games, g)
	}
	sort.SliceStable(out.Games, func(i, j int) bool {
		if out.Games[i].Mine != out.Games[j].Mine {
			return out.Games[i].Mine
		}
		return out.Games[i].EndsAt > out.Games[j].EndsAt
	})
	out.CanStart = mine == 0 && running < s.cfg.InstantMaxWorlds
	if !out.CanStart && !free && mine == 0 {
		out.WaitMs = max(minWaitMs, wait)
	}
	return out, nil
}

// Play puts the account into an instant game. With a world id it joins that running game (the one it already plays
// included); without, it continues the account's game or opens a new one while fewer than Config.InstantMaxWorlds
// run, and with all of them running it says how long until one ends.
func (s *Service) Play(ctx context.Context, acct *Account, worldID int64) (PlayResult, error) {
	s.instantMu.Lock()
	defer s.instantMu.Unlock()
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return PlayResult{}, err
	}
	mine, err := s.runningInstantOf(ctx, acct.ID)
	if err != nil {
		return PlayResult{}, err
	}
	if worldID != 0 {
		return s.playIn(ctx, acct, worlds, mine, worldID)
	}
	if mine != 0 {
		return PlayResult{WorldID: mine}, nil
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

func (s *Service) playIn(ctx context.Context, acct *Account, worlds []instantInfo, mine, worldID int64) (PlayResult, error) {
	if mine == worldID {
		return PlayResult{WorldID: mine}, nil
	}
	if mine != 0 {
		return PlayResult{}, &ErrValidation{Field: "world", Code: "inAnotherGame"}
	}
	closed := &ErrValidation{Field: "world", Code: "worldClosed"}
	for _, w := range worlds {
		if w.ID != worldID {
			continue
		}
		if !w.running || w.Humans >= w.Config.HumanSeats {
			return PlayResult{}, closed
		}
		res, err := s.joinInstant(ctx, acct, worldID)
		if errors.Is(err, ErrWorldEnded) || errors.Is(err, ErrWorldPaused) {
			return PlayResult{}, closed
		}
		return res, err
	}
	return PlayResult{}, closed
}

func (s *Service) joinInstant(ctx context.Context, acct *Account, id int64) (PlayResult, error) {
	if _, err := s.JoinWorld(ctx, acct, id, game.QuadrantRandom); err != nil {
		return PlayResult{}, err
	}
	s.MarkPresent(ctx, id)
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

// MarkPresent records that a human has the world open, at most once per presenceWriteEvery per world and process.
func (s *Service) MarkPresent(ctx context.Context, worldID int64) {
	now := s.clock.NowMs()
	s.presenceMu.Lock()
	if last, ok := s.presentAt[worldID]; ok && now-last < presenceWriteEvery.Milliseconds() {
		s.presenceMu.Unlock()
		return
	}
	if s.presentAt == nil {
		s.presentAt = map[int64]int64{}
	}
	s.presentAt[worldID] = now
	s.presenceMu.Unlock()
	if err := s.worlds.MarkPresent(ctx, worldID, now); err != nil && !errors.Is(err, ErrNotFound) && ctx.Err() == nil {
		slog.Error("instant: presence", "world", worldID, "err", err)
	}
}

// EndAbandonedInstant ends the running instant worlds no human has had open for instantAbandonAfter, counted from
// the later of the last presence and the moment the world started running. The ready and the paused worlds wait on
// purpose and are left alone.
func (s *Service) EndAbandonedInstant(ctx context.Context) (int, error) {
	worlds, err := s.instantWorlds(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, w := range worlds {
		if !w.running || s.clock.NowMs()-s.lastActive(&w.WorldRow) < instantAbandonAfter.Milliseconds() {
			continue
		}
		ended := false
		err := s.worlds.Run(ctx, w.ID, func(tx WorldTx) error {
			row := tx.Row()
			wall := s.clock.NowMs()
			if row.PausedAtMs != nil || row.State.EndedAt != nil || wall-s.lastActive(row) < instantAbandonAfter.Milliseconds() {
				return nil
			}
			now := WorldNow(row, wall)
			tx.SetNow(now)
			if _, err := processDue(tx, now, 0); err != nil {
				return err
			}
			if game.Ended(tx) {
				return nil
			}
			tx.SetNow(now)
			tx.LogAction(actorWorld, false, "abandon", struct{}{})
			game.EndAbandoned(tx)
			ended = true
			return nil
		})
		if err != nil {
			return n, err
		}
		if ended {
			n++
			s.wakeWorker(w.ID)
		}
	}
	return n, nil
}

// lastActive is the wall time a human last had the world open, or else when it started running; this process
// cannot have seen anyone before it started, so a restart gives everyone the full grace time again.
func (s *Service) lastActive(row *WorldRow) int64 {
	return max(row.LastPresentMs, row.StartedAtMs+row.PausedTotalMs, s.bootMs)
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
		if _, err := s.EndAbandonedInstant(ctx); err != nil && ctx.Err() == nil {
			slog.Error("instant: abandoned", "err", err)
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
