package app_test

import (
	"bytes"
	"context"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game/gametest"
)

func TestEveryNewWorldHasItsOwnSecret(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	long, err := f.svc.CreateWorld(ctx, admin, "Long", gametest.TestConfig())
	if err != nil {
		t.Fatal(err)
	}
	hosted, err := f.svc.CreatePlayerWorld(ctx, ala, hostForm("Hosted"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.EnsureReadyInstant(ctx); err != nil {
		t.Fatal(err)
	}
	ids := []int64{long, hosted.WorldID}
	worlds, err := f.worlds.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range worlds {
		if w.ID != long && w.ID != hosted.WorldID {
			ids = append(ids, w.ID)
		}
	}
	if len(ids) != 3 {
		t.Fatalf("worlds %v, want a long, a hosted and an instant one", ids)
	}
	var seen [][]byte
	for _, id := range ids {
		row, err := f.worlds.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(row.Secret) != 32 {
			t.Fatalf("world %d has a %d-byte secret", id, len(row.Secret))
		}
		for _, s := range seen {
			if bytes.Equal(s, row.Secret) {
				t.Fatalf("world %d shares a secret", id)
			}
		}
		seen = append(seen, row.Secret)
	}
}
