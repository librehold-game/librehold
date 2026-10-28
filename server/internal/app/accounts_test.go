package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
)

// plainHasher keeps the tests fast; argon2 has its own test.
type plainHasher struct{}

func (plainHasher) Hash(_ context.Context, pw string) (string, error) { return "plain:" + pw, nil }
func (plainHasher) Verify(_ context.Context, hash, pw string) (bool, error) {
	return hash == "plain:"+pw, nil
}

type accountsFixture struct {
	svc      *app.Service
	accounts *memory.Accounts
	clock    *fakeClock
	tokens   app.TokenSource
}

var accountsStart = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC).UnixMilli()

func newAccountsFixture(t *testing.T, cfg app.Config) *accountsFixture {
	t.Helper()
	f := &accountsFixture{accounts: memory.NewAccounts(), clock: &fakeClock{}, tokens: security.NewTokens()}
	f.clock.Set(accountsStart)
	f.svc = app.NewService(f.accounts, memory.NewWorlds(), memory.NewCounters(f.clock), f.clock, plainHasher{}, f.tokens, cfg)
	return f
}

func validInput() app.RegisterInput {
	return app.RegisterInput{Username: "Ala_1", Password: "long-password", PasswordConfirm: "long-password", Client: "1.2.3.4"}
}

func wantValidation(t *testing.T, err error, field, code string) {
	t.Helper()
	var ve *app.ErrValidation
	if !errors.As(err, &ve) || ve.Field != field || ve.Code != code {
		t.Fatalf("err %v, want %s/%s", err, field, code)
	}
}

func TestRegisterValidation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, field, code string
		edit              func(in *app.RegisterInput)
	}{
		{"usernameShort", "username", "usernameFormat", func(in *app.RegisterInput) { in.Username = "al" }},
		{"usernameChars", "username", "usernameFormat", func(in *app.RegisterInput) { in.Username = "ala cat" }},
		{"usernameLong", "username", "usernameFormat", func(in *app.RegisterInput) { in.Username = "a234567890123456789012345" }},
		{"passwordShort", "password", "passwordShort", func(in *app.RegisterInput) { in.Password, in.PasswordConfirm = "short", "short" }},
		{"passwordMismatch", "passwordConfirm", "passwordMismatch", func(in *app.RegisterInput) { in.PasswordConfirm = "other-password" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountsFixture(t, app.Config{})
			in := validInput()
			tc.edit(&in)
			_, _, err := f.svc.Register(ctx, in)
			wantValidation(t, err, tc.field, tc.code)
		})
	}
	t.Run("usernameTaken", func(t *testing.T) {
		f := newAccountsFixture(t, app.Config{})
		if _, _, err := f.svc.Register(ctx, validInput()); err != nil {
			t.Fatal(err)
		}
		in := validInput()
		in.Username = "ALA_1"
		_, _, err := f.svc.Register(ctx, in)
		wantValidation(t, err, "username", "usernameTaken")
	})
}

func TestRegisterIsOpenAndOpensASession(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	token, acct, err := f.svc.Register(ctx, validInput())
	if err != nil || token == "" || acct.Username != "Ala_1" {
		t.Fatalf("%q %+v %v", token, acct, err)
	}
	if got, err := f.svc.Authenticate(ctx, token); err != nil || got.ID != acct.ID {
		t.Fatalf("the new session does not authenticate: %v", err)
	}
}

func TestRegisterRateLimit(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	register := func(name, client string) error {
		in := validInput()
		in.Username, in.Client = name, client
		_, _, err := f.svc.Register(ctx, in)
		return err
	}
	bad := validInput()
	bad.Password = "short"
	if _, _, err := f.svc.Register(ctx, bad); err == nil {
		t.Fatal("a short password registered")
	}
	for i, name := range []string{"Ala", "Bartek", "Celina"} {
		if err := register(name, "1.2.3.4"); err != nil {
			t.Fatalf("registration %d: %v", i+1, err)
		}
	}
	if err := register("Darek", "1.2.3.4"); !errors.Is(err, app.ErrTooManyRegistrations) {
		t.Fatalf("4th registration from one address within an hour: %v", err)
	}
	if err := register("Darek", "5.6.7.8"); err != nil {
		t.Fatalf("another address: %v", err)
	}
	f.clock.Add(time.Hour.Milliseconds())
	if err := register("Ewa", "1.2.3.4"); err != nil {
		t.Fatalf("after an hour: %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	if _, _, err := f.svc.Register(ctx, validInput()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Login(ctx, "nobody", "long-password", "1.2.3.4"); !errors.Is(err, app.ErrBadCredentials) {
		t.Fatalf("unknown user: %v", err)
	}
	for i := range 5 {
		if _, _, err := f.svc.Login(ctx, "ala_1", "wrong-password", "1.2.3.4"); !errors.Is(err, app.ErrBadCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "1.2.3.4"); !errors.Is(err, app.ErrTooManyAttempts) {
		t.Fatalf("6th attempt: %v", err)
	}
	if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "5.6.7.8"); err != nil {
		t.Fatalf("another client: %v", err)
	}
	f.clock.Add((15 * time.Minute).Milliseconds())
	token, acct, err := f.svc.Login(ctx, "Ala_1", "long-password", "1.2.3.4")
	if err != nil || token == "" || acct.Username != "Ala_1" {
		t.Fatalf("after the window: %v", err)
	}
}

func TestSessionSlidingExpiry(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{SessionTTL: 24 * time.Hour})
	token, _, err := f.svc.Register(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	session := func() *app.Session {
		s, err := f.accounts.Session(ctx, f.tokens.Hash(token))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	created := session()
	f.clock.Add((30 * time.Minute).Milliseconds())
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	if s := session(); !s.LastSeenAt.Equal(created.LastSeenAt) {
		t.Fatal("renewed within the hour")
	}
	f.clock.Add((90 * time.Minute).Milliseconds())
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	renewed := session()
	if want := time.UnixMilli(f.clock.NowMs()).Add(24 * time.Hour); !renewed.ExpiresAt.Equal(want) {
		t.Fatalf("expires %v, want %v", renewed.ExpiresAt, want)
	}
	f.clock.Add((24*time.Hour + time.Minute).Milliseconds())
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("expired: %v", err)
	}
	if _, err := f.svc.Authenticate(ctx, "made-up"); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("unknown token: %v", err)
	}
}

