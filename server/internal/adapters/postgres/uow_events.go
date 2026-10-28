package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Events write through to the events table at once, so PopDue sees them in (due, seq) order; a read-only
// unit of work keeps them in eventOverlay instead.

func (u *uow) Schedule(key string, at int64) {
	if at < u.now {
		at = u.now
	}
	if u.readOnly {
		u.eventOverlay[key] = &at
		return
	}
	var seq int64
	err := u.tx.QueryRow(u.ctx, `insert into events (world_id, key, due_at, seq) values ($1, $2, $3, $4)
		on conflict (world_id, key) do update set due_at = excluded.due_at, seq = excluded.seq
		where events.due_at <> excluded.due_at
		returning seq`, u.row.ID, key, at, u.eventSeq).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	u.must(err)
	u.eventSeq++
	u.scheduled = true
}

func (u *uow) Unschedule(key string) {
	if u.readOnly {
		u.eventOverlay[key] = nil
		return
	}
	_, err := u.tx.Exec(u.ctx, `delete from events where world_id = $1 and key = $2`, u.row.ID, key)
	u.must(err)
}

func (u *uow) ScheduledAt(key string) (int64, bool) {
	if at, ok := u.eventOverlay[key]; ok {
		if at == nil {
			return 0, false
		}
		return *at, true
	}
	var at int64
	err := u.tx.QueryRow(u.ctx, `select due_at from events where world_id = $1 and key = $2`, u.row.ID, key).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	u.must(err)
	return at, true
}

func (u *uow) IsScheduled(key string) bool {
	_, ok := u.ScheduledAt(key)
	return ok
}

// PopDue removes and returns the next event with time ≤ until, in (due, seq) order. A read-only unit of work
// never pops.
func (u *uow) PopDue(until int64) (string, int64, bool) {
	if u.readOnly {
		return "", 0, false
	}
	var key string
	var at int64
	err := u.tx.QueryRow(u.ctx, `delete from events where world_id = $1 and key = (
			select key from events where world_id = $1 and due_at <= $2 order by due_at, seq limit 1)
		returning key, due_at`, u.row.ID, until).Scan(&key, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, false
	}
	u.must(err)
	if kind, _, _ := strings.Cut(key, ":"); kind == "stats" {
		u.statsRan = true
	}
	return key, at, true
}
