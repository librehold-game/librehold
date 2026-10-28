package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
)

// gatedHasher holds every hash until open is closed, so requests pile up inside it as they would behind a busy CPU.
type gatedHasher struct {
	open    chan struct{}
	waiting atomic.Int64
}

func (h *gatedHasher) wait(ctx context.Context) error {
	h.waiting.Add(1)
	select {
	case <-h.open:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *gatedHasher) Hash(ctx context.Context, pw string) (string, error) {
	if err := h.wait(ctx); err != nil {
		return "", err
	}
	return "plain:" + pw, nil
}

func (h *gatedHasher) Verify(ctx context.Context, hash, pw string) (bool, error) {
	if err := h.wait(ctx); err != nil {
		return false, err
	}
	return hash == "plain:"+pw, nil
}

// atOnce runs n calls together and returns the errors once every call that got through to the hasher is released.
func atOnce(h *gatedHasher, n int, call func(i int) error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	var done atomic.Int64
	for i := range n {
		wg.Go(func() {
			errs[i] = call(i)
			done.Add(1)
		})
	}
	for h.waiting.Load()+done.Load() < int64(n) {
		runtime.Gosched()
	}
	close(h.open)
	wg.Wait()
	return errs
}

func newGatedService(t *testing.T) (*app.Service, *memory.Accounts, *gatedHasher) {
	t.Helper()
	clock := &fakeClock{}
	clock.Set(accountsStart)
	accounts := memory.NewAccounts()
	h := &gatedHasher{open: make(chan struct{})}
	return app.NewService(accounts, memory.NewWorlds(), memory.NewCounters(clock), clock, h, security.NewTokens(), app.Config{}), accounts, h
}

func TestLoginLimitHoldsForAttemptsAtOnce(t *testing.T) {
	ctx := context.Background()
	svc, accounts, h := newGatedService(t)
	if err := accounts.Register(ctx, &app.Account{Username: "Ala_1", PasswordHash: "plain:long-password"}, time.UnixMilli(accountsStart)); err != nil {
		t.Fatal(err)
	}
	errs := atOnce(h, 30, func(int) error {
		_, _, err := svc.Login(ctx, "Ala_1", "wrong-password", "1.2.3.4")
		return err
	})
	checked := 0
	for _, err := range errs {
		if errors.Is(err, app.ErrBadCredentials) {
			checked++
		} else if !errors.Is(err, app.ErrTooManyAttempts) {
			t.Fatal(err)
		}
	}
	if checked != 5 {
		t.Fatalf("%d passwords checked out of 30 sent at once, want 5", checked)
	}
}

func TestRegisterLimitHoldsForRegistrationsAtOnce(t *testing.T) {
	ctx := context.Background()
	svc, _, h := newGatedService(t)
	errs := atOnce(h, 10, func(i int) error {
		in := validInput()
		in.Username = fmt.Sprintf("player%d", i)
		_, _, err := svc.Register(ctx, in)
		return err
	})
	opened := 0
	for _, err := range errs {
		if err == nil {
			opened++
		} else if !errors.Is(err, app.ErrTooManyRegistrations) {
			t.Fatal(err)
		}
	}
	if opened != 3 {
		t.Fatalf("%d accounts opened out of 10 sent at once, want 3", opened)
	}
}

func TestGuestLimitHoldsForGuestsAtOnce(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	var made atomic.Int64
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			if _, _, err := f.svc.CreateGuest(ctx, "1.2.3.4", "en"); err == nil {
				made.Add(1)
			} else if !errors.Is(err, app.ErrTooManyGuests) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if made.Load() != 10 {
		t.Fatalf("%d guests made out of 30 at once, want 10", made.Load())
	}
}

func TestSuccessfulLoginsDoNotCountAgainstTheAddress(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	if _, _, err := f.svc.Register(ctx, validInput()); err != nil {
		t.Fatal(err)
	}
	for i := range 30 {
		if _, _, err := f.svc.Login(ctx, "Ala_1", "long-password", "1.2.3.4"); err != nil {
			t.Fatalf("login %d: %v", i+1, err)
		}
	}
}

func TestTakenUsernameDoesNotCountAgainstRegistrations(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	if _, _, err := f.svc.Register(ctx, validInput()); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		_, _, err := f.svc.Register(ctx, validInput())
		wantValidation(t, err, "username", "usernameTaken")
	}
	for _, name := range []string{"Bartek", "Celina"} {
		in := validInput()
		in.Username = name
		if _, _, err := f.svc.Register(ctx, in); err != nil {
			t.Fatalf("%s after taken names: %v", name, err)
		}
	}
}

func TestLocalNetworkHasNoAddressLimits(t *testing.T) {
	ctx := context.Background()
	f := newAccountsFixture(t, app.Config{})
	for i := range 12 {
		in := validInput()
		in.Username, in.Client = fmt.Sprintf("player%d", i), app.LocalClient
		if _, _, err := f.svc.Register(ctx, in); err != nil {
			t.Fatalf("registration %d on the local network: %v", i+1, err)
		}
		if _, _, err := f.svc.CreateGuest(ctx, app.LocalClient, "en"); err != nil {
			t.Fatalf("guest %d on the local network: %v", i+1, err)
		}
	}
	for i := range 25 {
		_, _, err := f.svc.Login(ctx, fmt.Sprintf("guess%d", i), "long-password", app.LocalClient)
		if !errors.Is(err, app.ErrBadCredentials) {
			t.Fatalf("failed login %d on the local network: %v", i+1, err)
		}
	}
	for range 5 {
		_, _, _ = f.svc.Login(ctx, "player0", "wrong-password", app.LocalClient)
	}
	if _, _, err := f.svc.Login(ctx, "player0", "long-password", app.LocalClient); !errors.Is(err, app.ErrTooManyAttempts) {
		t.Fatalf("the per-username limit still holds on the local network: %v", err)
	}
}

func TestWorldRequestsArePacedPerAccount(t *testing.T) {
	ctx := context.Background()
	f := newAdminFixture(t)
	f.svc = app.NewService(f.accounts, f.worlds, memory.NewCounters(f.clock), f.clock, plainHasher{}, security.NewTokens(),
		app.Config{WorldRequestsPerMinute: 3})
	id := f.namedWorld(t, "A")
	if _, err := f.svc.JoinWorld(ctx, ala, id, game.QuadrantRandom); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.State(ctx, ala, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Order(ctx, ala, id, "mark-all-reports-read", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.State(ctx, ala, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Order(ctx, ala, id, "mark-all-reports-read", json.RawMessage(`{}`)); !errors.Is(err, app.ErrTooFast) {
		t.Fatalf("4th request within a minute: %v", err)
	}
	if _, err := f.svc.State(ctx, ala, id); !errors.Is(err, app.ErrTooFast) {
		t.Fatalf("5th request within a minute: %v", err)
	}
	f.clock.Add(time.Minute.Milliseconds())
	if _, err := f.svc.State(ctx, ala, id); err != nil {
		t.Fatalf("after a minute: %v", err)
	}
}
