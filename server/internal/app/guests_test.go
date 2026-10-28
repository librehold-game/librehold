package app_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"librehold/internal/app"
	"librehold/internal/game"
)

func TestGuestGetsUniqueNicknameAndSession(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	seen := map[string]bool{}
	for i := range 5 {
		token, acct, err := f.svc.CreateGuest(ctx, "1.2.3.4", "pl")
		if err != nil {
			t.Fatalf("guest %d: %v", i, err)
		}
		if !acct.IsGuest || acct.PasswordHash != "" || acct.Locale != "pl" {
			t.Fatalf("guest %+v", acct)
		}
		if !regexp.MustCompile(`^[A-Za-z]+[0-9]{2,3}$`).MatchString(acct.Username) || seen[acct.Username] {
			t.Fatalf("nickname %q (seen %v)", acct.Username, seen)
		}
		seen[acct.Username] = true
		if got, err := f.svc.Authenticate(ctx, token); err != nil || got.ID != acct.ID {
			t.Fatalf("session %+v %v", got, err)
		}
	}
}

func TestGuestRateLimitedPerAddress(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	for i := range 10 {
		if _, _, err := f.svc.CreateGuest(ctx, "9.9.9.9", "en"); err != nil {
			t.Fatalf("guest %d: %v", i, err)
		}
	}
	if _, _, err := f.svc.CreateGuest(ctx, "9.9.9.9", "en"); !errors.Is(err, app.ErrTooManyGuests) {
		t.Fatalf("11th guest: %v", err)
	}
	if _, _, err := f.svc.CreateGuest(ctx, "8.8.8.8", "en"); err != nil {
		t.Fatalf("another address: %v", err)
	}
	f.clock.Set(accountsStart + time.Hour.Milliseconds() + 1)
	if _, _, err := f.svc.CreateGuest(ctx, "9.9.9.9", "en"); err != nil {
		t.Fatalf("an hour later: %v", err)
	}
}

func TestGuestCannotLogIn(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	_, acct, err := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, pw := range []string{"", "anything"} {
		if _, _, err := f.svc.Login(ctx, acct.Username, pw, "1.2.3.4"); !errors.Is(err, app.ErrBadCredentials) {
			t.Fatalf("login with %q: %v", pw, err)
		}
	}
}

func TestGuestCannotJoinLongWorld(t *testing.T) {
	ctx := context.Background()
	f := newAdminFixture(t)
	wid := f.world(t)
	_, guest, err := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.JoinWorld(ctx, guest, wid, game.QuadrantRandom); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("guest joined a long world: %v", err)
	}
}

func TestClaimKeepsAccountIDAndSession(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	token, guest, err := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	if err != nil {
		t.Fatal(err)
	}
	in := app.RegisterInput{Username: "Ruler", Password: "long-password", PasswordConfirm: "long-password", Client: "1.2.3.4"}
	acct, err := f.svc.ClaimGuest(ctx, guest, in)
	if err != nil || acct.ID != guest.ID || acct.IsGuest || acct.Username != "Ruler" {
		t.Fatalf("claim %+v %v", acct, err)
	}
	if got, err := f.svc.Authenticate(ctx, token); err != nil || got.ID != guest.ID || got.IsGuest {
		t.Fatalf("session after claim %+v %v", got, err)
	}
	if _, _, err := f.svc.Login(ctx, "Ruler", "long-password", "1.2.3.4"); err != nil {
		t.Fatalf("login after claim: %v", err)
	}
	if _, err := f.svc.ClaimGuest(ctx, acct, in); !errors.Is(err, app.ErrForbidden) {
		t.Fatalf("claiming twice: %v", err)
	}
	_, other, _ := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	taken := in
	taken.Username = "RULER"
	_, err = f.svc.ClaimGuest(ctx, other, taken)
	wantValidation(t, err, "username", "usernameTaken")
}

func TestRenameGuest(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	_, guest, _ := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	acct, err := f.svc.RenameGuest(ctx, guest, "Nowe_Imie")
	if err != nil || acct.Username != "Nowe_Imie" {
		t.Fatalf("rename %+v %v", acct, err)
	}
	_, err = f.svc.RenameGuest(ctx, guest, "x")
	wantValidation(t, err, "username", "usernameFormat")
}

func TestPurgeGuestsSkipsActiveAndPlayers(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	_, old, _ := f.svc.CreateGuest(ctx, "1.2.3.4", "en")
	_, player, _ := f.svc.Register(ctx, validInput())
	f.clock.Set(accountsStart + 20*24*time.Hour.Milliseconds())
	_, fresh, _ := f.svc.CreateGuest(ctx, "5.5.5.5", "en")
	f.clock.Set(accountsStart + 31*24*time.Hour.Milliseconds())
	n, err := f.svc.PurgeGuests(ctx)
	if err != nil || n != 1 {
		t.Fatalf("purged %d %v", n, err)
	}
	if _, err := f.accounts.ByID(ctx, old.ID); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("old guest still there: %v", err)
	}
	for _, a := range []*app.Account{player, fresh} {
		if _, err := f.accounts.ByID(ctx, a.ID); err != nil {
			t.Fatalf("%s was purged: %v", a.Username, err)
		}
	}
}
