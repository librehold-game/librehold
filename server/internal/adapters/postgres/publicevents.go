package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (u *uow) AddPublicEvent(e game.PublicEvent) {
	e.At, e.WorldID = app.WallTime(&u.row, u.now), u.row.ID
	u.public = append(u.public, e)
}

func (u *uow) flushPublicEvents(b *pgx.Batch) {
	for _, e := range u.public {
		b.Queue(`insert into public_events (world_id, at, kind, attacker, defender, attacker_bot, defender_bot, troops, x, y)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			e.WorldID, e.At, e.Kind, e.Attacker, e.Defender, e.AttackerBot, e.DefenderBot, e.Troops, e.X, e.Y)
	}
	u.public = nil
}

func (s *Worlds) PublicEvents(ctx context.Context, q app.PublicEventQuery) ([]game.PublicEvent, error) {
	rows, err := s.pool.Query(ctx, `select e.at, e.kind, e.world_id, w.name, coalesce(w.config->>'kind', ''),
			coalesce(w.game_number, 0), e.attacker, e.defender, e.attacker_bot, e.defender_bot, e.troops, e.x, e.y
		from public_events e join worlds w on w.id = e.world_id
		where ($1 = 0 or e.world_id = $1) and (e.kind <> 'battle' or e.troops >= $2)
		order by e.at desc, e.world_id desc, e.seq desc
		limit nullif($3, 0)`, q.WorldID, q.MinTroops, q.Limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (game.PublicEvent, error) {
		var e game.PublicEvent
		err := r.Scan(&e.At, &e.Kind, &e.WorldID, &e.WorldName, &e.WorldKind, &e.GameNumber, &e.Attacker, &e.Defender,
			&e.AttackerBot, &e.DefenderBot, &e.Troops, &e.X, &e.Y)
		e.At = e.At.UTC()
		return e, err
	})
}

func (s *Worlds) LastBattleAt(ctx context.Context) (time.Time, bool, error) {
	var at *time.Time
	if err := s.pool.QueryRow(ctx, `select max(at) from public_events`).Scan(&at); err != nil || at == nil {
		return time.Time{}, false, err
	}
	return at.UTC(), true, nil
}
