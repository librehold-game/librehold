package memory

import (
	"context"
	"slices"
	"sync"

	"librehold/internal/app"
	"librehold/internal/game"
)

// timelines live beside the worlds; the packed ones outlive them, like the archive.
type timelines struct {
	mu      sync.Mutex
	entries map[int64][]game.TimelineEntry
	packed  map[int64][]byte
}

func (s *Worlds) appendTimeline(worldID int64, entries []game.TimelineEntry) {
	if len(entries) == 0 {
		return
	}
	l := &s.timelines
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[int64][]game.TimelineEntry{}
	}
	l.entries[worldID] = append(l.entries[worldID], entries...)
}

func (s *Worlds) dropTimeline(worldID int64) {
	s.timelines.mu.Lock()
	defer s.timelines.mu.Unlock()
	delete(s.timelines.entries, worldID)
}

func (s *Worlds) TimelineEntries(_ context.Context, worldID int64) ([]game.TimelineEntry, error) {
	s.timelines.mu.Lock()
	defer s.timelines.mu.Unlock()
	return slices.Clone(s.timelines.entries[worldID]), nil
}

func (s *Worlds) SavePackedTimeline(_ context.Context, worldID int64, data []byte, _ int) error {
	l := &s.timelines
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.packed == nil {
		l.packed = map[int64][]byte{}
	}
	if _, ok := l.packed[worldID]; !ok {
		l.packed[worldID] = slices.Clone(data)
	}
	delete(l.entries, worldID)
	return nil
}

func (s *Worlds) PackedTimeline(_ context.Context, worldID int64) ([]byte, error) {
	s.timelines.mu.Lock()
	defer s.timelines.mu.Unlock()
	data, ok := s.timelines.packed[worldID]
	if !ok {
		return nil, app.ErrNotFound
	}
	return slices.Clone(data), nil
}
