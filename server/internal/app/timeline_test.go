package app_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

func unpack(t *testing.T, blob []byte) game.TimelineFile {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var f game.TimelineFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestTimelinePackedAtArchive(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id, _ := endedGame(t, f)
	if left, err := f.worlds.TimelineEntries(ctx, id); err != nil || len(left) != 0 {
		t.Fatalf("entries after the archive %d %v", len(left), err)
	}
	blob, err := f.svc.GameTimeline(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	file := unpack(t, blob)
	row, _ := f.worlds.Get(ctx, id)
	if row.State.EndedAt == nil || file.EndAt != *row.State.EndedAt || len(file.Owners) == 0 || len(file.Events) == 0 {
		t.Fatalf("file: end %d (world %v), %d owners, %d events", file.EndAt, row.State.EndedAt, len(file.Owners), len(file.Events))
	}
	var conquests []game.Conquest
	_ = f.worlds.Read(ctx, id, func(w app.WorldTx) error { conquests = w.ConquestsSince(0); return nil })
	for _, c := range conquests {
		found := false
		for _, e := range file.Events {
			if e[1] == 1 && int64(e[0]) == c.T && int(e[2]) == c.X && int(e[3]) == c.Y && e[4] > 0 &&
				file.Owners[int(e[4])-1].PlayerID == c.ToID {
				found = true
			}
		}
		if !found {
			t.Fatalf("conquest %+v has no owner change in the timeline", c)
		}
	}
}

func TestTimelineOfEndedGameBeforeArchive(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	blob, err := f.svc.GameTimeline(ctx, id)
	if err != nil || len(unpack(t, blob).Events) == 0 {
		t.Fatalf("timeline of an ended game %v", err)
	}
	if _, err := f.worlds.PackedTimeline(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("reading the timeline saved it: %v", err)
	}
}

func TestTimelineOfRunningGameNotFound(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	if _, err := f.svc.GameTimeline(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("running game: %v", err)
	}
	if _, err := f.svc.GameTimeline(ctx, 999); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("unknown game: %v", err)
	}
}

// unrecorded is a store whose worlds kept no timeline entries, like one started before recording existed.
type unrecorded struct{ *memory.Worlds }

func (unrecorded) TimelineEntries(context.Context, int64) ([]game.TimelineEntry, error) {
	return nil, nil
}

func TestTimelineOfUnrecordedGameNotFound(t *testing.T) {
	ctx := context.Background()
	f := &instantFixture{worlds: memory.NewWorlds(), clock: &fakeClock{}}
	f.clock.Set(accountsStart)
	f.svc = app.NewService(memory.NewAccounts(), unrecorded{f.worlds}, memory.NewCounters(f.clock), f.clock, plainHasher{}, security.NewTokens(), app.Config{})
	id := f.play(t, f.guest(t)).WorldID
	f.clock.Add(61 * time.Minute.Milliseconds())
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GameTimeline(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("before the archive: %v", err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GameTimeline(ctx, id); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("after the archive: %v", err)
	}
}