func TestLogoutKillsSession(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	token, _, err := f.svc.Register(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("after logout: %v", err)
	}
}

func TestBootstrapAdminOnce(t *testing.T) {
	ctx := context.Background()
	if err := newAccountsFixture(t, app.Config{}).svc.BootstrapAdmin(ctx); err == nil {
		t.Fatal("no admin configured and none exists, but no error")
	}
	f := newAccountsFixture(t, app.Config{AdminUsername: "admin", AdminPassword: "admin-password"})
	for range 2 {
		if err := f.svc.BootstrapAdmin(ctx); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := f.accounts.List(ctx)
	if len(list) != 1 || !list[0].IsAdmin || list[0].Username != "admin" {
		t.Fatalf("accounts %+v", list)
	}
	if _, acct, err := f.svc.Login(ctx, "admin", "admin-password", "k"); err != nil || !acct.IsAdmin {
		t.Fatalf("admin login: %v", err)
	}
	f2 := newAccountsFixture(t, app.Config{})
	_ = f2.accounts.CreateAdmin(ctx, &app.Account{Username: "stary", PasswordHash: "x"})
	if err := f2.svc.BootstrapAdmin(ctx); err != nil {
		t.Fatalf("an existing admin needs no config: %v", err)
	}
}

func TestRegisterKeepsLocale(t *testing.T) {
	f := newAccountsFixture(t, app.Config{})
	in := validInput()
	in.Locale = "pl"
	_, acct, err := f.svc.Register(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.accounts.ByID(context.Background(), acct.ID); got.Locale != "pl" {
		t.Fatalf("locale %q", got.Locale)
	}
	in = validInput()
	in.Username, in.Client, in.Locale = "Bez_jezyka", "5.6.7.8", "xx"
	_, acct, err = f.svc.Register(context.Background(), in)
	if err != nil || acct.Locale != "en" {
		t.Fatalf("unknown locale falls back to en: %+v %v", acct, err)
	}
}

func TestUpdateMeRejectsUnknownLocale(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	_, acct, err := f.svc.Register(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	bad := "de"
	_, err = f.svc.UpdateMe(ctx, acct, app.MePatch{Locale: &bad})
	wantValidation(t, err, "locale", "locale")
	pl, done := "pl", true
	got, err := f.svc.UpdateMe(ctx, acct, app.MePatch{Locale: &pl, TutorialDone: &done})
	if err != nil || got.Locale != "pl" || !got.TutorialDone {
		t.Fatalf("update %+v %v", got, err)
	}
}

func TestAuthenticateTouchesAccountHourly(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	token, acct, err := f.svc.Register(ctx, validInput())
	if err != nil {
		t.Fatal(err)
	}
	later := accountsStart + 2*time.Hour.Milliseconds()
	f.clock.Set(later)
	if _, err := f.svc.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	got, _ := f.accounts.ByID(ctx, acct.ID)
	if want := time.UnixMilli(later).UTC(); !got.LastSeenAt.Equal(want) {
		t.Fatalf("last seen %v want %v", got.LastSeenAt, want)
	}
}

func TestLoginLimitedPerClientAcrossUsernames(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	if _, _, err := f.svc.Register(ctx, validInput()); err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		if _, _, err := f.svc.Login(ctx, fmt.Sprintf("guess%d", i), "long-password", "1.2.3.4"); !errors.Is(err, app.ErrBadCredentials) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "1.2.3.4"); !errors.Is(err, app.ErrTooManyAttempts) {
		t.Fatalf("21st failed-or-not attempt from the address: %v", err)
	}
	if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "5.6.7.8"); err != nil {
		t.Fatalf("another address: %v", err)
	}
	f.clock.Add((15 * time.Minute).Milliseconds())
	if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "1.2.3.4"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}
