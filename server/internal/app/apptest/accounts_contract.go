// Package apptest holds contract tests every adapter of an app port must pass.
package apptest

import (
	"context"
	"errors"
	"testing"
	"time"

	"librehold/internal/app"
)

var t0 = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

// RunAccountStoreContract checks an AccountStore; newStore returns an empty store.
func RunAccountStoreContract(t *testing.T, newStore func(t *testing.T) app.AccountStore) {
	ctx := context.Background()

	t.Run("register", func(t *testing.T) {
		s := newStore(t)
		a := &app.Account{Username: "Ala", PasswordHash: "h"}
		if err := s.Register(ctx, a, t0); err != nil {
			t.Fatal(err)
		}
		if a.ID == 0 {
			t.Fatal("account id not set")
		}
		byName, err := s.ByUsername(ctx, "ALA")
		if err != nil || byName.ID != a.ID || byName.Username != "Ala" || byName.IsAdmin {
			t.Fatalf("by username %+v %v", byName, err)
		}
		byID, err := s.ByID(ctx, a.ID)
		if err != nil || byID.Username != "Ala" {
			t.Fatalf("by id %+v %v", byID, err)
		}
		if _, err := s.ByUsername(ctx, "nobody"); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing user: %v", err)
		}
	})

	t.Run("a duplicate username in different case is taken", func(t *testing.T) {
		s := newStore(t)
		if err := s.Register(ctx, &app.Account{Username: "Bartek", PasswordHash: "h"}, t0); err != nil {
			t.Fatal(err)
		}
		if err := s.Register(ctx, &app.Account{Username: "BARTEK", PasswordHash: "h"}, t0); !errors.Is(err, app.ErrUsernameTaken) {
			t.Fatalf("err %v", err)
		}
		list, err := s.List(ctx)
		if err != nil || len(list) != 1 {
			t.Fatalf("list %+v %v", list, err)
		}
	})

	t.Run("admin, listing and password", func(t *testing.T) {
		s := newStore(t)
		if ok, err := s.AdminExists(ctx); err != nil || ok {
			t.Fatalf("admin exists %v %v", ok, err)
		}
		admin := &app.Account{Username: "admin", PasswordHash: "h", IsAdmin: true}
		if err := s.CreateAdmin(ctx, admin); err != nil || admin.ID == 0 {
			t.Fatal(err)
		}
		if ok, err := s.AdminExists(ctx); err != nil || !ok {
			t.Fatal("no admin")
		}
		if err := s.SetPasswordHash(ctx, admin.ID, "h2"); err != nil {
			t.Fatal(err)
		}
		got, _ := s.ByID(ctx, admin.ID)
		if got.PasswordHash != "h2" || !got.IsAdmin {
			t.Fatalf("%+v", got)
		}
		list, err := s.List(ctx)
		if err != nil || len(list) != 1 || list[0].Username != "admin" {
			t.Fatalf("list %+v %v", list, err)
		}
		if err := s.SetPasswordHash(ctx, 9999, "x"); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing account err %v", err)
		}
	})

	t.Run("session create, read, touch, delete", func(t *testing.T) {
		s := newStore(t)
		a := &app.Account{Username: "admin", PasswordHash: "h", IsAdmin: true}
		if err := s.CreateAdmin(ctx, a); err != nil {
			t.Fatal(err)
		}
		hash := []byte{1, 2, 3}
		if err := s.CreateSession(ctx, app.Session{TokenHash: hash, AccountID: a.ID, ExpiresAt: t0.Add(time.Hour), LastSeenAt: t0}); err != nil {
			t.Fatal(err)
		}
		got, err := s.Session(ctx, hash)
		if err != nil || got.AccountID != a.ID || !got.ExpiresAt.Equal(t0.Add(time.Hour)) || !got.LastSeenAt.Equal(t0) {
			t.Fatalf("session %+v %v", got, err)
		}
		if err := s.TouchSession(ctx, hash, t0.Add(time.Minute), t0.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, _ = s.Session(ctx, hash)
		if !got.LastSeenAt.Equal(t0.Add(time.Minute)) || !got.ExpiresAt.Equal(t0.Add(2*time.Hour)) {
			t.Fatalf("touched %+v", got)
		}
		if err := s.DeleteSession(ctx, hash); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Session(ctx, hash); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("after delete %v", err)
		}
	})

	t.Run("DeleteSessionsOf", func(t *testing.T) {
		s := newStore(t)
		a := &app.Account{Username: "a1", PasswordHash: "h"}
		if err := s.Register(ctx, a, t0); err != nil {
			t.Fatal(err)
		}
		admin := &app.Account{Username: "admin", PasswordHash: "h", IsAdmin: true}
		if err := s.CreateAdmin(ctx, admin); err != nil {
			t.Fatal(err)
		}
		for i, id := range []int64{a.ID, a.ID, admin.ID} {
			if err := s.CreateSession(ctx, app.Session{TokenHash: []byte{byte(i)}, AccountID: id, ExpiresAt: t0, LastSeenAt: t0}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.DeleteSessionsOf(ctx, a.ID); err != nil {
			t.Fatal(err)
		}
		for i, want := range []bool{false, false, true} {
			_, err := s.Session(ctx, []byte{byte(i)})
			if (err == nil) != want {
				t.Fatalf("session %d: %v", i, err)
			}
		}
	})

	t.Run("register keeps locale and guest flag and gives a unique player key", func(t *testing.T) {
		s := newStore(t)
		a := &app.Account{Username: "Gosc1", Locale: "pl", IsGuest: true}
		b := &app.Account{Username: "Gosc2", Locale: "en"}
		for _, x := range []*app.Account{a, b} {
			if err := s.Register(ctx, x, t0); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.ByID(ctx, a.ID)
		if err != nil || got.Locale != "pl" || !got.IsGuest || len(got.PlayerKey) != 32 || got.TutorialDone {
			t.Fatalf("guest %+v %v", got, err)
		}
		other, _ := s.ByID(ctx, b.ID)
		if other.PlayerKey == got.PlayerKey || other.IsGuest || !other.LastSeenAt.Equal(t0) {
			t.Fatalf("second %+v", other)
		}
	})

	t.Run("UpdateProfile persists every field it is given", func(t *testing.T) {
		s := newStore(t)
		a := &app.Account{Username: "Guest", Locale: "en", IsGuest: true}
		if err := s.Register(ctx, a, t0); err != nil {
			t.Fatal(err)
		}
		pl, yes, no, name, hash := "pl", true, false, "Kalvik", "hash"
		if err := s.UpdateProfile(ctx, a.ID, app.ProfilePatch{Locale: &pl, TutorialDone: &yes}); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.ByID(ctx, a.ID); got.Locale != "pl" || !got.TutorialDone || got.Username != "Guest" {
			t.Fatalf("after locale %+v", got)
		}
		if err := s.UpdateProfile(ctx, a.ID, app.ProfilePatch{Username: &name, PasswordHash: &hash, IsGuest: &no}); err != nil {
			t.Fatal(err)
		}
		got, err := s.ByUsername(ctx, "kalvik")
		if err != nil || got.ID != a.ID || got.PasswordHash != "hash" || got.IsGuest {
			t.Fatalf("after claim %+v %v", got, err)
		}
		if err := s.Register(ctx, &app.Account{Username: "Taken"}, t0); err != nil {
			t.Fatal(err)
		}
		taken := "TAKEN"
		if err := s.UpdateProfile(ctx, a.ID, app.ProfilePatch{Username: &taken}); !errors.Is(err, app.ErrUsernameTaken) {
			t.Fatalf("taken name: %v", err)
		}
		if err := s.UpdateProfile(ctx, 999, app.ProfilePatch{Locale: &pl}); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing account: %v", err)
		}
	})

	t.Run("TouchAccount, GuestsInactiveSince and DeleteAccount", func(t *testing.T) {
		s := newStore(t)
		old := &app.Account{Username: "Stary", IsGuest: true}
		fresh := &app.Account{Username: "Nowy", IsGuest: true}
		player := &app.Account{Username: "Player"}
		for _, x := range []*app.Account{old, fresh, player} {
			if err := s.Register(ctx, x, t0); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.TouchAccount(ctx, fresh.ID, t0.Add(40*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		ids, err := s.GuestsInactiveSince(ctx, t0.Add(24*time.Hour), 10)
		if err != nil || len(ids) != 1 || ids[0] != old.ID {
			t.Fatalf("inactive %v %v", ids, err)
		}
		if err := s.CreateSession(ctx, app.Session{TokenHash: []byte("old"), AccountID: old.ID, ExpiresAt: t0, LastSeenAt: t0}); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteAccount(ctx, old.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ByID(ctx, old.ID); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("deleted account: %v", err)
		}
		if _, err := s.Session(ctx, []byte("old")); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("its session: %v", err)
		}
	})

	t.Run("TestProblemReportsNewestFirstWithSender", func(t *testing.T) {
		s := newStore(t)
		ala := &app.Account{Username: "Ala", PasswordHash: "h"}
		if err := s.Register(ctx, ala, t0); err != nil {
			t.Fatal(err)
		}
		mine := &app.ProblemReport{CreatedAt: t0, Kind: "bug", Text: "The map does not load", AccountID: ala.ID,
			WorldID: 4, WorldName: "Game #4", Screen: "map", Build: "abc123", UserAgent: "Firefox"}
		visitor := &app.ProblemReport{CreatedAt: t0.Add(time.Minute), Kind: "question", Text: "How do I start?", Email: "x@y.pl"}
		for _, r := range []*app.ProblemReport{mine, visitor} {
			if err := s.AddProblemReport(ctx, r); err != nil {
				t.Fatal(err)
			}
			if r.ID == 0 || r.Status != "new" {
				t.Fatalf("added %+v", r)
			}
		}
		list, err := s.ProblemReports(ctx, 10)
		if err != nil || len(list) != 2 {
			t.Fatalf("list %+v %v", list, err)
		}
		v, m := list[0], list[1]
		if v.ID != visitor.ID || v.Username != "" || v.AccountID != 0 || v.Email != "x@y.pl" || v.Status != "new" ||
			!v.CreatedAt.Equal(visitor.CreatedAt) || v.ClosedAt != nil {
			t.Fatalf("visitor's %+v", v)
		}
		if m.ID != mine.ID || m.Username != "Ala" || m.AccountID != ala.ID || m.Kind != "bug" || m.Text != mine.Text ||
			m.WorldID != 4 || m.WorldName != "Game #4" || m.Screen != "map" || m.Build != "abc123" || m.UserAgent != "Firefox" {
			t.Fatalf("Ala's %+v", m)
		}
		if one, err := s.ProblemReports(ctx, 1); err != nil || len(one) != 1 || one[0].ID != visitor.ID {
			t.Fatalf("limit 1 %+v %v", one, err)
		}
	})

	t.Run("TestProblemReportOfDeletedAccountBecomesVisitor", func(t *testing.T) {
		s := newStore(t)
		ala := &app.Account{Username: "Ala", PasswordHash: "h"}
		if err := s.Register(ctx, ala, t0); err != nil {
			t.Fatal(err)
		}
		if err := s.AddProblemReport(ctx, &app.ProblemReport{CreatedAt: t0, Kind: "bug", Text: "Something broke", AccountID: ala.ID}); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteAccount(ctx, ala.ID); err != nil {
			t.Fatal(err)
		}
		list, err := s.ProblemReports(ctx, 10)
		if err != nil || len(list) != 1 || list[0].AccountID != 0 || list[0].Username != "" {
			t.Fatalf("list %+v %v", list, err)
		}
	})

	t.Run("TestProblemReportStatusAndPurge", func(t *testing.T) {
		s := newStore(t)
		var ids []int64
		for range 3 {
			r := &app.ProblemReport{CreatedAt: t0, Kind: "other", Text: "Some words here"}
			if err := s.AddProblemReport(ctx, r); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, r.ID)
		}
		status := func(id int64) app.ProblemReport {
			t.Helper()
			list, _ := s.ProblemReports(ctx, 10)
			for _, r := range list {
				if r.ID == id {
					return r
				}
			}
			t.Fatalf("report %d is gone", id)
			return app.ProblemReport{}
		}
		if err := s.SetProblemReportStatus(ctx, ids[0], "done", t0.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if r := status(ids[0]); r.Status != "done" || r.ClosedAt == nil || !r.ClosedAt.Equal(t0.Add(time.Hour)) {
			t.Fatalf("done %+v", r)
		}
		if err := s.SetProblemReportStatus(ctx, ids[0], "read", t0.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if r := status(ids[0]); r.Status != "read" || r.ClosedAt != nil {
			t.Fatalf("read again %+v", r)
		}
		_ = s.SetProblemReportStatus(ctx, ids[0], "done", t0.Add(time.Hour))
		_ = s.SetProblemReportStatus(ctx, ids[1], "done", t0.Add(3*time.Hour))
		n, err := s.DeleteClosedProblemReports(ctx, t0.Add(2*time.Hour))
		if err != nil || n != 1 {
			t.Fatalf("purge %d %v", n, err)
		}
		if list, _ := s.ProblemReports(ctx, 10); len(list) != 2 || list[0].ID == ids[0] || list[1].ID == ids[0] {
			t.Fatalf("after purge %+v", list)
		}
		if err := s.SetProblemReportStatus(ctx, 999, "read", t0); !errors.Is(err, app.ErrNotFound) {
			t.Fatalf("missing report: %v", err)
		}
	})
}
