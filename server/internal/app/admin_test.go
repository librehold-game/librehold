package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"librehold/internal/adapters/memory"
	"librehold/internal/adapters/security"
	"librehold/internal/app"
	"librehold/internal/game"
	"librehold/internal/game/gametest"
)

var admin = &app.Account{ID: 100, Username: "admin", IsAdmin: true}

type adminFixture struct {
	svc      *app.Service
	accounts *memory.Accounts
	worlds   *memory.Worlds
	clock    *fakeClock
}

func newAdminFixture(t *testing.T) *adminFixture {
	t.Helper()
	f := &adminFixture{accounts: memory.NewAccounts(), worlds: memory.NewWorlds(), clock: &fakeClock{}}
	f.clock.Set(accountsStart)
	f.svc = app.NewService(f.accounts, f.worlds, memory.NewCounters(f.clock), f.clock, plainHasher{}, security.NewTokens(), app.Config{})
	return f
}

func (f *adminFixture) world(t *testing.T) int64 {
	t.Helper()
	id, err := f.svc.CreateWorld(context.Background(), admin, "Café World 1", gametest.TestConfig())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestNonAdminForbidden(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.world(t)
	user := &app.Account{ID: 1, Username: "ala"}
	calls := map[string]func() error{
		"Accounts":      func() error { _, err := f.svc.Accounts(ctx, user); return err },
		"ResetPassword": func() error { return f.svc.ResetPassword(ctx, user, 1, "new-password") },
		"CreateWorld":   func() error { _, err := f.svc.CreateWorld(ctx, user, "x", gametest.TestConfig()); return err },
		"Worlds":        func() error { _, err := f.svc.Worlds(ctx, user); return err },
		"UpdateWorldSettings": func() error {
			return f.svc.UpdateWorldSettings(ctx, user, id, app.WorldSettingsPatch{"worldSpeed": json.RawMessage("2")})
		},
		"SetWorldStatus": func() error { return f.svc.SetWorldStatus(ctx, user, id, "closed") },
		"PauseWorld":     func() error { return f.svc.PauseWorld(ctx, user, id) },
		"ResumeWorld":    func() error { return f.svc.ResumeWorld(ctx, user, id) },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, app.ErrForbidden) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResetPasswordEndsSessions(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	token, acct, err := f.svc.Register(ctx, app.RegisterInput{Username: "ala", Password: "long-password",
		PasswordConfirm: "long-password", Client: "k"})
	if err != nil {
		t.Fatal(err)
	}
	wantValidation(t, f.svc.ResetPassword(ctx, admin, acct.ID, "short"), "password", "passwordShort")
	if err := f.svc.ResetPassword(ctx, admin, acct.ID, "new-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Authenticate(ctx, token); !errors.Is(err, app.ErrUnauthenticated) {
		t.Fatalf("old session: %v", err)
	}
	if _, _, err := f.svc.Login(ctx, "ala", "new-password", "k"); err != nil {
		t.Fatalf("new password: %v", err)
	}
	list, _ := f.svc.Accounts(ctx, admin)
	if len(list) != 1 || list[0].Username != "ala" {
		t.Fatalf("accounts %+v", list)
	}
}

func TestCreateWorldClampsAndOpens(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	cfg := gametest.TestConfig()
	cfg.WorldSpeed, cfg.MapSize, cfg.BotCount = 500, 250, 2
	cfg.HumanModifiers.Resources = 99
	id, err := f.svc.CreateWorld(ctx, admin, "Grand Café", cfg)
	if err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.Worlds(ctx, admin)
	if err != nil || len(list) != 1 {
		t.Fatalf("worlds %+v %v", list, err)
	}
	w := list[0]
	if w.ID != id || w.Status != "open" || w.Bots != 2 || w.StartedAtMs != accountsStart {
		t.Fatalf("world %+v", w)
	}
	if w.Config.WorldSpeed != 100 || w.Config.MapSize != 200 || w.Config.HumanModifiers.Resources != 10 {
		t.Fatalf("not clamped: %+v", w.Config)
	}
	if _, err := f.svc.CreateWorld(ctx, admin, "  ", cfg); err == nil {
		t.Fatal("a world without a name")
	}
	if err := f.svc.SetWorldStatus(ctx, admin, id, "closed"); err != nil {
		t.Fatal(err)
	}
	if row, _ := f.worlds.Get(ctx, id); row.Status != "closed" {
		t.Fatalf("status %s", row.Status)
	}
	wantValidation(t, f.svc.SetWorldStatus(ctx, admin, id, "deleted"), "status", "statusInvalid")
}

// The seed drives combat luck, so it must not follow from anything a player sees (the world's start time).
func TestWorldSeedIsSecret(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	a, b := f.namedWorld(t, "A"), f.namedWorld(t, "B")
	ra, _ := f.worlds.Get(ctx, a)
	rb, _ := f.worlds.Get(ctx, b)
	if ra.StartedAtMs != rb.StartedAtMs {
		t.Fatal("setup: both worlds should start at the same clock reading")
	}
	now := ra.StartedAtMs
	if ra.Seed == rb.Seed || ra.Seed == uint32(now)^uint32(now>>32) {
		t.Fatalf("seeds %d and %d follow from the start time %d", ra.Seed, rb.Seed, now)
	}
}

func TestFixedSettingsRejected(t *testing.T) {
	f := newAdminFixture(t)
	id := f.world(t)
	for _, field := range []string{"mapSize", "botCount", "barbarianDensity", "unknown"} {
		err := f.svc.UpdateWorldSettings(context.Background(), admin, id, app.WorldSettingsPatch{
			"worldSpeed": json.RawMessage("2"), field: json.RawMessage("200")})
		wantValidation(t, err, field, "fixedAtCreation")
	}
	if row, _ := f.worlds.Get(context.Background(), id); row.Config.WorldSpeed != 1 {
		t.Fatal("a refused patch was partly applied")
	}
}

func TestSpeedChangeSettlesAndKeepsTimers(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.world(t)
	start := f.clock.NowMs()
	var home, target string
	err := f.worlds.Run(ctx, id, func(w app.WorldTx) error {
		a, err := game.JoinHuman(w, "h1", "Ala", 1, game.QuadrantRandom)
		if err != nil {
			return err
		}
		b, err := game.JoinHuman(w, "h2", "Bartek", 2, game.QuadrantRandom)
		if err != nil {
			return err
		}
		home, target = a.ID, b.ID
		w.Player("h1").ProtectedUntil, w.Player("h2").ProtectedUntil = 0, 0
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ala := &app.Account{ID: 1}
	var buildEnd, arrives int64
	var cmdID string
	err = f.svc.InWorld(ctx, ala, id, func(w app.WorldTx, me game.PlayerID) error {
		job, err := game.EnqueueBuild(w, me, home, game.BWood)
		if err != nil {
			return err
		}
		buildEnd = job.CompletesAt
		to := w.Village(target)
		c, err := game.SendCommand(w, me, home, to.X, to.Y, game.UnitCounts{game.USpear: 5}, "attack", game.SendOpts{})
		if err != nil {
			return err
		}
		arrives, cmdID = c.ArrivesAt, c.ID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t0 := int64(1000)
	if t0 >= buildEnd || buildEnd >= arrives {
		t.Fatalf("timeline: change %d, build %d, attack %d", t0, buildEnd, arrives)
	}
	var atT0 game.Village
	err = f.worlds.Read(ctx, id, func(w app.WorldTx) error {
		atT0 = *gametest.CloneVillage(w.Village(home))
		w.SetNow(t0)
		game.Accrue(&atT0, t0, w.Config())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	log := &keyLog{}
	restore := game.SpyEvents(log.add)
	defer restore()
	f.clock.Set(start + t0)
	if err := f.svc.UpdateWorldSettings(ctx, admin, id, app.WorldSettingsPatch{"worldSpeed": json.RawMessage("8")}); err != nil {
		t.Fatal(err)
	}
	var stored *game.Village
	var newCfg game.WorldConfig
	err = f.worlds.Read(ctx, id, func(w app.WorldTx) error {
		stored, newCfg = w.Village(home), *w.Config()
		if at, ok := w.ScheduledAt("build:" + home); !ok || at != buildEnd {
			t.Errorf("build event at %d %v, want %d", at, ok, buildEnd)
		}
		if at, ok := w.ScheduledAt("cmd:" + cmdID); !ok || at != arrives {
			t.Errorf("attack event at %d %v, want %d", at, ok, arrives)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if newCfg.WorldSpeed != 8 {
		t.Fatalf("speed %v", newCfg.WorldSpeed)
	}
	if stored.ResourcesUpdatedAt != t0 || stored.Resources != atT0.Resources {
		t.Fatalf("settled at %d to %+v, want %d %+v", stored.ResourcesUpdatedAt, stored.Resources, t0, atT0.Resources)
	}

	mid := (t0 + buildEnd) / 2
	f.clock.Set(start + mid)
	err = f.svc.InWorld(ctx, ala, id, func(w app.WorldTx, me game.PlayerID) error {
		v := w.Village(home)
		game.Touch(w, v)
		want := math.Min(game.VillageStorageCapacity(stored),
			stored.Resources.Iron+game.VillageProductionPerHour(stored, game.Iron, &newCfg)*float64(mid-t0)/float64(game.MsPerHour))
		if math.Abs(v.Resources.Iron-want) > 1e-6 {
			t.Errorf("iron %v, want %v at the new pace", v.Resources.Iron, want)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	f.clock.Set(start + arrives + 1)
	if _, _, err := f.svc.ProcessDue(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	builds, attacks := 0, 0
	for _, k := range log.take() {
		switch k {
		case "build:" + home:
			builds++
		case "cmd:" + cmdID:
			attacks++
		}
	}
	if builds != 1 || attacks != 1 {
		t.Fatalf("build events %d, attack events %d", builds, attacks)
	}
}

func TestPauseResumeShiftsWorldTime(t *testing.T) {
	f := newAdminFixture(t)
	ctx := context.Background()
	id := f.world(t)
	f.clock.Add(60_000)
	if err := f.svc.PauseWorld(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	f.clock.Add((10 * time.Minute).Milliseconds())
	row, _ := f.worlds.Get(ctx, id)
	if row.PausedAtMs == nil || app.WorldNow(row, f.clock.NowMs()) != 60_000 {
		t.Fatalf("paused row %+v", row)
	}
	if err := f.svc.ResumeWorld(ctx, admin, id); err != nil {
		t.Fatal(err)
	}
	row, _ = f.worlds.Get(ctx, id)
	if row.PausedAtMs != nil || row.PausedTotalMs != (10*time.Minute).Milliseconds() {
		t.Fatalf("resumed row %+v", row)
	}
	f.clock.Add(5000)
	if got := app.WorldNow(row, f.clock.NowMs()); got != 65_000 {
		t.Fatalf("world now %d", got)
	}
	if err := f.svc.ResumeWorld(ctx, admin, id); err != nil {
		t.Fatalf("resuming a running world: %v", err)
	}
}

func TestWorldConfigClampsSightRadiusToLimits(t *testing.T) {
	cfg := game.DefaultWorldConfig
	cfg.SightRadius = game.WorldLimits.SightRadius.Max + 1
	if got := app.ClampWorldConfig(cfg).SightRadius; got != game.WorldLimits.SightRadius.Max {
		t.Fatalf("11 → %v", got)
	}
	cfg.SightRadius = 0
	if got := app.ClampWorldConfig(cfg).SightRadius; got != 0 {
		t.Fatalf("0 → %v", got)
	}
}
