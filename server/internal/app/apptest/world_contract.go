package apptest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

var t0ms = t0.UnixMilli()

// NewWorldRow is a small open test world: TestConfig with the given number of bots, started at t0.
func NewWorldRow(bots int) app.WorldRow {
	cfg := gametest.TestConfig()
	cfg.BotCount = bots
	return app.WorldRow{Name: "Café Test World", Status: "open", Seed: 7, Secret: testSecret, Config: cfg, StartedAtMs: t0ms}
}

var testSecret = bytes.Repeat([]byte{0x5e}, 32)

func createWorld(t *testing.T, s app.WorldStore, bots int) int64 {
	t.Helper()
	id, err := s.Create(context.Background(), NewWorldRow(bots), func(w app.WorldTx) error { return game.NewWorld(w) })
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("world id not set")
	}
	return id
}

func run(t *testing.T, s app.WorldStore, id int64, fn func(w app.WorldTx) error) {
	t.Helper()
	if err := s.Run(context.Background(), id, fn); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, s app.WorldStore, id int64, fn func(w app.WorldTx)) {
	t.Helper()
	if err := s.Read(context.Background(), id, func(w app.WorldTx) error { fn(w); return nil }); err != nil {
		t.Fatal(err)
	}
}

func firstVillage(t *testing.T, s app.WorldStore, id int64) string {
	t.Helper()
	var vid string
	read(t, s, id, func(w app.WorldTx) { vid = w.Players()[0].VillageIDs[0] })
	return vid
}

func villageName(t *testing.T, s app.WorldStore, id int64, vid string) string {
	t.Helper()
	var name string
	read(t, s, id, func(w app.WorldTx) { name = w.Village(vid).Name })
	return name
}

// WorldBackend is a WorldStore with the feeds it notifies.
type WorldBackend struct {
	Store   app.WorldStore
	Updates app.Updates
	Wakeups app.EventWakeups
}

