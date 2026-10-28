package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5"

	"librehold/internal/app"
	"librehold/internal/game"
)

// entry is one tracked entity: its current value (nil = absent), its JSON right after loading, and whether
// a row exists in the database.
type entry[T any] struct {
	v      *T
	orig   []byte
	stored bool
}

type idmap[K comparable, T any] map[K]*entry[T]

func marshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(&game.StoreError{Err: err})
	}
	return b
}

// track decodes a loaded row into the map, or returns the pointer already tracked under k.
func track[K comparable, T any](m idmap[K, T], k K, data []byte) *T {
	if e, ok := m[k]; ok {
		return e.v
	}
	v := new(T)
	if err := json.Unmarshal(data, v); err != nil {
		panic(&game.StoreError{Err: err})
	}
	m[k] = &entry[T]{v: v, orig: marshal(v), stored: true}
	return v
}

func (m idmap[K, T]) put(k K, v *T) {
	if e, ok := m[k]; ok {
		e.v = v
		return
	}
	m[k] = &entry[T]{v: v}
}

// changesOrig calls write for every entity to insert or update (with its JSON) and del for every stored one
// that is gone, in key order; both get the JSON as loaded (nil for a new entity).
func changesOrig[K ~string, T any](m idmap[K, T], write func(k K, v *T, data, orig []byte), del func(k K, orig []byte)) {
	for _, k := range sortedKeys(m) {
		e := m[k]
		switch {
		case e.v == nil && e.stored:
			del(k, e.orig)
		case e.v != nil:
			if b := marshal(e.v); !e.stored || !bytes.Equal(b, e.orig) {
				write(k, e.v, b, e.orig)
			}
		}
	}
}

func changes[K ~string, T any](m idmap[K, T], write func(k K, v *T, data []byte), del func(k K)) {
	changesOrig(m, func(k K, v *T, data, _ []byte) { write(k, v, data) }, func(k K, _ []byte) { del(k) })
}

func sortedKeys[K ~string, T any](m idmap[K, T]) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

type intelKey string // player + "\x00" + village

func makeIntelKey(player game.PlayerID, village string) intelKey {
	return intelKey(player + "\x00" + village)
}

type reportKey string // recipient + "\x00" + id

// uow is the Postgres game.World of one transaction. Entities load on first use and are written at flush
// when their JSON changed; events write through. A read-only uow keeps its writes, events included, in
// memory and never flushes.
type uow struct {
	ctx      context.Context
	tx       pgx.Tx
	readOnly bool

	row       app.WorldRow
	now       int64
	nextID    int64
	eventSeq  int64
	origRow   []byte // the row with its counters, to know whether to write it
	origWorld []byte // the row alone, to know whether players must hear of it

	villages   idmap[string, game.Village]
	players    idmap[string, game.Player]
	accounts   map[string]int64 // account ids as loaded
	allPlayers bool
	brains     idmap[string, game.BotBrain]
	humans     idmap[string, game.HumanState]
	commands   idmap[string, game.Command]
	intel      idmap[intelKey, game.Intel]
	sends      idmap[string, game.ScheduledSend]
	reports    idmap[reportKey, game.Report]
	newReports map[game.PlayerID][]*game.Report
	news       []*game.NewsItem
	conquests  []game.Conquest
	public     []game.PublicEvent
	actions    []game.LoggedAction
	timeline   []game.TimelineEntry

	eventOverlay map[string]*int64 // read-only: scheduled (time) or unscheduled (nil) keys
	scheduled    bool
	statsRan     bool
	changed      app.Changes // filled by flush

	touched map[game.PlayerID]bool
}

func (u *uow) must(err error) {
	if err != nil {
		panic(&game.StoreError{Err: err})
	}
}

func (u *uow) rowState() []byte {
	return marshal([]any{u.row, u.nextID, u.eventSeq, hostKeys(u.row.Host)})
}

