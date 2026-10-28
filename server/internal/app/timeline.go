package app

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"

	"librehold/internal/game"
)

// TimelineStore keeps a short world's timeline while it runs and, packed, after it is gone.
type TimelineStore interface {
	TimelineEntries(ctx context.Context, worldID int64) ([]game.TimelineEntry, error) // write order
	// SavePackedTimeline keeps the first blob saved and deletes the world's entries at once.
	SavePackedTimeline(ctx context.Context, worldID int64, data []byte, entries int) error
	PackedTimeline(ctx context.Context, worldID int64) ([]byte, error) // ErrNotFound when none
}

// timelineBlob packs a world's timeline as gzip JSON; ErrNotFound unless the world keeps one, has ended and recorded
// something (a world started before recording existed has nothing to replay).
func (s *Service) timelineBlob(ctx context.Context, worldID int64) ([]byte, int, error) {
	var file game.TimelineFile
	err := s.worlds.Read(ctx, worldID, func(w WorldTx) error {
		cfg := w.Config()
		if !game.RecordsTimeline(cfg) || w.State().EndedAt == nil {
			return ErrNotFound
		}
		entries, err := s.worlds.TimelineEntries(ctx, worldID)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			return ErrNotFound
		}
		file = game.PackTimeline(w.Seed(), cfg, *w.State().EndedAt, w.Players(), entries)
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(file); err != nil {
		return nil, 0, err
	}
	if err := zw.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), len(file.Events), nil
}

// packTimeline keeps an ended world's packed timeline for good and drops its entries; a second call keeps the first,
// and a world with nothing recorded keeps nothing.
func (s *Service) packTimeline(ctx context.Context, worldID int64) error {
	if _, err := s.worlds.PackedTimeline(ctx, worldID); err == nil {
		return nil
	}
	blob, n, err := s.timelineBlob(ctx, worldID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.worlds.SavePackedTimeline(ctx, worldID, blob, n)
}

// GameTimeline is a finished game's timeline as gzip JSON: the packed one, or packed on the fly for an ended world
// that is not archived yet. Running and unknown games are not found.
func (s *Service) GameTimeline(ctx context.Context, worldID int64) ([]byte, error) {
	blob, err := s.worlds.PackedTimeline(ctx, worldID)
	if !errors.Is(err, ErrNotFound) {
		return blob, err
	}
	blob, _, err = s.timelineBlob(ctx, worldID)
	return blob, err
}