// RunWorldStoreContract checks a WorldStore and its feeds; newBackend returns an empty one.
func RunWorldStoreContract(t *testing.T, newBackend func(t *testing.T) WorldBackend) {
	ctx := context.Background()
	newStore := func(t *testing.T) app.WorldStore { return newBackend(t).Store }

	t.Run("TestCreateAndReadBack", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 10)
		read(t, s, id, func(w app.WorldTx) {
			players := w.Players()
			if len(players) != 10 {
				t.Fatalf("%d players", len(players))
			}
			for _, p := range players {
				if w.Brain(p.ID) == nil || len(p.VillageIDs) != 1 {
					t.Fatalf("player %+v", p)
				}
				v := w.Village(p.VillageIDs[0])
				if v == nil || v.OwnerID != game.Ref(p.ID) {
					t.Fatalf("village of %s: %+v", p.ID, v)
				}
			}
			if w.Seed() != 7 || !bytes.Equal(w.Secret(), testSecret) || w.Config().BotCount != 10 || w.Row().Name != "Café Test World" {
				t.Fatalf("row %+v", w.Row())
			}
		})
		row, err := s.Get(ctx, id)
		if err != nil || row.ID != id || row.StartedAtMs != t0ms || row.Config.BotCount != 10 || !bytes.Equal(row.Secret, testSecret) {
			t.Fatalf("get %+v %v", row, err)
		}
		list, err := s.List(ctx)
		if err != nil || len(list) != 1 || list[0].ID != id || list[0].Bots != 10 || list[0].Humans != 0 {
			t.Fatalf("list %+v %v", list, err)
		}
		if _, ok, err := s.NextDue(ctx, id); err != nil || !ok {
			t.Fatalf("next due %v %v", ok, err)
		}
		if _, err := s.Get(ctx, id+100); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing world: %v", err)
		}
	})

	t.Run("TestMarkPresentKeepsTheLatestTime", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 1)
		if err := s.MarkPresent(ctx, id, 5000); err != nil {
			t.Fatal(err)
		}
		if err := s.MarkPresent(ctx, id, 3000); err != nil {
			t.Fatal(err)
		}
		row, _ := s.Get(ctx, id)
		list, _ := s.List(ctx)
		if row.LastPresentMs != 5000 || list[0].LastPresentMs != 5000 {
			t.Fatalf("get %d, list %d", row.LastPresentMs, list[0].LastPresentMs)
		}
		if err := s.Run(ctx, id, func(w app.WorldTx) error { w.Row().Name = "Renamed"; return nil }); err != nil {
			t.Fatal(err)
		}
		if row, _ = s.Get(ctx, id); row.LastPresentMs != 5000 || row.Name != "Renamed" {
			t.Fatalf("a run changed presence: %+v", row)
		}
	})

	t.Run("TestLockAccountRunsOneAtATime", func(t *testing.T) {
		s := newStore(t)
		var inside, overlaps atomic.Int32
		var wg sync.WaitGroup
		for range 6 {
			wg.Go(func() {
				err := s.LockAccount(ctx, 7, func() error {
					if inside.Add(1) > 1 {
						overlaps.Add(1)
					}
					time.Sleep(5 * time.Millisecond)
					inside.Add(-1)
					return nil
				})
				if err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		if overlaps.Load() != 0 {
			t.Fatalf("%d overlapping runs", overlaps.Load())
		}
	})

	t.Run("TestEndedWorldPersists", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		at := int64(3_600_000)
		run(t, s, id, func(w app.WorldTx) error {
			st := w.State()
			st.EndedAt, st.EndReason, st.WinnerID = &at, "time", "bot_1"
			st.Totals.AttacksSent = 5
			return nil
		})
		row, err := s.Get(ctx, id)
		if err != nil || row.State.EndedAt == nil || *row.State.EndedAt != at || row.State.WinnerID != "bot_1" || row.State.Totals.AttacksSent != 5 {
			t.Fatalf("row %+v %v", row, err)
		}
		read(t, s, id, func(w app.WorldTx) {
			if !game.Ended(w) || w.State().EndReason != "time" {
				t.Fatalf("state %+v", w.State())
			}
		})
	})

	t.Run("TestDeleteRemovesTheWorld", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		other := createWorld(t, s, 1)
		run(t, s, id, func(w app.WorldTx) error {
			_, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
			return err
		})
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("deleted world: %v", err)
		}
		if ms, _ := s.MembershipsOf(ctx, 1); len(ms) != 0 {
			t.Fatalf("memberships %+v", ms)
		}
		if _, err := s.Get(ctx, other); err != nil {
			t.Fatalf("the other world: %v", err)
		}
		if err := s.Delete(ctx, id); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("deleting twice: %v", err)
		}
	})

	t.Run("TestListCountsOnlyActiveHumans", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 1)
		run(t, s, id, func(w app.WorldTx) error {
			for i, h := range []game.PlayerID{"h1", "h2"} {
				if _, err := game.JoinHuman(w, h, "Player", int64(i+1), game.QuadrantRandom); err != nil {
					return err
				}
			}
			game.QuitPlayer(w, "h1")
			return nil
		})
		list, err := s.List(ctx)
		if err != nil || len(list) != 1 || list[0].Humans != 1 || list[0].Bots != 1 {
			t.Fatalf("list %+v %v", list, err)
		}
	})

	t.Run("TestListCountsOnlyActiveBots", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 3)
		run(t, s, id, func(w app.WorldTx) error {
			for _, p := range w.Players() {
				if !p.IsHuman {
					p.Status = game.StatusEliminated
					w.PutPlayer(p)
					break
				}
			}
			return nil
		})
		list, err := s.List(ctx)
		if err != nil || len(list) != 1 || list[0].Bots != 2 {
			t.Fatalf("list %+v %v", list, err)
		}
	})

	t.Run("TestActionLogKeptWithItsChange", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(1000)
			w.LogAction("key-a", false, "build", map[string]any{"villageId": "v_1", "building": "wood"})
			w.LogAction("bot_1", true, "recruit", map[string]any{"count": 3})
			return nil
		})
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.LogAction("key-a", false, "mint", map[string]any{})
			return errors.New("refused")
		})
		if err == nil {
			t.Fatal("the failing unit of work succeeded")
		}
		_ = s.Read(ctx, id, func(w app.WorldTx) error {
			w.LogAction("key-a", false, "read", map[string]any{})
			return nil
		})
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(2000)
			w.LogAction("key-a", false, "leave", struct{}{})
			return nil
		})
		log, err := s.ActionLog(ctx, id)
		if err != nil || len(log) != 3 {
			t.Fatalf("log %+v %v", log, err)
		}
		var payload map[string]any
		_ = json.Unmarshal(log[0].Payload, &payload)
		if log[0].Seq != 1 || log[0].At != 1000 || log[0].Actor != "key-a" || log[0].IsBot || log[0].Kind != "build" ||
			payload["building"] != "wood" || log[1].Seq != 2 || !log[1].IsBot || log[2].Seq != 3 || log[2].Kind != "leave" {
			t.Fatalf("log %+v", log)
		}
		for range 2 {
			if err := s.CountRejection(ctx, id, "build", "resources"); err != nil {
				t.Fatal(err)
			}
		}
		_ = s.CountRejection(ctx, id, "attack", "noUnits")
		rej, err := s.Rejections(ctx, id)
		if err != nil || len(rej) != 2 || rej[0] != (app.Rejection{Kind: "attack", Reason: "noUnits", Count: 1}) ||
			rej[1] != (app.Rejection{Kind: "build", Reason: "resources", Count: 2}) {
			t.Fatalf("rejections %+v %v", rej, err)
		}
		p := app.PackedLog{WorldID: id, Seed: 7, Secret: testSecret, Config: game.InstantWorldConfig, Build: "dev", Rows: 2, LastSeq: 2,
			Data: []byte{1, 2, 3}, PackedAtMs: t0ms}
		if err := s.SavePackedLog(ctx, p); err != nil {
			t.Fatal(err)
		}
		if rest, _ := s.ActionLog(ctx, id); len(rest) != 1 || rest[0].Seq != 3 {
			t.Fatalf("rows after packing %+v", rest)
		}
		again := p
		again.Rows, again.Data = 9, []byte{9}
		if err := s.SavePackedLog(ctx, again); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		got, err := s.PackedLog(ctx, id)
		if err != nil || got.Seed != 7 || !bytes.Equal(got.Secret, testSecret) || got.Config.Kind != game.KindInstant || got.Build != "dev" || got.Rows != 2 ||
			string(got.Data) != string([]byte{1, 2, 3}) || got.PackedAtMs != t0ms {
			t.Fatalf("packed %+v %v", got, err)
		}
		if _, err := s.PackedLog(ctx, id+1); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing packed log: %v", err)
		}
	})

	t.Run("TestForgetAccountReachesLeaversAndTheWinner", func(t *testing.T) {
		s := newStore(t)
		g := app.ArchivedGame{WorldID: 50, Name: "Instant #1", StartedAtMs: t0ms, EndedAtMs: t0ms + 1, WinnerName: "Ala",
			Summary: game.ArchiveSummary{EndReason: "time", WinnerID: "h1", Humans: 2, Guests: 2},
			Rows: []game.LeaderRow{
				{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Key: "k1", Points: 9},
				{Rank: 2, PlayerID: "h3", Name: "Ewa", Key: "k3", Points: 5},
				{Rank: 3, PlayerID: "bot_1", Name: "Bot", IsBot: true, Points: 1},
			}}
		if err := s.SaveArchive(ctx, g); err != nil {
			t.Fatal(err)
		}
		before, _ := s.ArchiveTotals(ctx, 0)
		if before.Players != 2 {
			t.Fatalf("players %+v", before)
		}
		if err := s.ForgetAccount(ctx, 1, "k1"); err != nil {
			t.Fatal(err)
		}
		if err := s.ForgetAccount(ctx, 3, "k3"); err != nil {
			t.Fatal(err)
		}
		got, err := s.ArchivedGame(ctx, 50)
		if err != nil || got.WinnerName != "" || got.Rows[0].Name != "" || got.Rows[0].AccountID != 0 || got.Rows[1].Name != "" ||
			got.Rows[2].Name != "Bot" || got.Rows[0].Key != "k1" || got.Summary.Guests != 2 {
			t.Fatalf("after forgetting %+v %v", got, err)
		}
		if after, _ := s.ArchiveTotals(ctx, 0); after != before {
			t.Fatalf("totals %+v → %+v", before, after)
		}
	})

	t.Run("TestArchiveOutlivesTheWorld", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 1)
		game1 := app.ArchivedGame{WorldID: id, Name: "Instant #1", StartedAtMs: t0ms, EndedAtMs: t0ms + 3_600_000,
			WinnerName: "Ala", Summary: game.ArchiveSummary{EndReason: "time", Humans: 2, ConquestsHumans: 3, HumanUnitsKilled: 500},
			Rows: []game.LeaderRow{
				{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Points: 900, Villages: 3, Conquered: 2, Killed: 400},
				{Rank: 2, PlayerID: "bot_1", Name: "Bot", IsBot: true, Points: 800, Villages: 2, Conquered: 4, Killed: 300},
				{Rank: 3, PlayerID: "h2", Name: "Ola", AccountID: 2, Points: 100, Villages: 1, Conquered: 1, Killed: 100},
			}}
		later := game1
		later.WorldID, later.EndedAtMs = id+1000, t0ms+10*3_600_000
		later.Rows = []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Points: 50, Villages: 1}}
		for _, g := range []app.ArchivedGame{game1, game1, later} {
			if err := s.SaveArchive(ctx, g); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		got, err := s.ArchivedGame(ctx, id)
		if err != nil || got.Name != "Instant #1" || got.WinnerName != "Ala" || got.Summary.ConquestsHumans != 3 ||
			len(got.Rows) != 3 || got.Rows[1].PlayerID != "bot_1" || !got.Rows[1].IsBot || got.Rows[2].AccountID != 2 {
			t.Fatalf("archived %+v %v", got, err)
		}
		if _, err := s.ArchivedGame(ctx, id+1); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing archive: %v", err)
		}
		all, err := s.ArchiveTotals(ctx, 0)
		if err != nil || all != (app.StatTotals{Games: 2, Players: 2, Conquests: 6, UnitsKilled: 1000}) {
			t.Fatalf("all time %+v %v", all, err)
		}
		recent, err := s.ArchiveTotals(ctx, t0ms+5*3_600_000)
		if err != nil || recent != (app.StatTotals{Games: 1, Players: 1, Conquests: 3, UnitsKilled: 500}) {
			t.Fatalf("recent %+v %v", recent, err)
		}
		games, err := s.AccountGames(ctx, 1)
		if err != nil || len(games) != 2 || games[0].WorldID != later.WorldID || games[1].Rank != 1 || games[1].Players != 3 ||
			games[1].Killed != 400 {
			t.Fatalf("games %+v %v", games, err)
		}
		if err := s.ForgetAccount(ctx, 2, ""); err != nil {
			t.Fatal(err)
		}
		got, _ = s.ArchivedGame(ctx, id)
		if got.Rows[2].Name != "" || got.Rows[2].AccountID != 0 || got.Rows[2].Points != 100 {
			t.Fatalf("forgotten row %+v", got.Rows[2])
		}
		if games, _ := s.AccountGames(ctx, 2); len(games) != 0 {
			t.Fatalf("forgotten games %+v", games)
		}
	})

	t.Run("TestGameNumbersNeverRepeat", func(t *testing.T) {
		s := newStore(t)
		n1, err := s.NextGameNumber(ctx)
		if err != nil || n1 < 1 {
			t.Fatalf("first number %d %v", n1, err)
		}
		id := createWorld(t, s, 1)
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		if n2, err := s.NextGameNumber(ctx); err != nil || n2 != n1+1 {
			t.Fatalf("second number %d after %d: %v", n2, n1, err)
		}
	})

	t.Run("TestGameNumberSurvivesArchive", func(t *testing.T) {
		s := newStore(t)
		row := NewWorldRow(1)
		row.GameNumber = 41
		id, err := s.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
		if err != nil {
			t.Fatal(err)
		}
		if got, err := s.Get(ctx, id); err != nil || got.GameNumber != 41 {
			t.Fatalf("get %+v %v", got, err)
		}
		if list, err := s.List(ctx); err != nil || len(list) != 1 || list[0].GameNumber != 41 {
			t.Fatalf("list %+v %v", list, err)
		}
		g := app.ArchivedGame{WorldID: id, Name: "Game #41", GameNumber: 41, StartedAtMs: t0ms, EndedAtMs: t0ms + 3_600_000,
			Summary: game.ArchiveSummary{EndReason: "time"},
			Rows:    []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Points: 10, Villages: 1}}}
		if err := s.SaveArchive(ctx, g); err != nil {
			t.Fatal(err)
		}
		if got, err := s.ArchivedGame(ctx, id); err != nil || got.GameNumber != 41 {
			t.Fatalf("archived %+v %v", got, err)
		}
		if games, err := s.AccountGames(ctx, 1); err != nil || len(games) != 1 || games[0].GameNumber != 41 {
			t.Fatalf("account games %+v %v", games, err)
		}
	})

	t.Run("TestArchiveListings", func(t *testing.T) {
		s := newStore(t)
		older := app.ArchivedGame{WorldID: 70, Name: "Game #7", GameNumber: 7, EndedAtMs: t0ms + 1000,
			Rows: []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1}, {Rank: 2, PlayerID: "bot_1", IsBot: true}}}
		newer := app.ArchivedGame{WorldID: 80, Name: "Game #8", GameNumber: 8, EndedAtMs: t0ms + 5000,
			Rows: []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1}, {Rank: 2, PlayerID: "h2", Name: "Ola", AccountID: 2}}}
		for _, g := range []app.ArchivedGame{older, newer} {
			if err := s.SaveArchive(ctx, g); err != nil {
				t.Fatal(err)
			}
		}
		all, err := s.ArchivedGameList(ctx, 0)
		want := []app.GameListing{{WorldID: 80, Name: "Game #8", GameNumber: 8, EndedAtMs: t0ms + 5000},
			{WorldID: 70, Name: "Game #7", GameNumber: 7, EndedAtMs: t0ms + 1000}}
		if err != nil || !slices.Equal(all, want) {
			t.Fatalf("list %+v %v", all, err)
		}
		if one, err := s.ArchivedGameList(ctx, 1); err != nil || len(one) != 1 || one[0].WorldID != 80 {
			t.Fatalf("limit 1 %+v %v", one, err)
		}
		latest, err := s.AccountsWithGames(ctx)
		if err != nil || !maps.Equal(latest, map[int64]int64{1: t0ms + 5000, 2: t0ms + 5000}) {
			t.Fatalf("accounts %+v %v", latest, err)
		}
	})

	t.Run("TestHostedWorldRoundTrips", func(t *testing.T) {
		s := newStore(t)
		plain := createWorld(t, s, 1)
		row := NewWorldRow(1)
		row.Config.Kind = game.KindPlayer
		host := app.HostedWorld{HostID: 3, InviteCode: "inv-1", ResultsCode: "res-1", HostName: "Ala", HostPausedMs: 5,
			Banned: []int64{4}, RegisteredOnly: true}
		row.Host = &host
		id, err := s.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
		if err != nil {
			t.Fatal(err)
		}
		if got, err := s.Get(ctx, id); err != nil || got.Host == nil || !reflect.DeepEqual(*got.Host, host) {
			t.Fatalf("get %+v %v", got, err)
		}
		if got, err := s.Get(ctx, plain); err != nil || got.Host != nil {
			t.Fatalf("a plain world has a host: %+v %v", got, err)
		}
		list, err := s.List(ctx)
		if err != nil || len(list) != 2 || list[0].Host != nil || list[1].Host == nil || !reflect.DeepEqual(*list[1].Host, host) {
			t.Fatalf("list %+v %v", list, err)
		}
		started := int64(7)
		run(t, s, id, func(w app.WorldTx) error {
			if w.Row().Host == nil || w.Row().Host.InviteCode != "inv-1" {
				t.Fatalf("run sees %+v", w.Row().Host)
			}
			w.Row().Host.StartedAtMs = &started
			w.Row().Host.InviteCode = "inv-2"
			w.Row().Host.Banned = append(w.Row().Host.Banned, 6)
			return nil
		})
		want := host
		want.StartedAtMs, want.InviteCode, want.Banned = &started, "inv-2", []int64{4, 6}
		if got, _ := s.Get(ctx, id); got.Host == nil || !reflect.DeepEqual(*got.Host, want) {
			t.Fatalf("after run %+v, want %+v", got.Host, want)
		}
		failed := errors.New("no")
		err = s.Run(ctx, id, func(w app.WorldTx) error {
			w.Row().Host.HostName = "Ewa"
			w.Row().Host.Banned[0] = 9
			return failed
		})
		if !errors.Is(err, failed) {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, id); !reflect.DeepEqual(*got.Host, want) {
			t.Fatalf("a failed run changed the host: %+v", got.Host)
		}
	})

	t.Run("TestWorldByInviteAndResultsCode", func(t *testing.T) {
		s := newStore(t)
		createWorld(t, s, 1)
		row := NewWorldRow(1)
		row.Host = &app.HostedWorld{HostID: 3, InviteCode: "inv-1", ResultsCode: "res-1", HostName: "Ala"}
		id, err := s.Create(ctx, row, func(w app.WorldTx) error { return game.NewWorld(w) })
		if err != nil {
			t.Fatal(err)
		}
		if got, err := s.WorldByInvite(ctx, "inv-1"); err != nil || got.ID != id || got.Host == nil {
			t.Fatalf("by invite %+v %v", got, err)
		}
		if got, err := s.WorldByResultsCode(ctx, "res-1"); err != nil || got.ID != id {
			t.Fatalf("by results code %+v %v", got, err)
		}
		run(t, s, id, func(w app.WorldTx) error { w.Row().Host.InviteCode = "inv-2"; return nil })
		if _, err := s.WorldByInvite(ctx, "inv-1"); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("a replaced code still works: %v", err)
		}
		if got, err := s.WorldByInvite(ctx, "inv-2"); err != nil || got.ID != id {
			t.Fatalf("new code %+v %v", got, err)
		}
		for _, code := range []string{"", "res-1"} {
			if _, err := s.WorldByInvite(ctx, code); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("invite %q: %v", code, err)
			}
		}
		if _, err := s.WorldByResultsCode(ctx, ""); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("empty results code: %v", err)
		}
	})

	t.Run("TestHostSettingsDefaultThenSaved", func(t *testing.T) {
		s := newStore(t)
		if got, err := s.HostSettings(ctx); err != nil || got != app.DefaultHostSettings {
			t.Fatalf("default %+v %v", got, err)
		}
		for _, want := range []app.HostSettings{{MaxSeats: 20, MaxBots: 8}, {MaxSeats: 4, MaxBots: 0}} {
			if err := s.SaveHostSettings(ctx, want); err != nil {
				t.Fatal(err)
			}
			if got, err := s.HostSettings(ctx); err != nil || got != want {
				t.Fatalf("saved %+v, read %+v %v", want, got, err)
			}
		}
	})

	t.Run("TestArchiveKeepsKindCodeAndHost", func(t *testing.T) {
		s := newStore(t)
		instant := app.ArchivedGame{WorldID: 90, Name: "Game #9", GameNumber: 9, Kind: game.KindInstant, StartedAtMs: t0ms,
			EndedAtMs: t0ms + 1000, WinnerName: "Ala", Summary: game.ArchiveSummary{EndReason: "time", Humans: 1, ConquestsHumans: 1},
			Rows: []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Points: 10}}}
		hosted := app.ArchivedGame{WorldID: 91, Name: "Friday at Zoë's", Kind: game.KindPlayer, ResultsCode: "res-abc", HostID: 2,
			HostName: "Ola", StartedAtMs: t0ms, EndedAtMs: t0ms + 5000, WinnerName: "Ala",
			Summary: game.ArchiveSummary{EndReason: "host", Humans: 2, ConquestsHumans: 4},
			Rows: []game.LeaderRow{{Rank: 1, PlayerID: "h1", Name: "Ala", AccountID: 1, Points: 30},
				{Rank: 2, PlayerID: "h2", Name: "Ola", AccountID: 2, Points: 20}}}
		for _, g := range []app.ArchivedGame{instant, hosted} {
			if err := s.SaveArchive(ctx, g); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.ArchivedGameByCode(ctx, "res-abc")
		if err != nil || got.WorldID != 91 || got.Kind != game.KindPlayer || got.ResultsCode != "res-abc" || got.HostID != 2 ||
			got.HostName != "Ola" || len(got.Rows) != 2 {
			t.Fatalf("by code %+v %v", got, err)
		}
		for _, code := range []string{"", "res-zzz"} {
			if _, err := s.ArchivedGameByCode(ctx, code); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("code %q: %v", code, err)
			}
		}
		if got, err := s.ArchivedGame(ctx, 90); err != nil || got.Kind != game.KindInstant || got.ResultsCode != "" || got.HostName != "" {
			t.Fatalf("instant %+v %v", got, err)
		}
		games, err := s.AccountGames(ctx, 1)
		if err != nil || len(games) != 2 || games[0].WorldID != 91 || games[0].Kind != game.KindPlayer || games[0].Code != "res-abc" ||
			games[1].Kind != game.KindInstant || games[1].Code != "" {
			t.Fatalf("account games %+v %v", games, err)
		}
		if mine, err := s.HostedGames(ctx, 2, 10); err != nil || len(mine) != 1 || mine[0].WorldID != 91 || mine[0].Code != "res-abc" ||
			mine[0].Players != 2 {
			t.Fatalf("hosted %+v %v", mine, err)
		}
		if none, err := s.HostedGames(ctx, 1, 10); err != nil || len(none) != 0 {
			t.Fatalf("hosted by a player %+v %v", none, err)
		}
		if totals, err := s.ArchiveTotals(ctx, 0); err != nil || totals != (app.StatTotals{Games: 1, Players: 1, Conquests: 1}) {
			t.Fatalf("totals %+v %v", totals, err)
		}
		if list, err := s.ArchivedGameList(ctx, 0); err != nil || len(list) != 1 || list[0].WorldID != 90 {
			t.Fatalf("list %+v %v", list, err)
		}
		if latest, err := s.AccountsWithGames(ctx); err != nil || !maps.Equal(latest, map[int64]int64{1: t0ms + 1000}) {
			t.Fatalf("accounts %+v %v", latest, err)
		}
		if err := s.ForgetAccount(ctx, 2, ""); err != nil {
			t.Fatal(err)
		}
		if got, err := s.ArchivedGameByCode(ctx, "res-abc"); err != nil || got.HostName != "" || got.HostID != 0 {
			t.Fatalf("forgotten host %+v %v", got, err)
		}
		if mine, err := s.HostedGames(ctx, 2, 10); err != nil || len(mine) != 0 {
			t.Fatalf("forgotten host's games %+v %v", mine, err)
		}
	})

	t.Run("TestTimelineKeptWithItsChange", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		battle := game.TimelineEntry{At: 2000, Kind: "battle", X: 3.5, Y: 4.25, FromX: 1, FromY: 2, Attacker: "h1", Defender: "bot_1", Troops: 120}
		run(t, s, id, func(w app.WorldTx) error {
			w.AddTimeline(game.TimelineEntry{At: 1000, Kind: "village", X: 5, Y: 6, Owner: "h1"})
			w.AddTimeline(battle)
			return nil
		})
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.AddTimeline(game.TimelineEntry{At: 3000, Kind: "owner", X: 5, Y: 6})
			return errors.New("refused")
		})
		if err == nil {
			t.Fatal("the failing unit of work succeeded")
		}
		_ = s.Read(ctx, id, func(w app.WorldTx) error {
			w.AddTimeline(game.TimelineEntry{At: 4000, Kind: "owner", X: 5, Y: 6})
			return nil
		})
		got, err := s.TimelineEntries(ctx, id)
		if err != nil || len(got) != 2 || got[0].Kind != "village" || got[0].Owner != "h1" || got[1] != battle {
			t.Fatalf("timeline %+v %v", got, err)
		}
	})

	t.Run("TestPackedTimelineOutlivesTheWorld", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		run(t, s, id, func(w app.WorldTx) error {
			w.AddTimeline(game.TimelineEntry{At: 1000, Kind: "village", X: 5, Y: 6, Owner: "h1"})
			return nil
		})
		if err := s.SavePackedTimeline(ctx, id, []byte{1, 2, 3}, 1); err != nil {
			t.Fatal(err)
		}
		if rest, err := s.TimelineEntries(ctx, id); err != nil || len(rest) != 0 {
			t.Fatalf("entries after packing %+v %v", rest, err)
		}
		if err := s.SavePackedTimeline(ctx, id, []byte{9}, 9); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		got, err := s.PackedTimeline(ctx, id)
		if err != nil || string(got) != string([]byte{1, 2, 3}) {
			t.Fatalf("packed %v %v", got, err)
		}
		if _, err := s.PackedTimeline(ctx, id+1); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing packed timeline: %v", err)
		}
	})

	t.Run("TestPublicEventKeptWithItsChange", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		e := game.PublicEvent{Kind: "battle", Attacker: "Ala", Troops: 120, X: 3, Y: 4}
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.SetNow(5000)
			w.AddPublicEvent(e)
			return errors.New("refused")
		})
		if err == nil {
			t.Fatal("the failing unit of work succeeded")
		}
		if got, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 10}); err != nil || len(got) != 0 {
			t.Fatalf("events after a failed run %+v %v", got, err)
		}
		if _, ok, err := s.LastBattleAt(ctx); err != nil || ok {
			t.Fatalf("last battle after a failed run: %v %v", ok, err)
		}
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(5000)
			w.AddPublicEvent(e)
			return nil
		})
		row, err := s.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 10})
		if err != nil || len(got) != 1 {
			t.Fatalf("events %+v %v", got, err)
		}
		g := got[0]
		if g.WorldID != id || g.WorldName != row.Name || g.WorldKind != "" || !g.At.Equal(app.WallTime(row, 5000)) ||
			g.Kind != "battle" || g.Attacker != "Ala" || g.Defender != "" || g.Troops != 120 || g.X != 3 || g.Y != 4 {
			t.Fatalf("event %+v", g)
		}
	})

	t.Run("TestPublicEventsNewestFirstAcrossWorlds", func(t *testing.T) {
		s := newStore(t)
		a, b := createWorld(t, s, 0), createWorld(t, s, 0)
		add := func(id, at int64) {
			run(t, s, id, func(w app.WorldTx) error {
				w.SetNow(at)
				w.AddPublicEvent(game.PublicEvent{Kind: "conquest", Attacker: "Ala", Troops: 1})
				return nil
			})
		}
		add(a, 1000)
		add(a, 2000)
		add(b, 1500)
		got, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 10})
		if err != nil || len(got) != 3 || got[0].WorldID != a || got[1].WorldID != b || got[2].WorldID != a ||
			!got[0].At.Equal(time.UnixMilli(t0ms+2000)) || !got[2].At.Equal(time.UnixMilli(t0ms+1000)) {
			t.Fatalf("events %+v %v", got, err)
		}
		only, err := s.PublicEvents(ctx, app.PublicEventQuery{WorldID: b, Limit: 10})
		if err != nil || len(only) != 1 || only[0].WorldID != b {
			t.Fatalf("world B %+v %v", only, err)
		}
	})

	t.Run("TestPublicEventsFilterSmallBattles", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		for i, e := range []game.PublicEvent{{Kind: "battle", Troops: 99}, {Kind: "battle", Troops: 100}, {Kind: "conquest", Troops: 5}} {
			run(t, s, id, func(w app.WorldTx) error {
				w.SetNow(int64(i+1) * 1000)
				w.AddPublicEvent(e)
				return nil
			})
		}
		big, err := s.PublicEvents(ctx, app.PublicEventQuery{MinTroops: game.LargeBattleTroops, Limit: 10})
		if err != nil || len(big) != 2 || big[0].Kind != "conquest" || big[1].Troops != 100 {
			t.Fatalf("large %+v %v", big, err)
		}
		if all, err := s.PublicEvents(ctx, app.PublicEventQuery{}); err != nil || len(all) != 3 {
			t.Fatalf("all %+v %v", all, err)
		}
		if one, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 1}); err != nil || len(one) != 1 || one[0].Kind != "conquest" {
			t.Fatalf("limit 1 %+v %v", one, err)
		}
	})

	t.Run("TestLastBattleAtCountsEverySize", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(7000)
			w.AddPublicEvent(game.PublicEvent{Kind: "battle", Troops: 5})
			return nil
		})
		at, ok, err := s.LastBattleAt(ctx)
		if err != nil || !ok || !at.Equal(time.UnixMilli(t0ms+7000)) {
			t.Fatalf("last battle %v %v %v", at, ok, err)
		}
	})

	t.Run("TestPublicEventsGoWithTheWorld", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 0)
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(1000)
			w.AddPublicEvent(game.PublicEvent{Kind: "battle", Troops: 500})
			return nil
		})
		if err := s.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
		if got, err := s.PublicEvents(ctx, app.PublicEventQuery{Limit: 10}); err != nil || len(got) != 0 {
			t.Fatalf("events of a deleted world %+v %v", got, err)
		}
		if _, ok, err := s.LastBattleAt(ctx); err != nil || ok {
			t.Fatalf("last battle of a deleted world: %v %v", ok, err)
		}
	})

	t.Run("TestRunPersistsOnlyOnSuccess", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		vid := firstVillage(t, s, id)
		before := villageName(t, s, id, vid)
		boom := errors.New("boom")
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.Village(vid).Name = "Renamed"
			w.Schedule("test:x", 5)
			w.Row().Name = "Other name"
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err %v", err)
		}
		read(t, s, id, func(w app.WorldTx) {
			if w.Village(vid).Name != before || w.IsScheduled("test:x") || w.Row().Name != "Café Test World" {
				t.Fatal("a failed run left changes")
			}
		})
		run(t, s, id, func(w app.WorldTx) error {
			w.Village(vid).Name = "Renamed"
			w.Schedule("test:x", 5)
			w.Row().Name = "Other name"
			return nil
		})
		read(t, s, id, func(w app.WorldTx) {
			if w.Village(vid).Name != "Renamed" || !w.IsScheduled("test:x") || w.Row().Name != "Other name" {
				t.Fatal("a successful run lost changes")
			}
		})
	})

	t.Run("TestPanicStoreErrorRollsBack", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		vid := firstVillage(t, s, id)
		before := villageName(t, s, id, vid)
		cause := errors.New("database")
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.Village(vid).Name = "Renamed"
			panic(&game.StoreError{Err: cause})
		})
		var se *game.StoreError
		if !errors.As(err, &se) || !errors.Is(err, cause) {
			t.Fatalf("err %v", err)
		}
		if got := villageName(t, s, id, vid); got != before {
			t.Fatalf("name %q", got)
		}
	})

	t.Run("TestReadNeverWrites", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		vid := firstVillage(t, s, id)
		before := villageName(t, s, id, vid)
		read(t, s, id, func(w app.WorldTx) {
			w.Village(vid).Name = "Renamed"
			w.Schedule("test:read", 5)
			w.PutCommand(&game.Command{ID: "cmd_read", Type: "attack", OriginID: vid, TargetID: vid})
			if !w.IsScheduled("test:read") {
				t.Error("a read's own schedule is invisible to it")
			}
		})
		read(t, s, id, func(w app.WorldTx) {
			if w.Village(vid).Name != before || w.IsScheduled("test:read") || w.Command("cmd_read") != nil {
				t.Fatal("a read wrote")
			}
		})
	})

	t.Run("TestQueriesSeeUnflushedChanges", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		run(t, s, id, func(w app.WorldTx) error {
			p := w.Players()[0]
			v := w.Village(p.VillageIDs[0])
			v.Name = "New"
			v.Points += 1000
			found := false
			for _, ref := range w.VillagesIn(v.X, v.Y, v.X, v.Y) {
				found = found || (ref.ID == v.ID && ref.Name == "New")
			}
			if !found {
				t.Error("VillagesIn misses a changed name")
			}
			fresh := &game.Village{ID: "vil_fresh", X: 1, Y: 1, Name: "Fresh Café", OwnerID: game.Ref(p.ID), Points: 5,
				SupportFrom: map[string]game.UnitCounts{}}
			w.PutVillage(fresh)
			if refs := w.VillagesIn(1, 1, 1, 1); len(refs) != 1 || refs[0].ID != "vil_fresh" {
				t.Errorf("VillagesIn misses a new village: %+v", refs)
			}
			want := 0
			for _, vid := range p.VillageIDs {
				want += w.Village(vid).Points
			}
			if got := w.PointsByOwner()[p.ID]; got != want+5 {
				t.Errorf("points %d, want %d", got, want+5)
			}
			fresh.SupportFrom[v.ID] = game.UnitCounts{3}
			if hosts := w.SupportHosts(v.ID); len(hosts) != 1 || hosts[0] != fresh {
				t.Errorf("support hosts %v", hosts)
			}
			return nil
		})
		var home string
		run(t, s, id, func(w app.WorldTx) error {
			home = w.Players()[0].VillageIDs[0]
			hosts := w.SupportHosts(home)
			if len(hosts) != 1 || hosts[0].ID != "vil_fresh" || hosts[0].SupportFrom[home][0] != 3 {
				t.Fatalf("stored support hosts %v", hosts)
			}
			delete(hosts[0].SupportFrom, home)
			if len(w.SupportHosts(home)) != 0 {
				t.Error("a withdrawn support is still listed")
			}
			return nil
		})
		read(t, s, id, func(w app.WorldTx) {
			if len(w.SupportHosts(home)) != 0 {
				t.Error("a withdrawn support is still stored")
			}
			if refs := w.VillagesIn(1, 1, 1, 1); len(refs) != 1 || refs[0].Name != "Fresh Café" {
				t.Errorf("stored refs %+v", refs)
			}
		})
	})

	t.Run("TestJoinedHumanIsAMember", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 2)
		run(t, s, id, func(w app.WorldTx) error {
			_, err := game.JoinHuman(w, "h5", "Ala", 5, game.QuadrantRandom)
			return err
		})
		ms, err := s.MembershipsOf(ctx, 5)
		if err != nil || len(ms) != 1 || ms[0] != (app.Membership{WorldID: id, PlayerID: "h5"}) {
			t.Fatalf("memberships %+v %v", ms, err)
		}
		if ms, _ := s.MembershipsOf(ctx, 6); len(ms) != 0 {
			t.Fatalf("stranger memberships %+v", ms)
		}
		read(t, s, id, func(w app.WorldTx) {
			p := w.Player("h5")
			if p == nil || p.AccountID != 5 || w.Human("h5") == nil || w.Brain("h5") != nil {
				t.Fatalf("human %+v", p)
			}
		})
		list, _ := s.List(ctx)
		if list[0].Humans != 1 || list[0].Bots != 2 {
			t.Fatalf("list %+v", list)
		}
	})

	t.Run("TestRunSerialisesPerWorld", func(t *testing.T) {
		s := newStore(t)
		id := createWorld(t, s, 1)
		vid := firstVillage(t, s, id)
		run(t, s, id, func(w app.WorldTx) error { w.Village(vid).Name = "0"; return nil })
		var wg sync.WaitGroup
		errs := make(chan error, 200)
		for range 2 {
			wg.Go(func() {
				for range 100 {
					errs <- s.Run(ctx, id, func(w app.WorldTx) error {
						v := w.Village(vid)
						n, err := strconv.Atoi(v.Name)
						v.Name = strconv.Itoa(n + 1)
						return err
					})
				}
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if got := villageName(t, s, id, vid); got != "200" {
			t.Fatalf("counter %s", got)
		}
	})

	t.Run("TestEventsPopInOrder", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		wake := subscribeWakeups(t, b.Wakeups)
		run(t, s, id, func(w app.WorldTx) error {
			w.Schedule("test:a", 5)
			w.Schedule("test:b", 5)
			w.Schedule("test:c", 3)
			w.Schedule("test:d", 4)
			w.Schedule("test:e", 9)
			return nil
		})
		waitWakeup(t, wake, id)
		run(t, s, id, func(w app.WorldTx) error {
			w.Schedule("test:a", 5) // same key and time: keeps its place
			w.Schedule("test:d", 6) // moves behind a and b
			w.Unschedule("test:e")
			return nil
		})
		var got []string
		run(t, s, id, func(w app.WorldTx) error {
			for {
				key, at, ok := w.PopDue(10)
				if !ok {
					return nil
				}
				if len(key) > 5 && key[:5] == "test:" {
					got = append(got, key+"@"+strconv.FormatInt(at, 10))
				}
			}
		})
		want := []string{"test:c@3", "test:a@5", "test:b@5", "test:d@6"}
		if len(got) != len(want) {
			t.Fatalf("popped %v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("popped %v, want %v", got, want)
			}
		}
		read(t, s, id, func(w app.WorldTx) {
			if w.IsScheduled("test:a") || w.IsScheduled("test:e") {
				t.Fatal("popped or unscheduled events remain")
			}
		})
	})

	t.Run("TestUpdatesOnlyAfterCommit", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		updates := subscribeUpdates(t, b.Updates)
		drain(t, s, id, updates)
		err := s.Run(ctx, id, func(w app.WorldTx) error {
			w.Village(v1).Name = "Rejected"
			return errors.New("boom")
		})
		if err == nil {
			t.Fatal("no error")
		}
		run(t, s, id, func(w app.WorldTx) error { w.Village(v2).Name = "Accepted"; return nil })
		u := next(t, updates)
		if u.PlayerID != "h2" || u.WorldID != id {
			t.Fatalf("first update after a rolled-back run: %+v", u)
		}
	})

	t.Run("TestUpdateTopics", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		updates := subscribeUpdates(t, b.Updates)
		drain(t, s, id, updates)
		var arrives int64
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(1000)
			for _, h := range []game.PlayerID{"h1", "h2"} {
				w.Player(h).ProtectedUntil = 0
			}
			target := w.Village(v2)
			c, err := game.SendCommand(w, "h1", v1, target.X, target.Y, someUnits(w.Village(v1)), "attack", game.SendOpts{})
			if err != nil {
				return err
			}
			arrives = c.ArrivesAt
			return nil
		})
		expectTopics(t, updates, map[game.PlayerID]string{"h1": app.TopicState, "h2": app.TopicState})
		Advance(t, s, id, 1000, arrives, 50)
		expectTopics(t, updates, map[game.PlayerID]string{"h1": app.TopicReports, "h2": app.TopicReports})
		run(t, s, id, func(w app.WorldTx) error { w.Village(v2).Name = "New name"; return nil })
		expectWorld(t, updates, app.TopicMap, v2)
		// A change of the world itself (pause, settings, status) reaches every open client.
		run(t, s, id, func(w app.WorldTx) error {
			at := int64(5)
			w.Row().PausedAtMs = &at
			return nil
		})
		for {
			if u := next(t, updates); u.PlayerID == "" && hasTopic(u, app.TopicState) {
				break
			}
		}
	})

	t.Run("TestInterceptMarksBothOwnersDirty", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		updates := subscribeUpdates(t, b.Updates)
		var attackID string
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(1000)
			for _, h := range []game.PlayerID{"h1", "h2"} {
				w.Player(h).ProtectedUntil = 0
			}
			w.Village(v1).Units[game.USpear] = 5
			target := w.Village(v2)
			c, err := game.SendCommand(w, "h1", v1, target.X, target.Y, game.UnitCounts{game.USpear: 5}, "attack", game.SendOpts{})
			if err != nil {
				return err
			}
			attackID = c.ID
			return nil
		})
		drain(t, s, id, updates)
		run(t, s, id, func(w app.WorldTx) error {
			w.Village(v2).Units[game.ULightCav] = 10
			_, err := game.SendIntercept(w, "h2", v2, attackID, game.UnitCounts{game.ULightCav: 10}, false)
			return err
		})
		expectTopics(t, updates, map[game.PlayerID]string{"h1": app.TopicState, "h2": app.TopicState})
	})

	t.Run("TestArmiesMarching", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		mk := func(w app.WorldTx, id, typ string, arrives int64) {
			game.AddCommand(w, &game.Command{ID: id, Type: typ, OriginID: v1, TargetID: v2, Units: game.UnitCounts{game.USpear: 1},
				DepartedAt: 0, ArrivesAt: arrives})
		}
		run(t, s, id, func(w app.WorldTx) error {
			mk(w, "c_a", "attack", 5000)
			mk(w, "c_s", "support", 5000)
			mk(w, "c_r", "return", 5000)
			mk(w, "c_t", "trade", 5000)
			mk(w, "c_old", "attack", 900)
			w.PutCommand(&game.Command{ID: "c_i", Type: "intercept", OriginID: v1, DepartedAt: 0, ArrivesAt: 5000})
			return nil
		})
		read(t, s, id, func(w app.WorldTx) {
			w.Command("c_s").ArrivesAt = 950 // changed in this unit of work, not yet stored
			var got []string
			for _, c := range w.ArmiesMarching(1000) {
				got = append(got, c.ID)
			}
			if want := []string{"c_a", "c_r"}; !slices.Equal(got, want) {
				t.Fatalf("armies marching at 1000: %v, want %v", got, want)
			}
		})
	})

	t.Run("TestMovingCommandsListsEveryArmy", func(t *testing.T) {
		s := newBackend(t).Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		run(t, s, id, func(w app.WorldTx) error {
			for _, c := range []struct {
				id, typ string
				arrives int64
			}{{"c_a", "attack", 5000}, {"c_s", "support", 5000}, {"c_r", "return", 5000}, {"c_t", "trade", 5000}, {"c_old", "attack", 900}} {
				game.AddCommand(w, &game.Command{ID: c.id, Type: c.typ, OriginID: v1, TargetID: v2, Units: game.UnitCounts{game.USpear: 1},
					DepartedAt: 0, ArrivesAt: c.arrives})
			}
			w.PutCommand(&game.Command{ID: "c_i", Type: "intercept", OriginID: v1, DepartedAt: 0, ArrivesAt: 5000})
			return nil
		})
		read(t, s, id, func(w app.WorldTx) {
			var got []string
			for _, c := range w.MovingCommands(1000) {
				got = append(got, c.ID)
			}
			if want := []string{"c_a", "c_i", "c_r", "c_s"}; !slices.Equal(got, want) {
				t.Fatalf("moving at 1000: %v, want %v", got, want)
			}
		})
	})

	t.Run("TestSightEventsNotifyBystander", func(t *testing.T) {
		b := newBackend(t)
		s := b.Store
		id := createWorld(t, s, 0)
		v1, v2 := joinTwo(t, s, id)
		var arrives int64
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(1000)
			for _, h := range []game.PlayerID{"h1", "h2"} {
				w.Player(h).ProtectedUntil = 0
			}
			if _, err := game.JoinHuman(w, "h3", "Cezary", 3, game.QuadrantRandom); err != nil {
				return err
			}
			a, t2 := w.Village(v1), w.Village(v2)
			x, y := (a.X+t2.X)/2, (a.Y+t2.Y)/2
			for game.VillageExistsAt(w, x, y) {
				x++
			}
			v := game.CreateVillage(x, y, "", "", w.Now())
			w.PutVillage(v)
			game.SetOwner(w, v, "h3")
			a.Units[game.USpear] = 5
			c, err := game.SendCommand(w, "h1", v1, t2.X, t2.Y, game.UnitCounts{game.USpear: 5}, "attack", game.SendOpts{})
			if err != nil {
				return err
			}
			arrives = c.ArrivesAt
			return nil
		})
		updates := subscribeUpdates(t, b.Updates)
		Advance(t, s, id, 1000, arrives-1, 50)
		expectTopics(t, updates, map[game.PlayerID]string{"h3": app.TopicState})
	})
}

