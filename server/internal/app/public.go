package app

import (
	"context"
	"errors"
	"time"

	"librehold/internal/game"
)

const (
	liveGameCacheFor = 30 * time.Second
	statsCacheFor    = time.Minute
	statsMinGames    = 20
	statsRecentSpan  = 24 * time.Hour
)

// PublicStats is the landing page's numbers: real totals, hidden until enough games were played.
type PublicStats struct {
	Visible bool       `json:"visible"`
	AllTime StatTotals `json:"allTime"`
	Last24h StatTotals `json:"last24h"`
	Open    *OpenGame  `json:"open,omitempty"`
}

// OpenGame is the instant game taking players right now.
type OpenGame struct {
	Seats      int   `json:"seats"`
	ClosesInMs int64 `json:"closesInMs"`
}

type statsCache struct {
	at    int64
	stats PublicStats
}

// PublicStats are cached for a minute, so the landing page never adds load to the game tables.
func (s *Service) PublicStats(ctx context.Context) (PublicStats, error) {
	now := s.clock.NowMs()
	s.mu.Lock()
	c := s.stats
	s.mu.Unlock()
	if c != nil && now-c.at < statsCacheFor.Milliseconds() {
		return c.stats, nil
	}
	all, err := s.worlds.ArchiveTotals(ctx, 0)
	if err != nil {
		return PublicStats{}, err
	}
	recent, err := s.worlds.ArchiveTotals(ctx, now-statsRecentSpan.Milliseconds())
	if err != nil {
		return PublicStats{}, err
	}
	out := PublicStats{Visible: all.Games >= statsMinGames, AllTime: all, Last24h: recent}
	if worlds, err := s.instantWorlds(ctx); err == nil {
		if seats, closes, ok := s.openInstant(worlds); ok {
			out.Open = &OpenGame{Seats: seats, ClosesInMs: closes}
		}
	}
	s.mu.Lock()
	s.stats = &statsCache{at: now, stats: out}
	s.mu.Unlock()
	return out, nil
}

// PublicGame is an instant game's page: from the archive, from the ended world while it isn't archived yet, or the
// running game's standings. A ready world nobody has entered and a long world are not found.
func (s *Service) PublicGame(ctx context.Context, worldID int64) (*ArchivedGame, error) {
	g, err := s.worlds.ArchivedGame(ctx, worldID)
	if err == nil {
		return publicRows(g), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	row, err := s.worlds.Get(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if ended, ok := archivedGame(row); ok {
		return publicRows(&ended), nil
	}
	if !game.IsInstant(&row.Config) || row.State.EndedAt != nil {
		return nil, ErrNotFound
	}
	return s.runningGame(ctx, worldID)
}

// ResolveGameRef is the world a /games/{ref} link names: an instant game by its id, a player world's game only by its
// results code and only once it has ended.
func (s *Service) ResolveGameRef(ctx context.Context, ref string) (int64, error) {
	if id, ok := positiveID(ref); ok {
		g, err := s.worlds.ArchivedGame(ctx, id)
		switch {
		case err == nil && g.Kind == game.KindPlayer:
			return 0, ErrNotFound
		case err == nil:
			return id, nil
		case !errors.Is(err, ErrNotFound):
			return 0, err
		}
		row, err := s.worlds.Get(ctx, id)
		if err != nil {
			return 0, err
		}
		if row.Config.Kind == game.KindPlayer {
			return 0, ErrNotFound
		}
		return id, nil
	}
	if ref == "" {
		return 0, ErrNotFound
	}
	g, err := s.worlds.ArchivedGameByCode(ctx, ref)
	if err == nil {
		return g.WorldID, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return 0, err
	}
	row, err := s.worlds.WorldByResultsCode(ctx, ref)
	if err != nil {
		return 0, err
	}
	if row.State.EndedAt == nil {
		return 0, ErrNotFound
	}
	return row.ID, nil
}

type liveGame struct {
	at   int64
	game *ArchivedGame
}

// runningGame builds a live game's standings at most once per liveGameCacheFor, so a shared link never loads the world.
func (s *Service) runningGame(ctx context.Context, worldID int64) (*ArchivedGame, error) {
	now := s.clock.NowMs()
	s.mu.Lock()
	c, ok := s.liveGames[worldID]
	s.mu.Unlock()
	if ok && now-c.at < liveGameCacheFor.Milliseconds() {
		return c.game, nil
	}
	var g *ArchivedGame
	err := s.worlds.Read(ctx, worldID, func(w WorldTx) error {
		row := w.Row()
		a := game.BuildArchive(w)
		if row.PausedAtMs != nil && a.Summary.Humans == 0 {
			return ErrNotFound
		}
		start := row.StartedAtMs + row.PausedTotalMs
		g = publicRows(&ArchivedGame{WorldID: row.ID, Name: row.Name, GameNumber: row.GameNumber, StartedAtMs: start,
			Running: true, EndsAtMs: start + int64(w.Config().DurationMin)*game.MsPerMinute, Summary: a.Summary, Rows: a.Rows})
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.liveGames == nil {
		s.liveGames = map[int64]liveGame{}
	}
	for id, old := range s.liveGames {
		if now-old.at >= liveGameCacheFor.Milliseconds() {
			delete(s.liveGames, id)
		}
	}
	s.liveGames[worldID] = liveGame{at: now, game: g}
	s.mu.Unlock()
	return g, nil
}

// publicRows drops what links a row to an account: the account id and the player key.
func publicRows(g *ArchivedGame) *ArchivedGame {
	out := *g
	out.Rows = make([]game.LeaderRow, len(g.Rows))
	for i, r := range g.Rows {
		r.AccountID, r.Key = 0, ""
		out.Rows[i] = r
	}
	return &out
}

// PublicProfile is an account's public page: its nickname, the tally of its instant games and the games. Its player
// worlds' games are private: only the account itself sees them.
type PublicProfile struct {
	Username string    `json:"username"`
	IsGuest  bool      `json:"isGuest"`
	Tally    Tally     `json:"tally"`
	Games    []GameRef `json:"games"`
}

type Tally struct {
	Games     int   `json:"games"`
	Wins      int   `json:"wins"`
	Conquered int   `json:"conquered"`
	Killed    int64 `json:"killed"`
}

func (s *Service) PublicProfile(ctx context.Context, username string, viewer *Account) (*PublicProfile, error) {
	acct, err := s.accounts.ByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	games, err := s.worlds.AccountGames(ctx, acct.ID)
	if err != nil {
		return nil, err
	}
	own := viewer != nil && viewer.ID == acct.ID
	p := &PublicProfile{Username: acct.Username, IsGuest: acct.IsGuest, Games: []GameRef{}}
	for _, g := range games {
		if g.Kind != game.KindPlayer || own {
			p.Games = append(p.Games, g)
		}
	}
	for _, g := range p.Games {
		p.Tally.Games++
		if g.Rank == 1 {
			p.Tally.Wins++
		}
		p.Tally.Conquered += g.Conquered
		p.Tally.Killed += g.Killed
	}
	return p, nil
}
