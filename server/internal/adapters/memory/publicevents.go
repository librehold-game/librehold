package memory

import (
	"cmp"
	"context"
	"slices"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

func (t *tx) AddPublicEvent(e game.PublicEvent) {
	e.At, e.WorldID = app.WallTime(t.row, t.Now()), t.row.ID
	t.MemWorld.AddPublicEvent(e)
}

type storedEvent struct {
	game.PublicEvent
	seq int
}

func (s *Worlds) PublicEvents(_ context.Context, q app.PublicEventQuery) ([]game.PublicEvent, error) {
	var all []storedEvent
	for _, w := range s.all() {
		w.mu.Lock()
		if q.WorldID == 0 || w.row.ID == q.WorldID {
			for i, e := range w.mem.PublicEvents() {
				if e.Kind == "battle" && e.Troops < q.MinTroops {
					continue
				}
				e.WorldName, e.WorldKind, e.GameNumber = w.row.Name, w.row.Config.Kind, w.row.GameNumber
				all = append(all, storedEvent{e, i})
			}
		}
		w.mu.Unlock()
	}
	slices.SortFunc(all, func(a, b storedEvent) int {
		return cmp.Or(b.At.Compare(a.At), cmp.Compare(b.WorldID, a.WorldID), cmp.Compare(b.seq, a.seq))
	})
	if q.Limit > 0 && len(all) > q.Limit {
		all = all[:q.Limit]
	}
	out := make([]game.PublicEvent, len(all))
	for i, e := range all {
		out[i] = e.PublicEvent
	}
	return out, nil
}

func (s *Worlds) LastBattleAt(ctx context.Context) (time.Time, bool, error) {
	last, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 1})
	if err != nil || len(last) == 0 {
		return time.Time{}, false, err
	}
	return last[0].At, true, nil
}