func joinTwo(t *testing.T, s app.WorldStore, id int64) (string, string) {
	t.Helper()
	var v1, v2 string
	run(t, s, id, func(w app.WorldTx) error {
		a, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		if err != nil {
			return err
		}
		b, err := game.JoinHuman(w, "h2", "Bartek", 2, game.QuadrantRandom)
		if err != nil {
			return err
		}
		v1, v2 = a.ID, b.ID
		return nil
	})
	return v1, v2
}

// someUnits is half of every unit type but nobles in v, at least one of the first present.
func someUnits(v *game.Village) game.UnitCounts {
	var u game.UnitCounts
	for i, n := range v.Units {
		if game.UnitID(i) != game.UNoble && n > 0 {
			u[i] = max(1, n/2)
		}
	}
	return u
}

func subscribeUpdates(t *testing.T, feed app.Updates) <-chan app.Update {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch, err := feed.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func subscribeWakeups(t *testing.T, feed app.EventWakeups) <-chan int64 {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ch, err := feed.Subscribe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

const feedTimeout = 10 * time.Second

func next(t *testing.T, ch <-chan app.Update) app.Update {
	t.Helper()
	select {
	case u := <-ch:
		return u
	case <-time.After(feedTimeout):
		t.Fatal("no update")
		return app.Update{}
	}
}

func waitWakeup(t *testing.T, ch <-chan int64, id int64) {
	t.Helper()
	for {
		select {
		case got := <-ch:
			if got == id {
				return
			}
		case <-time.After(feedTimeout):
			t.Fatal("no wakeup")
		}
	}
}

// drain commits a marker village no other commit touches and discards updates up to and including its map
// update, the last update of that commit, so earlier commits' updates are behind the test.
func drain(t *testing.T, s app.WorldStore, id int64, ch <-chan app.Update) {
	t.Helper()
	marker := "marker_" + strconv.Itoa(int(markers.Add(1)))
	run(t, s, id, func(w app.WorldTx) error {
		size := w.Config().MapSize
		for x := 0; x < size; x++ {
			if len(w.VillagesIn(x, 0, x, 0)) == 0 {
				w.PutVillage(&game.Village{ID: marker, X: x, Y: 0, Name: marker, SupportFrom: map[string]game.UnitCounts{}})
				return nil
			}
		}
		return errors.New("no free tile for a marker")
	})
	expectWorld(t, ch, app.TopicMap, marker)
}

var markers atomic.Int64

func hasTopic(u app.Update, topic string) bool {
	for _, x := range u.Topics {
		if x == topic {
			return true
		}
	}
	return false
}

// expectTopics waits until every listed player got an update with its topic.
func expectTopics(t *testing.T, ch <-chan app.Update, want map[game.PlayerID]string) {
	t.Helper()
	missing := maps.Clone(want)
	for len(missing) > 0 {
		u := next(t, ch)
		if topic, ok := missing[u.PlayerID]; ok && hasTopic(u, topic) {
			delete(missing, u.PlayerID)
		}
	}
}

func expectWorld(t *testing.T, ch <-chan app.Update, topic, villageID string) {
	t.Helper()
	for {
		u := next(t, ch)
		if u.PlayerID == "" && hasTopic(u, topic) && slices.Contains(u.VillageIDs, villageID) {
			return
		}
	}
}

// Advance plays world id from world time `from` to `until` through the store, at most batch events per
// unit of work, the way the event worker does; returns until.
func Advance(t *testing.T, s app.WorldStore, id int64, from, until int64, batch int) int64 {
	t.Helper()
	cur := from
	for more := true; more; {
		run(t, s, id, func(w app.WorldTx) error {
			w.SetNow(cur)
			for range batch {
				key, at, ok := w.PopDue(until)
				if !ok {
					more = false
					return nil
				}
				w.SetNow(at)
				cur = at
				if err := game.Dispatch(w, key); err != nil {
					return fmt.Errorf("%s at %d: %w", key, at, err)
				}
				for _, h := range w.TouchedHumans() {
					game.SettleQuests(w, h)
				}
			}
			return nil
		})
	}
	return until
}
