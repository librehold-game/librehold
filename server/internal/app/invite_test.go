package app_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"librehold/internal/app"
	"librehold/internal/game"
)

// member is a registered account of its own for joining by invite.
func member(n int) *app.Account {
	return &app.Account{ID: int64(100 + n), Username: "player" + strconv.Itoa(n), PlayerKey: "key" + strconv.Itoa(n)}
}

// startHosted runs the world's clock from now, as the host's Start does.
func (f *instantFixture) startHosted(t *testing.T, id int64) {
	t.Helper()
	err := f.worlds.Run(context.Background(), id, func(w app.WorldTx) error {
		row, now := w.Row(), f.clock.NowMs()
		row.Host.StartedAtMs = &now
		row.PausedTotalMs += now - *row.PausedAtMs
		row.PausedAtMs = nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *instantFixture) changeHost(t *testing.T, id int64, fn func(w app.WorldTx)) {
	t.Helper()
	if err := f.worlds.Run(context.Background(), id, func(w app.WorldTx) error { fn(w); return nil }); err != nil {
		t.Fatal(err)
	}
}

func joinCode(t *testing.T, err error) string {
	t.Helper()
	var invalid *app.ErrValidation
	if !errors.As(err, &invalid) {
		t.Fatalf("not a refusal: %v", err)
	}
	return invalid.Code
}

func TestInviteJoinLandsInVillage(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	joiner := member(1)
	res, err := f.svc.JoinByInvite(ctx, joiner, card.InviteCode)
	if err != nil || res.WorldID != card.WorldID || res.Watch {
		t.Fatalf("join %+v %v", res, err)
	}
	st, err := f.svc.State(ctx, joiner, card.WorldID)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Player.VillageIDs) != 1 || st.Hosted == nil || !st.Hosted.Waiting || st.Hosted.Humans != 1 || st.Hosted.Seats != 10 ||
		st.Hosted.HostName != "ala" || st.Hosted.PausedByHost || st.Instant == nil {
		t.Fatalf("state hosted %+v instant %+v", st.Hosted, st.Instant)
	}
	if again, err := f.svc.JoinByInvite(ctx, joiner, card.InviteCode); err != nil || again.WorldID != card.WorldID {
		t.Fatalf("again %+v %v", again, err)
	}
	if hosted, _ := f.svc.Hosted(ctx, ala); hosted.Humans != 1 || hosted.Players[0].Name != "player1" {
		t.Fatalf("host sees %+v", hosted.Players)
	}
	lobby, err := f.svc.Lobby(ctx, joiner)
	if err != nil || len(lobby.Playing) != 1 || lobby.Playing[0].ID != card.WorldID || len(lobby.Mine) != 0 {
		t.Fatalf("lobby %+v %v", lobby, err)
	}
}

func TestEleventhJoinerIsToldTheWorldIsFull(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	for i := range 10 {
		if _, err := f.svc.JoinByInvite(ctx, member(i), card.InviteCode); err != nil {
			t.Fatal(err)
		}
	}
	if code := joinCode(t, func() error { _, err := f.svc.JoinByInvite(ctx, member(10), card.InviteCode); return err }()); code != "worldFull" {
		t.Fatalf("eleventh: %s", code)
	}
	info, err := f.svc.InviteInfo(ctx, card.InviteCode)
	if err != nil || info.Open || info.Reason != "worldFull" || info.Humans != 10 || info.Seats != 10 || info.Status != "waiting" ||
		info.Name != "Ours" || info.Host != "ala" {
		t.Fatalf("info %+v %v", info, err)
	}
}

func TestLateJoining(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, app.HostForm{Name: "Ours", LateJoinMin: ptr(10)})
	f.startHosted(t, card.WorldID)
	f.clock.Add(9 * game.MsPerMinute)
	if _, err := f.svc.JoinByInvite(ctx, member(1), card.InviteCode); err != nil {
		t.Fatalf("9 min after Start: %v", err)
	}
	f.changeHost(t, card.WorldID, func(w app.WorldTx) { now := f.clock.NowMs(); w.Row().PausedAtMs = &now })
	f.clock.Add(5 * game.MsPerMinute)
	if _, err := f.svc.JoinByInvite(ctx, member(2), card.InviteCode); err != nil {
		t.Fatalf("during a host pause inside the window: %v", err)
	}
	f.changeHost(t, card.WorldID, func(w app.WorldTx) {
		row := w.Row()
		row.PausedTotalMs += f.clock.NowMs() - *row.PausedAtMs
		row.PausedAtMs = nil
	})
	f.clock.Add(game.MsPerMinute)
	if code := joinCode(t, func() error { _, err := f.svc.JoinByInvite(ctx, member(3), card.InviteCode); return err }()); code != "joinClosed" {
		t.Fatalf("10 min after Start: %s", code)
	}
	if info, _ := f.svc.InviteInfo(ctx, card.InviteCode); info.Open || info.Reason != "joinClosed" || info.Status != "running" {
		t.Fatalf("info %+v", info)
	}

	closed := f.host(t, ola, app.HostForm{Name: "Closed", LateJoinMin: ptr(0)})
	if _, err := f.svc.JoinByInvite(ctx, member(4), closed.InviteCode); err != nil {
		t.Fatalf("before Start: %v", err)
	}
	f.startHosted(t, closed.WorldID)
	if code := joinCode(t, func() error { _, err := f.svc.JoinByInvite(ctx, member(5), closed.InviteCode); return err }()); code != "joinClosed" {
		t.Fatalf("right after Start: %s", code)
	}
}

