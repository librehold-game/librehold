package memory

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"sync"

	"librehold/internal/adapters/fanout"
	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

type world struct {
	mu  sync.Mutex
	row app.WorldRow
	mem *gametest.MemWorld
}

// tx is a unit of work over a private copy of a world.
type tx struct {
	*gametest.MemWorld
	row       *app.WorldRow
	statsRan  bool
	scheduled bool
}

func (t *tx) Schedule(key string, at int64) {
	cur, ok := t.ScheduledAt(key)
	t.MemWorld.Schedule(key, at)
	if now, _ := t.ScheduledAt(key); !ok || now != cur {
		t.scheduled = true
	}
}

func (t *tx) PopDue(until int64) (string, int64, bool) {
	key, at, ok := t.MemWorld.PopDue(until)
	if kind, _, _ := strings.Cut(key, ":"); ok && kind == "stats" {
		t.statsRan = true
	}
	return key, at, ok
}

func (t *tx) Row() *app.WorldRow        { return t.row }
func (t *tx) State() *game.WorldState   { return &t.row.State }
func (t *tx) Seed() uint32              { return t.row.Seed }
func (t *tx) Secret() []byte            { return t.row.Secret }
func (t *tx) Config() *game.WorldConfig { return &t.row.Config }
func (t *tx) ClockOrigin() int64        { return app.ClockOrigin(t.row, t.Now()) }

// Worlds is the in-memory WorldStore. Its feeds publish after a unit of work is kept.
type Worlds struct {
	mu           sync.Mutex
	nextID       int64
	nextGame     int64
	worlds       map[int64]*world
	archives     map[int64]app.ArchivedGame
	hostSettings *app.HostSettings
	logs         actionLogs
	timelines    timelines
	updates      *fanout.Hub[app.Update]
	wakeups      *fanout.Hub[int64]
}

func NewWorlds() *Worlds {
	return &Worlds{worlds: map[int64]*world{}, updates: fanout.New[app.Update](), wakeups: fanout.New[int64]()}
}

func (s *Worlds) Updates() app.Updates      { return s.updates }
func (s *Worlds) Wakeups() app.EventWakeups { return s.wakeups }

func (s *Worlds) publish(beforeRow *app.WorldRow, before *gametest.MemWorld, t *tx) {
	c := changes(before, t)
	c.World = beforeRow != nil && !bytes.Equal(jsonOf(*beforeRow), jsonOf(*t.row))
	for _, u := range c.Updates(t.row.ID) {
		s.updates.Publish(u)
	}
	if t.scheduled {
		s.wakeups.Publish(t.row.ID)
	}
}

func (s *Worlds) get(id int64) (*world, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w := s.worlds[id]; w != nil {
		return w, nil
	}
	return nil, app.ErrNotFound
}

func (s *Worlds) all() []*world {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*world, 0, len(s.worlds))
	for _, w := range s.worlds {
		out = append(out, w)
	}
	return out
}

// apply runs fn on a private copy of the world's state and returns it, or the error; a panic with a
// *game.StoreError becomes the error.
func apply(row app.WorldRow, mem *gametest.MemWorld, fn func(app.WorldTx) error) (t *tx, err error) {
	row = cloneRow(row)
	t = &tx{MemWorld: mem, row: &row}
	t.TouchedHumans()
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*game.StoreError)
			if !ok {
				panic(r)
			}
			t, err = nil, se
		}
	}()
	if err := fn(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Worlds) Create(_ context.Context, row app.WorldRow, init func(app.WorldTx) error) (int64, error) {
	s.mu.Lock()
	s.nextID++
	row.ID = s.nextID
	s.mu.Unlock()
	empty := gametest.NewMemWorld(row.Seed, row.Config, 0)
	t, err := apply(row, empty.Clone(), init)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.worlds[row.ID] = &world{row: *t.row, mem: t.MemWorld}
	s.mu.Unlock()
	s.appendLog(row.ID, t.TakeActions())
	s.appendTimeline(row.ID, t.TakeTimeline())
	s.publish(nil, empty, t)
	return row.ID, nil
}

func (s *Worlds) Delete(_ context.Context, worldID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.worlds[worldID] == nil {
		return app.ErrNotFound
	}
	delete(s.worlds, worldID)
	s.dropTimeline(worldID)
	return nil
}

func (s *Worlds) NextGameNumber(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextGame++
	return s.nextGame, nil
}

func (s *Worlds) Run(_ context.Context, worldID int64, fn func(app.WorldTx) error) error {
	w, err := s.get(worldID)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	before, beforeRow := w.mem, w.row
	t, err := apply(w.row, before.Clone(), fn)
	if err != nil {
		return err
	}
	w.row, w.mem = *t.row, t.MemWorld
	s.appendLog(worldID, t.TakeActions())
	s.appendTimeline(worldID, t.TakeTimeline())
	s.publish(&beforeRow, before, t)
	return nil
}

func (s *Worlds) Read(_ context.Context, worldID int64, fn func(app.WorldTx) error) error {
	w, err := s.get(worldID)
	if err != nil {
		return err
	}
	w.mu.Lock()
	row, mem := w.row, w.mem.Clone()
	w.mu.Unlock()
	_, err = apply(row, mem, fn)
	return err
}

func (s *Worlds) List(context.Context) ([]app.WorldSummary, error) {
	var out []app.WorldSummary
	for _, w := range s.all() {
		w.mu.Lock()
		sum := app.WorldSummary{WorldRow: cloneRow(w.row)}
		for _, p := range w.mem.Players() {
			if p.IsHuman {
				if p.Status == game.StatusActive {
					sum.Humans++
				}
			} else if p.Status == game.StatusActive {
				sum.Bots++
			}
		}
		w.mu.Unlock()
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Worlds) Get(_ context.Context, worldID int64) (*app.WorldRow, error) {
	w, err := s.get(worldID)
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	row := cloneRow(w.row)
	return &row, nil
}

func (s *Worlds) MembershipsOf(_ context.Context, accountID int64) ([]app.Membership, error) {
	var out []app.Membership
	for _, w := range s.all() {
		w.mu.Lock()
		for _, p := range w.mem.Players() {
			if p.IsHuman && p.AccountID == accountID {
				out = append(out, app.Membership{WorldID: w.row.ID, PlayerID: p.ID})
			}
		}
		w.mu.Unlock()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].WorldID < out[j].WorldID })
	return out, nil
}

func (s *Worlds) NextDue(_ context.Context, worldID int64) (int64, bool, error) {
	w, err := s.get(worldID)
	if err != nil {
		return 0, false, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	at, ok := w.mem.NextDue()
	return at, ok, nil
}

var (
	_ app.WorldStore = (*Worlds)(nil)
	_ app.WorldTx    = (*tx)(nil)
)
