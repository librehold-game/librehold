package memory

import (
	"context"
	"sort"
	"strconv"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (s *Worlds) SaveArchive(_ context.Context, g app.ArchivedGame) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.archives == nil {
		s.archives = map[int64]app.ArchivedGame{}
	}
	if _, ok := s.archives[g.WorldID]; !ok {
		if g.Kind == "" {
			g.Kind = game.KindInstant
		}
		g.Rows = append([]game.LeaderRow(nil), g.Rows...)
		s.archives[g.WorldID] = g
	}
	return nil
}

func (s *Worlds) ArchivedGame(_ context.Context, worldID int64) (*app.ArchivedGame, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.archives[worldID]
	if !ok {
		return nil, app.ErrNotFound
	}
	g.Rows = append([]game.LeaderRow(nil), g.Rows...)
	return &g, nil
}

func (s *Worlds) ArchivedGameByCode(ctx context.Context, code string) (*app.ArchivedGame, error) {
	s.mu.Lock()
	id, found := int64(0), false
	for _, g := range s.archives {
		if code != "" && g.ResultsCode == code {
			id, found = g.WorldID, true
		}
	}
	s.mu.Unlock()
	if !found {
		return nil, app.ErrNotFound
	}
	return s.ArchivedGame(ctx, id)
}

func (s *Worlds) HostedGames(_ context.Context, hostID int64, limit int) ([]app.GameRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []app.GameRef{}
	for _, g := range s.archives {
		if hostID != 0 && g.HostID == hostID {
			out = append(out, app.GameRef{WorldID: g.WorldID, Name: g.Name, Kind: g.Kind, Code: g.ResultsCode, EndedAtMs: g.EndedAtMs,
				Players: len(g.Rows)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EndedAtMs != out[j].EndedAtMs {
			return out[i].EndedAtMs > out[j].EndedAtMs
		}
		return out[i].WorldID > out[j].WorldID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// public is an archived game the site's totals and listings count: an instant game, not a private player world.
func public(g app.ArchivedGame) bool { return g.Kind == game.KindInstant }

func (s *Worlds) ArchiveTotals(_ context.Context, sinceMs int64) (app.StatTotals, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var t app.StatTotals
	players := map[string]bool{}
	for _, g := range s.archives {
		if g.EndedAtMs < sinceMs || !public(g) {
			continue
		}
		t.Games++
		t.Conquests += int64(g.Summary.ConquestsHumans)
		t.UnitsKilled += g.Summary.HumanUnitsKilled
		for _, r := range g.Rows {
			if who := playerOf(r); who != "" {
				players[who] = true
			}
		}
	}
	t.Players = int64(len(players))
	return t, nil
}

func (s *Worlds) AccountGames(_ context.Context, accountID int64) ([]app.GameRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []app.GameRef
	for _, g := range s.archives {
		for _, r := range g.Rows {
			if !r.IsBot && r.AccountID == accountID {
				out = append(out, app.GameRef{WorldID: g.WorldID, Name: g.Name, GameNumber: g.GameNumber, Kind: g.Kind, Code: g.ResultsCode,
					EndedAtMs: g.EndedAtMs, Rank: r.Rank, Players: len(g.Rows), Points: r.Points, Conquered: r.Conquered, Killed: r.Killed})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndedAtMs > out[j].EndedAtMs })
	return out, nil
}

func (s *Worlds) ArchivedGameList(_ context.Context, limit int) ([]app.GameListing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]app.GameListing, 0, len(s.archives))
	for _, g := range s.archives {
		if public(g) {
			out = append(out, app.GameListing{WorldID: g.WorldID, Name: g.Name, GameNumber: g.GameNumber, EndedAtMs: g.EndedAtMs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EndedAtMs > out[j].EndedAtMs })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Worlds) AccountsWithGames(context.Context) (map[int64]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[int64]int64{}
	for _, g := range s.archives {
		if !public(g) {
			continue
		}
		for _, r := range g.Rows {
			if !r.IsBot && r.AccountID != 0 && g.EndedAtMs > out[r.AccountID] {
				out[r.AccountID] = g.EndedAtMs
			}
		}
	}
	return out, nil
}

// playerOf is who a human row counts as among a game's players: its player key, else its account.
func playerOf(r game.LeaderRow) string {
	switch {
	case r.IsBot:
		return ""
	case r.Key != "":
		return r.Key
	case r.AccountID != 0:
		return strconv.FormatInt(r.AccountID, 10)
	}
	return ""
}

func (s *Worlds) ForgetAccount(_ context.Context, accountID int64, playerKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, g := range s.archives {
		if accountID != 0 && g.HostID == accountID {
			g.HostID, g.HostName = 0, ""
		}
		for i, r := range g.Rows {
			if r.IsBot || !(r.AccountID == accountID || playerKey != "" && r.Key == playerKey) {
				continue
			}
			if r.PlayerID == g.Summary.WinnerID {
				g.WinnerName = ""
			}
			g.Rows[i].AccountID, g.Rows[i].Name = 0, ""
		}
		s.archives[id] = g
	}
	return nil
}