func loadUow(ctx context.Context, tx pgx.Tx, worldID int64, readOnly bool) (*uow, error) {
	u := &uow{ctx: ctx, tx: tx, readOnly: readOnly,
		villages: idmap[string, game.Village]{}, players: idmap[string, game.Player]{}, accounts: map[string]int64{},
		brains: idmap[string, game.BotBrain]{}, humans: idmap[string, game.HumanState]{},
		commands: idmap[string, game.Command]{}, intel: idmap[intelKey, game.Intel]{},
		sends: idmap[string, game.ScheduledSend]{}, reports: idmap[reportKey, game.Report]{},
		newReports: map[game.PlayerID][]*game.Report{}, eventOverlay: map[string]*int64{},
		touched: map[game.PlayerID]bool{}}
	row, err := scanWorldRow(tx.QueryRow(ctx, `select `+worldCols+`, next_id, event_seq from worlds where id = $1`, worldID),
		&u.nextID, &u.eventSeq)
	if err != nil {
		return nil, err
	}
	u.row = *row
	u.origRow = u.rowState()
	u.origWorld = marshal(u.row)
	return u, nil
}

// call runs fn, turning a panic with a *game.StoreError into the error.
func call(u *uow, fn func(app.WorldTx) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*game.StoreError)
			if !ok {
				panic(r)
			}
			err = se
		}
	}()
	return fn(u)
}

func (u *uow) Row() *app.WorldRow        { return &u.row }
func (u *uow) State() *game.WorldState   { return &u.row.State }
func (u *uow) Now() int64                { return u.now }
func (u *uow) SetNow(t int64)            { u.now = t }
func (u *uow) Seed() uint32              { return u.row.Seed }
func (u *uow) Secret() []byte            { return u.row.Secret }
func (u *uow) Config() *game.WorldConfig { return &u.row.Config }
func (u *uow) ClockOrigin() int64        { return app.ClockOrigin(&u.row, u.now) }

func (u *uow) NewID(prefix string) string {
	id := prefix + "_" + strconv.FormatInt(u.nextID, 36)
	u.nextID++
	return id
}

func (u *uow) touch(owner game.PlayerRef) {
	if id := game.PlayerID(owner); game.IsHumanID(id) {
		u.touched[id] = true
	}
}

func (u *uow) Notify(p game.PlayerID) {
	u.changed.Viewers = append(u.changed.Viewers, game.Ref(p))
}

func (u *uow) TouchedHumans() []game.PlayerID {
	out := make([]game.PlayerID, 0, len(u.touched))
	for id := range u.touched {
		out = append(out, id)
	}
	sort.Strings(out)
	u.touched = map[game.PlayerID]bool{}
	return out
}

// flush writes every change in one batch.
func (u *uow) flush() error {
	b := &pgx.Batch{}
	id := u.row.ID
	u.flushVillages(b)
	u.flushPlayers(b)
	u.flushCommands(b)
	u.flushRecords(b)
	u.flushActions(b)
	u.flushTimeline(b)
	u.notify(b)
	if state := u.rowState(); !bytes.Equal(state, u.origRow) {
		r := u.row
		args := append([]any{id, r.Name, r.Status, int64(r.Seed), marshal(r.Config), r.StartedAtMs, r.PausedAtMs, r.PausedTotalMs, u.nextID,
			u.eventSeq, marshal(r.State)}, hostArgs(r.Host)...)
		b.Queue(`update worlds set name = $2, status = $3, seed = $4, config = $5, started_at_ms = $6, paused_at_ms = $7,
			paused_total_ms = $8, next_id = $9, event_seq = $10, state = $11, host_account_id = $12, invite_code = $13,
			results_code = $14, host = $15 where id = $1`, args...)
	}
	if b.Len() == 0 {
		return nil
	}
	return u.tx.SendBatch(u.ctx, b).Close()
}

var _ app.WorldTx = (*uow)(nil)
