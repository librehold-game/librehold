package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (u *uow) LogAction(actor string, isBot bool, kind string, payload any) {
	u.actions = append(u.actions, game.NewLoggedAction(u.now, actor, isBot, kind, payload))
}

// flushActions numbers the entries after the world's last one; the world's lock keeps the numbering safe.
func (u *uow) flushActions(b *pgx.Batch) {
	for _, a := range u.actions {
		b.Queue(`insert into action_log (world_id, seq, at, actor, is_bot, kind, payload)
			values ($1, (select coalesce(max(seq), 0) + 1 from action_log where world_id = $1), $2, $3, $4, $5, $6)`,
			u.row.ID, a.At, a.Actor, a.IsBot, a.Kind, []byte(a.Payload))
	}
	u.actions = nil
}

func (s *Worlds) ActionLog(ctx context.Context, worldID int64) ([]game.LoggedAction, error) {
	rows, err := s.pool.Query(ctx, `select seq, at, actor, is_bot, kind, payload from action_log
		where world_id = $1 order by seq`, worldID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (game.LoggedAction, error) {
		var a game.LoggedAction
		var payload []byte
		err := r.Scan(&a.Seq, &a.At, &a.Actor, &a.IsBot, &a.Kind, &payload)
		a.Payload = json.RawMessage(payload)
		return a, err
	})
}

func (s *Worlds) CountRejection(ctx context.Context, worldID int64, kind, reason string) error {
	_, err := s.pool.Exec(ctx, `insert into action_rejections (world_id, kind, reason, count) values ($1, $2, $3, 1)
		on conflict (world_id, kind, reason) do update set count = action_rejections.count + 1`, worldID, kind, reason)
	return err
}

func (s *Worlds) Rejections(ctx context.Context, worldID int64) ([]app.Rejection, error) {
	rows, err := s.pool.Query(ctx, `select kind, reason, count from action_rejections where world_id = $1
		order by kind, reason`, worldID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.Rejection, error) {
		var x app.Rejection
		err := r.Scan(&x.Kind, &x.Reason, &x.Count)
		return x, err
	})
}

func (s *Worlds) SavePackedLog(ctx context.Context, p app.PackedLog) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `insert into action_log_packed (world_id, seed, secret, config, build, rows, last_seq, data, packed_at_ms)
			values ($1, $2, coalesce($3, ''::bytea), $4, $5, $6, $7, $8, $9) on conflict (world_id) do nothing`,
			p.WorldID, int64(p.Seed), p.Secret, marshal(p.Config), p.Build, p.Rows, p.LastSeq, p.Data, p.PackedAtMs)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `delete from action_log where world_id = $1 and seq <= $2`, p.WorldID, p.LastSeq)
		return err
	})
}

func (s *Worlds) PackedLog(ctx context.Context, worldID int64) (*app.PackedLog, error) {
	p := &app.PackedLog{WorldID: worldID}
	var seed int64
	var cfg []byte
	err := s.pool.QueryRow(ctx, `select seed, secret, config, build, rows, last_seq, data, packed_at_ms from action_log_packed
		where world_id = $1`, worldID).Scan(&seed, &p.Secret, &cfg, &p.Build, &p.Rows, &p.LastSeq, &p.Data, &p.PackedAtMs)
	if err != nil {
		return nil, notFound(err)
	}
	p.Seed = uint32(seed)
	return p, json.Unmarshal(cfg, &p.Config)
}
