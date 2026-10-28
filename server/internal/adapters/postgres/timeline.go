package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"librehold/internal/game"
)

func (u *uow) AddTimeline(e game.TimelineEntry) { u.timeline = append(u.timeline, e) }

// flushTimeline numbers the entries after the world's last one; the world's lock keeps the numbering safe.
func (u *uow) flushTimeline(b *pgx.Batch) {
	for _, e := range u.timeline {
		b.Queue(`insert into timeline (world_id, seq, at, entry)
			values ($1, (select coalesce(max(seq), 0) + 1 from timeline where world_id = $1), $2, $3)`,
			u.row.ID, e.At, marshal(e))
	}
	u.timeline = nil
}

func (s *Worlds) TimelineEntries(ctx context.Context, worldID int64) ([]game.TimelineEntry, error) {
	rows, err := s.pool.Query(ctx, `select entry from timeline where world_id = $1 order by seq`, worldID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (game.TimelineEntry, error) {
		var raw []byte
		var e game.TimelineEntry
		if err := r.Scan(&raw); err != nil {
			return e, err
		}
		return e, json.Unmarshal(raw, &e)
	})
}

func (s *Worlds) SavePackedTimeline(ctx context.Context, worldID int64, data []byte, entries int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `insert into timeline_packed (world_id, entries, data, packed_at_ms) values ($1, $2, $3, $4)
			on conflict (world_id) do nothing`, worldID, entries, data, time.Now().UnixMilli())
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `delete from timeline where world_id = $1`, worldID)
		return err
	})
}

func (s *Worlds) PackedTimeline(ctx context.Context, worldID int64) ([]byte, error) {
	var data []byte
	if err := s.pool.QueryRow(ctx, `select data from timeline_packed where world_id = $1`, worldID).Scan(&data); err != nil {
		return nil, notFound(err)
	}
	return data, nil
}
