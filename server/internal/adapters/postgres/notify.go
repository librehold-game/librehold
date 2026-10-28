package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"librehold/internal/adapters/fanout"
	"librehold/internal/app"
)

const (
	updatesChannel = "lh_updates"
	eventsChannel  = "lh_events"
)

// notify queues the unit of work's notifications; Postgres delivers them only if the transaction commits.
func (u *uow) notify(b *pgx.Batch) {
	u.changed.News = len(u.news) > 0
	u.changed.Ranking = u.statsRan
	u.changed.World = !bytes.Equal(marshal(u.row), u.origWorld)
	for _, up := range u.changed.Updates(u.row.ID) {
		b.Queue(`select pg_notify($1, $2)`, updatesChannel, string(marshal(up)))
	}
	if u.scheduled {
		b.Queue(`select pg_notify($1, $2)`, eventsChannel, strconv.FormatInt(u.row.ID, 10))
	}
}

// Listener receives the notifications of every server instance on one dedicated connection and fans them
// out; it reconnects with backoff, and notifications sent while it is disconnected are lost.
type Listener struct {
	pool      *pgxpool.Pool
	updates   *fanout.Hub[app.Update]
	wakeups   *fanout.Hub[int64]
	ready     chan struct{}
	readyOnce sync.Once
}

func NewListener(pool *pgxpool.Pool) *Listener {
	return &Listener{pool: pool, updates: fanout.New[app.Update](), wakeups: fanout.New[int64](), ready: make(chan struct{})}
}

func (l *Listener) Updates() app.Updates      { return l.updates }
func (l *Listener) Wakeups() app.EventWakeups { return l.wakeups }

// Ready is closed once the listener is listening for the first time.
func (l *Listener) Ready() <-chan struct{} { return l.ready }

// Run listens until ctx ends.
func (l *Listener) Run(ctx context.Context) {
	backoff := 100 * time.Millisecond
	for ctx.Err() == nil {
		err := l.listen(ctx, func() { backoff = 100 * time.Millisecond })
		if ctx.Err() != nil {
			return
		}
		slog.Warn("listener: connection lost", "err", err, "retry", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, 5*time.Second)
	}
}

func (l *Listener) listen(ctx context.Context, connected func()) error {
	pc, err := l.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	conn := pc.Hijack()
	defer func() { _ = conn.Close(context.Background()) }()
	for _, ch := range []string{updatesChannel, eventsChannel} {
		if _, err := conn.Exec(ctx, "listen "+ch); err != nil {
			return err
		}
	}
	connected()
	l.readyOnce.Do(func() { close(l.ready) })
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		switch n.Channel {
		case updatesChannel:
			var up app.Update
			if err := json.Unmarshal([]byte(n.Payload), &up); err != nil {
				slog.Warn("listener: bad update", "payload", n.Payload, "err", err)
				continue
			}
			l.updates.Publish(up)
		case eventsChannel:
			if id, err := strconv.ParseInt(n.Payload, 10, 64); err == nil {
				l.wakeups.Publish(id)
			}
		}
	}
}
