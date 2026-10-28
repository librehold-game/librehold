package app

import (
	"context"
	"errors"

	"librehold/internal/game"
)

// ArchivedGame is an ended short world as the archive keeps it, after its world rows are gone.
type ArchivedGame struct {
	WorldID     int64               `json:"worldId"`
	Name        string              `json:"name"`
	GameNumber  int                 `json:"gameNumber,omitempty"`
	Kind        game.WorldKind      `json:"kind,omitempty"`
	ResultsCode string              `json:"-"` // a player world's private results link
	HostID      int64               `json:"-"`
	HostName    string              `json:"hostName,omitempty"`
	StartedAtMs int64               `json:"startedAt"` // wall ms
	EndedAtMs   int64               `json:"endedAt"`   // wall ms
	WinnerName  string              `json:"winnerName"`
	WinnerIsBot bool                `json:"winnerIsBot"`
	Running     bool                `json:"running,omitempty"` // a live instant game, never stored
	EndsAtMs    int64               `json:"endsAt,omitempty"`  // wall ms, while running
	Summary     game.ArchiveSummary `json:"summary"`
	Rows        []game.LeaderRow    `json:"rows"`
}

// StatTotals add up archived games; players, conquests and kills count humans only.
type StatTotals struct {
	Games       int64 `json:"games"`
	Players     int64 `json:"players"`
	Conquests   int64 `json:"conquests"`
	UnitsKilled int64 `json:"unitsKilled"`
}

// GameRef is one archived game of an account, for its profile.
type GameRef struct {
	WorldID    int64          `json:"worldId"`
	Name       string         `json:"name"`
	GameNumber int            `json:"gameNumber,omitempty"`
	Kind       game.WorldKind `json:"kind,omitempty"`
	Code       string         `json:"code,omitempty"` // a player world's results code
	EndedAtMs  int64          `json:"endedAt"`
	Rank       int            `json:"rank"`
	Players    int            `json:"players"`
	Points     int            `json:"points"`
	Conquered  int            `json:"conquered"`
	Killed     int64          `json:"killed"`
}

// GameListing is an archived game as lists name it.
type GameListing struct {
	WorldID    int64
	Name       string
	GameNumber int
	EndedAtMs  int64
}

// ArchiveStore keeps ended short worlds for good (implemented by the world stores). Player worlds are private: the
// totals and listings count instant games only.
type ArchiveStore interface {
	SaveArchive(ctx context.Context, g ArchivedGame) error // idempotent per world
	ArchivedGame(ctx context.Context, worldID int64) (*ArchivedGame, error)
	// ArchivedGameByCode is a player world's game by its results code; ErrNotFound for any other.
	ArchivedGameByCode(ctx context.Context, code string) (*ArchivedGame, error)
	ArchiveTotals(ctx context.Context, sinceMs int64) (StatTotals, error)
	AccountGames(ctx context.Context, accountID int64) ([]GameRef, error)
	// HostedGames are the games an account hosted, newest first.
	HostedGames(ctx context.Context, hostID int64, limit int) ([]GameRef, error)
	// ArchivedGameList is the archive newest first; limit <= 0 lists everything.
	ArchivedGameList(ctx context.Context, limit int) ([]GameListing, error)
	// AccountsWithGames maps every account on an archived leaderboard to the end of its latest game.
	AccountsWithGames(ctx context.Context) (map[int64]int64, error)
	// ForgetAccount blanks a deleted account's name and link on old leaderboards, found by its id or its player key
	// (rows of games it left), the winner's name where it won and the host's name and link where it hosted; the
	// numbers and the key stay.
	ForgetAccount(ctx context.Context, accountID int64, playerKey string) error
}

// archivedGame turns an ended world's row into its archive record.
func archivedGame(row *WorldRow) (ArchivedGame, bool) {
	s := row.State
	if s.EndedAt == nil || s.Archive == nil {
		return ArchivedGame{}, false
	}
	start := row.StartedAtMs + row.PausedTotalMs
	g := ArchivedGame{WorldID: row.ID, Name: row.Name, GameNumber: row.GameNumber, Kind: row.Config.Kind, StartedAtMs: start,
		EndedAtMs: start + *s.EndedAt, Summary: s.Archive.Summary, Rows: s.Archive.Rows}
	if h := row.Host; h != nil {
		g.ResultsCode, g.HostID, g.HostName = h.ResultsCode, h.HostID, h.HostName
	}
	for _, r := range g.Rows {
		if r.PlayerID == s.WinnerID {
			g.WinnerName, g.WinnerIsBot = r.Name, r.IsBot
		}
	}
	return g, true
}

// archiveWorld copies an ended world's archive to the permanent store once.
func (s *Service) archiveWorld(ctx context.Context, id int64) error {
	row, err := s.worlds.Get(ctx, id)
	if err != nil {
		return err
	}
	g, ok := archivedGame(row)
	if !ok || row.State.Archived {
		return nil
	}
	if game.RecordsTimeline(&row.Config) {
		if err := s.packTimeline(ctx, id); err != nil {
			return err
		}
	}
	if err := s.worlds.SaveArchive(ctx, g); err != nil {
		return err
	}
	return s.worlds.Run(ctx, id, func(w WorldTx) error {
		w.State().Archived = true
		return nil
	})
}

// ArchiveEnded archives every short world that has ended and isn't archived yet.
func (s *Service) ArchiveEnded(ctx context.Context) error {
	worlds, err := s.shortWorlds(ctx)
	if err != nil {
		return err
	}
	for _, w := range worlds {
		if w.State.EndedAt != nil && !w.State.Archived {
			if err := s.archiveWorld(ctx, w.ID); err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
		}
	}
	return nil
}
