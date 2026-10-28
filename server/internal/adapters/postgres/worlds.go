package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/app"
)

// Worlds is the Postgres WorldStore: every Run is one transaction holding the world's advisory lock.
type Worlds struct{ pool *pgxpool.Pool }

func NewWorlds(pool *pgxpool.Pool) *Worlds { return &Worlds{pool: pool} }

const worldLockClass = 42

const worldCols = `id, name, status, seed, secret, config, started_at_ms, paused_at_ms, paused_total_ms, state, coalesce(game_number, 0),
	host_account_id, invite_code, results_code, host`

func scanWorldRow(row pgx.Row, extra ...any) (*app.WorldRow, error) {
	var r app.WorldRow
	var seed int64
	var config, state, host []byte
	var hostID *int64
	var invite, results *string
	dest := append([]any{&r.ID, &r.Name, &r.Status, &seed, &r.Secret, &config, &r.StartedAtMs, &r.PausedAtMs, &r.PausedTotalMs, &state, &r.GameNumber,
		&hostID, &invite, &results, &host}, extra...)
	if err := row.Scan(dest...); err != nil {
		return nil, notFound(err)
	}
	h, err := scanHost(hostID, invite, results, host)
	if err != nil {
		return nil, err
	}
	r.Host = h
	r.Seed = uint32(seed)
	if err := json.Unmarshal(config, &r.Config); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(state, &r.State); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Worlds) Create(ctx context.Context, row app.WorldRow, init func(app.WorldTx) error) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		args := append([]any{row.Name, row.Status, int64(row.Seed), row.Secret, marshal(row.Config), row.StartedAtMs, row.PausedAtMs, row.PausedTotalMs,
			marshal(row.State), row.GameNumber}, hostArgs(row.Host)...)
		err := tx.QueryRow(ctx, `insert into worlds (name, status, seed, secret, config, started_at_ms, paused_at_ms, paused_total_ms, next_id,
				state, game_number, host_account_id, invite_code, results_code, host)
			values ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, nullif($10, 0), $11, $12, $13, $14) returning id`, args...).Scan(&id)
		if err != nil {
			return err
		}
		u, err := loadUow(ctx, tx, id, false)
		if err != nil {
			return err
		}
		if err := call(u, init); err != nil {
			return err
		}
		return u.flush()
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Worlds) Run(ctx context.Context, worldID int64, fn func(app.WorldTx) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1, $2::int)`, worldLockClass, worldID); err != nil {
			return err
		}
		u, err := loadUow(ctx, tx, worldID, false)
		if err != nil {
			return err
		}
		if err := call(u, fn); err != nil {
			return err
		}
		return u.flush()
	})
}

func (s *Worlds) Read(ctx context.Context, worldID int64, fn func(app.WorldTx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	u, err := loadUow(ctx, tx, worldID, true)
	if err != nil {
		return err
	}
	return call(u, fn)
}

func (s *Worlds) List(ctx context.Context) ([]app.WorldSummary, error) {
	rows, err := s.pool.Query(ctx, `select `+worldCols+`,
			(select count(*) from players p where p.world_id = w.id and p.is_human and p.status = 'active'),
			(select count(*) from players p where p.world_id = w.id and not p.is_human and p.status = 'active')
		from worlds w order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []app.WorldSummary
	for rows.Next() {
		var sum app.WorldSummary
		row, err := scanWorldRow(rows, &sum.Humans, &sum.Bots)
		if err != nil {
			return nil, err
		}
		sum.WorldRow = *row
		out = append(out, sum)
	}
	return out, rows.Err()
}

func (s *Worlds) Delete(ctx context.Context, worldID int64) error {
	tag, err := s.pool.Exec(ctx, `delete from worlds where id = $1`, worldID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return app.ErrNotFound
	}
	return nil
}

func (s *Worlds) Get(ctx context.Context, worldID int64) (*app.WorldRow, error) {
	return scanWorldRow(s.pool.QueryRow(ctx, `select `+worldCols+` from worlds where id = $1`, worldID))
}

func (s *Worlds) MembershipsOf(ctx context.Context, accountID int64) ([]app.Membership, error) {
	rows, err := s.pool.Query(ctx, `select world_id, id from players where account_id = $1 order by world_id`, accountID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Membership, error) {
		var m app.Membership
		err := r.Scan(&m.WorldID, &m.PlayerID)
		return m, err
	})
}

func (s *Worlds) NextDue(ctx context.Context, worldID int64) (int64, bool, error) {
	var at int64
	err := s.pool.QueryRow(ctx, `select due_at from events where world_id = $1 order by due_at, seq limit 1`, worldID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return at, true, nil
}

func (s *Worlds) NextGameNumber(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, `select nextval('game_numbers')`).Scan(&n)
	return n, err
}

var _ app.WorldStore = (*Worlds)(nil)
