package app_test

import (
	"context"
	"errors"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

var ala = &app.Account{ID: 1, Username: "ala"}

func (f *adminFixture) namedWorld(t *testing.T, name string) int64 {
	t.Helper()
	id, err := f.svc.CreateWorld(context.Background(), admin, name, gametest.TestConfig())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func cardIDs(cards []app.WorldCard) []int64 {
	var out []int64
	for _, c := range cards {
		out = append(out, c.ID)
	}
	return out
}

func TestLobbyListsMineAndOpen(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	a, b, c := f.namedWorld(t, "A"), f.namedWorld(t, "B"), f.namedWorld(t, "C")
	if err := f.svc.SetWorldStatus(ctx, admin, c, "closed"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PauseWorld(ctx, admin, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.JoinWorld(ctx, ala, a, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	lobby, err := f.svc.Lobby(ctx, ala)
	if err != nil {
		t.Fatal(err)
	}
	if got := cardIDs(lobby.Mine); len(got) != 1 || got[0] != a {
		t.Fatalf("mine %v", got)
	}
	if got := cardIDs(lobby.Open); len(got) != 1 || got[0] != b {
		t.Fatalf("open %v", got)
	}
	if mine := lobby.Mine[0]; mine.Name != "A" || mine.Humans != 1 || mine.Paused || mine.Settings.WorldSpeed != 1 {
		t.Fatalf("card %+v", mine)
	}
	if !lobby.Open[0].Paused {
		t.Fatal("a paused world is not marked")
	}
}

func TestJoinClosedWorldRefused(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.namedWorld(t, "A")
	if err := f.svc.SetWorldStatus(ctx, admin, id, "closed"); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom)
	wantValidation(t, err, "world", "worldClosed")
	if _, err := f.svc.JoinWorld(ctx, ala, id+50, game.QuadrantRandom); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("missing world: %v", err)
	}
}

func TestJoinTwiceRefused(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.namedWorld(t, "A")
	me, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantNE)
	if err != nil || me != "h1" {
		t.Fatalf("join: %q %v", me, err)
	}
	_, err = f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom)
	wantValidation(t, err, "world", "alreadyMember")
	err = f.worlds.Read(ctx, id, func(w app.WorldTx) error {
		if p := w.Player("h1"); p == nil || p.Name != "ala" || len(p.VillageIDs) != 1 {
			t.Fatalf("player %+v", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLeaveThenRejoin(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.namedWorld(t, "A")
	if err := f.svc.LeaveWorld(ctx, ala, id); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("leave without joining: %v", err)
	}
	if _, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LeaveWorld(ctx, ala, id); err != nil {
		t.Fatal(err)
	}
	lobby, _ := f.svc.Lobby(ctx, ala)
	if len(lobby.Mine) != 0 || len(lobby.Open) != 1 {
		t.Fatalf("after leaving: %+v", lobby)
	}
	me, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom)
	if err != nil || me != "h1_2" {
		t.Fatalf("rejoin: %q %v", me, err)
	}
	err = f.worlds.Read(ctx, id, func(w app.WorldTx) error {
		old, fresh := w.Player("h1"), w.Player("h1_2")
		if old.Status != game.StatusQuit || old.AccountID != 0 {
			t.Errorf("old player %+v", old)
		}
		if fresh.Status != game.StatusActive || fresh.AccountID != 1 || len(fresh.VillageIDs) != 1 || fresh.VillageIDs[0] == old.VillageIDs[0] {
			t.Errorf("fresh player %+v", fresh)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.LeaveWorld(ctx, ala, id); err != nil {
		t.Fatal(err)
	}
	if me, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom); err != nil || me != "h1_3" {
		t.Fatalf("third join: %q %v", me, err)
	}
}

func TestRejoinsAreCapped(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.namedWorld(t, "A")
	for i := range 3 {
		if _, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom); err != nil {
			t.Fatalf("join %d: %v", i+1, err)
		}
		if err := f.svc.LeaveWorld(ctx, ala, id); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom)
	wantValidation(t, err, "world", "rejoinLimit")
	if _, err := f.svc.JoinWorld(ctx, &app.Account{ID: 2, Username: "bob"}, id, game.QuadrantRandom); err != nil {
		t.Fatalf("another account: %v", err)
	}
}
