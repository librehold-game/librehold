package memory

import (
	"context"
	"slices"
	"strings"
	"sync"

	"librehold/internal/app"
	"librehold/internal/game"
)

// actionLogs live beside the worlds and, like the archive, outlive them.
type actionLogs struct {
	mu         sync.Mutex
	rows       map[int64][]game.LoggedAction
	rejections map[int64]map[[2]string]int
	packed     map[int64]app.PackedLog
}

// appendLog numbers a kept unit of work's entries after the world's last one.
func (s *Worlds) appendLog(worldID int64, entries []game.LoggedAction) {
	if len(entries) == 0 {
		return
	}
	l := &s.logs
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rows == nil {
		l.rows = map[int64][]game.LoggedAction{}
	}
	rows := l.rows[worldID]
	seq := int64(0)
	if len(rows) > 0 {
		seq = rows[len(rows)-1].Seq
	} else if p, ok := l.packed[worldID]; ok {
		seq = p.LastSeq
	}
	for _, e := range entries {
		seq++
		e.Seq = seq
		rows = append(rows, e)
	}
	l.rows[worldID] = rows
}

func (s *Worlds) ActionLog(_ context.Context, worldID int64) ([]game.LoggedAction, error) {
	s.logs.mu.Lock()
	defer s.logs.mu.Unlock()
	return slices.Clone(s.logs.rows[worldID]), nil
}

func (s *Worlds) CountRejection(_ context.Context, worldID int64, kind, reason string) error {
	l := &s.logs
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rejections == nil {
		l.rejections = map[int64]map[[2]string]int{}
	}
	if l.rejections[worldID] == nil {
		l.rejections[worldID] = map[[2]string]int{}
	}
	l.rejections[worldID][[2]string{kind, reason}]++
	return nil
}

func (s *Worlds) Rejections(_ context.Context, worldID int64) ([]app.Rejection, error) {
	s.logs.mu.Lock()
	defer s.logs.mu.Unlock()
	out := []app.Rejection{}
	for k, n := range s.logs.rejections[worldID] {
		out = append(out, app.Rejection{Kind: k[0], Reason: k[1], Count: n})
	}
	slices.SortFunc(out, func(a, b app.Rejection) int {
		if c := strings.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(a.Reason, b.Reason)
	})
	return out, nil
}

func (s *Worlds) SavePackedLog(_ context.Context, p app.PackedLog) error {
	l := &s.logs
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.packed == nil {
		l.packed = map[int64]app.PackedLog{}
	}
	if _, ok := l.packed[p.WorldID]; !ok {
		p.Data = slices.Clone(p.Data)
		l.packed[p.WorldID] = p
	}
	l.rows[p.WorldID] = slices.DeleteFunc(l.rows[p.WorldID], func(e game.LoggedAction) bool { return e.Seq <= p.LastSeq })
	return nil
}

func (s *Worlds) PackedLog(_ context.Context, worldID int64) (*app.PackedLog, error) {
	s.logs.mu.Lock()
	defer s.logs.mu.Unlock()
	p, ok := s.logs.packed[worldID]
	if !ok {
		return nil, app.ErrNotFound
	}
	p.Data = slices.Clone(p.Data)
	return &p, nil
}