func TestRegisteredOnlyRefusesGuests(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, app.HostForm{Name: "Ours", RegisteredOnly: true})
	guest := &app.Account{ID: 300, Username: "Guest_300", IsGuest: true, PlayerKey: "g"}
	if code := joinCode(t, func() error { _, err := f.svc.JoinByInvite(ctx, guest, card.InviteCode); return err }()); code != "registeredOnly" {
		t.Fatalf("guest: %s", code)
	}
	if info, _ := f.svc.InviteInfo(ctx, card.InviteCode); !info.RegisteredOnly || !info.Open {
		t.Fatalf("info %+v", info)
	}
	if _, err := f.svc.JoinByInvite(ctx, member(1), card.InviteCode); err != nil {
		t.Fatal(err)
	}
	open := f.host(t, ola, hostForm("Open"))
	if _, err := f.svc.JoinByInvite(ctx, guest, open.InviteCode); err != nil {
		t.Fatalf("a guest on an open world: %v", err)
	}
}

func TestBannedAccountCannotRejoin(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	kicked := member(1)
	if _, err := f.svc.JoinByInvite(ctx, kicked, card.InviteCode); err != nil {
		t.Fatal(err)
	}
	f.changeHost(t, card.WorldID, func(w app.WorldTx) {
		id := game.PlayerID("h" + strconv.FormatInt(kicked.ID, 10))
		game.KickPlayer(w, id)
		w.Player(id).AccountID = 0
		w.Row().Host.Banned = append(w.Row().Host.Banned, kicked.ID)
	})
	if code := joinCode(t, func() error { _, err := f.svc.JoinByInvite(ctx, kicked, card.InviteCode); return err }()); code != "banned" {
		t.Fatalf("rejoin: %s", code)
	}
}

func TestHostIsSentToWatch(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	res, err := f.svc.JoinByInvite(ctx, ala, card.InviteCode)
	if err != nil || !res.Watch || res.WorldID != card.WorldID {
		t.Fatalf("host %+v %v", res, err)
	}
	if hosted, _ := f.svc.Hosted(ctx, ala); hosted.Humans != 0 {
		t.Fatalf("the host became a player: %+v", hosted.Players)
	}
	if _, err := f.svc.State(ctx, ala, card.WorldID); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("the host is a member: %v", err)
	}
}

func TestReplacedInviteFindsNothing(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, ala, hostForm("Ours"))
	inside := member(1)
	if _, err := f.svc.JoinByInvite(ctx, inside, card.InviteCode); err != nil {
		t.Fatal(err)
	}
	f.changeHost(t, card.WorldID, func(w app.WorldTx) { w.Row().Host.InviteCode = "replacedreplacedrepl" })
	for _, code := range []string{card.InviteCode, "guessedguessedguesse", ""} {
		if _, err := f.svc.InviteInfo(ctx, code); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("info %q: %v", code, err)
		}
		if _, err := f.svc.JoinByInvite(ctx, member(2), code); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("join %q: %v", code, err)
		}
	}
	if _, err := f.svc.JoinByInvite(ctx, member(2), "replacedreplacedrepl"); err != nil {
		t.Fatalf("new link: %v", err)
	}
	if _, err := f.svc.State(ctx, inside, card.WorldID); err != nil {
		t.Fatalf("a player inside lost the world: %v", err)
	}
}

func TestPlayerWorldNotJoinableById(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	card := f.host(t, ala, hostForm("Ours"))
	if _, err := f.svc.JoinWorld(context.Background(), member(1), card.WorldID, game.QuadrantRandom); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("join by id: %v", err)
	}
}
