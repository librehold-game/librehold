package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (s *Worlds) SaveArchive(ctx context.Context, g app.ArchivedGame) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `insert into instant_archive (world_id, name, game_number, started_at_ms, ended_at_ms, winner_name,
				winner_is_bot, summary, kind, results_code, host_account_id, host_name)
			values ($1, $2, nullif($3, 0), $4, $5, $6, $7, $8, coalesce(nullif($9, ''), 'instant'), $10, $11, $12)
			on conflict (world_id) do nothing`,
			g.WorldID, g.Name, g.GameNumber, g.StartedAtMs, g.EndedAtMs, g.WinnerName, g.WinnerIsBot, marshal(g.Summary), string(g.Kind),
			nullIfEmpty(g.ResultsCode), accountArg(g.HostID), g.HostName)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		b := &pgx.Batch{}
		for _, r := range g.Rows {
			b.Queue(`insert into instant_leaderboard (world_id, rank, player_id, name, is_bot, account_id, player_key, points,
				villages, conquered, killed) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				g.WorldID, r.Rank, string(r.PlayerID), r.Name, r.IsBot, accountArg(r.AccountID), r.Key, r.Points, r.Villages,
				r.Conquered, r.Killed)
		}
		return tx.SendBatch(ctx, b).Close()
	})
}

func (s *Worlds) ArchivedGame(ctx context.Context, worldID int64) (*app.ArchivedGame, error) {
	g := &app.ArchivedGame{WorldID: worldID}
	var summary []byte
	var kind string
	err := s.pool.QueryRow(ctx, `select name, coalesce(game_number, 0), started_at_ms, ended_at_ms, winner_name, winner_is_bot, summary,
			kind, coalesce(results_code, ''), coalesce(host_account_id, 0), host_name
		from instant_archive where world_id = $1`, worldID).Scan(&g.Name, &g.GameNumber, &g.StartedAtMs, &g.EndedAtMs, &g.WinnerName,
		&g.WinnerIsBot, &summary, &kind, &g.ResultsCode, &g.HostID, &g.HostName)
	if err != nil {
		return nil, notFound(err)
	}
	g.Kind = game.WorldKind(kind)
	if err := json.Unmarshal(summary, &g.Summary); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `select rank, player_id, name, is_bot, coalesce(account_id, 0), player_key, points, villages,
			conquered, killed
		from instant_leaderboard where world_id = $1 order by rank`, worldID)
	if err != nil {
		return nil, err
	}
	g.Rows, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (game.LeaderRow, error) {
		var l game.LeaderRow
		var id string
		err := r.Scan(&l.Rank, &id, &l.Name, &l.IsBot, &l.AccountID, &l.Key, &l.Points, &l.Villages, &l.Conquered, &l.Killed)
		l.PlayerID = game.PlayerID(id)
		return l, err
	})
	if g.Rows == nil {
		g.Rows = []game.LeaderRow{}
	}
	return g, err
}

func (s *Worlds) ArchiveTotals(ctx context.Context, sinceMs int64) (app.StatTotals, error) {
	var t app.StatTotals
	err := s.pool.QueryRow(ctx, `select count(*),
			coalesce(sum((summary->>'conquestsHumans')::bigint), 0),
			coalesce(sum((summary->>'humanUnitsKilled')::bigint), 0),
			(select count(distinct coalesce(nullif(l.player_key, ''), l.account_id::text))
				from instant_leaderboard l join instant_archive a using (world_id)
				where not l.is_bot and (l.player_key <> '' or l.account_id is not null) and a.ended_at_ms >= $1 and a.kind = 'instant')
		from instant_archive where ended_at_ms >= $1 and kind = 'instant'`, sinceMs).Scan(&t.Games, &t.Conquests, &t.UnitsKilled, &t.Players)
	return t, err
}

func (s *Worlds) AccountGames(ctx context.Context, accountID int64) ([]app.GameRef, error) {
	rows, err := s.pool.Query(ctx, `select a.world_id, a.name, coalesce(a.game_number, 0), a.kind, coalesce(a.results_code, ''),
			a.ended_at_ms, l.rank, (select count(*) from instant_leaderboard x where x.world_id = a.world_id), l.points, l.conquered, l.killed
		from instant_leaderboard l join instant_archive a using (world_id)
		where l.account_id = $1 and not l.is_bot order by a.ended_at_ms desc`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.GameRef, error) {
		var g app.GameRef
		var kind string
		err := r.Scan(&g.WorldID, &g.Name, &g.GameNumber, &kind, &g.Code, &g.EndedAtMs, &g.Rank, &g.Players, &g.Points, &g.Conquered, &g.Killed)
		g.Kind = game.WorldKind(kind)
		return g, err
	})
}

func (s *Worlds) ArchivedGameByCode(ctx context.Context, code string) (*app.ArchivedGame, error) {
	var id int64
	if err := s.pool.QueryRow(ctx, `select world_id from instant_archive where results_code = $1`, code).Scan(&id); err != nil {
		return nil, notFound(err)
	}
	return s.ArchivedGame(ctx, id)
}

func (s *Worlds) HostedGames(ctx context.Context, hostID int64, limit int) ([]app.GameRef, error) {
	rows, err := s.pool.Query(ctx, `select a.world_id, a.name, a.kind, coalesce(a.results_code, ''), a.ended_at_ms,
			(select count(*) from instant_leaderboard x where x.world_id = a.world_id)
		from instant_archive a where a.host_account_id = $1 order by a.ended_at_ms desc, a.world_id desc limit $2`, hostID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.GameRef, error) {
		var g app.GameRef
		var kind string
		err := r.Scan(&g.WorldID, &g.Name, &kind, &g.Code, &g.EndedAtMs, &g.Players)
		g.Kind = game.WorldKind(kind)
		return g, err
	})
}

func (s *Worlds) ArchivedGameList(ctx context.Context, limit int) ([]app.GameListing, error) {
	q := `select world_id, name, coalesce(game_number, 0), ended_at_ms from instant_archive where kind = 'instant'
		order by ended_at_ms desc, world_id desc`
	args := []any{}
	if limit > 0 {
		q += ` limit $1`
		args = append(args, limit)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.GameListing, error) {
		var g app.GameListing
		err := r.Scan(&g.WorldID, &g.Name, &g.GameNumber, &g.EndedAtMs)
		return g, err
	})
}

func (s *Worlds) AccountsWithGames(ctx context.Context) (map[int64]int64, error) {
	rows, err := s.pool.Query(ctx, `select l.account_id, max(a.ended_at_ms) from instant_leaderboard l join instant_archive a using (world_id)
		where l.account_id is not null and not l.is_bot and a.kind = 'instant' group by l.account_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var id, at int64
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}

func (s *Worlds) ForgetAccount(ctx context.Context, accountID int64, playerKey string) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		const theirs = `not l.is_bot and (l.account_id = $1 or ($2 <> '' and l.player_key = $2))`
		_, err := tx.Exec(ctx, `update instant_archive a set winner_name = '' from instant_leaderboard l
			where l.world_id = a.world_id and l.player_id = a.summary->>'winnerId' and `+theirs, accountID, playerKey)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update instant_leaderboard l set name = '', account_id = null where `+theirs, accountID, playerKey)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update instant_archive set host_name = '', host_account_id = null where host_account_id = $1`, accountID)
		return err
	})
}
