package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

// hoster has an id no guest of the fixture gets.
var hoster = &app.Account{ID: 500, Username: "gamehost", PlayerKey: "khost"}

func refusal(t *testing.T, err error, code string) {
	t.Helper()
	var invalid *app.ErrValidation
	if !errors.As(err, &invalid) || invalid.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func (f *instantFixture) row(t *testing.T, id int64) *app.WorldRow {
	t.Helper()
	row, err := f.worlds.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func (f *instantFixture) worldNow(t *testing.T, id int64) int64 {
	t.Helper()
	return app.WorldNow(f.row(t, id), f.clock.NowMs())
}

// hostedWithPlayers is a waiting world of hoster's with n members joined.
func (f *instantFixture) hostedWithPlayers(t *testing.T, form app.HostForm, n int) *app.HostedCard {
	t.Helper()
	card := f.host(t, hoster, form)
	for i := range n {
		if _, err := f.svc.JoinByInvite(context.Background(), member(i+1), card.InviteCode); err != nil {
			t.Fatal(err)
		}
	}
	return card
}

func TestOnlyTheHostControls(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.hostedWithPlayers(t, hostForm("Ours"), 1)
	instant := f.play(t, f.guest(t)).WorldID
	for _, wid := range []int64{card.WorldID, instant} {
		acct := ola
		if wid == instant {
			acct = hoster
		}
		_, inviteErr := f.svc.ReplaceInvite(ctx, acct, wid)
		for name, err := range map[string]error{
			"start": f.svc.HostStart(ctx, acct, wid), "pause": f.svc.HostPause(ctx, acct, wid), "resume": f.svc.HostResume(ctx, acct, wid),
			"end": f.svc.HostEnd(ctx, acct, wid), "kick": f.svc.HostKick(ctx, acct, wid, "h101"), "invite": inviteErr,
			"joinRule": f.svc.SetRegisteredOnly(ctx, acct, wid, true),
		} {
			if !errors.Is(err, app.ErrForbidden) {
				t.Fatalf("%s on world %d by %s: %v", name, wid, acct.Username, err)
			}
		}
	}
	if f.row(t, card.WorldID).Host.StartedAtMs != nil {
		t.Fatal("someone else started it")
	}
}

func TestStartRunsTheClock(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.hostedWithPlayers(t, app.HostForm{Name: "Ours", DurationMin: 30}, 1)
	f.clock.Add(7 * game.MsPerMinute)
	if err := f.svc.HostStart(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	refusal(t, f.svc.HostStart(ctx, hoster, card.WorldID), "alreadyStarted")
	if now := f.worldNow(t, card.WorldID); now != 0 {
		t.Fatalf("world time at Start %d", now)
	}
	f.clock.Add(5 * game.MsPerMinute)
	if now := f.worldNow(t, card.WorldID); now != 5*game.MsPerMinute {
		t.Fatalf("five minutes later %d", now)
	}
	if hosted, _ := f.svc.Hosted(ctx, hoster); hosted.Status != "running" {
		t.Fatalf("status %s", hosted.Status)
	}
	f.clock.Add(25 * game.MsPerMinute)
	if _, _, err := f.svc.ProcessDue(ctx, card.WorldID, 0); err != nil {
		t.Fatal(err)
	}
	if s := f.row(t, card.WorldID).State; s.EndedAt == nil || *s.EndedAt != 30*game.MsPerMinute || s.EndReason != "time" {
		t.Fatalf("state %+v", s)
	}
	if starts := entriesOf(actionLog(t, f, card.WorldID), "start"); len(starts) != 1 || starts[0].Actor != "host" || starts[0].At != 0 {
		t.Fatalf("start entries %+v", starts)
	}
}

func TestStartNeedsAPlayerAndEndWhileWaitingDeletes(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.host(t, hoster, hostForm("Empty"))
	refusal(t, f.svc.HostStart(ctx, hoster, card.WorldID), "noPlayers")
	if err := f.svc.HostEnd(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.worlds.Get(ctx, card.WorldID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("the waiting world is still there: %v", err)
	}
	if _, err := f.svc.InviteInfo(ctx, card.InviteCode); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("its invite still works: %v", err)
	}
	if _, err := f.worlds.PackedLog(ctx, card.WorldID); err != nil {
		t.Fatalf("its log was not kept: %v", err)
	}
	if hosted, _ := f.svc.Hosted(ctx, hoster); hosted != nil {
		t.Fatalf("still hosting %+v", hosted)
	}
	f.host(t, hoster, hostForm("Again"))
}

func TestPauseCapsAndAutoResume(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.hostedWithPlayers(t, hostForm("Ours"), 1)
	id := card.WorldID
	refusal(t, f.svc.HostPause(ctx, hoster, id), "notStarted")
	if err := f.svc.HostStart(ctx, hoster, id); err != nil {
		t.Fatal(err)
	}
	refusal(t, f.svc.HostResume(ctx, hoster, id), "notPaused")
	f.clock.Add(3 * game.MsPerMinute)
	if err := f.svc.HostPause(ctx, hoster, id); err != nil {
		t.Fatal(err)
	}
	refusal(t, f.svc.HostPause(ctx, hoster, id), "alreadyPaused")
	if hosted, _ := f.svc.Hosted(ctx, hoster); hosted.Status != "paused" || hosted.PauseEndsAtMs != f.clock.NowMs()+10*game.MsPerMinute {
		t.Fatalf("hosted %+v", hosted)
	}
	f.clock.Add(12 * game.MsPerMinute)
	if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
		t.Fatal(err)
	}
	row := f.row(t, id)
	if row.PausedAtMs != nil || row.Host.HostPausedMs != 10*game.MsPerMinute || f.worldNow(t, id) != 5*game.MsPerMinute {
		t.Fatalf("after the sweep: paused %v, used %d, world time %d", row.PausedAtMs, row.Host.HostPausedMs, f.worldNow(t, id))
	}
	resumes := entriesOf(actionLog(t, f, id), "resume")
	if len(resumes) == 0 || resumes[len(resumes)-1].Actor != "world" || resumes[len(resumes)-1].At != 3*game.MsPerMinute {
		t.Fatalf("resume entries %+v", resumes)
	}
	for range 2 {
		if err := f.svc.HostPause(ctx, hoster, id); err != nil {
			t.Fatal(err)
		}
		f.clock.Add(15 * game.MsPerMinute)
		if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if used := f.row(t, id).Host.HostPausedMs; used != 30*game.MsPerMinute {
		t.Fatalf("budget used %d", used)
	}
	refusal(t, f.svc.HostPause(ctx, hoster, id), "pauseLimit")

	other := f.host(t, ola, hostForm("Theirs"))
	if _, err := f.svc.JoinByInvite(ctx, member(9), other.InviteCode); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HostStart(ctx, ola, other.WorldID); err != nil {
		t.Fatal(err)
	}
	f.changeHost(t, other.WorldID, func(w app.WorldTx) { w.Row().Host.HostPausedMs = 25 * game.MsPerMinute })
	before := f.worldNow(t, other.WorldID)
	if err := f.svc.HostPause(ctx, ola, other.WorldID); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(8 * game.MsPerMinute)
	if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
		t.Fatal(err)
	}
	if row := f.row(t, other.WorldID); row.PausedAtMs != nil || row.Host.HostPausedMs != 30*game.MsPerMinute ||
		f.worldNow(t, other.WorldID) != before+3*game.MsPerMinute {
		t.Fatalf("the last five minutes: paused %v, used %d, world time %d → %d", row.PausedAtMs, row.Host.HostPausedMs, before,
			f.worldNow(t, other.WorldID))
	}
}

func TestKickBansAndFreesTheSeat(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.hostedWithPlayers(t, hostForm("Ours"), 2)
	if err := f.svc.HostStart(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	refusal(t, f.svc.HostKick(ctx, hoster, card.WorldID, "bot_0"), "notAPlayer")
	if err := f.svc.HostKick(ctx, hoster, card.WorldID, "h101"); err != nil {
		t.Fatal(err)
	}
	refusal(t, f.svc.HostKick(ctx, hoster, card.WorldID, "h101"), "notAPlayer")
	err := f.worlds.Read(ctx, card.WorldID, func(w app.WorldTx) error {
		if p := w.Player("h101"); p.Status != game.StatusQuit || len(p.VillageIDs) != 0 || p.AccountID != 0 {
			t.Fatalf("kicked player %+v", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.State(ctx, member(1), card.WorldID); !errors.Is(err, app.ErrNotMember) {
		t.Fatalf("the kicked account is still a member: %v", err)
	}
	_, err = f.svc.JoinByInvite(ctx, member(1), card.InviteCode)
	refusal(t, err, "banned")
	if hosted, _ := f.svc.Hosted(ctx, hoster); hosted.Humans != 1 || f.row(t, card.WorldID).Host.Banned[0] != member(1).ID {
		t.Fatalf("hosted %+v", hosted)
	}
	if kicks := entriesOf(actionLog(t, f, card.WorldID), "kick"); len(kicks) != 1 || kicks[0].Actor != "host" {
		t.Fatalf("kick entries %+v", kicks)
	}
}

func TestAbandonedWaitingWorldDeletedAfterTwoHours(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	waiting := f.host(t, hoster, hostForm("Forgotten"))
	started := f.host(t, ola, hostForm("Played"))
	if _, err := f.svc.JoinByInvite(ctx, member(1), started.InviteCode); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HostStart(ctx, ola, started.WorldID); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(2*time.Hour.Milliseconds() - game.MsPerMinute)
	if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.worlds.Get(ctx, waiting.WorldID); err != nil {
		t.Fatalf("deleted before two hours: %v", err)
	}
	f.clock.Add(game.MsPerMinute)
	if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.worlds.Get(ctx, waiting.WorldID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("kept after two hours: %v", err)
	}
	if _, err := f.svc.InviteInfo(ctx, waiting.InviteCode); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("its invite: %v", err)
	}
	f.clock.Add(time.Hour.Milliseconds())
	if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.worlds.Get(ctx, started.WorldID); err != nil {
		t.Fatalf("a started world was swept: %v", err)
	}
}

func TestEndNowShowsResults(t *testing.T) {
	f := newInstantFixture(t, app.Config{})
	ctx := context.Background()
	card := f.hostedWithPlayers(t, hostForm("Ours"), 1)
	if err := f.svc.HostStart(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(2 * game.MsPerMinute)
	if err := f.svc.HostPause(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	f.clock.Add(game.MsPerMinute)
	if err := f.svc.HostEnd(ctx, hoster, card.WorldID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HostEnd(ctx, hoster, card.WorldID); !errors.Is(err, app.ErrWorldEnded) {
		t.Fatalf("a second end: %v", err)
	}
	st, err := f.svc.State(ctx, member(1), card.WorldID)
	if err != nil || st.Instant == nil || st.Instant.Ended == nil || st.Instant.Ended.Reason != "host" || st.Paused ||
		st.Instant.Ended.At != 2*game.MsPerMinute {
		t.Fatalf("state %+v %v", st.Instant, err)
	}
	if err := f.svc.ArchiveEnded(ctx); err != nil {
		t.Fatal(err)
	}
	g, err := f.worlds.ArchivedGame(ctx, card.WorldID)
	if err != nil || g.Kind != game.KindPlayer || g.ResultsCode != f.row(t, card.WorldID).Host.ResultsCode || g.HostName != "gamehost" ||
		g.Summary.EndReason != "host" {
		t.Fatalf("archived %+v %v", g, err)
	}
	if hosted, _ := f.svc.Hosted(ctx, hoster); hosted.Status != "ended" || hosted.ResultsCode == "" {
		t.Fatalf("hosted %+v", hosted)
	}
}

func TestReplayReproducesHostedGame(t *testing.T) {
	ctx := context.Background()
	f := newInstantFixture(t, app.Config{})
	card := f.host(t, hoster, app.HostForm{Name: "Replayed", Bots: ptr(2)})
	id := card.WorldID
	a, b := f.guest(t), f.guest(t)
	for _, g := range []*app.Account{a, b} {
		if _, err := f.svc.JoinByInvite(ctx, g, card.InviteCode); err != nil {
			t.Fatal(err)
		}
	}
	f.clock.Add(2 * game.MsPerMinute)
	if err := f.svc.HostStart(ctx, hoster, id); err != nil {
		t.Fatal(err)
	}
	order := func(acct *app.Account, name, body string) {
		_, _ = f.svc.Order(ctx, acct, id, name, json.RawMessage(body))
	}
	buildings := []string{"wood", "clay", "iron", "main", "barracks"}
	kicked := false
	for minute := range 15 {
		f.clock.Add(time.Minute.Milliseconds())
		players := []*app.Account{a}
		if !kicked {
			players = append(players, b)
		}
		for i, acct := range players {
			home, barb := homeOf(t, f, id, acct)
			order(acct, "build", fmt.Sprintf(`{"villageId":%q,"building":%q}`, home.ID, buildings[(minute+i)%len(buildings)]))
			order(acct, "recruit", fmt.Sprintf(`{"villageId":%q,"unit":"axe","count":3}`, home.ID))
			switch {
			case barb == nil:
			case minute == 2:
				order(acct, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"axe":60,"lightCav":20},"type":"attack"}`,
					home.ID, barb.X, barb.Y))
			case minute%4 == 1:
				order(acct, "send", fmt.Sprintf(`{"originId":%q,"x":%d,"y":%d,"units":{"scout":1},"type":"attack"}`,
					home.ID, barb.X, barb.Y))
			}
		}
		switch minute {
		case 5:
			if err := f.svc.HostPause(ctx, hoster, id); err != nil {
				t.Fatal(err)
			}
			f.clock.Add(12 * game.MsPerMinute)
			if err := f.svc.SweepPlayerWorlds(ctx); err != nil {
				t.Fatal(err)
			}
		case 10:
			var bid game.PlayerID
			_ = f.worlds.Read(ctx, id, func(w app.WorldTx) error {
				for _, p := range w.Players() {
					if p.AccountID == b.ID {
						bid = p.ID
					}
				}
				return nil
			})
			if err := f.svc.HostKick(ctx, hoster, id, bid); err != nil {
				t.Fatal(err)
			}
			kicked = true
		}
	}
	if err := f.svc.HostEnd(ctx, hoster, id); err != nil {
		t.Fatal(err)
	}
	row := f.row(t, id)
	log := actionLog(t, f, id)
	for _, kind := range []string{"start", "pause", "kick", "end"} {
		if es := entriesOf(log, kind); len(es) != 1 || es[0].Actor != "host" {
			t.Fatalf("%s entries %+v", kind, es)
		}
	}
	if rs := entriesOf(log, "resume"); len(rs) != 1 || rs[0].Actor != "world" {
		t.Fatalf("resume entries %+v", rs)
	}

	replayWorlds := memory.NewWorlds()
	svc := app.NewService(memory.NewAccounts(), replayWorlds, memory.NewCounters(&fakeClock{}), &fakeClock{}, plainHasher{}, security.NewTokens(), app.Config{})
	rid, err := app.Replay(ctx, svc, log, row.Seed, row.Secret)
	if err != nil {
		t.Fatal(err)
	}
	want := worldDump(t, f.worlds, id)
	for _, acct := range []*app.Account{a, b} {
		want = strings.ReplaceAll(want, `"`+acct.Username+`"`, `"`+acct.PlayerKey+`"`)
	}
	got := worldDump(t, replayWorlds, rid)
	if got != want {
		gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("replay differs at line %d:\n got %s\nwant %s", i, gl[i], wl[i])
			}
		}
		t.Fatalf("replay differs in length: %d vs %d lines", len(gl), len(wl))
	}
	ended, _ := replayWorlds.Get(ctx, rid)
	if ended.State.EndReason != "host" || *ended.State.EndedAt != *row.State.EndedAt {
		t.Fatalf("replay ended %+v, the game %+v", ended.State, row.State)
	}
}
